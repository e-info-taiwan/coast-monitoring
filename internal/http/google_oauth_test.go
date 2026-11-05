package httpx

import (
	"context"
	"encoding/json"
	"github.com/coreos/go-oidc/v3/oidc"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewGoogleOAuthProviderRequiresConfig(t *testing.T) {
	_, err := newGoogleOAuthProvider(context.Background(), GoogleOAuthConfig{
		ClientID: "client-id",
	}, "https://accounts.google.com")

	if err == nil {
		t.Fatal("newGoogleOAuthProvider error = nil, want missing config error")
	}
}

func TestNewGoogleOAuthProviderUsesOIDCDiscovery(t *testing.T) {
	issuer := "https://oidc.example.invalid"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeTestJSON(t, w, map[string]any{
				"issuer":                                issuer,
				"authorization_endpoint":                issuer + "/auth",
				"token_endpoint":                        issuer + "/token",
				"jwks_uri":                              issuer + "/keys",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/keys":
			writeTestJSON(t, w, map[string]any{"keys": []any{}})
		default:
			http.NotFound(w, r)
		}
	})
	client := &http.Client{Transport: discoveryTransport{handler}}
	ctx := oidc.ClientContext(context.Background(), client)

	provider, err := newGoogleOAuthProvider(ctx, GoogleOAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://app.example.com/api/auth/google/callback",
	}, issuer)
	if err != nil {
		t.Fatalf("newGoogleOAuthProvider error = %v", err)
	}

	authURL := provider.AuthCodeURL("state-token")
	if !strings.HasPrefix(authURL, issuer+"/auth?") {
		t.Fatalf("AuthCodeURL = %q, want local auth endpoint", authURL)
	}
	for _, want := range []string{"client_id=client-id", "redirect_uri=https%3A%2F%2Fapp.example.com%2Fapi%2Fauth%2Fgoogle%2Fcallback", "scope=openid+email+profile", "state=state-token"} {
		if !strings.Contains(authURL, want) {
			t.Fatalf("AuthCodeURL = %q, missing %q", authURL, want)
		}
	}
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, value map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("Encode discovery response error = %v", err)
	}
}

type discoveryTransport struct{ handler http.Handler }

func (d discoveryTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	d.handler.ServeHTTP(w, r)
	return w.Result(), nil
}
