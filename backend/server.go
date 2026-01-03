package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/mmalcek/mt940"
)

type transactionView struct {
	ValueDate string `json:"value_date"`
	EntryDate string `json:"entry_date"`
	DCMark    string `json:"dc_mark"`
	Amount    string `json:"amount"`
	Code      string `json:"code"`
	Reference string `json:"reference"`
	Details   string `json:"details,omitempty"`
	Raw61     string `json:"raw_61"`
}

type fileTransactions struct {
	File         string            `json:"file"`
	BaseName     string            `json:"base_name"`
	Error        string            `json:"error,omitempty"`
	Transactions []transactionView `json:"transactions,omitempty"`
	NbpRate      float64           `json:"nbp_rate,omitempty"`
	NbpDate      string            `json:"nbp_date,omitempty"`
	NbpError     string            `json:"nbp_error,omitempty"`
}

type transactionsResponse struct {
	Files []fileTransactions `json:"files"`
}

type uploadResponse struct {
	Files []uploadResult `json:"files"`
}

type uploadResult struct {
	Source    string `json:"source"`
	PDFPath   string `json:"pdf_path,omitempty"`
	MT940Path string `json:"mt940_path,omitempty"`
	Error     string `json:"error,omitempty"`
}

type trancheInput struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
	Rate   float64 `json:"rate"`
}

type trancheRequest struct {
	Tranches []trancheInput `json:"tranches"`
}

type calculateRequest struct {
	Tranches []trancheInput `json:"tranches"`
}

type reportSummary struct {
	TotalFXDifference float64 `json:"total_fx_difference"`
	TotalOutflow      float64 `json:"total_outflow"`
	TotalCovered      float64 `json:"total_covered"`
	MissingCoverage   float64 `json:"missing_coverage"`
}

type reportUsage struct {
	TransactionDate string  `json:"transaction_date"`
	TransactionRef  string  `json:"transaction_ref"`
	AmountUsed      float64 `json:"amount_used"`
	TrancheDate     string  `json:"tranche_date"`
	TrancheRate     float64 `json:"tranche_rate"`
	NbpRate         float64 `json:"nbp_rate"`
	FXDifference    float64 `json:"fx_difference"`
}

type trancheUsageEntry struct {
	TransactionDate string  `json:"transaction_date"`
	TransactionRef  string  `json:"transaction_ref"`
	AmountUsed      float64 `json:"amount_used"`
	NbpRate         float64 `json:"nbp_rate"`
	FXDifference    float64 `json:"fx_difference"`
	Remaining       float64 `json:"remaining"`
}

type trancheReport struct {
	Date       string              `json:"date"`
	Rate       float64             `json:"rate"`
	Amount     float64             `json:"amount"`
	Remaining  float64             `json:"remaining"`
	Usages     []trancheUsageEntry `json:"usages"`
	Source     string              `json:"source"`
	SourceNote string              `json:"source_note,omitempty"`
}

type reportTransaction struct {
	Date        string        `json:"date"`
	Reference   string        `json:"reference"`
	Amount      float64       `json:"amount"`
	NbpRate     float64       `json:"nbp_rate"`
	FXTotalDiff float64       `json:"fx_total_diff"`
	Usages      []reportUsage `json:"usages"`
}

type calculateResponse struct {
	Summary        reportSummary       `json:"summary"`
	Transactions   []reportTransaction `json:"transactions"`
	Tranches       []trancheReport     `json:"tranches"`
	UsedTranches   []trancheInput      `json:"used_tranches"`
	AutoTranches   []trancheInput      `json:"auto_tranches"`
	Warnings       []string            `json:"warnings"`
	Error          string              `json:"error,omitempty"`
	SourceFiles    []string            `json:"source_files"`
	UncoveredDates map[string]float64  `json:"uncovered_dates"`
}

func StartServer(addr string) error {
	if err := ensureDataDirs(); err != nil {
		return err
	}
	e := NewServer()
	log.Printf("mt940 api listening on %s", addr)
	return e.Start(addr)
}

