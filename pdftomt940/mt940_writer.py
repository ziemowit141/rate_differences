from __future__ import annotations

import textwrap
from datetime import date
from decimal import Decimal, ROUND_HALF_UP
from typing import Iterable

from .citibank_parser import Statement, Transaction


def build_mt940(statement: Statement) -> str:
    lines: list[str] = []
    lines.append(f":20:{_statement_reference(statement)}")
    lines.append(f":25:{statement.account_number}")
    lines.append(f":28:{statement.statement_number}")
    lines.append(
        _format_balance(
            tag=":60F:",
            is_debit=statement.opening_balance_is_debit,
            balance_date=statement.opening_balance_date,
            currency=statement.currency,
            amount=statement.opening_balance,
        )
    )

    for index, transaction in enumerate(statement.transactions, start=1):
        lines.extend(_format_transaction(transaction, index))

    lines.append(
        _format_balance(
            tag=":62F:",
            is_debit=statement.closing_balance_is_debit,
            balance_date=statement.closing_balance_date,
            currency=statement.currency,
            amount=statement.closing_balance,
        )
    )

    return "\n".join(lines) + "\n"


def _statement_reference(statement: Statement) -> str:
    reference = statement.statement_number.strip().replace(" ", "")
    return reference or "NONREF"


def _format_balance(*, tag: str, is_debit: bool, balance_date: date, currency: str, amount: Decimal) -> str:
    sign = "D" if is_debit else "C"
    date_fragment = balance_date.strftime("%y%m%d")
    formatted_amount = _format_amount(amount)
    return f"{tag}{sign}{date_fragment}{currency}{formatted_amount}"


def _format_transaction(transaction: Transaction, sequence_number: int) -> list[str]:
    dc_mark = "D" if transaction.is_debit else "C"
    value_fragment = transaction.value_date.strftime("%y%m%d")
    entry_fragment = transaction.booking_date.strftime("%m%d")
    amount_fragment = _format_amount(transaction.amount)
    code = _transaction_code(transaction.description)
    reference = _extract_reference(transaction.details) or f"SEQ{sequence_number:04d}"

    transaction_lines = [
        f":61:{value_fragment}{entry_fragment}{dc_mark}{amount_fragment}{code}//{reference}"
    ]

    description_parts = [transaction.description]
    description_parts.extend(transaction.details)
    full_description = " | ".join(_clean_detail(part) for part in description_parts if part)

    if full_description:
        for wrapped in _wrap_86(full_description):
            transaction_lines.append(f":86:{wrapped}")

    return transaction_lines


def _transaction_code(description: str) -> str:
    upper = description.upper()
    if "OPŁATA" in upper or "CHARGE" in upper or "PROWIZJA" in upper:
        return "NCHG"
    if "ROZL." in upper or "ROZLICZENIE" in upper or "FX" in upper:
        return "NFXP"
    return "NTRF"


def _extract_reference(details: Iterable[str]) -> str | None:
    for line in details:
        if ":" not in line:
            continue
        label, value = line.split(":", 1)
        label = label.strip().lower()
        value = value.strip()
        if not value:
            continue
        if "referencje" in label:
            return value.replace(" ", "")
        if "nr ref" in label:
            return value.replace(" ", "")
    return None


def _wrap_86(text: str) -> list[str]:
    return textwrap.wrap(text, width=65, break_long_words=False, break_on_hyphens=False) or [""]


def _clean_detail(text: str) -> str:
    return " ".join(text.split())


def _format_amount(amount: Decimal) -> str:
    rounded = amount.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)
    return f"{rounded:.2f}".replace(".", ",")
