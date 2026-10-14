/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { existsSync, readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

// Host-side only (bun run dev/build, not the Docker build - see
// docs/architecture/environments.md's note on Dockerfiles never touching
// secrets/ directly). Mirrors how docker-compose's env_file reads
// secrets/backend.env for the backend; Vite doesn't look outside its own
// directory for env files by default, so this reads the one place
// secrets/frontend.env actually lives (repo root, not services/frontend/).
// package.json is "type": "module" - no __dirname here, derive it instead.
const here = dirname(fileURLToPath(import.meta.url));
const secretsEnvPath = resolve(here, "../../secrets/frontend.env");
if (existsSync(secretsEnvPath)) {
  for (const line of readFileSync(secretsEnvPath, "utf-8").split("\n")) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const [key, ...rest] = trimmed.split("=");
    if (key && process.env[key] === undefined) {
      process.env[key] = rest.join("=");
    }
  }
}

export default defineConfig({
  plugins: [react()],
  server: {
    // 7373, not Vite's 5173 default - chosen specifically to dodge
    // collisions with other projects' dev servers on this machine (5173 is
    // Vite's default for everyone, so any other Vite-based repo running
    // locally at the same time contends for it). Also hardcoded into
    // internal/accounts/redirect.go's OAuth redirect_uri allowlist, and
    // needs registering as a callback URL in the Google/GitHub OAuth app
    // consoles (see secrets/backend.env's own comments) - not something
    // this repo/tooling can do for you. PORT overrides it only so a second
    // concurrent dev server (e.g. another agent session on the same repo) can
    // bind a free port instead of colliding here; unset, it stays 7373 and
    // OAuth keeps working.
    port: Number(process.env.PORT) || 7373,
    strictPort: true,
    // Local dev only - in-cluster this is Traefik routing, not a Vite proxy.
    // ws: true matters here specifically for the ttyd terminal proxy under
    // /api/v1/sessions/{id}/terminal - that path upgrades to a websocket.
    proxy: {
      "/api": { target: "http://localhost:7374", ws: true },
      // Admin dashboard's reverse-proxied Grafana/ArgoCD links (see
      // internal/api/adminproxy.go) - same split as /api, Traefik does
      // this routing in-cluster instead. Deliberately these exact
      // sub-paths, not a blanket "/admin" - that would also swallow the
      // frontend's own client-side /admin route (the dashboard page
      // itself) before React Router ever sees it.
      "/admin/grafana": { target: "http://localhost:7374" },
      "/admin/argocd": { target: "http://localhost:7374" },
      // Vault: /ui and /v1 are Vault's own hardcoded paths, not ours -
      // see internal/api/routes.go for why it's mounted here instead of
      // under /admin/vault.
      "/ui": { target: "http://localhost:7374" },
      "/v1": { target: "http://localhost:7374" },
      // Swagger UI + the per-caller filtered spec it fetches (see
      // internal/api/apidocs.go). Settings links here, so without this entry
      // the link hits Vite instead of the backend and dead-ends. In-cluster
      // the equivalent is the backend chart's ingress.userPaths - the same
      // list expressed twice, once per environment, so a path added to one
      // needs adding to the other.
      "/swagger": { target: "http://localhost:7374" },
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
  },
});
