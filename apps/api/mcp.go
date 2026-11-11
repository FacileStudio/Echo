package main

import (
	"context"
	"crypto/sha256"
	"log/slog"

	"github.com/FacileStudio/Echo/apps/api/modules/mcp"
	"github.com/FacileStudio/porte/mcpauth"
	mcpkit "github.com/FacileStudio/tronc/mcp"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

// newMCP builds the MCP server when OIDC is on and MCP_RESOURCE is set, and
// nothing otherwise. The verifier runs Registre discovery here, so a wrong
// issuer fails at boot like the OIDC kit does. A person's first MCP call
// creates their account through the OIDC kit, as a first web login would.
func newMCP(ctx context.Context, app *apiDeps, tools mcp.Deps, log *slog.Logger) (*mcp.Service, error) {
	cfg := app.cfg
	if !cfg.Porte.Enabled() || cfg.MCPResource == "" {
		return nil, nil
	}
	sqlDB, err := app.db.DB()
	if err != nil {
		return nil, err
	}
	verifier, err := mcpVerifier(ctx, app, log)
	if err != nil {
		return nil, err
	}
	log.Info("mcp server on", slog.String("resource", cfg.MCPResource))
	return mcp.NewService(tools, mcp.Config{
		Resource: cfg.MCPResource,
		Issuer:   cfg.Porte.Issuer,
		Version:  version,
		Origins:  cfg.CORSAllowedOrigins,
		StateKey: mcpStateKey(cfg.Porte.ClientSecret),
		Verifier: verifier,
		Pending:  mcpkit.PostgresPending{DB: sqlDB},
		Logger:   log,
	})
}

// mcpVerifier checks Registre access tokens bound to MCP_RESOURCE, and
// creates the account of a person whose first Echo visit is an MCP call.
func mcpVerifier(ctx context.Context, app *apiDeps, log *slog.Logger) (auth.TokenVerifier, error) {
	return mcpauth.Verifier(ctx, mcpauth.Config{
		Issuer:      app.cfg.Porte.Issuer,
		Resource:    app.cfg.MCPResource,
		ScopePrefix: "echo",
		Identities:  app.auth.identities,
		Provision:   true,
		Accounts:    app.auth.oidc,
		Logger:      log,
	})
}

// mcpStateKey signs the confirmation questions handed to clients. It is
// derived from the OIDC client secret, so it is secret, stable across
// restarts and needs no variable of its own.
func mcpStateKey(clientSecret string) []byte {
	key := sha256.Sum256([]byte("echo-mcp-state:" + clientSecret))
	return key[:]
}
