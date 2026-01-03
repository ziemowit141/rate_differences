# Citibank PDF to MT940 + MT940 Viewer

Convert Citibank PDF statements to MT940, inspect MT940 files, and view transactions via a Go API + React UI.

## Requirements

- `swift` with PDFKit available (macOS).
- Python 3.
- For `main.py`, install the MT940 parser:
  - `python3 -m pip install mt-940`
- Go 1.22+ (for the API).
- Node 16+ (for the React UI).

## Convert PDF to MT940

```bash
python -m pdftomt940 --statement cmd/mt940api/data/statements/statement.pdf
```

Write to a file:

```bash
python -m pdftomt940 --statement cmd/mt940api/data/statements/statement.pdf --output cmd/mt940api/data/mt940s/bank.mt940
```

## Read MT940 summary

```bash
python main.py --statement cmd/mt940api/data/mt940s/bank.mt940
```

## Go API

Start the API:

```bash
go run ./cmd/mt940api --addr :8080
```

Fetch transactions (default reads `cmd/mt940api/data/mt940s/*.mt940`):

```bash
curl -s http://localhost:8080/transactions
```

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
