package desktop

import (
	"encoding/json"
	"net/http"
)

// FollowRedirects adapts an ordinary HTTP handler for the Wails asset server.
//
// WebKit (macOS and Linux) does not follow 3xx responses delivered through a
// custom URL scheme handler; it just shows an empty page. Every form in the
// app posts and then redirects (POST/redirect/GET), so redirects are rewritten
// into a tiny page that navigates with location.replace. Replacing rather than
// pushing keeps the POST out of history, exactly as a real redirect would.
func FollowRedirects(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&redirectWriter{ResponseWriter: w}, r)
	})
}

type redirectWriter struct {
	http.ResponseWriter
	wroteHeader bool
	redirected  bool
}

func (rw *redirectWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true

	header := rw.Header()
	location := header.Get("Location")
	if code < 300 || code >= 400 || location == "" {
		rw.ResponseWriter.WriteHeader(code)
		return
	}

	rw.redirected = true
	header.Del("Location")
	header.Del("Content-Length")
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	rw.ResponseWriter.WriteHeader(http.StatusOK)
	_, _ = rw.ResponseWriter.Write(bouncePage(location))
}

// Write discards the body of a redirect (Go's "<a href=...>Found</a>").
func (rw *redirectWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	if rw.redirected {
		return len(b), nil
	}
	return rw.ResponseWriter.Write(b)
}

func bouncePage(location string) []byte {
	// encoding/json escapes <, > and &, so the URL cannot break out of the
	// script element.
	target, _ := json.Marshal(location)
	return []byte(`<!doctype html><meta charset="utf-8">` +
		`<meta name="color-scheme" content="light dark">` +
		`<script>location.replace(` + string(target) + `)</script>`)
}