func NewServer() *echo.Echo {
	_ = ensureDataDirs()
	e := echo.New()
	e.Use(middleware.BodyLimit("50M"))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			"Content-Type",
			"Authorization",
			"Accept",
		},
	}))
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.GET("/health", healthHandler)
	e.GET("/transactions", transactionsHandler)
	e.POST("/upload", uploadHandler)
	e.DELETE("/files/:base", deleteHandler)
	e.POST("/calculate", calculateHandler)
	return e
}

func ensureDataDirs() error {
	if err := os.MkdirAll(statementDir(), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(mt940Dir(), 0o755); err != nil {
		return err
	}
	return nil
}

func healthHandler(c echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

func transactionsHandler(c echo.Context) error {
	log.Printf("transactions: dir=%q pattern=%q", c.QueryParam("dir"), c.QueryParam("pattern"))
	dir := strings.TrimSpace(c.QueryParam("dir"))
	if dir == "" {
		dir = mt940Dir()
	}
	pattern := strings.TrimSpace(c.QueryParam("pattern"))
	if pattern == "" {
		pattern = "*.mt940"
	}

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return c.String(http.StatusBadRequest, "dir not found")
	}

	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid pattern")
	}
	sort.Strings(matches)

	response := transactionsResponse{Files: make([]fileTransactions, 0, len(matches))}
	nbpCache := map[string]nbpRateResult{}
	for _, path := range matches {
		log.Printf("transactions: parsing %s", path)
		result := fileTransactions{
			File:     path,
			BaseName: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		}
		data, err := os.ReadFile(path)
		if err != nil {
			result.Error = err.Error()
			response.Files = append(response.Files, result)
			continue
		}
		message, err := parseMT940(data)
		if err != nil {
			result.Error = err.Error()
			response.Files = append(response.Files, result)
			continue
		}
		result.Transactions = parseTransactions(message.Transactions)
		if date := statementDateFromBase(result.BaseName); date != "" {
			log.Printf("nbp: lookup USD rate for %s", date)
			rate := fetchNBPRateCached(nbpCache, date)
			if rate.Err != nil {
				log.Printf("nbp: error for %s: %v", date, rate.Err)
				result.NbpError = rate.Err.Error()
			} else {
				result.NbpRate = rate.Mid
				result.NbpDate = rate.Date
			}
		}
		response.Files = append(response.Files, result)
	}

	return writeJSON(c, response)
}

func uploadHandler(c echo.Context) error {
	log.Printf("upload: incoming request from %s", c.RealIP())
	form, err := c.MultipartForm()
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid multipart form")
	}
	files := form.File["files"]
	if len(files) == 0 {
		return c.String(http.StatusBadRequest, "no files uploaded")
	}
	log.Printf("upload: received %d files", len(files))

	results := make([]uploadResult, 0, len(files))
	anyError := false
	for index, file := range files {
		result := uploadResult{Source: file.Filename}
		tempBase := buildTempName(index)
		pdfPath := filepath.Join(statementDir(), tempBase+".pdf")
		mt940Path := filepath.Join(mt940Dir(), tempBase+".mt940")

		if err := saveUploadedFile(file, pdfPath); err != nil {
			log.Printf("upload: save failed for %s: %v", file.Filename, err)
			result.Error = err.Error()
			anyError = true
			results = append(results, result)
			continue
		}

		if err := runConversion(pdfPath, mt940Path); err != nil {
			log.Printf("upload: conversion failed for %s: %v", file.Filename, err)
			result.Error = err.Error()
			anyError = true
			results = append(results, result)
			continue
		}

		finalBase := chooseStatementBase(mt940Path)
		finalPDFPath := filepath.Join(statementDir(), finalBase+".pdf")
		finalMT940Path := filepath.Join(mt940Dir(), finalBase+".mt940")
		if err := os.Rename(pdfPath, finalPDFPath); err != nil {
			log.Printf("upload: rename pdf failed %s -> %s: %v", pdfPath, finalPDFPath, err)
			result.Error = err.Error()
			anyError = true
			results = append(results, result)
			continue
		}
		if err := os.Rename(mt940Path, finalMT940Path); err != nil {
			log.Printf("upload: rename mt940 failed %s -> %s: %v", mt940Path, finalMT940Path, err)
			result.Error = err.Error()
			anyError = true
			results = append(results, result)
			continue
		}

		result.PDFPath = finalPDFPath
		result.MT940Path = finalMT940Path
		log.Printf("upload: stored %s -> %s", result.PDFPath, result.MT940Path)
		results = append(results, result)
	}

	if anyError {
		return c.JSON(http.StatusInternalServerError, uploadResponse{Files: results})
	}
	return writeJSON(c, uploadResponse{Files: results})
}

