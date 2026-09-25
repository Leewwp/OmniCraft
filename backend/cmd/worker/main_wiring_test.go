package main

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// Ticket #671: the worker must run the release configuration gate just like
// the API server (spec: "worker 在 release 模式和 server 一样执行发布门").
// This is a source-contract check (routes_test.go / judge_wiring_test.go
// precedent): cmd/worker/main.go is a thin process entry whose wiring is not
// reachable from an in-process test.

func TestWorkerMainRunsValidateRelease(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	require.NoError(t, err)

	src := string(raw)
	loadIdx := regexp.MustCompile(`config\.Load\(\)`).FindStringIndex(src)
	require.NotNil(t, loadIdx, "worker main must call config.Load()")
	valIdx := regexp.MustCompile(`cfg\.ValidateRelease\(\)`).FindStringIndex(src)
	require.NotNil(t, valIdx, "worker main must call cfg.ValidateRelease() (ticket #671)")
	require.Less(t, loadIdx[1], valIdx[0], "ValidateRelease must run after Load")
	require.Regexp(t, `os\.Exit\(1\)`, src[valIdx[1]:], "ValidateRelease failure must exit non-zero")
}
