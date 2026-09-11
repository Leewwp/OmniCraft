package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// CacheableAnonymousGET is the shared caching contract for the contracted
// anonymous read endpoints (SP-16 #448): anonymous 200 JSON responses gain
// `Cache-Control: public, max-age=60, s-maxage=<n>` plus a strong
// body-hash ETag honouring If-None-Match revalidation.
//
// The response body is buffered (bounded) so the content-addressed ETag can
// be compared before anything hits the wire; a matching If-None-Match is
// answered as an empty 304. Authenticated responses are never marked
// publicly cacheable — they vary by viewer and must not poison shared
// caches. Endpoints that set their own ETag (the guide endpoint) keep it.
func CacheableAnonymousGET(sMaxAgeSeconds int) gin.HandlerFunc {
	const maxCacheableBodyBytes = 512 * 1024
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		recorder := &cacheBodyWriter{ResponseWriter: c.Writer, limit: maxCacheableBodyBytes}
		c.Writer = recorder
		c.Next()

		if recorder.truncated {
			// Large payloads streamed through; nothing left to decide.
			return
		}
		status := recorder.Status()
		body := recorder.buffer

		if status != http.StatusOK || GetUserID(c) != 0 || recorder.Header().Get("ETag") != "" {
			recorder.flushBuffered(status)
			return
		}
		ct := recorder.Header().Get("Content-Type")
		if ct != "" && !strings.HasPrefix(ct, "application/json") {
			recorder.flushBuffered(status)
			return
		}

		sum := sha256.Sum256(body)
		etag := `"` + hex.EncodeToString(sum[:])[:32] + `"`
		if match := c.GetHeader("If-None-Match"); match != "" && ifNoneMatchHits(match, etag) {
			recorder.Header().Del("Content-Length")
			recorder.WriteHeader(http.StatusNotModified)
			recorder.WriteHeaderNow()
			return
		}
		recorder.Header().Set("Cache-Control", "public, max-age=60, s-maxage="+strconv.Itoa(clampSeconds(sMaxAgeSeconds)))
		recorder.Header().Set("ETag", etag)
		recorder.flushBuffered(status)
	}
}

func ifNoneMatchHits(header, etag string) bool {
	if header == "*" {
		return true
	}
	for _, part := range strings.Split(header, ",") {
		if part = strings.TrimSpace(part); part == etag || part == "W/"+etag {
			return true
		}
	}
	return false
}

func clampSeconds(v int) int {
	if v < 1 {
		return 1
	}
	if v > 86400 {
		return 86400
	}
	return v
}

// cacheBodyWriter buffers the response body up to a byte limit; beyond the
// limit everything already buffered is flushed through and the writer
// degrades to pass-through (response too large for content-hash caching).
type cacheBodyWriter struct {
	gin.ResponseWriter
	buffer    []byte
	limit     int
	truncated bool
}

func (w *cacheBodyWriter) Write(data []byte) (int, error) {
	if w.truncated {
		return w.ResponseWriter.Write(data)
	}
	if len(w.buffer)+len(data) > w.limit {
		w.truncated = true
		w.passThrough(w.Status())
		return w.ResponseWriter.Write(data)
	}
	w.buffer = append(w.buffer, data...)
	return len(data), nil
}

func (w *cacheBodyWriter) WriteString(data string) (int, error) {
	if w.truncated {
		return w.ResponseWriter.WriteString(data)
	}
	if len(w.buffer)+len(data) > w.limit {
		w.truncated = true
		w.passThrough(w.Status())
		return w.ResponseWriter.WriteString(data)
	}
	w.buffer = append(w.buffer, data...)
	return len(data), nil
}

// passThrough flushes whatever was buffered so far with the recorded status
// and marks the writer as streaming mode.
func (w *cacheBodyWriter) passThrough(status int) {
	if len(w.buffer) > 0 {
		w.ResponseWriter.WriteHeader(status)
		_, _ = w.ResponseWriter.Write(w.buffer)
		w.buffer = nil
	}
}

// flushBuffered commits the buffered body (and status) to the real writer.
func (w *cacheBodyWriter) flushBuffered(status int) {
	w.ResponseWriter.WriteHeader(status)
	if len(w.buffer) > 0 {
		_, _ = w.ResponseWriter.Write(w.buffer)
	}
}
