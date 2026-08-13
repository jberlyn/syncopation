package api

import (
	"log/slog"
	"net/http"
	"time"
)

// customResponseWriter captures the HTTP status code
type customResponseWriter struct {
	http.ResponseWriter
	status int
}

func (c *customResponseWriter) WriteHeader(status int) {
	c.status = status
	c.ResponseWriter.WriteHeader(status)
}

// LoggingMiddleware logs incoming HTTP requests using structured logging,
// analogous to how Pino is used in a Node/Express backend.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		crw := &customResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(crw, r)

		latency := time.Since(start)

		// Scrub sensitive headers before logging
		scrubbedHeaders := r.Header.Clone()
		if scrubbedHeaders.Get("Authorization") != "" {
			scrubbedHeaders.Set("Authorization", "[REDACTED]")
		}
		if scrubbedHeaders.Get("X-API-AUTH") != "" {
			scrubbedHeaders.Set("X-API-AUTH", "[REDACTED]")
		}

		slog.Info("HTTP Request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", crw.status),
			slog.Duration("latency", latency),
			slog.Any("headers", scrubbedHeaders),
		)
	})
}
