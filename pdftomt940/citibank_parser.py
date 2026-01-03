from __future__ import annotations

import re
from collections.abc import Sequence
from dataclasses import dataclass
from datetime import date
from decimal import Decimal, ROUND_HALF_UP
from typing import Optional


@dataclass
class Transaction:
    booking_date: date
    value_date: date
    amount: Decimal
    is_debit: bool
    description: str
    details: list[str]


@dataclass
class Statement:
    account_number: str
    currency: str
    statement_number: str
    opening_balance_date: date
    opening_balance: Decimal
    opening_balance_is_debit: bool
    closing_balance_date: date
    closing_balance: Decimal
    closing_balance_is_debit: bool
    transactions: list[Transaction]


DATE_PATTERN = re.compile(r"(?P<day>\d{2})\.(?P<month>\d{2})\.(?P<year>\d{2})")
TRANSACTION_LINE = re.compile(
    r"^(?P<booking>\d{2}\.\d{2}\.\d{2})\s+(?P<description>.+?)\s+(?P<value>\d{2}\.\d{2}\.\d{2})\s+(?P<amount>[0-9,.]+)$"
)
OPENING_CLOSING_LINE = re.compile(
    r"^(?P<booking>\d{2}\.\d{2}\.\d{2})\s+(?P<label>Saldo początkowe|Saldo końcowe)\s+(?P<amount>[0-9,.]+)$"
)
ACCOUNT_LINE = re.compile(r"^Nr\s+(?P<account>\d+)\s+(?P<currency>[A-Z]{3})$")
STATEMENT_NO_LINE = re.compile(r"^Numer wyciągu\s*:(?P<number>.+)$")


def parse_statement_text(lines: Sequence[str]) -> Statement:
    account_number: Optional[str] = None
    currency: Optional[str] = None
    statement_number: Optional[str] = None
    opening_balance: Optional[Decimal] = None
    opening_date: Optional[date] = None
    closing_balance: Optional[Decimal] = None
    closing_date: Optional[date] = None
    transactions: list[Transaction] = []
    transaction_blocks: list[tuple[int, str]] = []

    debit_count = debit_total = credit_count = credit_total = None

    for idx, line in enumerate(lines):
        stripped = line.strip()
        if not stripped:
            continue

        if account_number is None:
            account_match = ACCOUNT_LINE.match(stripped)
            if account_match:
                account_number = account_match.group("account")
                currency = account_match.group("currency")
                continue

        if statement_number is None:
            statement_match = STATEMENT_NO_LINE.match(stripped)
            if statement_match:
                statement_number = statement_match.group("number").strip()
                continue

        if opening_balance is None or closing_balance is None:
            balance_match = OPENING_CLOSING_LINE.match(stripped)
            if balance_match:
                amount = parse_decimal(balance_match.group("amount"))
                booking_date = parse_date(balance_match.group("booking"))
                label = balance_match.group("label")
                if label == "Saldo początkowe":
                    opening_balance = amount
                    opening_date = booking_date
                else:
                    closing_balance = amount
                    closing_date = booking_date
                continue

        trans_match = TRANSACTION_LINE.match(stripped)
        if trans_match:
            transaction_blocks.append((idx, stripped))
            continue

        if "Ilo" in stripped:
            normalized = stripped.replace("œ", "ś").replace("’", "'")
            if "obciąż" in normalized and ":" in normalized:
                count, total = _parse_summary_values(normalized)
                debit_count = count
                debit_total = total
                continue
            if "uznań" in normalized and ":" in normalized:
                count, total = _parse_summary_values(normalized)
                credit_count = count
                credit_total = total
                continue

    if any(value is None for value in (account_number, currency, statement_number)):
        raise ValueError("Nie udało się odczytać danych nagłówka (konto, waluta, numer wyciągu).")
    if any(value is None for value in (opening_balance, opening_date, closing_balance, closing_date)):
        raise ValueError("Nie udało się odczytać salda początkowego lub końcowego.")

    transactions = _build_transactions(
        lines=lines,
        transaction_blocks=transaction_blocks,
        debit_count=debit_count,
        debit_total=debit_total,
        credit_count=credit_count,
        credit_total=credit_total,
    )

    opening_is_debit = opening_balance < Decimal("0")
    closing_is_debit = closing_balance < Decimal("0")

    calculated_balance = (-opening_balance if opening_is_debit else opening_balance)
    for txn in transactions:
        calculated_balance += -txn.amount if txn.is_debit else txn.amount

    expected_closing = -closing_balance if closing_is_debit else closing_balance
    if (calculated_balance - expected_closing).copy_abs() > Decimal("0.01"):
        closing_is_debit = calculated_balance < Decimal("0")
        closing_balance = abs(calculated_balance)

    return Statement(
        account_number=account_number,
        currency=currency,
        statement_number=statement_number,
        opening_balance_date=opening_date,
        opening_balance=abs(opening_balance),
        opening_balance_is_debit=opening_is_debit,
        closing_balance_date=closing_date,
        closing_balance=abs(closing_balance),
        closing_balance_is_debit=closing_is_debit,
        transactions=transactions,
    )


