package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

type config struct {
	repoConfig

	runnerConfig
	noDefaultLabels bool

	once bool
}

type repoConfig struct {
	hostname string
	repoID   int64
	owner    string
	repo     string
}

type runnerConfig struct {
	name    string
	prefix  string
	groupID int64
	cwd     string
	labels  []string
}

type stdio struct {
	in  io.Reader
	out io.Writer
	err io.Writer
}

func main() {
	cmd := (&config{}).command()
	if err := cmd.Execute(); err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(0x80 + int(syscall.SIGINT))
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			cmd.ErrOrStderr().Write(exitErr.Stderr)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		var errHint *errorHint
		if errors.As(err, &errHint) {
			fmt.Fprintf(cmd.ErrOrStderr(), "Recommended resolution: %s\n", errHint.hint)
		}

		os.Exit(1)
	}
}

func hintf(err error, format string, a ...any) error {
	return &errorHint{error: err, hint: fmt.Sprintf(format, a...)}
}

type errorHint struct {
	error
	hint string
}

func (e *errorHint) Unwrap() error {
	return e.error
}

func (c *config) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "github-runner [flags] [<OWNER>/<REPO>]",
		Short:         "Create a just-in-time runner.",
		Long:          "Create a just-in-time GitHub runner and listen for incoming jobs.",
		Args:          cobra.MaximumNArgs(1),
		RunE:          c.run,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.Flags().BoolP("help", "h", false, "Print the help message for the command.")

	cmd.Flags().StringVar(&c.hostname, "hostname", "", `Specify GitHub API hostname. (default "github.com")`)
	cmd.Flags().Int64Var(&c.repoID, "repository-id", 0, "Restrict access to specific repository.")

	cmd.Flags().StringVarP(&c.name, "name", "n", "", "Specify the runner name (must be unique).")
	cmd.Flags().StringVarP(&c.prefix, "prefix", "p", "", "Specify a prefix for the runner name. (default based on hostname)")
	cmd.Flags().Int64VarP(&c.groupID, "group-id", "g", 1, "Add runner to a custom group.")
	cmd.Flags().StringVarP(&c.cwd, "cwd", "C", "", `Set the working directory for the job. (default "_work")`)
	cmd.Flags().StringArrayVarP(&c.labels, "label", "l", nil, "Add labels to the runner.")
	cmd.Flags().BoolVar(&c.noDefaultLabels, "no-default-labels", false, "Do not add the default runner labels.")

	cmd.Flags().BoolVarP(&c.once, "once", "1", false, "Exit after running a single job.")

	cmd.MarkFlagsMutuallyExclusive("name", "prefix")

	return cmd
}

func (c *config) run(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)
	defer stop()
	std := stdio{in: cmd.InOrStdin(), out: cmd.OutOrStdout(), err: cmd.ErrOrStderr()}

	if err := c.setOwnerRepo(ctx, args); err != nil {
		return err
	}
	// Ignore errors since private repositories are invisible at this point.
	_ = c.ensureRepoID(ctx)
	if err := c.ensureNameOrPrefix(); err != nil {
		return hintf(err, "pass --prefix=<RUNNER NAME> as an argument")
	}
	if !c.noDefaultLabels {
		c.appendDefaultLabels()
	}

	for {
		client, err := c.oauthClient(ctx, std)
		if err != nil {
			return err
		}

		jitconfig, err := c.configureRunner(ctx, std, client)
		if err != nil {
			return err
		}
		defer func() {
			if err := c.deleteRunner(context.WithoutCancel(ctx), client, jitconfig.GetRunner()); err != nil {
				fmt.Fprintln(std.err, err)
			}
		}()

		if err := execute(ctx, std, jitconfig.GetEncodedJITConfig()); err != nil {
			return err
		}

		if c.once {
			return nil
		}
	}
}

func (c *repoConfig) setOwnerRepo(ctx context.Context, args []string) error {
	if len(args) > 0 {
		segments := strings.Split(args[0], "/")
		if len(segments) < 1 {
			return fmt.Errorf("too few segments: %s", args[0])
		}
		if len(segments) > 2 {
			return fmt.Errorf("too many segments: %s", args[0])
		}

		c.owner = segments[0]
		if len(segments) > 1 {
			c.repo = segments[1]
		}
		return nil
	}

	if c.repoID != 0 {
		return nil
	}

	hostname, owner, repo, err := currentRepo(ctx)
	if err != nil {
		return hintf(err, "pass <OWNER>/<REPO> as an argument")
	}

	if c.hostname == "" && hostname != "github.com" {
		c.hostname = hostname
	}
	c.owner = owner
	c.repo = repo
	return nil
}

func (c *runnerConfig) ensureNameOrPrefix() error {
	if c.name != "" || c.prefix != "" {
		return nil
	}

	var err error
	c.prefix, err = os.Hostname()
	if err != nil {
		c.prefix = ""
		return fmt.Errorf("cannot infer runner name: %w", err)
	}
	return nil
}

var osLabels = map[string]string{
	"darwin":  "macOS",
	"linux":   "Linux",
	"windows": "Windows",
}
var archLabels = map[string]string{
	"386":   "X86",
	"arm":   "ARM",
	"arm64": "ARM64",
	"amd64": "X64",
}

func (c *runnerConfig) appendDefaultLabels() {
	c.labels = append(c.labels, "self-hosted")
	if system, ok := osLabels[runtime.GOOS]; ok {
		c.labels = append(c.labels, system)
	}
	if arch, ok := archLabels[runtime.GOARCH]; ok {
		c.labels = append(c.labels, arch)
	}
}
