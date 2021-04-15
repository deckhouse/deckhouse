package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	header      string
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w}
}

func (rw *responseWriter) Status() int {
	return rw.status
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}

	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
	rw.wroteHeader = true
}

// Logs the incoming HTTP request and part of response
func LoggingMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Errorf("err %s", err)
			}
		}()

		start := time.Now()
		wrapped := wrapResponseWriter(w)
		next.ServeHTTP(wrapped, r)
		logHTTPReq(wrapped, r, start)
	})
}

func logHTTPReq(w *responseWriter, r *http.Request, startTime time.Time) {
	log.Tracef("%+v", r)
	if skipHTTPRequestLogging(r) {
		return
	}
	remoteAddr := r.RemoteAddr
	if r.Header.Get("x-real-ip") != "" {
		remoteAddr = r.Header.Get("x-real-ip")
	}

	logentry := fmt.Sprintf("%s %s %s %s %d %v",
		remoteAddr,
		r.Host,
		r.Method,
		r.URL.EscapedPath(),
		w.status,
		time.Since(startTime))
	if r.Header.Get("Referer") != "" {
		logentry += fmt.Sprintf(" referer:%s", r.Header.Get("Referer"))
	}
	if r.Header.Get("x-original-uri") != "" {
		logentry += fmt.Sprintf(" x-original-uri:%s", r.Header.Get("x-original-uri"))
	}
	if w.Header().Get("x-accel-redirect") != "" {
		logentry += fmt.Sprintf(" x-redirect:%s", w.Header().Get("X-Accel-Redirect"))
	}
	log.Infoln(logentry)
}

// Checks to skip logging some requests
func skipHTTPRequestLogging(r *http.Request) bool {
	switch r.URL.String() {
	case "/favicon.png":
		return true
	case "/favicon.ico":
		return true
	case "/health":
		return true
	}
	return strings.HasPrefix(r.URL.String(), "/favicon-")
}
