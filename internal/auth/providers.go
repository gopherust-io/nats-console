package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"github.com/gopherust-io/nats-consol/internal/config"
)

const (
	ProviderLegacy    = "oidc"
	ProviderGoogle    = "google"
	ProviderGitHub    = "github"
	ProviderGitLab    = "gitlab"
	ProviderMicrosoft = "microsoft"
)

type ProviderInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type oauthProvider struct {
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
	oidcHTTP *http.Client
	id       string
	name     string
	github   bool
}

func buildProviders(cfg config.Config) (map[string]*oauthProvider, error) {
	providers := make(map[string]*oauthProvider)

	if cfg.OIDCEnabled {
		if cfg.OIDCIssuer == "" || cfg.OIDCClientID == "" || cfg.OIDCRedirectURL == "" {
			return nil, errors.New("OIDC_ENABLED requires OIDC_ISSUER, OIDC_CLIENT_ID, and OIDC_REDIRECT_URL")
		}
		p, err := newOIDCProvider(cfg, ProviderLegacy, "SSO", cfg.OIDCIssuer, cfg.OIDCDiscoveryURL, cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.OIDCRedirectURL)
		if err != nil {
			return nil, err
		}
		providers[ProviderLegacy] = p
	}

	if cfg.OIDCGoogleEnabled {
		if cfg.OIDCGoogleClientID == "" || cfg.OIDCGoogleClientSecret == "" {
			return nil, errors.New("OIDC_GOOGLE_ENABLED requires OIDC_GOOGLE_CLIENT_ID and OIDC_GOOGLE_CLIENT_SECRET")
		}
		p, err := newOIDCProvider(cfg, ProviderGoogle, "Google", "https://accounts.google.com", "", cfg.OIDCGoogleClientID, cfg.OIDCGoogleClientSecret, cfg.OAuthRedirectURL(ProviderGoogle))
		if err != nil {
			return nil, err
		}
		providers[ProviderGoogle] = p
	}

	if cfg.OIDCGitHubEnabled {
		if cfg.OIDCGitHubClientID == "" || cfg.OIDCGitHubClientSecret == "" {
			return nil, errors.New("OIDC_GITHUB_ENABLED requires OIDC_GITHUB_CLIENT_ID and OIDC_GITHUB_CLIENT_SECRET")
		}
		providers[ProviderGitHub] = &oauthProvider{
			id:     ProviderGitHub,
			name:   "GitHub",
			github: true,
			oauth: &oauth2.Config{
				ClientID:     cfg.OIDCGitHubClientID,
				ClientSecret: cfg.OIDCGitHubClientSecret,
				RedirectURL:  cfg.OAuthRedirectURL(ProviderGitHub),
				Endpoint:     github.Endpoint,
				Scopes:       []string{"read:user", "user:email"},
			},
		}
	}

	if cfg.OIDCGitLabEnabled {
		if cfg.OIDCGitLabClientID == "" || cfg.OIDCGitLabClientSecret == "" {
			return nil, errors.New("OIDC_GITLAB_ENABLED requires OIDC_GITLAB_CLIENT_ID and OIDC_GITLAB_CLIENT_SECRET")
		}
		base := strings.TrimSuffix(cfg.OIDCGitLabBaseURL, "/")
		if base == "" {
			base = "https://gitlab.com"
		}
		p, err := newOIDCProvider(cfg, ProviderGitLab, "GitLab", base, "", cfg.OIDCGitLabClientID, cfg.OIDCGitLabClientSecret, cfg.OAuthRedirectURL(ProviderGitLab))
		if err != nil {
			return nil, err
		}
		providers[ProviderGitLab] = p
	}

	if cfg.OIDCMicrosoftEnabled {
		if cfg.OIDCMicrosoftClientID == "" || cfg.OIDCMicrosoftClientSecret == "" {
			return nil, errors.New("OIDC_MICROSOFT_ENABLED requires OIDC_MICROSOFT_CLIENT_ID and OIDC_MICROSOFT_CLIENT_SECRET")
		}
		tenant := cfg.OIDCMicrosoftTenant
		if tenant == "" {
			tenant = "common"
		}
		issuer := fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", tenant)
		p, err := newOIDCProvider(cfg, ProviderMicrosoft, "Microsoft", issuer, "", cfg.OIDCMicrosoftClientID, cfg.OIDCMicrosoftClientSecret, cfg.OAuthRedirectURL(ProviderMicrosoft))
		if err != nil {
			return nil, err
		}
		providers[ProviderMicrosoft] = p
	}

	return providers, nil
}

