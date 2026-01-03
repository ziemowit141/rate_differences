package main

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type mt940Transaction struct {
	BookingDate time.Time
	ValueDate   time.Time
	AmountCents int64
	IsDebit     bool
	Description string
	Details     []string
}

type mt940Statement struct {
	AccountNumber         string
	Currency              string
	StatementNumber       string
	OpeningBalanceDate    time.Time
	OpeningBalanceCents   int64
	OpeningBalanceIsDebit bool
	ClosingBalanceDate    time.Time
	ClosingBalanceCents   int64
	ClosingBalanceIsDebit bool
	Transactions          []mt940Transaction
}

var (
	transactionLinePattern = regexp.MustCompile(`^(\d{2}\.\d{2}\.\d{2})\s+(.+?)\s+(\d{2}\.\d{2}\.\d{2})\s+([0-9,.]+)$`)
	openingClosingPattern  = regexp.MustCompile(`^(\d{2}\.\d{2}\.\d{2})\s+(Saldo początkowe|Saldo końcowe)\s+([0-9,.]+)$`)
	accountLinePattern     = regexp.MustCompile(`^Nr\s+(\d+)\s+([A-Z]{3})$`)
	statementNoPattern     = regexp.MustCompile(`^Numer wyciągu\s*:(.+)$`)
)

func convertPDFToMT940(pdfPath, mt940Path string) error {
	lines, err := extractPDFTextLines(pdfPath)
	if err != nil {
		return err
	}
	statement, err := parseStatementText(lines)
	if err != nil {
		return err
	}
	payload := buildMT940(statement)
	if err := os.MkdirAll(filepath.Dir(mt940Path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(mt940Path, []byte(payload), 0o644)
}

func extractPDFTextLines(pdfPath string) ([]string, error) {
	if _, err := os.Stat(pdfPath); err != nil {
		return nil, fmt.Errorf("missing pdf: %w", err)
	}
	binaryPath, err := ensureExtractorBinary()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(binaryPath, pdfPath)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("pdf extract failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("pdf extract failed: %w", err)
	}
	raw := bytes.Split(output, []byte("\n"))
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		lines = append(lines, strings.TrimRight(string(line), "\r\n"))
	}
	return lines, nil
}

func ensureExtractorBinary() (string, error) {
	binPath := filepath.Join("cmd", "mt940api", "bin", "extract")
	if fileExists(binPath) {
		return binPath, nil
	}
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		return "", fmt.Errorf("swiftc not found; cannot build extractor")
	}
	scriptPath := filepath.Join("cmd", "mt940api", "tools", "extract.swift")
	if !fileExists(scriptPath) {
		return "", fmt.Errorf("missing extract.swift")
	}
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		return "", err
	}
	cmd := exec.Command(swiftc, scriptPath, "-o", binPath)
	cmd.Env = append(os.Environ(),
		"SWIFT_MODULE_CACHE_PATH="+filepath.Join("cmd", "mt940api", ".swift-cache", "swift"),
		"CLANG_MODULE_CACHE_PATH="+filepath.Join("cmd", "mt940api", ".swift-cache", "clang"),
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("swiftc failed: %s", strings.TrimSpace(string(output)))
	}
	return binPath, nil
}