def _build_transactions(
    *,
    lines: Sequence[str],
    transaction_blocks: Sequence[tuple[int, str]],
    debit_count: Optional[int],
    debit_total: Optional[Decimal],
    credit_count: Optional[int],
    credit_total: Optional[Decimal],
) -> list[Transaction]:
    entries: list[_RawTransaction] = []
    indices = [idx for idx, _ in transaction_blocks]
    for position, (line_index, raw_line) in enumerate(transaction_blocks):
        next_index = indices[position + 1] if position + 1 < len(indices) else len(lines)
        block_lines: list[str] = []
        for i in range(line_index + 1, next_index):
            candidate = lines[i].strip()
            if not candidate:
                continue
            if TRANSACTION_LINE.match(candidate) or OPENING_CLOSING_LINE.match(candidate):
                break
            if "Ilo" in candidate:
                break
            block_lines.append(candidate)
        match = TRANSACTION_LINE.match(raw_line)
        assert match, "Line must match transaction pattern"
        booking_date = parse_date(match.group("booking"))
        value_date = parse_date(match.group("value"))
        amount = parse_decimal(match.group("amount"))
        description = match.group("description").strip()
        entries.append(
            _RawTransaction(
                booking_date=booking_date,
                value_date=value_date,
                amount=amount,
                description=description,
                details=block_lines,
            )
        )

    sign_map = assign_signs(
        [entry.amount for entry in entries],
        debit_count=debit_count,
        debit_total=debit_total,
        credit_count=credit_count,
        credit_total=credit_total,
    )

    transactions: list[Transaction] = []
    for entry, is_debit in zip(entries, sign_map):
        transactions.append(
            Transaction(
                booking_date=entry.booking_date,
                value_date=entry.value_date,
                amount=entry.amount,
                is_debit=is_debit,
                description=entry.description,
                details=entry.details,
            )
        )
    return transactions


@dataclass
class _RawTransaction:
    booking_date: date
    value_date: date
    amount: Decimal
    description: str
    details: list[str]


def assign_signs(
    amounts: Sequence[Decimal],
    *,
    debit_count: Optional[int],
    debit_total: Optional[Decimal],
    credit_count: Optional[int],
    credit_total: Optional[Decimal],
) -> list[bool]:
    if not amounts:
        return []

    cents = [to_cents(value) for value in amounts]
    total_cents = sum(cents)

    if debit_count is None or debit_total is None or credit_count is None or credit_total is None:
        return [_guess_debit(amount) for amount in amounts]

    debit_target = to_cents(debit_total)
    credit_target = to_cents(credit_total)

    if debit_count == 0:
        return [False] * len(amounts)
    if credit_count == 0:
        return [True] * len(amounts)

    selection = _choose_indices_with_sum(cents, debit_count, debit_target)
    if selection is None:
        raise ValueError("Nie udało się dopasować sumy obciążeń do poszczególnych transakcji.")

    sign_map = []
    for index in range(len(cents)):
        sign_map.append(index in selection)

    calculated_credit = total_cents - sum(cents[i] for i in selection)
    if calculated_credit != credit_target:
        raise ValueError("Suma uznań nie odpowiada transakcjom z wyciągu.")

    return sign_map


def _guess_debit(_: Decimal) -> bool:
    return True


def _choose_indices_with_sum(amounts: Sequence[int], count: int, target: int) -> Optional[set[int]]:
    from functools import lru_cache

    @lru_cache(maxsize=None)
    def search(position: int, remaining: int, needed: int) -> Optional[tuple[int, ...]]:
        if remaining == 0:
            return () if needed == 0 else None
        if position == len(amounts):
            return None
        if needed < 0:
            return None

        current = amounts[position]
        with_current = search(position + 1, remaining - 1, needed - current)
        if with_current is not None:
            return (position,) + with_current
        return search(position + 1, remaining, needed)

    result = search(0, count, target)
    if result is None:
        return None
    return set(result)


def parse_decimal(raw: str) -> Decimal:
    normalised = raw.replace(" ", "").replace(",", "")
    return Decimal(normalised).quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)


def parse_date(raw: str) -> date:
    match = DATE_PATTERN.match(raw)
    if not match:
        raise ValueError(f"Niepoprawny format daty: {raw}")
    year = int(match.group("year"))
    year += 2000 if year < 70 else 1900
    return date(year, int(match.group("month")), int(match.group("day")))


def to_cents(amount: Decimal) -> int:
    return int((amount * 100).to_integral_value(rounding=ROUND_HALF_UP))


def _parse_summary_values(line: str) -> tuple[int, Decimal]:
    _, tail = line.split(":", 1)
    tail = tail.strip()
    parts = tail.split()
    if len(parts) < 2:
        raise ValueError(f"Niepoprawny wiersz podsumowania: {line}")
    count = int(parts[0])
    total = parse_decimal(parts[1])
    return count, total
