package main

import (
	"os"
	"strings"
	"testing"
)

func TestMainCallsValidateReleaseAfterLoadAndBeforeExternalInit(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	text := string(src)
	loadIdx := strings.Index(text, "cfg := config.Load()")
	validateIdx := strings.Index(text, "cfg.ValidateRelease()")
	dbIdx := strings.Index(text, "database.Init")
	if loadIdx < 0 {
		t.Fatal("main must load config")
	}
	if validateIdx < 0 {
		t.Fatal("main must call cfg.ValidateRelease()")
	}
	if validateIdx < loadIdx {
		t.Fatal("ValidateRelease must run after config load")
	}
	if dbIdx < 0 {
		t.Fatal("main must initialize database")
	}
	if validateIdx > dbIdx {
		t.Fatal("ValidateRelease must run before database initialization")
	}
	redisIdx := strings.Index(text, "redisclient.Init")
	if redisIdx < 0 {
		t.Fatal("main must initialize redis")
	}
	if validateIdx > redisIdx {
		t.Fatal("ValidateRelease must run before redis initialization")
	}
}
