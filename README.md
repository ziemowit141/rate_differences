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

The backend now converts PDFs with a Go parser and a Swift extractor. If the extractor binary is missing, it will compile `cmd/mt940api/tools/extract.swift` using `swiftc` and store the binary in `cmd/mt940api/bin/extract`.

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
