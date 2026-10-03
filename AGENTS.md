# bgtutor-mcp

Standalone Go MCP server for Bulgarian podcast lessons. The binary is named
`bgtutor`; the Go module is `github.com/snonux/bgtutor-mcp`.

## Development

- Build: `go build -o bgtutor ./cmd/bgtutor` (or `task`).
- Test: `go test -race ./...` (or `task test`).
- Validate citizenship assets: `go run ./cmd/bgtutor validate --mode citizenship --data-dir ../bgtutor-assets`.
- Validate episodes: `go run ./cmd/bgtutor validate`.
- Run locally: `go run ./cmd/bgtutor serve`.
- Build container: `docker build -t bgtutor:0.30.0 .`.

The default library is `data/`; override it with `--data-dir` or
`BGTUTOR_DATA_DIR`. Set `BGTUTOR_TOKEN` to listen beyond localhost.

The Taskfile uses Go Task. On systems where `task` runs Taskwarrior, use
`go-task`, `go-task test`, and `go-task validate` instead.

## Layout

- `cmd/bgtutor/`: serve, validate, and publish commands.
- `internal/bgtutor/`: episode library, citizenship lessons/exams/progress, and vocabulary notebook.
- `CITIZENSHIP.md`: citizenship teaching plan, MCP tools and asset format.
- `internal/bgtutor/mcpserver/`: MCP tools and HTTP transport.
- `internal/version.go`: CLI and server release version.
- `data/episodes/001-cooking-basics/`: sample episode used by tests.
- `examples/`: sample source transcript.

## Episode preparation

Follow `PREPARE.md` (Claude Code: `.claude/skills/bgtutor-prepare/SKILL.md`).
The agent writes the episode itself; no LLM API is involved. `FORMAT.md`
defines the data format. Validate and publish before serving an episode.
Prepared real episodes are committed to https://github.com/snonux/bgtutor-assets
under `episodes/`; only the `001-cooking-basics` sample belongs here.
The learner's notebook under `data/vocabulary/` is ignored by Git.

## Code guidelines

Update comments alongside code so they explain current behavior and reasoning.
When a function reaches 50 lines, try to split it into functions around 30 lines.
Keep the CLI small and place reusable behavior in `internal/`.
Use a feature branch, update documentation, and run the full test suite.
