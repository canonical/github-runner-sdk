package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"golang.org/x/oauth2"

	"github.com/canonical/sdks/github-runner/24.04/internal/oauth2w"
)

const clientID = "Iv23liTDRAG9hbRN13UN"
const appURL = "https://github.com/apps/runner-sdk-for-workshop"

func (c *repoConfig) oauthClient(ctx context.Context, std stdio) (*http.Client, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)

	source, err := c.tokenSource(ctx, std)
	if err != nil {
		return nil, err
	}
	return oauth2w.NewClientContext(ctx, source), nil
}

func (c *repoConfig) tokenSource(ctx context.Context, std stdio) (oauth2w.TokenSourceContext, error) {
	if source := maybeEnvSource(); source != nil {
		return source, nil
	}

	config := &oauth2w.Config{Config: oauth2.Config{
		ClientID: clientID,
		Endpoint: c.oauthEndpoint(),
	}}
	cache, err := newTokenCache(c.repoID)
	if err != nil {
		return nil, err
	}

	if source := maybeCachingSource(ctx, config, cache); source != nil {
		return source, nil
	}

	token, err := deviceAuth(ctx, std, config, c.repoID)
	if err != nil {
		return nil, err
	}

	return cachingSource(config, cache, token)
}

func maybeEnvSource() oauth2w.TokenSourceContext {
	accessToken := os.Getenv("GITHUB_TOKEN")
	if accessToken == "" {
		return nil
	}

	token := &oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "bearer",
	}
	return oauth2w.StaticTokenSourceContext(token)
}

func newTokenCache(repoID int64) (*tokenCache, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(cache), os.ModePerm); err != nil {
		return nil, err
	}

	dir := filepath.Join(cache, "github-runner")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	path := filepath.Join(dir, fmt.Sprintf("%v.json", repoID))
	return &tokenCache{path: path}, nil
}

func maybeCachingSource(ctx context.Context, config *oauth2w.Config, cache *tokenCache) oauth2w.TokenSourceContext {
	source, err := cachingSource(config, cache, nil)
	if err != nil {
		return nil
	}

	if _, err := source.TokenContext(ctx); err != nil {
		return nil
	}
	return source
}

func deviceAuth(ctx context.Context, std stdio, config *oauth2w.Config, repoID int64) (*oauth2.Token, error) {
	sym := newSymbols(std.out)
	esc := newEscapes(std.out)

	fmt.Fprintf(std.out, "\n%s Adding a runner requires limited access to your GitHub account:\n\n", sym.lock)
	fmt.Fprintf(std.out, "1. Install or configure the GitHub App at %s\n\n", esc.linkURL(appURL))
	prompt := "2. Enter code %s at %s\n\n"

	accessOptions := []oauth2.AuthCodeOption{}
	if repoID != 0 {
		option := oauth2.SetAuthURLParam("repository_id", strconv.FormatInt(repoID, 10))
		accessOptions = append(accessOptions, option)
	}

	for {
		response, err := config.DeviceAuth(ctx)
		if err != nil {
			return nil, err
		}

		fmt.Fprintf(std.out, prompt, response.UserCode, esc.linkURL(response.VerificationURI))

		token, err := config.DeviceAccessToken(ctx, response, accessOptions...)
		var retrieveErr *oauth2.RetrieveError
		retryCodes := []string{"bad_verification_code", "expired_token"}
		if errors.As(err, &retrieveErr) && slices.Contains(retryCodes, retrieveErr.ErrorCode) {
			fmt.Fprintln(std.err, err)
			prompt = "Enter code %s at %s to try again\n\n"
			continue
		}
		if err != nil {
			return nil, err
		}
		if token.ExpiresIn == 0 {
			// FIXME: https://github.com/golang/oauth2/issues/782
			token.ExpiresIn = int64(time.Until(token.Expiry) / time.Second)
		}

		fmt.Fprintf(std.out, "%s%s%s Logged in\n", esc.green, sym.tick, esc.end)

		extra := knownExtra(token)
		extra["repository_id"] = repoID
		return token.WithExtra(extra), nil
	}
}
