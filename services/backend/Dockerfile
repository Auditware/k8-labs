# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
# GIT_SHA is passed by CI (--build-arg GIT_SHA=<short sha>) and stamped into the
# binary so /healthz reports which commit is running; defaults to "dev" for a
# plain local `docker build`. See internal/buildinfo.
ARG GIT_SHA=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags "-X backend/internal/buildinfo.SHA=${GIT_SHA}" \
      -o /out/backend ./cmd/server

# distroless nonroot: no shell, no package manager - smallest attack surface
# for a pod that also has to satisfy the "restricted" PodSecurity profile.
# Runs as non-root by default, nothing to configure.
FROM gcr.io/distroless/static-debian12:nonroot AS final
COPY --from=build /out/backend /backend
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/backend"]
