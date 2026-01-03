from __future__ import annotations

import argparse
import sys
from pathlib import Path

try:
    import mt940  # type: ignore
except ModuleNotFoundError as exc:
    raise SystemExit(
        "Biblioteka 'mt-940' nie jest dostępna. Zainstaluj ją poleceniem: python3 -m pip install mt-940"
    ) from exc


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Wyświetl podstawowe dane z pliku MT940."
    )
    parser.add_argument(
        "--statement",
        required=True,
        type=Path,
        help="Ścieżka do pliku MT940.",
    )
    return parser.parse_args()


def load_statement(path: Path):
    if not path.is_file():
        raise FileNotFoundError(f"Brak pliku MT940: {path}")
    content = path.read_text(encoding="utf-8")
    return mt940.parse(content)


def main() -> None:
    args = parse_args()
    try:
        doc = load_statement(args.statement)
    except (FileNotFoundError, UnicodeDecodeError, ValueError) as error:
        print(f"Błąd: {error}", file=sys.stderr)
        sys.exit(1)

    account_id = getattr(doc, "account_identification", "Nieznane konto")
    opening_balance = getattr(doc, "opening_balance", None)
    closing_balance = getattr(doc, "closing_balance", None)
    transactions = list(getattr(doc, "transactions", []))

    print(f"Konto: {account_id}")
    if opening_balance is not None:
        print(
            f"Saldo początkowe ({opening_balance.date:%Y-%m-%d}): {opening_balance.amount} {opening_balance.currency}"
        )
    if closing_balance is not None:
        print(
            f"Saldo końcowe   ({closing_balance.date:%Y-%m-%d}): {closing_balance.amount} {closing_balance.currency}"
        )
    print(f"Liczba transakcji: {len(transactions)}")

    default_currency = None
    if closing_balance is not None and getattr(closing_balance, "currency", None):
        default_currency = closing_balance.currency
    elif opening_balance is not None and getattr(opening_balance, "currency", None):
        default_currency = opening_balance.currency

    for idx, transaction in enumerate(transactions, start=1):
        info = _extract_transaction_info(transaction, default_currency)
        parts = [
            f"{idx:03d}.",
            info["date"],
            info["signed_amount"],
        ]
        if info["currency"]:
            parts.append(info["currency"])
        if info["code"]:
            parts.append(f"— {info['code']}")
        if info["narrative"]:
            if not info["code"]:
                parts.append("—")
            parts.append(info["narrative"])
        print(" ".join(part for part in parts if part))
        print("\n")


def _extract_transaction_info(
    transaction, default_currency: str | None
) -> dict[str, str]:
    data = getattr(transaction, "data", {}) or {}

    amount = data.get("amount")
    if amount is None:
        amount = data.get("entry_amount")

    currency = (
        data.get("currency")
        or getattr(transaction, "currency", None)
        or default_currency
        or ""
    )

    txn_date = getattr(transaction, "date", None) or data.get("date")
    date_str = txn_date.strftime("%Y-%m-%d") if txn_date else "????-??-??"

    status = data.get("status") or data.get("debit_credit") or ""
    is_debit = None
    if isinstance(status, str) and status:
        status = status.upper()
        if status.startswith("D"):
            is_debit = True
        elif status.startswith("C"):
            is_debit = False

    if is_debit is None:
        is_debit_callable = getattr(transaction, "is_debit", None)
        if callable(is_debit_callable):
            try:
                is_debit = bool(is_debit_callable())
            except Exception:
                is_debit = None

    if is_debit is None:
        is_debit = bool(getattr(transaction, "debit", False))

    sign = "-" if is_debit else "+"
    amount_str = str(amount) if amount is not None else "?"
    signed_amount = f"{sign}{amount_str}"

    details = data.get("transaction_details") or data.get("description") or ""
    if isinstance(details, (list, tuple)):
        narrative = " ".join(str(part) for part in details if part).strip()
    else:
        narrative = str(details).strip()

    code = data.get("transaction_code") or data.get("swift_code") or ""
    if isinstance(code, (list, tuple)):
        code = ", ".join(str(part) for part in code if part)

    return {
        "date": date_str,
        "signed_amount": signed_amount,
        "currency": str(currency).strip(),
        "code": str(code).strip(),
        "narrative": narrative,
    }


if __name__ == "__main__":
    main()
