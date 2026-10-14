.PHONY: run build test tidy swagger

run:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -o bin/backend ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy

# Regenerates docs/ from the @-annotations above each handler in
# internal/api - nothing does this automatically, same "no real migration
# tool yet" caveat as internal/db/schema.go. Committed (not gitignored):
# cmd/server/main.go blank-imports the generated package, so a fresh
# clone needs it to even compile, not just to browse /api/v1/docs/.
swagger:
	go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/server/main.go -o docs
