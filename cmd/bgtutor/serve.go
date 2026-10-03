package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/snonux/bgtutor-mcp/internal/bgtutor/mcpserver"
)

func newServeCmd() *cobra.Command {
	var addr, mode string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the selected tutor mode (Streamable HTTP at /mcp)",
		Long: `Run the MCP server. Set BGTUTOR_TOKEN to require a bearer token
("Authorization: Bearer <token>" or "?token=<token>" on the URL). Without a
token the server refuses to listen on anything but localhost.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dataDir, _ := cmd.Flags().GetString("data-dir")
			return serve(cmd.Context(), dataDir, addr, os.Getenv("BGTUTOR_TOKEN"), mode)
		},
	}
	cmd.Flags().StringVar(&mode, "mode", envOr("BGTUTOR_MODE", "podcast"), "tutor mode: podcast or citizenship (env BGTUTOR_MODE)")
	cmd.Flags().StringVar(&addr, "addr", envOr("BGTUTOR_ADDR", "127.0.0.1:8080"), "listen address (env BGTUTOR_ADDR)")
	return cmd
}

func newHTTPServer(dataDir, addr, token, mode string) (*http.Server, error) {
	if token == "" && !isLoopback(addr) {
		return nil, fmt.Errorf("refusing to listen on %s without BGTUTOR_TOKEN; set a token or use 127.0.0.1", addr)
	}
	if st, err := os.Stat(dataDir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("data dir %q does not exist", dataDir)
	}
	server, err := mcpserver.NewWithMode(dataDir, mode)
	if err != nil {
		return nil, err
	}
	// Keep SDK diagnostics at warning level while recording every HTTP request
	// through a separate access logger, so normal traffic is visible in pod logs.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	accessLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	srv := &http.Server{
		Addr:              addr,
		Handler:           mcpserver.Handler(server, mcpserver.HTTPOptions{Token: token, Logger: logger, AccessLogger: accessLogger}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv, nil
}

func serve(ctx context.Context, dataDir, addr, token, mode string) error {
	srv, err := newHTTPServer(dataDir, addr, token, mode)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	// ListenAndServe returns as soon as Shutdown starts, so wait for
	// Shutdown to finish draining in-flight requests before returning.
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	auth := "none (localhost only)"
	if token != "" {
		auth = "bearer token"
	}
	fmt.Fprintf(os.Stderr, "bgtutor: serving %s (%s mode) at http://%s/mcp (auth: %s)\n", dataDir, mode, addr, auth)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-shutdownDone
	return nil
}

// isLoopback reports whether addr only binds to the local machine.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
