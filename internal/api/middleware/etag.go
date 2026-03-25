package middleware

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
)

// ETag buffers GET 200 responses, computes a SHA-256 tag, and short-circuits
// with 304 on If-None-Match match. Non-GET and non-200 pass through unchanged.
//
// Cache-Control is "private, no-cache": browsers revalidate (getting the 304
// benefit) but CDNs never cache. "private" is required because leaderboard
// picks are hidden until lock time, so two users see different bodies for the
// same URL.
func ETag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		buf := &responseBuffer{header: make(http.Header), status: http.StatusOK}
		next.ServeHTTP(buf, r)

		if buf.status != http.StatusOK {
			for k, vals := range buf.header {
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(buf.status)
			_, _ = w.Write(buf.body.Bytes())
			return
		}

		sum := sha256.Sum256(buf.body.Bytes())
		etag := fmt.Sprintf(`"%x"`, sum)

		for k, vals := range buf.header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "private, no-cache")

		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.body.Bytes())
	})
}

type responseBuffer struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (b *responseBuffer) Header() http.Header       { return b.header }
func (b *responseBuffer) Write(p []byte) (int, error) { return b.body.Write(p) }
func (b *responseBuffer) WriteHeader(status int)      { b.status = status }
