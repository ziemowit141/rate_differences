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

    swiftc_path = shutil.which("swiftc")
    binary_path = _ensure_extractor_binary(script_path, swiftc_path)

    cache_root = script_path.parent / ".swift-cache"
    swift_cache = cache_root / "swift"
    clang_cache = cache_root / "clang"
    swift_cache.mkdir(parents=True, exist_ok=True)
    clang_cache.mkdir(parents=True, exist_ok=True)

    env = os.environ.copy()
    env["SWIFT_MODULE_CACHE_PATH"] = str(swift_cache.resolve())
    env["CLANG_MODULE_CACHE_PATH"] = str(clang_cache.resolve())

    command = [str(binary_path), str(pdf_path)] if binary_path else [swift_path, str(script_path), str(pdf_path)]
    try:
        result = subprocess.run(
            command,
            check=True,
            capture_output=True,
            text=True,
            env=env,
        )
    except subprocess.CalledProcessError as exc:
        runner = "swiftc" if binary_path else "swift"
        raise PdfExtractionError(
            f"Nie udało się wyodrębnić tekstu z PDF (`{runner}` zakończył się kodem {exc.returncode})."
        ) from exc

    return [line.rstrip("\n") for line in result.stdout.splitlines()]


def _ensure_extractor_binary(script_path: Path, swiftc_path: str | None) -> Path | None:
    if swiftc_path is None:
        return None

    bin_dir = script_path.parent / ".swift-bin"
    bin_path = bin_dir / "extract"

    if bin_path.is_file() and bin_path.stat().st_mtime >= script_path.stat().st_mtime:
        return bin_path

    bin_dir.mkdir(parents=True, exist_ok=True)
    cache_root = script_path.parent / ".swift-cache"
    swift_cache = cache_root / "swift"
    clang_cache = cache_root / "clang"
    swift_cache.mkdir(parents=True, exist_ok=True)
    clang_cache.mkdir(parents=True, exist_ok=True)

    env = os.environ.copy()
    env["SWIFT_MODULE_CACHE_PATH"] = str(swift_cache.resolve())
    env["CLANG_MODULE_CACHE_PATH"] = str(clang_cache.resolve())

    try:
        subprocess.run(
            [swiftc_path, str(script_path), "-o", str(bin_path)],
            check=True,
            capture_output=True,
            text=True,
            env=env,
        )
    except subprocess.CalledProcessError as exc:
        raise PdfExtractionError(
            f"Nie udało się skompilować ekstraktora PDF (swiftc kod {exc.returncode})."
        ) from exc

    return bin_path
