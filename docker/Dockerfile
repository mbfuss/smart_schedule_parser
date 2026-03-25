FROM golang:1.25-bookworm AS backend

WORKDIR /app

# 1. deps — rebuilt only when go.mod / go.sum change
COPY ./go.mod ./go.sum ./
RUN go mod download

# 2. source — ordered from least to most frequently changed
COPY ./pkg ./pkg
COPY ./internal ./internal
COPY ./cmd ./cmd

RUN go build -o parser_server ./cmd/parser/main.go

FROM python:3.10-slim AS final

RUN pip install --no-cache-dir pandas pdfplumber

WORKDIR /app

# binary from build stage
COPY --from=backend /app/parser_server ./

# scripts and config come straight from the build context —
# changes here do not invalidate the Go build cache above
COPY ./scripts ./scripts
COPY ./.env ./

CMD ["/app/parser_server"]