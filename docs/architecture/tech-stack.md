# Technology stack

Picked for the same reasons as the infra choices elsewhere in this repo: mainstream over trendy, own the code over depend on a service, and a clear deferred-upgrade path rather than building for scale you don't have yet.

## Backend (`services/backend/`)

| Concern | Choice | Why |
|---|---|---|
| Language | Go | given |
| HTTP router | stdlib `net/http` (Go 1.22+ `ServeMux`) | Method+path routing is good enough natively now - no framework dependency to outgrow. Reach for `chi` only if middleware ergonomics genuinely become a problem. |
| Kubernetes client | `client-go` (official) | Provisions/tears down tenant microVMs, reads the labs catalog from cluster state. |
| Auth | `golang-jwt/jwt/v5` | Stateless JWT per the existing MVP call - no DB. |
| Logging | `log/slog` (stdlib) | Structured JSON logs, zero dependency, pairs directly with Alloy/Loki. |
| Testing | stdlib `testing` + `testify` | Standard pairing, no exotic runner. |
| Layout | `cmd/`, `internal/`, `pkg/` | Standard Go project layout. |
| Database | **Postgres**, via `pgx` (no ORM, raw SQL). Self-hosted on the System VM, reusing the storage class already set up for Vault's Raft storage. Backs the module catalog, auth, and session history/dashboard data. |

## Frontend (`services/frontend/`)

| Concern | Choice | Why |
|---|---|---|
| Language/runtime | TypeScript + Bun | given |
| Build tool | Vite | given |
| UI framework | **React** | Largest ecosystem, most battle-tested - same "mainstream over trendy" call as nginx-ingress over Traefik elsewhere in this repo. |
| Styling | Tailwind CSS | Fast to build with, standard Vite pairing. |
| Components | shadcn/ui (Radix primitives) | Code is copied into the repo, not installed as an opaque dependency - fits "own it, don't depend on it." |
| Data fetching/cache | TanStack Query | Handles the catalog/dashboard/session data fetching cleanly. |
| Global state | Zustand | Lightweight - auth/session state doesn't need Redux's boilerplate. |
| Terminal rendering | xterm.js | The standard client for a ttyd websocket stream. |
| Live telemetry | **Server-Sent Events**, not WebSocket | One-directional (server → browser), simpler than WS, built-in browser auto-reconnect, survives Cloudflare's proxy more reliably. WebSocket stays scoped to ttyd's terminal, which genuinely needs bidirectional. |
| Testing | Vitest | Native Vite pairing, shares config. |

## API contract (backend ↔ frontend)

**REST + JSON**, not GraphQL or gRPC - simplest thing that works at this scale. The OpenAPI spec itself now exists - swaggo/swag, generated from `@`-annotations above each handler in `internal/api` (`make swagger` in `services/backend`, see its own comment for why this doesn't run automatically), served at `/api/v1/docs/` (Swagger UI) and `/api/v1/docs/doc.json` (raw spec). Framework-agnostic despite the name - swaggo's annotation/generation engine doesn't care that this is stdlib `net/http`, not Gin/Echo/chi; only the tiny UI-serving handler (`github.com/swaggo/http-swagger/v2`) is genuinely a stdlib-native alternative to that ecosystem's Gin-specific `gin-swagger` package. Generating TypeScript client types from this spec (`openapi-typescript` or similar) is still the open follow-up - the frontend still hand-writes its `fetch` calls in `lib/api.ts`.

If stronger typed contracts are wanted later, **Connect** (connectrpc.com) is the upgrade path - gRPC-like codegen for both Go and TypeScript, works natively over plain HTTP/JSON, no gRPC-Web workarounds. Not needed now.

## Labs (`services/labs/`)

**Deliberately not one stack.** Each training module is its own vulnerable app, and the whole point is exposing real vulnerability classes across different real-world ecosystems - a Node/Express app for a prototype-pollution lesson, a Python/Flask app for SSRF, a Go app for a path-traversal bug, etc. Pick the language that fits what the module is teaching, not a house standard. `_template/` is a starting point, not a language mandate.

## Container builds

Dockerfile-based for everything, uniformly - works the same for the backend, frontend, and every lab regardless of its language. Optional later: **`ko`** for the Go backend specifically (builds a container image directly from Go source, no Dockerfile) - a lean upgrade, not required for MVP.
