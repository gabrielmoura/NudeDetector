# Repository Guidelines

## Project Structure & Module Organization

Go 1.26 API for image-pornography classification with ONNX Runtime. `pornography_score` is the `porn` class probability; keep inference and moderation policy separate.

- `cmd/api/main.go` wires configuration, dependencies, and the HTTP server.
- `internal/` holds focused packages: `api/`, `inference/`, `moderation/`, `downloader/`, `imageprocessor/`, and `observability/`.
- `models/model.onnx` and `models/model.json` are runtime artifacts. Treat them as versioned inputs; change them together and document their provenance.
- `ref/NudeNet/` is upstream reference material, not service code.

## Build, Test, and Development Commands

Run commands from the repository root:

```bash
go build -o nude-detector ./cmd/api   # build the API binary
go test ./...                         # run all Go tests
go test -race ./...                   # check tests for data races
go vet ./...                          # run static checks
docker build -t nude-detector .       # build the production image
```

Start locally with explicit model paths, for example:

```bash
ONNXRUNTIME_SHARED_LIBRARY_PATH=/path/to/libonnxruntime.so.1.22.0 ./nude-detector
```

The service exposes port `8080`; use `/health`, `/ready`, and `/metrics` locally. Before a live `curl` test, ensure ONNX Runtime supports the committed model. The last run used Runtime model versions 1–14 with a version-22 model and failed; a Go build does not validate inference. No `Makefile` or committed tests exist.

## Coding Style & Naming Conventions

Format Go code with `gofmt` (tabs). Use `PascalCase` for exports, `camelCase` otherwise, and short lowercase package names. Prefer small functions, early returns, wrapped errors, named struct fields, initialized serialized collections, and `context.Context` first.

## Testing Guidelines

Place unit tests beside code as `*_test.go`; name them `Test<Behavior>` (for example, `TestClassifierRejectsOversizedImage`). Cover unsafe inputs, metadata, scores, thresholds, queue-full `503`, and both request formats. Use table-driven tests; run `go test -race ./...` and `go vet ./...` before review.

## Commit & Pull Request Guidelines

There are no existing commits to derive a convention from. Use short, imperative, scoped subjects, such as `api: validate image content type`. Keep commits focused. PRs explain behavior and security impact, list validation, link issues, and include API examples. Never commit secrets, user images, or model binaries without provenance and licensing.
