package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	e := echo.New()
	e.Use(middleware.BodyLimit("50M"))
	e.GET("/health", healthHandler)
	e.GET("/transactions", transactionsHandler)
	e.POST("/upload", uploadHandler)
	e.DELETE("/files/:base", deleteHandler)

	log.Printf("mt940 api listening on %s", *addr)
	if err := e.Start(*addr); err != nil {
		log.Fatal(err)
	}
}

func healthHandler(c echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

func transactionsHandler(c echo.Context) error {
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
	for _, path := range matches {
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
		response.Files = append(response.Files, result)
	}

	return writeJSON(c, response)
}

func uploadHandler(c echo.Context) error {
	form, err := c.MultipartForm()
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid multipart form")
	}
	files := form.File["files"]
	if len(files) == 0 {
		return c.String(http.StatusBadRequest, "no files uploaded")
	}

	results := make([]uploadResult, 0, len(files))
	for index, file := range files {
		result := uploadResult{Source: file.Filename}
		tempBase := buildTempName(index)
		pdfPath := filepath.Join(statementDir(), tempBase+".pdf")
		mt940Path := filepath.Join(mt940Dir(), tempBase+".mt940")

		if err := saveUploadedFile(file, pdfPath); err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		if err := runConversion(pdfPath, mt940Path); err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		finalBase := chooseStatementBase(mt940Path)
		finalPDFPath := filepath.Join(statementDir(), finalBase+".pdf")
		finalMT940Path := filepath.Join(mt940Dir(), finalBase+".mt940")
		if err := os.Rename(pdfPath, finalPDFPath); err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		if err := os.Rename(mt940Path, finalMT940Path); err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		result.PDFPath = finalPDFPath
		result.MT940Path = finalMT940Path
		results = append(results, result)
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

func deleteHandler(c echo.Context) error {
	base := strings.TrimSpace(c.Param("base"))
	if base == "" || strings.Contains(base, "/") || strings.Contains(base, "\\") {
		return c.String(http.StatusBadRequest, "invalid base name")
	}

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
	if err := os.MkdirAll(filepath.Dir(mt940Path), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("python3", "-m", "pdftomt940", "--statement", pdfPath, "--output", mt940Path)
	cmd.Dir = repoRoot()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("conversion failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
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

func statementDir() string {
	return filepath.Join("cmd", "mt940api", "data", "statements")
}

func mt940Dir() string {
	return filepath.Join("cmd", "mt940api", "data", "mt940s")
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

func writeJSON(c echo.Context, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("encode error: %v", err))
	}
	return c.Blob(http.StatusOK, "application/json", data)
}
