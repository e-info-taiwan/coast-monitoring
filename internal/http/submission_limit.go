package httpx

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// Do not trust user supplied X-Forwarded-For. Bound memory even under many clients.
func newSubmissionLimiter() func(http.Handler) http.Handler {
	type bucket struct {
		start time.Time
		count int
	}
	buckets := map[string]bucket{}
	var mu sync.Mutex
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			now := time.Now()
			mu.Lock()
			for key, b := range buckets {
				if now.Sub(b.start) >= time.Minute {
					delete(buckets, key)
				}
			}
			b, exists := buckets[host]
			if !exists {
				b.start = now
			}
			allowed := b.count < 30 && (exists || len(buckets) < 4096)
			if allowed {
				b.count++
				buckets[host] = b
			}
			mu.Unlock()
			if !allowed {
				w.Header().Set("Retry-After", "60")
				writeError(w, 429, "送出過於頻繁，請稍候再試；本機草稿已保留")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
