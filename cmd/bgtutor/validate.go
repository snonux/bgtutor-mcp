package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/snonux/bgtutor-mcp/internal/bgtutor"
)

func newValidateCmd() *cobra.Command {
	var mode string
	cmd := &cobra.Command{
		Use:   "validate [ID...]",
		Short: "Check podcast episodes or citizenship tests (all if no IDs given)",
		RunE: func(cmd *cobra.Command, ids []string) error {
			switch mode {
			case "podcast":
				return validateEpisodes(cmd, ids)
			case "citizenship":
				return validateCitizenship(cmd, ids)
			default:
				return fmt.Errorf("unknown mode %q; choose podcast or citizenship", mode)
			}
		},
	}
	cmd.Flags().StringVar(&mode, "mode", envOr("BGTUTOR_MODE", "podcast"), "library to validate: podcast or citizenship (env BGTUTOR_MODE)")
	return cmd
}

func validateEpisodes(cmd *cobra.Command, ids []string) error {
	eps, err := loadEpisodes(cmd, ids)
	if err != nil {
		return err
	}
	bad := 0
	for _, ep := range eps {
		if !reportEpisode(ep) {
			bad++
		}
	}
	if bad > 0 {
		return fmt.Errorf("%d of %d episodes have problems", bad, len(eps))
	}
	return nil
}

func validateCitizenship(cmd *cobra.Command, ids []string) error {
	dataDir, _ := cmd.Flags().GetString("data-dir")
	lib := bgtutor.NewCitizenshipLibrary(filepath.Join(dataDir, "citizenship-test"))
	catalog, err := lib.Catalog()
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		for _, test := range catalog.Tests {
			ids = append(ids, test.ID)
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("no citizenship tests found")
	}
	bad := 0
	for _, id := range ids {
		test, err := lib.Test(id)
		if err != nil {
			cmd.Printf("INVALID %s: %v\n", id, err)
			bad++
			continue
		}
		cmd.Printf("ready   %s (%d questions)\n", id, len(test.Questions))
	}
	if bad > 0 {
		return fmt.Errorf("%d of %d citizenship tests have problems", bad, len(ids))
	}
	cmd.Printf("%d study documents available\n", len(catalog.Materials))
	return nil
}

func newPublishCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "publish EPISODE_ID",
		Short: "Validate a draft episode and mark it ready, so the MCP server serves it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !bgtutor.ValidEpisodeID(args[0]) {
				return fmt.Errorf("invalid episode id %q", args[0])
			}
			ep, err := bgtutor.Publish(filepath.Join(episodesDir(cmd), args[0]))
			if err != nil {
				reportEpisode(ep)
				return err
			}
			fmt.Printf("published %s (%d paragraphs)\n", ep.ID, len(ep.Paragraphs))
			return nil
		},
	}
}

// loadEpisodes loads the named episodes, or all of them when ids is empty.
func loadEpisodes(cmd *cobra.Command, ids []string) ([]*bgtutor.Episode, error) {
	dir := episodesDir(cmd)
	if len(ids) == 0 {
		return bgtutor.NewLibrary(dir).Episodes()
	}
	eps := make([]*bgtutor.Episode, 0, len(ids))
	for _, id := range ids {
		if !bgtutor.ValidEpisodeID(id) {
			return nil, fmt.Errorf("invalid episode id %q", id)
		}
		eps = append(eps, bgtutor.LoadEpisode(filepath.Join(dir, id)))
	}
	return eps, nil
}

// reportEpisode prints one status line plus every content error, and
// reports whether the episode's content is valid.
func reportEpisode(ep *bgtutor.Episode) bool {
	switch {
	case len(ep.Errors) > 0:
		fmt.Printf("INVALID %s\n", ep.ID)
		for _, e := range ep.Errors {
			fmt.Printf("  - %s\n", e)
		}
		return false
	case ep.Ready():
		fmt.Printf("ready   %s (%d paragraphs)\n", ep.ID, len(ep.Paragraphs))
	default:
		fmt.Printf("valid   %s (%d paragraphs, status %q; run bgtutor publish %s)\n",
			ep.ID, len(ep.Paragraphs), ep.Meta.Status, ep.ID)
	}
	return true
}

func episodesDir(cmd *cobra.Command) string {
	dataDir, _ := cmd.Flags().GetString("data-dir")
	return filepath.Join(dataDir, "episodes")
}
