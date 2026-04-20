package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func currentRepo(ctx context.Context) (string, string, string, error) {
	remote, err := currentRemote(ctx)
	if err != nil || remote == "" {
		remote = "origin"
	}

	key := fmt.Sprintf("remote.%s.url", remote)
	out, err := exec.CommandContext(ctx, "git", "config", "--get", key).Output()
	if err != nil {
		return "", "", "", fmt.Errorf("cannot find URL for remote %q: %w", remote, err)
	}

	address := strings.TrimSpace(string(out))
	url, err := parseAddress(address)
	if err != nil {
		return "", "", "", fmt.Errorf("cannot find repository: %w", err)
	}

	hostname, owner, repo, err := parseRepository(url)
	if err != nil {
		return "", "", "", fmt.Errorf("cannot find repository: %w", err)
	}

	return hostname, owner, repo, nil
}

func currentRemote(ctx context.Context) (string, error) {
	branch, err := currentBranch(ctx)
	if err != nil {
		return "", err
	}

	key := fmt.Sprintf("branch.%s.pushRemote", branch)
	out, err := exec.CommandContext(ctx, "git", "config", "--get", key).Output()
	if err == nil {
		return strings.TrimSuffix(string(out), "\n"), nil
	}

	key = fmt.Sprintf("branch.%s.remote", branch)
	out, err = exec.CommandContext(ctx, "git", "config", "--get", key).Output()
	if err == nil {
		return strings.TrimSuffix(string(out), "\n"), nil
	}

	return "", fmt.Errorf("cannot find remote for %q branch: %w", branch, err)
}

func currentBranch(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "branch", "--show-current").Output()
	if err != nil {
		return "", fmt.Errorf("cannot find git branch: %w", err)
	}

	if branch := strings.TrimSuffix(string(out), "\n"); branch != "" {
		return branch, nil
	}
	return "", errors.New("cannot find git branch")
}