func parseStatementText(lines []string) (mt940Statement, error) {
	var statement mt940Statement
	var openingBalanceCents *int64
	var openingDate *time.Time
	var closingBalanceCents *int64
	var closingDate *time.Time
	var transactionBlocks []transactionBlock
	var debitCount, creditCount *int
	var debitTotal, creditTotal *int64

	for idx, line := range lines {
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}

		if statement.AccountNumber == "" {
			if match := accountLinePattern.FindStringSubmatch(stripped); match != nil {
				statement.AccountNumber = match[1]
				statement.Currency = match[2]
				continue
			}
		}

		if statement.StatementNumber == "" {
			if match := statementNoPattern.FindStringSubmatch(stripped); match != nil {
				statement.StatementNumber = strings.TrimSpace(match[1])
				continue
			}
		}

		if openingBalanceCents == nil || closingBalanceCents == nil {
			if match := openingClosingPattern.FindStringSubmatch(stripped); match != nil {
				amount, err := parseDecimalCents(match[3])
				if err != nil {
					return statement, err
				}
				date, err := parseDate(match[1])
				if err != nil {
					return statement, err
				}
				if match[2] == "Saldo początkowe" {
					openingBalanceCents = &amount
					openingDate = &date
				} else {
					closingBalanceCents = &amount
					closingDate = &date
				}
				continue
			}
		}

		if match := transactionLinePattern.FindStringSubmatch(stripped); match != nil {
			transactionBlocks = append(transactionBlocks, transactionBlock{Index: idx, Line: stripped})
			continue
		}

		if strings.Contains(stripped, "Ilo") {
			normalized := strings.NewReplacer("œ", "ś", "’", "'").Replace(stripped)
			if strings.Contains(normalized, "obciąż") && strings.Contains(normalized, ":") {
				count, total, err := parseSummaryValues(normalized)
				if err != nil {
					return statement, err
				}
				debitCount = &count
				debitTotal = &total
				continue
			}
			if strings.Contains(normalized, "uznań") && strings.Contains(normalized, ":") {
				count, total, err := parseSummaryValues(normalized)
				if err != nil {
					return statement, err
				}
				creditCount = &count
				creditTotal = &total
				continue
			}
		}
	}

	if statement.AccountNumber == "" || statement.Currency == "" || statement.StatementNumber == "" {
		return statement, fmt.Errorf("missing header data (account, currency, statement number)")
	}
	if openingBalanceCents == nil || openingDate == nil || closingBalanceCents == nil || closingDate == nil {
		return statement, fmt.Errorf("missing opening/closing balance")
	}

	transactions, err := buildTransactions(lines, transactionBlocks, debitCount, debitTotal, creditCount, creditTotal)
	if err != nil {
		return statement, err
	}

	statement.OpeningBalanceDate = *openingDate
	statement.OpeningBalanceCents = absInt64(*openingBalanceCents)
	statement.OpeningBalanceIsDebit = *openingBalanceCents < 0
	statement.ClosingBalanceDate = *closingDate
	statement.ClosingBalanceCents = absInt64(*closingBalanceCents)
	statement.ClosingBalanceIsDebit = *closingBalanceCents < 0
	statement.Transactions = transactions

	calculated := signedBalance(statement.OpeningBalanceCents, statement.OpeningBalanceIsDebit)
	for _, txn := range transactions {
		if txn.IsDebit {
			calculated -= txn.AmountCents
		} else {
			calculated += txn.AmountCents
		}
	}
	expected := signedBalance(statement.ClosingBalanceCents, statement.ClosingBalanceIsDebit)
	if absInt64(calculated-expected) > 1 {
		statement.ClosingBalanceIsDebit = calculated < 0
		statement.ClosingBalanceCents = absInt64(calculated)
	}

	return statement, nil
}

type transactionBlock struct {
	Index int
	Line  string
}

type rawTransaction struct {
	BookingDate time.Time
	ValueDate   time.Time
	AmountCents int64
	Description string
	Details     []string
}

