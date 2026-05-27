package httpx

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const googleIssuerURL = "https://accounts.google.com"

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type oidcGoogleOAuthProvider struct {
	oauthConfig oauth2.Config
	verifier    *oidc.IDTokenVerifier
}

func NewGoogleOAuthProvider(ctx context.Context, cfg GoogleOAuthConfig) (GoogleOAuthProvider, error) {
	return newGoogleOAuthProvider(ctx, cfg, googleIssuerURL)
}

func newGoogleOAuthProvider(ctx context.Context, cfg GoogleOAuthConfig, issuerURL string) (GoogleOAuthProvider, error) {
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	cfg.ClientSecret = strings.TrimSpace(cfg.ClientSecret)
	cfg.RedirectURL = strings.TrimSpace(cfg.RedirectURL)
	issuerURL = strings.TrimSpace(issuerURL)
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURL == "" || issuerURL == "" {
		return nil, errors.New("google oauth client id, client secret, redirect url, and issuer url are required")
	}

	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("create oidc provider: %w", err)
	}
	return &oidcGoogleOAuthProvider{
		oauthConfig: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
	}, nil
}

func (p *oidcGoogleOAuthProvider) AuthCodeURL(state string) string {
	return p.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *oidcGoogleOAuthProvider) Exchange(ctx context.Context, code string) (OAuthToken, error) {
	token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return OAuthToken{}, err
	}
	idToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(idToken) == "" {
		return OAuthToken{}, errors.New("google oauth token response missing id_token")
	}
	return OAuthToken{IDToken: idToken}, nil
}

func (p *oidcGoogleOAuthProvider) VerifyIDToken(ctx context.Context, token OAuthToken) (GoogleIdentity, error) {
	idToken, err := p.verifier.Verify(ctx, strings.TrimSpace(token.IDToken))
	if err != nil {
		return GoogleIdentity{}, err
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return GoogleIdentity{}, err
	}
	return GoogleIdentity{
		Subject:       idToken.Subject,
		Email:         claims.Email,
		Name:          claims.Name,
		EmailVerified: claims.EmailVerified,
	}, nil
}
