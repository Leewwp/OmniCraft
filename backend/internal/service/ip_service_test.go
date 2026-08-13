package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/internal/model"
)

// B3: the IP review input carries the IP cover URL, keeps the pure text
// synchronous path (no attachments, no async callback for IP).
func TestIPReviewInputCarriesCoverURL(t *testing.T) {
	ip := &model.IP{
		ID:          7,
		Name:        "示例 IP",
		Slug:        "example-ip",
		Description: "desc",
		CoverURL:    "https://cdn.example.test/uploads/42/ip/cover.png",
	}
	in := ipReviewInput(ip, 42)

	require.Equal(t, "ip", in.TargetType)
	require.Equal(t, int64(7), in.TargetID)
	require.Equal(t, "示例 IP", in.Title)
	require.Equal(t, "desc", in.Description)
	require.Equal(t, int64(42), in.AuthorID)
	require.Equal(t, ip.CoverURL, in.CoverImageURL, "IP cover must enter the image review input")
	require.Empty(t, in.Attachments, "IP has no attachments; the fact is preserved")
}

func TestIPReviewInputEmptyCoverStaysEmpty(t *testing.T) {
	ip := &model.IP{ID: 9, Name: "no cover", Slug: "no-cover"}
	in := ipReviewInput(ip, 1)
	require.Empty(t, in.CoverImageURL)
}