func buildTransactions(lines []string, blocks []transactionBlock, debitCount *int, debitTotal *int64, creditCount *int, creditTotal *int64) ([]mt940Transaction, error) {
	entries := make([]rawTransaction, 0, len(blocks))
	indices := make([]int, len(blocks))
	for i, block := range blocks {
		indices[i] = block.Index
	}

	for pos, block := range blocks {
		nextIndex := len(lines)
		if pos+1 < len(indices) {
			nextIndex = indices[pos+1]
		}
		details := []string{}
		for i := block.Index + 1; i < nextIndex; i++ {
			candidate := strings.TrimSpace(lines[i])
			if candidate == "" {
				continue
			}
			if transactionLinePattern.MatchString(candidate) || openingClosingPattern.MatchString(candidate) || strings.Contains(candidate, "Ilo") {
				break
			}
			details = append(details, candidate)
		}

		match := transactionLinePattern.FindStringSubmatch(block.Line)
		if match == nil {
			return nil, fmt.Errorf("invalid transaction line: %s", block.Line)
		}
		bookingDate, err := parseDate(match[1])
		if err != nil {
			return nil, err
		}
		valueDate, err := parseDate(match[3])
		if err != nil {
			return nil, err
		}
		amount, err := parseDecimalCents(match[4])
		if err != nil {
			return nil, err
		}
		entries = append(entries, rawTransaction{
			BookingDate: bookingDate,
			ValueDate:   valueDate,
			AmountCents: amount,
			Description: strings.TrimSpace(match[2]),
			Details:     details,
		})
	}

	signs, err := assignSigns(entries, debitCount, debitTotal, creditCount, creditTotal)
	if err != nil {
		return nil, err
	}

	transactions := make([]mt940Transaction, 0, len(entries))
	for i, entry := range entries {
		transactions = append(transactions, mt940Transaction{
			BookingDate: entry.BookingDate,
			ValueDate:   entry.ValueDate,
			AmountCents: entry.AmountCents,
			IsDebit:     signs[i],
			Description: entry.Description,
			Details:     entry.Details,
		})
	}
	return transactions, nil
}

func assignSigns(entries []rawTransaction, debitCount *int, debitTotal *int64, creditCount *int, creditTotal *int64) ([]bool, error) {
	if len(entries) == 0 {
		return []bool{}, nil
	}
	cents := make([]int64, len(entries))
	for i, entry := range entries {
		cents[i] = entry.AmountCents
	}

	if debitCount == nil || debitTotal == nil || creditCount == nil || creditTotal == nil {
		signs := make([]bool, len(entries))
		for i := range signs {
			signs[i] = true
		}
		return signs, nil
	}

	if *debitCount == 0 {
		signs := make([]bool, len(entries))
		return signs, nil
	}
	if *creditCount == 0 {
		signs := make([]bool, len(entries))
		for i := range signs {
			signs[i] = true
		}
		return signs, nil
	}

	selection, ok := chooseIndicesWithSum(cents, *debitCount, *debitTotal)
	if !ok {
		return nil, fmt.Errorf("unable to match debit totals")
	}
	signs := make([]bool, len(entries))
	for i := range signs {
		signs[i] = selection[i]
	}
	sumDebit := int64(0)
	for i, isDebit := range signs {
		if isDebit {
			sumDebit += cents[i]
		}
	}
	if sumDebit != *debitTotal {
		return nil, fmt.Errorf("debit totals mismatch")
	}
	return signs, nil
}

func chooseIndicesWithSum(amounts []int64, count int, target int64) ([]bool, bool) {
	type key struct {
		Pos   int
		Count int
		Sum   int64
	}
	memo := map[key]([]bool){}
	var search func(pos int, remaining int, needed int64) ([]bool, bool)
	search = func(pos int, remaining int, needed int64) ([]bool, bool) {
		if remaining == 0 {
			if needed == 0 {
				return make([]bool, len(amounts)), true
			}
			return nil, false
		}
		if pos == len(amounts) || needed < 0 {
			return nil, false
		}
		k := key{Pos: pos, Count: remaining, Sum: needed}
		if cached, ok := memo[k]; ok {
			return cached, true
		}

		withCurrent, ok := search(pos+1, remaining-1, needed-amounts[pos])
		if ok {
			withCurrent[pos] = true
			memo[k] = withCurrent
			return withCurrent, true
		}
		withoutCurrent, ok := search(pos+1, remaining, needed)
		if ok {
			memo[k] = withoutCurrent
			return withoutCurrent, true
		}
		return nil, false
	}
	result, ok := search(0, count, target)
	if !ok {
		return nil, false
	}
	return result, true
}

func parseSummaryValues(line string) (int, int64, error) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("invalid summary line")
	}
	tail := strings.Fields(strings.TrimSpace(parts[1]))
	if len(tail) < 2 {
		return 0, 0, fmt.Errorf("invalid summary line")
	}
	count, err := strconv.Atoi(tail[0])
	if err != nil {
		return 0, 0, err
	}
	total, err := parseDecimalCents(tail[1])
	if err != nil {
		return 0, 0, err
	}
	return count, total, nil
}

