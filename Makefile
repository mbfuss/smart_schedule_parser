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
	docker compose -f docker/docker-compose.yml build --no-cache

.PHONY: docker-up
docker-up:
	docker compose -f docker/docker-compose.yml up --build

.PHONY: docker-up-d
docker-up-d:
	docker compose -f docker/docker-compose.yml up --build -d

.PHONY: docker-down
docker-down:
	docker compose -f docker/docker-compose.yml down

.PHONY: docker-down-v
docker-down-v:
	docker compose -f docker/docker-compose.yml down -v

# Tools
GO_TOOL_ENTRY = go tool -modfile=tools/go.mod

.PHONY: lint
lint: go-check
	${GO_TOOL_ENTRY} golangci-lint run ./...
