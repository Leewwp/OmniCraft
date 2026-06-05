package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMainCallsValidateReleaseAfterLoadingConfig(t *testing.T) {
	src, err := os.ReadFile("main.go")
	require.NoError(t, err)

	source := string(src)
	loadIndex := strings.Index(source, "cfg := config.Load()")
	validateIndex := strings.Index(source, "cfg.ValidateRelease()")

	require.NotEqual(t, -1, loadIndex, "main must load config")
	require.NotEqual(t, -1, validateIndex, "main must call cfg.ValidateRelease()")
	require.Greater(t, validateIndex, loadIndex, "release validation must happen after config load")
}
