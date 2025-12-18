FROM golang:1.25-bookworm AS backend
WORKDIR /app
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY . .
RUN go build -o parser_server ./cmd/parser/main.go

FROM python:3.10-slim AS final

# Healthcheck tooling + HTTPS trust
RUN apt-get update \
  && apt-get install -y --no-install-recommends wget ca-certificates \
  && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir pandas pdfplumber

WORKDIR /app
COPY --from=backend /app/scripts /app/scripts
COPY --from=backend /app/parser_server /app/.env /app/

CMD ["/app/parser_server"]