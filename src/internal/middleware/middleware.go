package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			b := make([]byte, 16)
			rand.Read(b)
			id = hex.EncodeToString(b)
		} else {
			// Strip control characters from client-supplied request ID.
			var b strings.Builder
			b.Grow(len(id))
			for _, c := range id {
				if c >= 32 {
					b.WriteRune(c)
				}
			}
			id = b.String()
			if id == "" {
				b2 := make([]byte, 16)
				rand.Read(b2)
				id = hex.EncodeToString(b2)
			}
		}
		w.Header().Set("X-Request-ID", id)
		r.Header.Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

func RecoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf(`{"level":"ERROR","msg":"panic recovered","error":"%v","path":"%s","request_id":"%s"}`, err, r.URL.Path, r.Header.Get("X-Request-ID"))
				http.Error(w, `{"error":{"message":"internal server error"}}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start)

		log.Printf(`{"level":"INFO","method":"%s","path":"%s","status":%d,"duration_ms":%d,"remote_addr":"%s","user_agent":"%s","request_id":"%s"}`,
			r.Method, r.URL.Path, rw.statusCode, duration.Milliseconds(),
			r.RemoteAddr, r.UserAgent(), r.Header.Get("X-Request-ID"))
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rpm      int
}

type visitor struct {
	count    int
	lastSeen time.Time
}

func NewRateLimiter(rpm int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rpm:      rpm,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 2*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Allow(ip string) bool {
	if rl.rpm <= 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists || time.Since(v.lastSeen) > time.Minute {
		rl.visitors[ip] = &visitor{count: 1, lastSeen: time.Now()}
		return true
	}

	if v.count >= rl.rpm {
		return false
	}

	v.count++
	v.lastSeen = time.Now()
	return true
}

// Reset clears rate-limit counters for one IP, or all IPs when ip is empty.
func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if ip == "" {
		rl.visitors = make(map[string]*visitor)
		return
	}
	delete(rl.visitors, ip)
}

func clientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	return strings.TrimSpace(strings.Split(ip, ",")[0])
}

func RateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only rate-limit API traffic.
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				next.ServeHTTP(w, r)
				return
			}

			ip := clientIP(r)

			if !rl.Allow(ip) {
				w.Header().Set("Retry-After", "60")
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.rpm))
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded; retry in 60 seconds or POST /api/rate-limit/reset"}`))
				log.Printf(`{"level":"WARN","msg":"rate limit exceeded","remote_addr":"%s","path":"%s","request_id":"%s"}`, ip, r.URL.Path, r.Header.Get("X-Request-ID"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
