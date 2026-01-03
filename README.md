# Citibank PDF to MT940 + MT940 Viewer

Convert Citibank PDF statements to MT940, inspect MT940 files, and view transactions via a Go API + React UI.

## Requirements

- `swift` with PDFKit available (macOS).
- Go 1.22+ (for the API).
- Node 16+ (for the React UI).

## Convert PDF to MT940

The backend now converts PDFs using Go + a Swift extractor binary. You can trigger it by uploading PDFs or by calling the API.

## Go API

Start the API:

```bash
go run ./cmd/mt940api --addr :8080
```

The backend now converts PDFs with a Go parser and a Swift extractor. The Swift source is embedded (`backend/tools/extract.swift`) and compiled on demand into the user cache directory.

Data files are stored in:
- `${RATE_DIFF_HOME}/statements` and `${RATE_DIFF_HOME}/mt940s` if `RATE_DIFF_HOME` is set
- otherwise `~/Library/Caches/rate_differences/data/...` on macOS

Fetch transactions (default reads `cmd/mt940api/data/mt940s/*.mt940`):

```bash
curl -s http://localhost:8080/transactions
```

The API response now includes NBP USD rate (`nbp_rate`) for the statement date (requires internet access).

Upload PDFs (multipart form field `files[]`):

```bash
curl -s -X POST http://localhost:8080/upload \
  -F 'files=@/path/to/statement1.pdf' \
  -F 'files=@/path/to/statement2.pdf'
```

Delete a statement by base name:

```bash
curl -s -X DELETE http://localhost:8080/files/2025-01-07
```

Calculate report (send tranches manually):

```bash
curl -s -X POST http://localhost:8080/calculate \
  -H 'Content-Type: application/json' \
  -d '{"tranches":[{"date":"2025-01-02","amount":10000,"rate":4.12}]}'
```

## React UI

```bash
cd frontend
npm install
npm run dev
```

The UI is served by Vite (usually `http://localhost:5173`) and proxies to the Go API.

## Run both (dev)

```bash
make dev
```

## Wails (macOS app)

Build an Intel macOS app:

```bash
make wails-build
```

The `.app` bundle will be created under `wails/RateDifferences/build/bin/`.

If the build output is stale, run:

```bash
make wails-rebuild
```

The Wails build bundles a precompiled Swift extractor in the app resources. On first run, the backend copies it into the user cache so no Swift toolchain is required at runtime.

### GitHub Actions (Intel build)

There is a workflow that builds the Intel macOS app on `macos-13` and uploads a zip:

`.github/workflows/wails-macos-intel.yml`
