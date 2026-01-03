from __future__ import annotations

import argparse
import sys
from pathlib import Path

from . import PdfExtractionError, convert_pdf_to_mt940


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Konwersja wyciągu Citibank (PDF) do formatu MT940."
    )
    parser.add_argument(
        "--statement",
        required=True,
        type=Path,
        help="Ścieżka do pliku PDF z wyciągiem.",
    )
    parser.add_argument(
        "--output",
        type=Path,
        help="Opcjonalna ścieżka zapisu wygenerowanego pliku MT940. Domyślnie wypisuje na stdout.",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()

    try:
        mt940_payload = convert_pdf_to_mt940(args.statement)
    except (FileNotFoundError, PdfExtractionError, ValueError) as error:
        print(f"Błąd: {error}", file=sys.stderr)
        sys.exit(1)

    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(mt940_payload, encoding="utf-8")
        print(f"Zapisano wynik MT940 do pliku: {args.output}")
    else:
        print(mt940_payload)


if __name__ == "__main__":
    main()
