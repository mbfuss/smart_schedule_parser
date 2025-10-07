.PHONY: go-check
go-check:
	@GO_VERSION=$$(go version | awk '{print $$3}' | sed 's/go//'); \
	case $$GO_VERSION in \
		1.25.*) echo "Go version $$GO_VERSION OK";; \
		*) echo "ERROR: Go version must be 1.25.X, found $$GO_VERSION" && exit 1;; \
	esac

.PHONY: run
run: go-check
	go run cmd/parser/main.go

.PHONY: test
test: go-check
	go test -v ./...

.PHONY: docker-build
docker-build:
	docker compose -f docker/docker-compose.yml build

.PHONY: docker-up
docker-up:
	docker compose -f docker/docker-compose.yml up

.PHONY: docker-down
docker-down:
	docker compose -f docker/docker-compose.yml down

# Tools
GO_TOOL_ENTRY = go tool -modfile=tools/go.mod

.PHONY: lint
lint: go-check
	${GO_TOOL_ENTRY} golangci-lint run ./...
