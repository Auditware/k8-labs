# syntax=docker/dockerfile:1

FROM oven/bun:1-alpine AS build
WORKDIR /app
COPY package.json bun.lockb* ./
RUN bun install --frozen-lockfile
COPY . .
# Vite bakes VITE_* vars into the bundle at build time, not read at container
# start - see docs/architecture/environments.md. This Dockerfile only knows
# about generic build args, same as any other; it doesn't read secrets/
# itself (that convention holds - see secrets/README.md). Whoever invokes
# `docker build` supplies the values (secrets/frontend.env locally, CI's own
# secret store in the real pipeline):
#   docker build --build-arg VITE_GOOGLE_CLIENT_ID=... --build-arg VITE_GITHUB_CLIENT_ID=...
ARG VITE_GOOGLE_CLIENT_ID
ARG VITE_GITHUB_CLIENT_ID
ARG VITE_POSTHOG_PROJECT_TOKEN
ARG VITE_POSTHOG_HOST
ENV VITE_GOOGLE_CLIENT_ID=$VITE_GOOGLE_CLIENT_ID
ENV VITE_GITHUB_CLIENT_ID=$VITE_GITHUB_CLIENT_ID
ENV VITE_POSTHOG_PROJECT_TOKEN=$VITE_POSTHOG_PROJECT_TOKEN
ENV VITE_POSTHOG_HOST=$VITE_POSTHOG_HOST
RUN bun run build

# Same Bun runtime serves the built static bundle - no nginx, one fewer
# runtime image to patch, consistent with the rest of the repo's
# self-host-and-own-it choices.
FROM oven/bun:1-alpine AS final
WORKDIR /app
RUN addgroup -S app && adduser -S app -G app
COPY --from=build /app/dist ./dist
COPY --from=build /app/serve.ts ./serve.ts
USER app
EXPOSE 8080
CMD ["bun", "run", "serve.ts"]