func parseMT940(data []byte) (mt940Message, error) {
	payload := normalizeLineEndings(data)
	if !bytes.Contains(payload, []byte("{1:")) {
		payload = wrapWithSwiftHeader(payload)
	}
	message, err := mt940.Parse(payload)
	if err != nil {
		return mt940Message{}, err
	}
	return mt940Message{
		Header:       message.Header,
		Fields:       message.Fields,
		Transactions: message.Transactions,
	}, nil
}

func normalizeLineEndings(data []byte) []byte {
	if bytes.Contains(data, []byte("\r\n")) {
		return data
	}
	return bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
}

func wrapWithSwiftHeader(data []byte) []byte {
	if !bytes.HasSuffix(data, []byte("\r\n")) {
		data = append(data, []byte("\r\n")...)
	}
	header := []byte("{1:F01FOOBARXXXX0000000000}{2:I940FOOBARXXXXN}{4:\r\n")
	footer := []byte("\r\n-}")
	var out []byte
	out = append(out, header...)
	out = append(out, data...)
	out = append(out, footer...)
	return out
}

type mt940Message struct {
	Header       string
	Fields       map[string]interface{}
	Transactions []map[string]interface{}
}

var field61Pattern = regexp.MustCompile(`^(\d{6})(\d{4})([DC])([0-9,]+)([A-Z0-9]{3,4})//(.+)$`)

func parseTransactions(raw []map[string]interface{}) []transactionView {
	transactions := make([]transactionView, 0, len(raw))
	for _, entry := range raw {
		raw61, _ := entry["F_61"].(string)
		raw86, _ := entry["F_86"].(string)
		view := transactionView{
			Raw61:   strings.TrimSpace(raw61),
			Details: strings.TrimSpace(raw86),
		}
		if matches := field61Pattern.FindStringSubmatch(view.Raw61); matches != nil {
			view.ValueDate = matches[1]
			view.EntryDate = matches[2]
			view.DCMark = matches[3]
			view.Amount = matches[4]
			view.Code = matches[5]
			view.Reference = strings.TrimSpace(matches[6])
		}
		transactions = append(transactions, view)
	}
	return transactions
}

func calculateHandler(c echo.Context) error {
	var req calculateRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "invalid json body")
	}
	for _, tranche := range req.Tranches {
		if !datePattern.MatchString(tranche.Date) {
			return c.String(http.StatusBadRequest, "invalid tranche date")
		}
		if tranche.Amount <= 0 || tranche.Rate <= 0 {
			return c.String(http.StatusBadRequest, "invalid tranche amount or rate")
		}
	}

	files, transactions, warnings := loadTransactionsFromDisk()
	autoTranches := extractAutoTranches(transactions)
	allTranches := append([]trancheInput{}, req.Tranches...)
	allTranches = append(allTranches, autoTranches...)
	sort.Slice(allTranches, func(i, j int) bool {
		return allTranches[i].Date < allTranches[j].Date
	})

	report := buildReport(transactions, allTranches)
	report.Warnings = append(report.Warnings, warnings...)
	report.SourceFiles = files
	report.AutoTranches = autoTranches
	report.UsedTranches = allTranches

	return writeJSON(c, report)
}

