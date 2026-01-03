from __future__ import annotations

import os
import shutil
import subprocess
from pathlib import Path


class PdfExtractionError(RuntimeError):
    """Raised when text extraction from PDF fails."""


def extract_text_lines(pdf_path: Path) -> list[str]:
    if not pdf_path.is_file():
        raise FileNotFoundError(f"Brak pliku PDF: {pdf_path}")

    swift_path = shutil.which("swift")
    if swift_path is None:
        raise PdfExtractionError("Polecenie `swift` nie jest dostępne w systemie – wymagane do odczytu PDF.")

    script_path = Path(__file__).with_name("extract.swift")
    if not script_path.is_file():
        raise PdfExtractionError(f"Nie znaleziono skryptu pomocniczego: {script_path}")

    cache_root = script_path.parent / ".swift-cache"
    swift_cache = cache_root / "swift"
    clang_cache = cache_root / "clang"
    swift_cache.mkdir(parents=True, exist_ok=True)
    clang_cache.mkdir(parents=True, exist_ok=True)

    env = os.environ.copy()
    env["SWIFT_MODULE_CACHE_PATH"] = str(swift_cache.resolve())
    env["CLANG_MODULE_CACHE_PATH"] = str(clang_cache.resolve())

    try:
        result = subprocess.run(
            [swift_path, str(script_path), str(pdf_path)],
            check=True,
            capture_output=True,
            text=True,
            env=env,
        )
    except subprocess.CalledProcessError as exc:
        raise PdfExtractionError(
            f"Nie udało się wyodrębnić tekstu z PDF (`swift` zakończył się kodem {exc.returncode})."
        ) from exc

    return [line.rstrip("\n") for line in result.stdout.splitlines()]
