package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubmissionLimiterRejectsSpoofedForwardedAddresses(t *testing.T) {
	calls := 0
	handler := newSubmissionLimiter()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(http.StatusNoContent) }))
	for i := 0; i < 31; i++ {
		req := httptest.NewRequest(http.MethodPost, "/submit", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("X-Forwarded-For", string(rune('a'+i)))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		expected := http.StatusNoContent
		if i == 30 {
			expected = http.StatusTooManyRequests
		}
		if response.Code != expected {
			t.Fatalf("request %d status=%d want=%d", i, response.Code, expected)
		}
		if i == 30 && response.Header().Get("Retry-After") == "" {
			t.Fatal("missing retry guidance")
		}
	}
	if calls != 30 {
		t.Fatalf("received %d submissions", calls)
	}
	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	req.RemoteAddr = "192.0.2.2:5678"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusNoContent {
		t.Fatal("unrelated client was blocked")
	}
}
