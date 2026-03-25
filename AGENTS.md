# AGENTS.md

## What this repo does
- `smart_schedule_parser` is an HTTP service that turns a schedule website into nested JSON: `Building -> Institute -> StudyForm -> Group -> Schedule`.
- The only public route is `GET /getschedule` in `internal/handlers/handlers.go`; it expects `urlSchedule` **without** a scheme, because `internal/provider/provider.go` prepends `"https://"`.
- Example request from `README.md`: `http://localhost:8080/getschedule?urlSchedule=www.vavilovsar.ru/ucheba/raspisanie-zanyatii`.

## Runtime architecture
- Entry point: `cmd/parser/main.go` initializes logging to both stdout and `log/app.log`, builds the DI container, then starts `net/http`.
- Wiring lives in `internal/di/di.go`: config load -> `crawler.NewCrawler()` -> `parser.NewPDFParser("scripts/pdf2csv.py")` -> `provider.NewProvider(...)` -> handler registration.
- Request flow is: handler -> provider -> crawler downloads PDFs into `OUTPUT_DIR` -> provider walks PDFs and parses them concurrently -> provider groups results into `internal/resource` structs -> JSON response.
- `internal/provider/provider.go` is the orchestration layer; read it before changing request semantics, concurrency, or output structure.

## Critical repo-specific behaviors
- Every `GetBuilding` call starts by deleting all contents of `OUTPUT_DIR` via `clearDir(...)`. Do not assume `pdf_list/` is persistent across requests.
- The crawler is tightly coupled to the Vavilov site structure: `internal/crawler/crawler.go` filters links by `/uk`, `/institut`, and `forma-obucheniya`, and prefixes PDF/institute links with `https://www.vavilovsar.ru`.
- PDF parsing is split across Go and Python: Go shells out to `python3 scripts/pdf2csv.py <pdf>` (`internal/parser/pdf.go`), and the Python script uses `pdfplumber` + `pandas`.
- The parser treats merged PDF table cells as the literal string `Merged`; that sentinel is produced by `scripts/pdf2csv.py` and consumed heavily in `internal/parser/pdf.go`.
- Week type logic is encoded in `resource.WeekType`; `nil` means the lesson applies to both weeks, not “unknown”.

## Key files to inspect before editing
- `internal/provider/provider.go` — request orchestration, directory cleanup, concurrent PDF parsing (`errgroup`, `runtime.NumCPU()`).
- `internal/crawler/crawler.go` — HTML traversal rules, file layout under `pdf_list/`, Vavilov-specific assumptions.
- `internal/parser/pdf.go` — table-to-schedule algorithm, time parsing, merged-cell handling, weekday assignment.
- `internal/resource/resource.go` — response schema and JSON tags; changing these affects the public API.
- `internal/parser/pdf_test.go` — the most valuable behavior spec; it uses a real fixture PDF from `test/pdf/`.

## Local workflows
- Required versions are enforced in `Makefile` / `.go-version`: Go `1.25.x` (repo currently pins `1.25.8`).
- Local run also requires `python3` plus `pandas` and `pdfplumber`, because the Go binary invokes the Python script at runtime.
- Main commands:
  - `make run`
  - `make test`
  - `make lint`
  - `make docker-build`
  - `make docker-up`
- Config comes from root `.env` via `internal/config/config.go`; required keys are `HOST`, `PORT`, `OUTPUT_DIR`. Missing `OUTPUT_DIR` is created automatically.

## Docker/integration notes
- `docker/Dockerfile` builds the Go binary in a Go image, then runs it in `python:3.10-slim` with Python deps installed.
- `docker/docker-compose.yml` mounts `../pdf_list` and `../log` into the container and rewrites `.env` so the server binds to `0.0.0.0`.
- If you move or rename `scripts/pdf2csv.py`, update the hardcoded path in `internal/di/di.go` and the test constant in `internal/parser/pdf_test.go`.

## Editing guidance for agents
- Preserve the folder-based metadata contract: provider derives `building/institute/form` from the relative PDF path under `OUTPUT_DIR`.
- Preserve the `urlSchedule` query parameter unless you intentionally change the public API in `handlers` and `README.md` together.
- When changing parser behavior, run `go test ./...` and pay special attention to `TestPDFParser_ParsePDF_Success`, `..._EmptyFile`, and `..._CanceledContext`.
- Keep logs and errors informative; this codebase already relies on `zerolog` and wrapped errors for cross-layer debugging.

