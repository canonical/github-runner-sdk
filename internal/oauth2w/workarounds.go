// Workarounds for https://github.com/golang/oauth2/issues/262.
package oauth2w

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

type TokenSourceContext interface {
	TokenContext(ctx context.Context) (*oauth2.Token, error)
}

func StaticTokenSourceContext(token *oauth2.Token) TokenSourceContext {
	return &tokenSourceContext{Token: token}
}

type Config struct {
	oauth2.Config
}

func (c *Config) TokenSourceContext(token *oauth2.Token) TokenSourceContext {
	return &tokenSourceContext{Config: c, Token: token}
}

type tokenSourceContext struct {
	*Config
	*oauth2.Token
}

func (s *tokenSourceContext) TokenContext(ctx context.Context) (*oauth2.Token, error) {
	if s.Config == nil {
		return s.Token, nil
	}
	token, err := s.TokenSource(ctx, s.Token).Token()
	if err != nil {
		return token, err
	}
	s.Token = token
	return token, err
}

func NewClientContext(ctx context.Context, source TokenSourceContext) *http.Client {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(nil))
	transport := client.Transport.(*oauth2.Transport)
	transport.Source = nil
	client.Transport = transportContext{Transport: transport, actualSource: source}
	return client
}

type transportContext struct {
	*oauth2.Transport
	actualSource TokenSourceContext
}

func (t transportContext) RoundTrip(request *http.Request) (*http.Response, error) {
	t.Source = tokenSource{TokenSourceContext: t.actualSource, Context: request.Context()}
	response, err := t.Transport.RoundTrip(request)
	t.Source = nil
	return response, err
}

type tokenSource struct {
	TokenSourceContext
	context.Context
}

func (s tokenSource) Token() (*oauth2.Token, error) {
	return s.TokenContext(s.Context)
}
