package main

import (
	"context"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"
)

func execute(ctx context.Context, std stdio, jitconfig string) error {
	runner, err := actionsRunner()
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, runner, "--jitconfig", jitconfig)
	cmd.Dir = filepath.Dir(runner)
	cmd.Stdin = std.in
	cmd.Stdout = std.out
	cmd.Stderr = std.err
	cmd.Cancel = func() error {
		return cmd.Process.Signal(os.Interrupt)
	}
	cmd.WaitDelay = 5 * time.Second

	// Workaround https://github.com/dotnet/runtime/issues/27626.
	esc := newEscapes(std.out)
	defer std.out.Write([]byte(esc.keypadEnd))

	return cmd.Run()
}

func actionsRunner() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		u, err := user.Current()
		if err != nil {
			return "", err
		}
		home = u.HomeDir
	}

	return filepath.Join(home, "actions-runner", "run.sh"), nil
}