func parseDecimalCents(raw string) (int64, error) {
	clean := strings.ReplaceAll(raw, " ", "")
	clean = strings.ReplaceAll(clean, ",", "")
	value, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(value * 100)), nil
}

func parseDate(raw string) (time.Time, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid date: %s", raw)
	}
	day, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, err
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, err
	}
	year, err := strconv.Atoi(parts[2])
	if err != nil {
		return time.Time{}, err
	}
	if year < 70 {
		year += 2000
	} else {
		year += 1900
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
}

func buildMT940(statement mt940Statement) string {
	lines := []string{
		fmt.Sprintf(":20:%s", statementReference(statement.StatementNumber)),
		fmt.Sprintf(":25:%s", statement.AccountNumber),
		fmt.Sprintf(":28:%s", statement.StatementNumber),
		formatBalance(":60F:", statement.OpeningBalanceIsDebit, statement.OpeningBalanceDate, statement.Currency, statement.OpeningBalanceCents),
	}

	for idx, txn := range statement.Transactions {
		lines = append(lines, formatTransaction(txn, idx+1)...)
	}

	lines = append(lines, formatBalance(":62F:", statement.ClosingBalanceIsDebit, statement.ClosingBalanceDate, statement.Currency, statement.ClosingBalanceCents))
	return strings.Join(lines, "\n") + "\n"
}

func statementReference(number string) string {
	ref := strings.ReplaceAll(strings.TrimSpace(number), " ", "")
	if ref == "" {
		return "NONREF"
	}
	return ref
}

func formatBalance(tag string, isDebit bool, balanceDate time.Time, currency string, amountCents int64) string {
	sign := "C"
	if isDebit {
		sign = "D"
	}
	return fmt.Sprintf("%s%s%s%s%s", tag, sign, balanceDate.Format("060102"), currency, formatAmount(amountCents))
}

func formatTransaction(txn mt940Transaction, seq int) []string {
	dcMark := "C"
	if txn.IsDebit {
		dcMark = "D"
	}
	valueFragment := txn.ValueDate.Format("060102")
	entryFragment := txn.BookingDate.Format("0102")
	amountFragment := formatAmount(txn.AmountCents)
	code := transactionCode(txn.Description)
	reference := extractReference(txn.Details)
	if reference == "" {
		reference = fmt.Sprintf("SEQ%04d", seq)
	}

	lines := []string{
		fmt.Sprintf(":61:%s%s%s%s%s//%s", valueFragment, entryFragment, dcMark, amountFragment, code, reference),
	}

	descriptionParts := []string{txn.Description}
	descriptionParts = append(descriptionParts, txn.Details...)
	full := strings.Join(cleanDetails(descriptionParts), " | ")
	if full != "" {
		for _, wrapped := range wrap86(full, 65) {
			lines = append(lines, fmt.Sprintf(":86:%s", wrapped))
		}
	}
	return lines
}

func transactionCode(description string) string {
	upper := strings.ToUpper(description)
	if strings.Contains(upper, "OPŁATA") || strings.Contains(upper, "CHARGE") || strings.Contains(upper, "PROWIZJA") {
		return "NCHG"
	}
	if strings.Contains(upper, "ROZL.") || strings.Contains(upper, "ROZLICZENIE") || strings.Contains(upper, "FX") {
		return "NFXP"
	}
	return "NTRF"
}

func extractReference(details []string) string {
	for _, line := range details {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		label := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		if value == "" {
			continue
		}
		if strings.Contains(label, "referencje") || strings.Contains(label, "nr ref") {
			return strings.ReplaceAll(value, " ", "")
		}
	}
	return ""
}

func cleanDetails(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.Join(strings.Fields(part), " ")
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func wrap86(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	lines := []string{}
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	lines = append(lines, current)
	return lines
}

func formatAmount(cents int64) string {
	value := absInt64(cents)
	return fmt.Sprintf("%d,%02d", value/100, value%100)
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func signedBalance(cents int64, isDebit bool) int64 {
	if isDebit {
		return -cents
	}
	return cents
}
