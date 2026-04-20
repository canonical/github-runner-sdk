package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v73/github"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
)

func (c *repoConfig) ensureRepoID(ctx context.Context) error {
	if c.repoID != 0 {
		return nil
	}
	if c.repo == "" {
		return errors.New("cannot infer repository ID for organization")
	}

	client, err := c.client(&http.Client{Timeout: 10 * time.Second})
	if err != nil {
		return err
	}

	repo, _, err := client.Repositories.Get(ctx, c.owner, c.repo)
	if err != nil {
		return err
	}

	c.repoID = repo.GetID()
	return nil
}

func (c *repoConfig) oauthEndpoint() oauth2.Endpoint {
	endpoint := endpoints.GitHub

	if c.hostname != "" {
		for _, url := range []*string{&endpoint.AuthURL, &endpoint.DeviceAuthURL, &endpoint.TokenURL} {
			*url = strings.Replace(*url, "github.com", c.hostname, 1)
		}
	}

	return endpoint
}

func (c *config) configureRunner(ctx context.Context, std stdio, httpClient *http.Client) (*github.JITRunnerConfig, error) {
	client, err := c.client(httpClient)
	if err != nil {
		return nil, err
	}

	if err := c.ensureOwnerRepo(ctx, client); err != nil {
		return nil, err
	}
	_ = c.hintRepoID(ctx, std, client)

	request := &github.GenerateJITConfigRequest{
		Name:          c.name,
		RunnerGroupID: c.groupID,
		Labels:        c.labels,
	}
	if c.cwd != "" {
		request.WorkFolder = &c.cwd
	}

	suffix := make([]byte, 4)
	for {
		if c.name == "" {
			_, _ = rand.Read(suffix)
			request.Name = c.prefix + "-" + hex.EncodeToString(suffix)
		}

		jitconfig, response, err := c.generate(ctx, client, request)
		if response != nil {
			if c.name == "" && response.StatusCode == http.StatusConflict {
				continue
			}
			if err != nil && response.StatusCode == http.StatusNotFound {
				return nil, hintf(err, "check for spelling errors, or configure the GitHub App at %q", appURL)
			}
			if err != nil && response.StatusCode == http.StatusForbidden && c.repo == "" {
				return nil, hintf(err, "ensure the GitHub App is installed on %q at %q", c.owner, appURL)
			}
		}
		if err != nil {
			return nil, err
		}
		return jitconfig, nil
	}
}

func (c *repoConfig) client(httpClient *http.Client) (*github.Client, error) {
	client := github.NewClient(httpClient)

	if c.hostname != "" {
		url := "https://" + c.hostname
		var err error
		client, err = client.WithEnterpriseURLs(url, url)
		if err != nil {
			return nil, hintf(err, "pass --hostname as an argument")
		}
	}

	return client, nil
}

func (c *repoConfig) ensureOwnerRepo(ctx context.Context, client *github.Client) error {
	if c.owner != "" || c.repoID == 0 {
		return nil
	}

	repo, _, err := client.Repositories.GetByID(ctx, c.repoID)
	if err != nil {
		err = fmt.Errorf("cannot lookup repository from ID %v: %w", c.repoID, err)
		return hintf(err, "pass <OWNER>/<REPO> as an argument")
	}

	c.owner = repo.Owner.GetLogin()
	c.repo = repo.GetName()
	return nil
}

func (c *repoConfig) hintRepoID(ctx context.Context, std stdio, client *github.Client) error {
	if c.repoID != 0 || c.repo == "" {
		return nil
	}

	repo, _, err := client.Repositories.Get(ctx, c.owner, c.repo)
	if err != nil {
		return err
	}

	sym := newSymbols(std.out)
	fmt.Fprintf(std.out, "\n%s Consider setting --repository-id=%v next time to improve security\n", sym.lock, repo.GetID())
	return nil
}

func (c *repoConfig) generate(ctx context.Context, client *github.Client, request *github.GenerateJITConfigRequest) (*github.JITRunnerConfig, *github.Response, error) {
	if c.repo == "" {
		return client.Actions.GenerateOrgJITConfig(ctx, c.owner, request)
	}
	return client.Actions.GenerateRepoJITConfig(ctx, c.owner, c.repo, request)
}

func (c *repoConfig) deleteRunner(ctx context.Context, httpClient *http.Client, runner *github.Runner) error {
	client, err := c.client(httpClient)
	if err != nil {
		return err
	}

	runnerID := runner.GetID()
	if runnerID == 0 {
		return fmt.Errorf("cannot delete runner with ID %v", runnerID)
	}

	if c.repo == "" {
		_, err = client.Actions.RemoveOrganizationRunner(ctx, c.owner, runnerID)
	} else {
		_, err = client.Actions.RemoveRunner(ctx, c.owner, c.repo, runnerID)
	}
	return err
}