func deleteHandler(c echo.Context) error {
	base := strings.TrimSpace(c.Param("base"))
	if base == "" || strings.Contains(base, "/") || strings.Contains(base, "\\") {
		return c.String(http.StatusBadRequest, "invalid base name")
	}
	log.Printf("delete: base=%s", base)

	pdfPath := filepath.Join(statementDir(), base+".pdf")
	mt940Path := filepath.Join(mt940Dir(), base+".mt940")

	wasPDF := fileExists(pdfPath)
	wasMT940 := fileExists(mt940Path)

	if err := removeIfExists(pdfPath); err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	if err := removeIfExists(mt940Path); err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	deleted := []string{}
	if wasPDF {
		deleted = append(deleted, pdfPath)
	}
	if wasMT940 {
		deleted = append(deleted, mt940Path)
	}
	if len(deleted) == 0 {
		return c.String(http.StatusNotFound, "file not found")
	}

	return writeJSON(c, map[string]any{
		"base":    base,
		"deleted": deleted,
	})
}

func saveUploadedFile(file *multipart.FileHeader, dest string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

func runConversion(pdfPath, mt940Path string) error {
	return convertPDFToMT940(pdfPath, mt940Path)
}

func buildTempName(index int) string {
	base := time.Now().Format("20060102-150405")
	return fmt.Sprintf("upload-%s-%02d", base, index+1)
}

func chooseStatementBase(mt940Path string) string {
	statementDate, err := extractStatementDate(mt940Path)
	if err != nil {
		statementDate = time.Now().Format("2006-01-02")
	}
	return uniqueName(statementDate)
}

func uniqueName(base string) string {
	if !fileExists(filepath.Join(statementDir(), base+".pdf")) &&
		!fileExists(filepath.Join(mt940Dir(), base+".mt940")) {
		return base
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%02d", base, i)
		if !fileExists(filepath.Join(statementDir(), candidate+".pdf")) &&
			!fileExists(filepath.Join(mt940Dir(), candidate+".mt940")) {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d", base, time.Now().Unix())
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func dataRoot() string {
	if env := os.Getenv("RATE_DIFF_HOME"); env != "" {
		return env
	}
	cwd, _ := os.Getwd()
	if cwd != "" {
		candidate := filepath.Join(cwd, "cmd", "mt940api", "data")
		if fileExists(candidate) {
			return candidate
		}
	}
	cache, err := os.UserCacheDir()
	if err == nil && cache != "" {
		return filepath.Join(cache, "rate_differences", "data")
	}
	return filepath.Join(os.TempDir(), "rate_differences", "data")
}

func statementDir() string {
	return filepath.Join(dataRoot(), "statements")
}

func mt940Dir() string {
	return filepath.Join(dataRoot(), "mt940s")
}

func extractStatementDate(mt940Path string) (string, error) {
	data, err := os.ReadFile(mt940Path)
	if err != nil {
		return "", err
	}
	message, err := parseMT940(data)
	if err != nil {
		return "", err
	}
	field, ok := message.Fields["F_62F"].(string)
	if !ok || field == "" {
		field, ok = message.Fields["F_60F"].(string)
		if !ok || field == "" {
			return "", fmt.Errorf("missing balance date")
		}
	}
	date, err := parseMT940Date(field)
	if err != nil {
		return "", err
	}
	return date, nil
}

var balanceDatePattern = regexp.MustCompile(`^[CD](\d{6})`)

func parseMT940Date(field string) (string, error) {
	field = strings.TrimSpace(field)
	matches := balanceDatePattern.FindStringSubmatch(field)
	if matches == nil {
		return "", fmt.Errorf("invalid balance field")
	}
	raw := matches[1]
	year, err := parseYear(raw[0:2])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d-%s-%s", year, raw[2:4], raw[4:6]), nil
}

func parseYear(raw string) (int, error) {
	if len(raw) != 2 {
		return 0, fmt.Errorf("invalid year")
	}
	year := (int(raw[0]-'0') * 10) + int(raw[1]-'0')
	if year < 70 {
		return 2000 + year, nil
	}
	return 1900 + year, nil
}

type nbpRateResult struct {
	Mid  float64
	Date string
	Err  error
}

func statementDateFromBase(base string) string {
	if len(base) < 10 {
		return ""
	}
	date := base[:10]
	if !datePattern.MatchString(date) {
		return ""
	}
	return date
}

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func fetchNBPRateCached(cache map[string]nbpRateResult, date string) nbpRateResult {
	if cached, ok := cache[date]; ok {
		return cached
	}
	result := fetchNBPRate(date)
	cache[date] = result
	return result
}

func fetchNBPRate(date string) nbpRateResult {
	url := fmt.Sprintf("https://api.nbp.pl/api/exchangerates/rates/a/usd/%s/?format=json", date)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nbpRateResult{Err: err}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "rate-differences/1.0 (+local)")

	resp, err := client.Do(req)
	if err != nil {
		return nbpRateResult{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nbpRateResult{Err: fmt.Errorf("nbp http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))}
	}

	var payload struct {
		Code  string `json:"code"`
		Rates []struct {
			EffectiveDate string  `json:"effectiveDate"`
			Mid           float64 `json:"mid"`
		} `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nbpRateResult{Err: err}
	}
	if len(payload.Rates) == 0 {
		return nbpRateResult{Err: fmt.Errorf("nbp empty rates")}
	}
	return nbpRateResult{
		Mid:  payload.Rates[0].Mid,
		Date: payload.Rates[0].EffectiveDate,
	}
}

type parsedTxn struct {
	Date      string
	Amount    float64
	DCMark    string
	Reference string
	Details   string
}

type trancheState struct {
	Date       string
	Amount     float64
	Rate       float64
	Source     string
	SourceNote string
	Original   float64
	ReportIdx  int
}

func loadTransactionsFromDisk() ([]string, []parsedTxn, []string) {
	dir := mt940Dir()
	matches, err := filepath.Glob(filepath.Join(dir, "*.mt940"))
	if err != nil {
		return nil, nil, []string{"invalid mt940 pattern"}
	}
	sort.Strings(matches)

	warnings := []string{}
	transactions := []parsedTxn{}
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to read %s: %v", path, err))
			continue
		}
		message, err := parseMT940(data)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to parse %s: %v", path, err))
			continue
		}
		for _, txn := range parseTransactions(message.Transactions) {
			date, err := parseValueDate(txn.ValueDate)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("invalid date in %s: %v", path, err))
				continue
			}
			amount, err := parseAmount(txn.Amount)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("invalid amount in %s: %v", path, err))
				continue
			}
			transactions = append(transactions, parsedTxn{
				Date:      date,
				Amount:    amount,
				DCMark:    txn.DCMark,
				Reference: txn.Reference,
				Details:   txn.Details,
			})
		}
	}
	return matches, transactions, warnings
}

func parseValueDate(raw string) (string, error) {
	if len(raw) != 6 {
		return "", fmt.Errorf("invalid value date")
	}
	year, err := parseYear(raw[0:2])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d-%s-%s", year, raw[2:4], raw[4:6]), nil
}

func parseAmount(raw string) (float64, error) {
	clean := strings.ReplaceAll(raw, ",", ".")
	value, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func extractAutoTranches(transactions []parsedTxn) []trancheInput {
	auto := []trancheInput{}
	for _, txn := range transactions {
		if txn.DCMark != "C" {
			continue
		}
		upper := strings.ToUpper(txn.Details)
		if strings.Contains(upper, "ROZL") || strings.Contains(upper, "FX") || strings.Contains(upper, "WALUT") {
			auto = append(auto, trancheInput{
				Date:   txn.Date,
				Amount: txn.Amount,
				Rate:   0,
			})
		}
	}
	return auto
}

func buildTrancheQueue(tranches []trancheInput, cache map[string]nbpRateResult, report *calculateResponse) []trancheState {
	queue := make([]trancheState, 0, len(tranches))
	for _, tranche := range tranches {
		source := "manual"
		if tranche.Rate == 0 {
			source = "statement"
			nbp := fetchNBPRateCached(cache, tranche.Date)
			if nbp.Err != nil {
				report.Warnings = append(report.Warnings, fmt.Sprintf("nbp %s: %v", tranche.Date, nbp.Err))
			} else {
				tranche.Rate = nbp.Mid
			}
		}
		reportIndex := len(report.Tranches)
		report.Tranches = append(report.Tranches, trancheReport{
			Date:      tranche.Date,
			Rate:      tranche.Rate,
			Amount:    tranche.Amount,
			Remaining: tranche.Amount,
			Usages:    []trancheUsageEntry{},
			Source:    source,
		})
		if source == "statement" {
			report.Tranches[reportIndex].SourceNote = "auto from statement"
		}
		queue = append(queue, trancheState{
			Date:       tranche.Date,
			Amount:     tranche.Amount,
			Rate:       tranche.Rate,
			Source:     source,
			SourceNote: report.Tranches[reportIndex].SourceNote,
			Original:   tranche.Amount,
			ReportIdx:  reportIndex,
		})
	}
	return queue
}

func buildReport(transactions []parsedTxn, tranches []trancheInput) calculateResponse {
	report := calculateResponse{
		Transactions:   []reportTransaction{},
		Warnings:       []string{},
		UncoveredDates: map[string]float64{},
		Tranches:       []trancheReport{},
	}

	nbpCache := map[string]nbpRateResult{}
	queue := buildTrancheQueue(tranches, nbpCache, &report)

	for _, txn := range transactions {
		if txn.DCMark != "D" {
			continue
		}
		rate := fetchNBPRateCached(nbpCache, txn.Date)
		if rate.Err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("nbp %s: %v", txn.Date, rate.Err))
		}

		remaining := txn.Amount
		transactionReport := reportTransaction{
			Date:        txn.Date,
			Reference:   txn.Reference,
			Amount:      txn.Amount,
			NbpRate:     rate.Mid,
			FXTotalDiff: 0,
			Usages:      []reportUsage{},
		}

		for remaining > 0 && len(queue) > 0 {
			current := &queue[0]
			use := current.Amount
			if use > remaining {
				use = remaining
			}
			diff := 0.0
			if current.Rate > 0 && rate.Mid > 0 {
				diff = (rate.Mid - current.Rate) * use
			}
			transactionReport.Usages = append(transactionReport.Usages, reportUsage{
				TransactionDate: txn.Date,
				TransactionRef:  txn.Reference,
				AmountUsed:      use,
				TrancheDate:     current.Date,
				TrancheRate:     current.Rate,
				NbpRate:         rate.Mid,
				FXDifference:    diff,
			})
			transactionReport.FXTotalDiff += diff

			if current.ReportIdx >= 0 && current.ReportIdx < len(report.Tranches) {
				entry := trancheUsageEntry{
					TransactionDate: txn.Date,
					TransactionRef:  txn.Reference,
					AmountUsed:      use,
					NbpRate:         rate.Mid,
					FXDifference:    diff,
					Remaining:       current.Amount - use,
				}
				report.Tranches[current.ReportIdx].Usages = append(report.Tranches[current.ReportIdx].Usages, entry)
				report.Tranches[current.ReportIdx].Remaining = current.Amount - use
			}

			current.Amount -= use
			remaining -= use
			if current.Amount <= 0 {
				queue = queue[1:]
			}
		}

		if remaining > 0 {
			report.Warnings = append(report.Warnings, fmt.Sprintf("missing tranche coverage for %s: %.2f", txn.Date, remaining))
			report.UncoveredDates[txn.Date] += remaining
		}

		report.Summary.TotalFXDifference += transactionReport.FXTotalDiff
		report.Summary.TotalOutflow += txn.Amount
		report.Summary.TotalCovered += txn.Amount - remaining
		report.Summary.MissingCoverage += remaining
		report.Transactions = append(report.Transactions, transactionReport)
	}

	if report.Summary.MissingCoverage > 0 {
		report.Error = "Missing tranche coverage. Please add tranches or statements to cover all outgoing transactions."
	}

	return report
}

func writeJSON(c echo.Context, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("encode error: %v", err))
	}
	return c.Blob(http.StatusOK, "application/json", data)
}
