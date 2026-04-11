package httpserver

import (
	"net/http"
	"time"
)

type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (r *ResponseWriter) WriteHeader(status int) {
	r.statusCode = status
	r.ResponseWriter.WriteHeader(status)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &ResponseWriter{w, 200}
		start := time.Now()
		next.ServeHTTP(rw, r)
		time.Sleep(time.Millisecond * 10)
		duration := time.Since(start)
		s.logger.LogHTTPRequest(r, duration, rw.statusCode)
	})
}
