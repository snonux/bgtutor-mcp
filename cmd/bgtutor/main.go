// Command bgtutor is the standalone Bulgarian tutor MCP server.
//
//	bgtutor serve     run the MCP server (Streamable HTTP), with podcast or citizenship mode
//	bgtutor validate  check podcast or citizenship content
//	bgtutor publish   mark a valid draft episode as ready
//
// Episodes are prepared by a coding agent (Claude Code, Codex, ...) following
// PREPARE.md, not by this binary. See README.md.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/snonux/bgtutor-mcp/internal"
)

func main() {
	root := &cobra.Command{
		Use:           "bgtutor",
		Short:         "Bulgarian Tutor: podcast lessons and citizenship preparation over MCP",
		Version:       internal.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String("data-dir", envOr("BGTUTOR_DATA_DIR", "data"),
		"library directory holding episodes/, citizenship-test/ and learner data (env BGTUTOR_DATA_DIR)")
	root.AddCommand(newServeCmd(), newValidateCmd(), newPublishCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "bgtutor:", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
