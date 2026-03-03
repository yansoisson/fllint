# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Always take a look at README.md to build a deep understanding of the project and its purpose.

Remember that the 1 folder philosophy is essential, you should never forget that. And that the project isn't only for nerds, so the entire system should be extremely robust with a good UX (clean error messages for everything that can go wrong), never forget that too.

Push to github is already configured too (just use git push -u origin main to push after commit)

The project should respect the computer of the user. It should be possible to terminate all background processes by quitting the app. And on MacOS, the "Fllint" text on the bottom should disappear if the app isn't active anymore, and, as mentioned earlier, the one folder structure is essential. 

## Build & Development Commands

```bash
make build          # Full production build (frontend + Go binary, CGO_ENABLED=1)
make dev            # Run both dev servers concurrently (Vite :5173 + Go :8420)
make dev-frontend   # Vite dev server only (proxies /api to :8420)
make dev-backend    # Go server only with -tags dev (no frontend embed)
make test           # go test ./...
make fmt            # go fmt ./...
make clean          # Remove binary, frontend/build, .svelte-kit
make dist-macos     # Build macOS .app distribution → dist/Fllint/
make dist-clean     # Remove dist/ folder
```

Frontend-specific (from `frontend/`):
```bash
npm run dev         # Vite dev server on :5173
npm run build       # Production static build → build/
npm run check       # svelte-check type validation
```

## Architecture

Single-binary local AI chat app. Go backend serves a SvelteKit SPA embedded via `//go:embed all:frontend/build`.

### Backend (Go + chi)

- **Entry points**: `main.go` (prod, embeds frontend) / `main_dev.go` (dev, skips embed) — controlled by `//go:build dev` tag
- **Bootstrap**: `cmd/root.go` — resolves paths via `internal/paths`, loads config, creates managers, starts HTTP server, opens browser, runs systray on main goroutine (macOS AppKit requirement)
- **`internal/paths/`**: `Resolve()` returns `AppPaths{BinDir, DataDir, ModelsDir}`. Detection priority: env vars → macOS `.app` bundle → CWD defaults. Extensible for Linux AppImage.
- **`internal/llm/`**: `Engine` interface with `ChatStream` returning `<-chan Token`. `LlamaCppEngine` manages a `llama-server` child process and talks to it via OpenAI-compatible HTTP API (`/v1/chat/completions`). `Manager` handles model discovery (scans `modelsDir/` for `.gguf` files), engine lifecycle, and model switching with RWMutex. `StubEngine` kept for development.
- **`internal/chat/`**: SSE streaming handler (`http.Flusher`), conversation CRUD. `Store` persists conversations as individual JSON files in `{dataDir}/conversations/`.
- **`internal/server/`**: chi router, middleware stack, SPA fallback serving. No `middleware.Timeout` — it wraps ResponseWriter and breaks SSE Flusher.
- **`internal/config/`**: JSON config with env var overrides (`FLLINT_PORT`, `FLLINT_DATA_DIR`, `FLLINT_MODELS_DIR`)
- **`internal/image/`**: Multipart upload (10MB limit), UUID filenames, serves via `/api/uploads/*`
- **`internal/launcher/`**: `fyne.io/systray` (must run on main goroutine), platform-specific browser open

### Frontend (SvelteKit 2 + Svelte 5)

- **Adapter**: `adapter-static` in SPA mode (fallback: `index.html`, `ssr = false`)
- **State**: `lib/stores.svelte.ts` uses Svelte 5 runes (`$state`, `$effect`), exports getter/action functions
- **API client**: `lib/api.ts` — `streamChat()` uses `fetch` + `ReadableStream` async generator (not EventSource, which doesn't support POST)
- **Path aliases**: `$components` → `src/components`, `$lib` → `src/lib`
- **Vite proxy**: `/api` → `http://localhost:8420` in dev, with SSE-compatible headers

## Key Conventions

- **Svelte 5 syntax**: `$props()` not `export let`, `{@render children()}` not `<slot />`, `onclick={}` not `on:click={}`, no nested `<button>` elements
- **Rune files**: `.svelte.ts` extension required for files using runes outside components
- **Embed directive**: Must use `all:frontend/build` prefix (SvelteKit outputs `_app/` which Go embed skips without `all:`)
- **Thread safety**: Config, Manager, and Store all use `sync.RWMutex`
- **SSE protocol**: Server sends `data: {json}\n\n` tokens, `data: [DONE]\n\n` to signal completion

## API Routes

All under `/api/`:
- `GET/POST /conversations`, `GET/DELETE /conversations/{id}` — conversation CRUD
- `POST /chat` — SSE streaming chat endpoint (returns structured JSON errors with `{error, code}`)
- `GET /models`, `PUT /models/active`, `POST /models/refresh` — model management
- `GET /status` — engine status (`{engine_state, error, model_name, has_binary, has_models}`)
- `POST /image/upload`, `GET /uploads/*` — image handling
- `GET/PUT /config` — app configuration

## LLM Backend (llama.cpp)

- **Binary**: `bin/llama-server` — user-provided, discovered on startup
- **Models**: `models/*.gguf` — scanned on startup and via `/api/models/refresh`
- **Engine state machine**: `idle → starting → ready → error/stopping`
- **Process management**: child process with SIGTERM→SIGKILL shutdown, health poll via `GET /health`, crash detection
- **Streaming**: POST to `/v1/chat/completions` with `stream: true`, parse OpenAI SSE format
- **Port**: OS-assigned random available port per engine instance

## Environment Variables

- `FLLINT_PORT` (default: 8420)
- `FLLINT_DATA_DIR` (default: ./data, or auto-detected from .app bundle)
- `FLLINT_MODELS_DIR` (default: ./models, or auto-detected from .app bundle)
- `FLLINT_BIN_DIR` (default: ./bin, or auto-detected from .app bundle)

## macOS Distribution

`make dist-macos` produces a `dist/Fllint/` folder:

```
Fllint/
  Fllint.app/             ← Double-click to launch
    Contents/
      Info.plist
      MacOS/fllint        ← Go binary
      Resources/
        icon.icns
        bin/llama-server  ← Bundled inference server
  Data/
    models/               ← Pre-bundled and user-added models
    conversations/        ← Chat history
```

Path resolution (`internal/paths/`) auto-detects the `.app` bundle and resolves all paths relative to the `Fllint/` folder. Env vars override auto-detection for dev/debugging. The `packaging/macos/` directory contains `Info.plist` and `build-app.sh`.
