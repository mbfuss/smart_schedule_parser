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
