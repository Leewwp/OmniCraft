package handler

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// SP-16 #448: the hand-written OpenAPI 3.1 contract of the anonymous
// read-only v1 subset. Single source of truth lives beside this handler;
// docs/reference/openapi.v1.json is the browsing copy kept byte-identical
// by the contract test.
//
//go:embed openapi/openapi.v1.json
var openAPIV1FS embed.FS

var (
	openAPIV1Once  sync.Once
	openAPIV1Bytes []byte
)

// OpenAPIV1Spec exposes the embedded spec bytes (used by the contract test).
func OpenAPIV1Spec() []byte {
	openAPIV1Once.Do(func() {
		raw, err := openAPIV1FS.ReadFile("openapi/openapi.v1.json")
		if err != nil {
			panic("openapi.v1.json must be embedded: " + err.Error())
		}
		openAPIV1Bytes = raw
	})
	return openAPIV1Bytes
}

type OpenAPIV1Handler struct{}

func NewOpenAPIV1Handler() *OpenAPIV1Handler { return &OpenAPIV1Handler{} }

// Serve returns the spec with an ETag derived from its bytes, so consumers
// can revalidate cheaply across releases.
func (h *OpenAPIV1Handler) Serve(c *gin.Context) {
	body := OpenAPIV1Spec()
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:])[:32] + `"`
	c.Header("Cache-Control", "public, max-age=300, s-maxage=3600")
	c.Header("ETag", etag)
	if match := c.GetHeader("If-None-Match"); match != "" && etagMatches(match, etag) {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, "application/json", body)
}