func newOIDCProvider(cfg config.Config, id, name, issuer, discoveryURL, clientID, clientSecret, redirectURL string) (*oauthProvider, error) {
	if discoveryURL == "" {
		discoveryURL = issuer
	}
	providerCtx := context.Background()
	var oidcHTTP *http.Client

	if discoveryURL != issuer {
		issuerURL, err := url.Parse(issuer)
		if err != nil {
			return nil, fmt.Errorf("%s issuer url: %w", id, err)
		}
		discoveryParsed, err := url.Parse(discoveryURL)
		if err != nil {
			return nil, fmt.Errorf("%s discovery url: %w", id, err)
		}
		if issuerURL.Host != discoveryParsed.Host {
			oidcHTTP = hostRewriteHTTPClient(issuerURL.Host, discoveryParsed.Host, cfg)
			providerCtx = oidc.ClientContext(providerCtx, oidcHTTP)
		}
		providerCtx = oidc.InsecureIssuerURLContext(providerCtx, issuer)
	}

	provider, err := oidc.NewProvider(providerCtx, discoveryURL)
	if err != nil {
		return nil, fmt.Errorf("%s oidc provider: %w", id, err)
	}

	return &oauthProvider{
		id:       id,
		name:     name,
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
		oidcHTTP: oidcHTTP,
	}, nil
}

func (p *oauthProvider) authURL(state string) string {
	return p.oauth.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (p *oauthProvider) exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	if p.oidcHTTP != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, p.oidcHTTP)
	}
	return p.oauth.Exchange(ctx, code)
}

type oidcProfile struct {
	Email    string `json:"email"`
	Username string `json:"preferred_username"`
	Name     string `json:"name"`
}

func (p *oauthProvider) userFromOIDC(ctx context.Context, code string) (username, email, sub string, err error) {
	token, err := p.exchange(ctx, code)
	if err != nil {
		return "", "", "", fmt.Errorf("oidc exchange: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", "", "", errors.New("missing id_token")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", "", "", fmt.Errorf("verify id_token: %w", err)
	}

	var profile oidcProfile
	_ = idToken.Claims(&profile)

	username = profile.Username
	if username == "" {
		username = profile.Name
	}
	if username == "" {
		username = profile.Email
	}
	if username == "" {
		username = idToken.Subject
	}

	return username, profile.Email, p.id + ":" + idToken.Subject, nil
}

type githubUser struct {
	Login string `json:"login"`
	Email string `json:"email"`
	ID    int64  `json:"id"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (p *oauthProvider) userFromGitHub(ctx context.Context, code string) (username, email, sub string, err error) {
	token, err := p.exchange(ctx, code)
	if err != nil {
		return "", "", "", fmt.Errorf("github exchange: %w", err)
	}
	client := p.oauth.Client(ctx, token)

	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return "", "", "", fmt.Errorf("github user: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("github user: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var user githubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", "", "", fmt.Errorf("github user decode: %w", err)
	}

	email = user.Email
	if email == "" {
		email, _ = fetchGitHubPrimaryEmail(client)
	}

	username = user.Login
	if username == "" {
		username = email
	}
	if username == "" {
		username = fmt.Sprintf("github-%d", user.ID)
	}

	return username, email, fmt.Sprintf("github:%d", user.ID), nil
}

func fetchGitHubPrimaryEmail(client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var emails []githubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}
	for _, item := range emails {
		if item.Primary && item.Verified {
			return item.Email, nil
		}
	}
	for _, item := range emails {
		if item.Verified {
			return item.Email, nil
		}
	}
	if len(emails) > 0 {
		return emails[0].Email, nil
	}
	return "", nil
}
