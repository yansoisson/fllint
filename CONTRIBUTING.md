# Contributing to Fllint

Thanks for your interest in contributing to Fllint!

## Getting Started

1. Fork and clone the repository
2. Install dependencies: Go 1.25+, Node.js 20+
3. Run the dev servers: `make dev`
4. Open `http://localhost:5173` in your browser

See `CLAUDE.md` for detailed architecture documentation.

## Development

```bash
make dev            # Start Go backend (:8420) + Vite frontend (:5173)
make build          # Full production build
make test           # Run Go tests
make fmt            # Format Go code
```

After making frontend changes, you may need to clear the Vite cache:
```bash
rm -rf frontend/node_modules/.vite
```

## Pull Requests

- Keep PRs focused on a single change
- Test on macOS with Apple Silicon before submitting
- If adding a new API endpoint, update the API Routes section in `CLAUDE.md`
- Version bumps and tagging are handled by maintainers

## Key Conventions

- **Go**: Thread-safe access with `sync.RWMutex`, no `middleware.Timeout` (breaks SSE)
- **Svelte 5**: Use runes (`$state`, `$derived`, `$effect`), `$props()` not `export let`, `onclick={}` not `on:click={}`
- **Files using runes outside components** must use `.svelte.ts` extension
- **Single-folder principle**: Never write files outside the app's data directory

## Reporting Issues

Open an issue on GitHub with:
- What you expected to happen
- What actually happened
- Your macOS version and Mac model
- Steps to reproduce
