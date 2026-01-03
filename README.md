# Citibank PDF to MT940

Convert Citibank PDF statements to MT940 and inspect MT940 files.

## Requirements

- `swift` with PDFKit available (macOS).
- Python 3.
- For `main.py`, install the MT940 parser:
  - `python3 -m pip install mt-940`

## Convert PDF to MT940

```bash
python -m pdftomt940 --statement statement.pdf
```

Write to a file:

```bash
python -m pdftomt940 --statement statement.pdf --output bank.mt940
```

## Read MT940 summary

```bash
python main.py --statement bank.mt940
```
