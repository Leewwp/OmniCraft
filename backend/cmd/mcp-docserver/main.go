package main

// mcp-docserver is the document-editing MCP server (SP-23 M2, #567): a
// stdio server exposing draft_read / draft_suggest / draft_apply_edit over
// the agent workspace draft store. It is launched as a subprocess by the MCP
// client bridge (SP-23 M3) or directly by an MCP inspector; identity is
// bound at launch time via environment, never per call:
//
//	OMNICRAFT_DOC_USER_ID   required — the user whose drafts are editable
//	DB_DSN / .env           PostgreSQL wiring via the shared config loader
//	AGENT_HMAC_SECRET       confirmation-token HMAC key (falls back to the
//	                        agent hmac_secret from config.yaml)
//
// No anonymous writes: a missing user id exits before serving anything.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/mcpdocserver"
	"omnicraft/backend/internal/pkg/database"
	"omnicraft/backend/internal/repository"
)

func main() {
	userID, err := strconv.ParseInt(os.Getenv("OMNICRAFT_DOC_USER_ID"), 10, 64)
	if err != nil || userID <= 0 {
		slog.Error("OMNICRAFT_DOC_USER_ID must be a positive user id (no anonymous draft access)")
		os.Exit(2)
	}
	cfg := config.Load()
	secret := []byte(os.Getenv("AGENT_HMAC_SECRET"))
	if len(secret) == 0 {
		secret = []byte(cfg.Agent.HMACSecret)
	}
	db := database.Init(cfg)
	server := mcpdocserver.NewServer(
		repository.NewAgentDraftRepository(db),
		mcpdocserver.Options{UserID: userID, ConfirmSecret: secret},
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := server.Run(ctx, &sdkmcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "mcp-docserver: %v\n", err)
		os.Exit(1)
	}
}
