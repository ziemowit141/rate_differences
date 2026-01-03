package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"
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
	Error        string            `json:"error,omitempty"`
	Transactions []transactionView `json:"transactions,omitempty"`
}

type transactionsResponse struct {
	Files []fileTransactions `json:"files"`
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	e := echo.New()
	e.GET("/health", healthHandler)
	e.GET("/transactions", transactionsHandler)

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
		dir = "mt940s"
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
		result := fileTransactions{File: path}
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

func writeJSON(c echo.Context, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("encode error: %v", err))
	}
	return c.Blob(http.StatusOK, "application/json", data)
}
