package main

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"

	"github.com/canonical/sdks/github-runner/24.04/internal/oauth2w"
)

type cachingTokenSource struct {
	oauth2w.TokenSourceContext

	cache *tokenCache
	token *oauth2.Token
}

func cachingSource(config *oauth2w.Config, cache *tokenCache, token *oauth2.Token) (oauth2w.TokenSourceContext, error) {
	if token == nil {
		var err error
		token, err = cache.restore()
		if err != nil {
			return nil, err
		}
	} else if err := cache.store(token); err != nil {
		return nil, err
	}

	source := config.TokenSourceContext(token)
	return &cachingTokenSource{
		TokenSourceContext: source,
		cache:              cache,
		token:              token,
	}, nil
}

func (s *cachingTokenSource) TokenContext(ctx context.Context) (*oauth2.Token, error) {
	token, err := s.TokenSourceContext.TokenContext(ctx)
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "incorrect_client_credentials" {
		err = hintf(err, "the refresh token may have been used already; consider reinstalling the GitHub App to revoke all active tokens")
	}
	if err != nil {
		_ = s.cache.forget()
		return token, err
	}

	if token.AccessToken != s.token.AccessToken {
		extra := knownExtra(s.token)
		maps.Copy(extra, knownExtra(token))
		s.token = token.WithExtra(extra)

		if err := s.cache.store(s.token); err != nil {
			return nil, err
		}
	}

	return s.token, nil
}

type tokenCache struct {
	path string
}

func (c *tokenCache) store(token *oauth2.Token) error {
	dir, base := filepath.Split(c.path)
	file, err := os.CreateTemp(dir, base+".*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer func() {
		if name != "" {
			_ = os.Remove(name)
		}
	}()

	t, err := splitExtra(token)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(file).Encode(t); err != nil {
		// Avoid leaking the token here.
		return errors.New("cannot encode token to JSON")
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	file = nil

	if err := os.Rename(name, c.path); err != nil {
		return err
	}
	name = ""
	return nil
}

func (c *tokenCache) restore() (*oauth2.Token, error) {
	file, err := os.Open(c.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var token tokenJSON
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&token); err != nil {
		// Avoid leaking the token here.
		return nil, errors.New("cannot decode token from JSON")
	}

	return token.mergeExtra(), nil
}

func (c *tokenCache) forget() error {
	return os.Remove(c.path)
}

type tokenJSON struct {
	oauth2.Token

	RefreshTokenExpiry    time.Time `json:"refresh_token_expiry,omitzero"`
	RefreshTokenExpiresIn int64     `json:"refresh_token_expires_in,omitempty"`

	RepoID int64 `json:"repository_id,omitempty"`
}

func splitExtra(token *oauth2.Token) (*tokenJSON, error) {
	data, err := json.Marshal(knownExtra(token))
	if err != nil {
		return nil, err
	}

	var t tokenJSON
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}

	t.Token = *token.WithExtra(nil)
	if t.RefreshTokenExpiry.IsZero() && t.RefreshTokenExpiresIn != 0 && !t.Expiry.IsZero() && t.ExpiresIn != 0 {
		delta := time.Duration(t.RefreshTokenExpiresIn-t.ExpiresIn) * time.Second
		t.RefreshTokenExpiry = t.Expiry.Add(delta)
	}

	t.ExpiresIn = 0
	t.RefreshTokenExpiresIn = 0
	return &t, nil
}

func (t *tokenJSON) mergeExtra() *oauth2.Token {
	token := t.Token

	now := time.Now()
	token.ExpiresIn = int64(token.Expiry.Sub(now) / time.Second)

	extra := map[string]any{}
	if !t.RefreshTokenExpiry.IsZero() {
		extra["refresh_token_expiry"] = t.RefreshTokenExpiry
		expiresIn := int64(t.RefreshTokenExpiry.Sub(now) / time.Second)
		extra["refresh_token_expires_in"] = expiresIn
	}
	if t.RepoID != 0 {
		extra["repository_id"] = t.RepoID
	}

	return token.WithExtra(extra)
}

func knownExtra(token *oauth2.Token) map[string]any {
	extra := map[string]any{}
	for _, key := range []string{"refresh_token_expiry", "refresh_token_expires_in", "repository_id"} {
		if value := token.Extra(key); value != nil && value != "" {
			extra[key] = value
		}
	}
	return extra
}
