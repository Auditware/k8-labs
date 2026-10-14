# Contributing to K8 Labs

Thanks for your interest. New labs, bug fixes, and documentation improvements are all welcome.

## Getting set up

Follow the local-development section of [docs/self-hosting.md](docs/self-hosting.md). The
docker-compose path is enough for most backend and frontend work; the Tilt + k3d path is needed to
exercise lab session provisioning.

## Ways to contribute

- **New labs.** This is the most valuable contribution. A lab is a manifest bundle plus a guided
  course, not a container image. Start with [services/labs/AUTHORING.md](services/labs/AUTHORING.md)
  and the template in [`services/labs/_template/`](services/labs/_template/). A good lab teaches one
  specific misconfiguration or technique, ends in a captured flag, and explains what it proves.
- **Bug fixes and features** in the backend (Go), frontend (React/TypeScript), or the platform charts.
- **Docs** - the architecture notes in [docs/architecture/](docs/architecture/), this guide, or the
  per-lab write-ups.

## Before you open a PR

Run the same checks CI runs (see [.github/workflows/ci.yml](.github/workflows/ci.yml)):

```bash
# backend
cd services/backend && go build ./... && go vet ./... && go test ./...

# frontend
cd services/frontend && bun install && bun run build && bun run test

# charts
helm lint apps/backend/
```

The backend tests include a content gate: every course lesson is validated against
`services/labs/schema/step.schema.json`, so a malformed lesson fails the build rather than reaching a
student as a blank pane.

## Guidelines

- Keep changes focused and match the style of the surrounding code.
- A new lab should come with its course content and a short README explaining the lesson.
- Never commit real secrets. Config and secrets are templated under [`secrets/`](secrets/); only the
  `*.example` files are committed.
- Labs are intentionally vulnerable by design. If you think you have found a vulnerability in the
  **platform itself** (not a lab), see [SECURITY.md](SECURITY.md) instead of opening a public issue.

## License

By contributing, you agree that your contributions are licensed under the [MIT License](LICENSE).
