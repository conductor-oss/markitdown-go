# REQUIREMENTS.md

## Functional requirements
- Convert common documents to Markdown:
  - PDF (text + basic tables)
  - DOCX (OOXML)
  - PPTX (OOXML)
  - XLSX (OOXML)
  - XLS (legacy binary)
  - HTML, RSS/Atom, CSV, plain text, ZIP of supported files
- Pure Go only: no Python or external runtimes.
- Feature parity with the reference implementation where feasible.
- Deterministic output suitable for golden tests.
- Output must be valid Markdown text (UTF-8). Strip non-printable/control characters and avoid raw/binary output.
- If extraction produces unreadable/garbled text, return a clear Markdown placeholder message instead of raw output.
- PDF extraction should apply heuristics to repair spacing and common simple cipher encodings when detected.
- PDF extraction must skip pages containing images (no OCR, no image processing).
- Optional PDFium-backed PDF extraction (pure-Go WebAssembly) should be available via build tags to improve text fidelity.

## Non-functional requirements
- Target platforms: Linux (amd64/arm64), macOS (amd64/arm64), Windows (amd64 best effort).
- No download-on-first-use; all required assets are shipped with the library.
- Prefer small, pure-Go dependencies; avoid CGO where possible.

## Dependency policy
- Pure-Go only; avoid CGO and external runtimes.
- New dependencies must be:
  - Actively maintained or stable and low-risk.
  - Small in scope and narrowly used.
  - Compatible with target platforms (Linux/macOS/Windows).
- Avoid transitive bloat; prefer stdlib or existing deps.
- If a feature requires large assets/binaries, embed at build time (no download-on-first-use).
- Document tradeoffs when adding unavoidable dependencies.

## Release process
- Bump version and update `CHANGELOG.md` (if present).
- Run `go test ./...` and ensure goldens pass.
- Verify CLI smoke tests on Linux/macOS; Windows best effort.
- Tag release in git and publish artifacts (if applicable).
- If adding/altering dependencies, run `go mod tidy` and review diffs.

## Compatibility matrix
| OS      | Arch   | Status      | Notes |
|---------|--------|-------------|-------|
| Linux   | amd64  | Supported   | Primary target |
| Linux   | arm64  | Supported   | Primary target |
| macOS   | amd64  | Supported   | Primary target |
| macOS   | arm64  | Supported   | Primary target |
| Windows | amd64  | Best effort | CLI + core library |

## CLI requirements
- Provide a simple CLI to convert files/dirs/globs/URIs.
- Support recursive directory traversal and output to file or directory.
