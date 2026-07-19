// Command intervals is the read-only (plus gated write) Model Context Protocol
// server wrapping the Intervals.icu API. It is a single-user, self-hosted MCP
// service: one deployment serves one athlete's Intervals.icu account.
//
// Transport is chosen by environment:
//
//	INTERVALS_LISTEN_ADDR  -- if set (e.g. 127.0.0.1:8081), serve Streamable HTTP
//	                          on that address; otherwise serve over stdio (local).
//	MCP_BEARER_TOKEN       -- required in HTTP mode: the shared bearer token every
//	                          request must present. Generate a long random value
//	                          and give it to the MCP client; Caddy terminates TLS.
//
// Credentials (never hardcoded):
//
//	INTERVALS_API_KEY      -- Intervals.icu API key (HTTP Basic password).
//	INTERVALS_ATHLETE_ID   -- athlete id, e.g. "i123456" ("0" also works).
package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/goosegit97/intervals-mcp/internal/config"
	"github.com/goosegit97/intervals-mcp/internal/intervals"
	mcputil "github.com/goosegit97/intervals-mcp/internal/mcp"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	config.LoadDotEnv()

	svc, err := intervals.NewService()
	if err != nil {
		slog.Error("intervals: config error", "err", err)
		os.Exit(1)
	}

	addr := strings.TrimSpace(os.Getenv("INTERVALS_LISTEN_ADDR"))
	if addr == "" {
		runStdio(svc)
		return
	}
	if err := runHTTP(addr, svc); err != nil {
		slog.Error("intervals: server error", "err", err)
		os.Exit(1)
	}
}

// runStdio serves the MCP server over stdio (local development / direct MCP
// clients). A clean stdin EOF or cancelled context is a normal shutdown.
func runStdio(svc *intervals.Service) {
	err := intervals.NewServer(svc).Run(context.Background(), &mcp.StdioTransport{})
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		slog.Error("intervals: server error", "err", err)
		os.Exit(1)
	}
}

// runHTTP serves the MCP server over Streamable HTTP, gated by a single shared
// bearer token (MCP_BEARER_TOKEN). /healthz is unauthenticated.
func runHTTP(addr string, svc *intervals.Service) error {
	token := strings.TrimSpace(os.Getenv("MCP_BEARER_TOKEN"))
	if token == "" {
		return errors.New("MCP_BEARER_TOKEN must be set when serving HTTP")
	}
	return mcputil.ServeResourceServer(mcputil.ResourceServerConfig{
		Addr:    addr,
		Service: "intervals",
		Auth:    mcputil.StaticTokenMiddleware(token),
	}, intervals.NewServer(svc))
}
