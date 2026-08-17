package httpx

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/example/autoparts-core/internal/platform/api"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func RequestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if len(id) < 8 || len(id) > 128 || strings.ContainsAny(id, "\r\n\t ") {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

func WrapStatus(w http.ResponseWriter) *statusWriter { return &statusWriter{ResponseWriter: w} }
func (w *statusWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}
func (w *statusWriter) Bytes() int { return w.bytes }

func AccessLog(trustProxy bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		sw := WrapStatus(w)
		next.ServeHTTP(sw, r)
		event := map[string]any{
			"ts":          time.Now().UTC().Format(time.RFC3339Nano),
			"level":       "info",
			"event":       "http_request",
			"request_id":  RequestIDFrom(r.Context()),
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      sw.Status(),
			"bytes":       sw.Bytes(),
			"duration_ms": time.Since(started).Milliseconds(),
			"remote_ip":   ClientIP(r, trustProxy),
		}
		if r.Pattern != "" {
			event["route"] = r.Pattern
		}
		b, _ := json.Marshal(event)
		log.Print(string(b))
	})
}

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				b, _ := json.Marshal(map[string]any{
					"ts":         time.Now().UTC().Format(time.RFC3339Nano),
					"level":      "error",
					"event":      "panic_recovered",
					"request_id": RequestIDFrom(r.Context()),
					"path":       r.URL.Path,
					"panic":      fmt.Sprint(rec),
					"stack":      string(debug.Stack()),
				})
				log.Print(string(b))
				api.WriteJSON(w, http.StatusInternalServerError, api.ErrorBody{Error: api.APIError{Code: "internal_error", Message: "unexpected server error"}})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func SecurityHeaders(enableHSTS bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(self)")
		if enableHSTS {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if raw := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); raw != "" {
			if first := strings.TrimSpace(strings.Split(raw, ",")[0]); first != "" {
				return first
			}
		}
		if raw := strings.TrimSpace(r.Header.Get("X-Real-IP")); raw != "" {
			return raw
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

type rateBucket struct {
	windowStart time.Time
	count       int
}

type RateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]rateBucket
	limit      int
	window     time.Duration
	trustProxy bool
	lastSweep  time.Time
}

func NewRateLimiter(limit int, window time.Duration, trustProxy bool) *RateLimiter {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{buckets: make(map[string]rateBucket), limit: limit, window: window, trustProxy: trustProxy, lastSweep: time.Now()}
}

func (l *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := ClientIP(r, l.trustProxy)
		now := time.Now()
		allowed, retry := l.allow(key, now)
		if !allowed {
			seconds := int(retry.Round(time.Second) / time.Second)
			if seconds < 1 {
				seconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			api.WriteJSON(w, http.StatusTooManyRequests, api.ErrorBody{Error: api.APIError{Code: "rate_limited", Message: "too many requests; retry later"}})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastSweep) > l.window*2 {
		cutoff := now.Add(-l.window * 2)
		for k, b := range l.buckets {
			if b.windowStart.Before(cutoff) {
				delete(l.buckets, k)
			}
		}
		l.lastSweep = now
	}
	b := l.buckets[key]
	if b.windowStart.IsZero() || now.Sub(b.windowStart) >= l.window {
		l.buckets[key] = rateBucket{windowStart: now, count: 1}
		return true, 0
	}
	if b.count >= l.limit {
		return false, l.window - now.Sub(b.windowStart)
	}
	b.count++
	l.buckets[key] = b
	return true, 0
}
