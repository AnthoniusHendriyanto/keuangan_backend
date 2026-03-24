package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"keuangan_backend/internal/model"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// Common errors returned by the parser.
var (
	ErrEmptyPDF       = errors.New("pdf: extracted text is empty, the file may be image-based or encrypted")
	ErrInvalidFile    = errors.New("pdf: failed to read or parse the file, ensure it is a valid PDF")
	ErrNoTransactions = errors.New("pdf: no transaction rows could be extracted from the statement")
)

// transactionRegex matches rows like: 15/03/2026 STARBUCKS COFFEE 55.000,00
// Supports amounts with dots (thousands) and optional comma (decimals).
var transactionRegex = regexp.MustCompile(`(\d{2}/\d{2}/\d{4})\s+(.+?)\s+([\d.]+(?:,\d{2})?)`)

// Parser handles extracting transactions from PDF files.
type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

// ParseStatement processes an Indonesian bank statement PDF.
// Returns ErrInvalidFile if the PDF cannot be read, ErrEmptyPDF if no text
// is extractable, and ErrNoTransactions if the regex finds zero matches.
func (p *Parser) ParseStatement(rs io.ReadSeeker) ([]model.ExtractedTransaction, error) {
	content, err := p.extractText(rs)
	if err != nil {
		return nil, err
	}

	transactions := p.extractTransactions(content)
	if len(transactions) == 0 {
		return nil, ErrNoTransactions
	}

	return transactions, nil
}

// extractText pulls raw text content from the PDF using pdfcpu.
// It extracts content streams to a temp directory and reads them back.
func (p *Parser) extractText(rs io.ReadSeeker) (string, error) {
	// Read all bytes to write to a temp file (pdfcpu file-based API is more reliable)
	data, err := io.ReadAll(rs)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidFile, err)
	}

	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return "", fmt.Errorf("%w: file does not start with PDF header", ErrInvalidFile)
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "statement-*.pdf")
	if err != nil {
		return "", fmt.Errorf("%w: could not create temp file", ErrInvalidFile)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("%w: could not write temp file", ErrInvalidFile)
	}
	tmpFile.Close()

	// Extract content to a temp directory
	tmpDir, err := os.MkdirTemp("", "pdf-extract-*")
	if err != nil {
		return "", fmt.Errorf("%w: could not create temp dir", ErrInvalidFile)
	}
	defer os.RemoveAll(tmpDir)

	err = api.ExtractContentFile(tmpPath, tmpDir, nil, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidFile, err)
	}

	// Read all extracted content files
	var textBuilder strings.Builder
	files, _ := filepath.Glob(filepath.Join(tmpDir, "*"))
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		textBuilder.Write(content)
		textBuilder.WriteByte('\n')
	}

	text := strings.TrimSpace(textBuilder.String())
	if text == "" {
		return "", ErrEmptyPDF
	}

	return text, nil
}

// ParseStatementFromBytes is a convenience wrapper for byte slices.
func (p *Parser) ParseStatementFromBytes(data []byte) ([]model.ExtractedTransaction, error) {
	return p.ParseStatement(bytes.NewReader(data))
}

// extractTransactions applies the regex pattern to raw text and returns
// parsed transaction rows. This is the core logic tested by unit tests.
func (p *Parser) extractTransactions(content string) []model.ExtractedTransaction {
	matches := transactionRegex.FindAllStringSubmatch(content, -1)

	var transactions []model.ExtractedTransaction
	for i, match := range matches {
		if len(match) < 4 {
			continue
		}

		dateStr := match[1]
		desc := strings.TrimSpace(match[2])
		amountStr := match[3]

		date, err := parseDate(dateStr)
		if err != nil {
			continue // skip rows with unparseable dates
		}

		amount, err := parseIDRAmount(amountStr)
		if err != nil {
			continue // skip rows with unparseable amounts
		}

		transactions = append(transactions, model.ExtractedTransaction{
			TempID:          fmt.Sprintf("pdf-%d", i),
			AmountIDR:       amount,
			TransactionDate: date,
			Description:     desc,
			Category:        guessCategory(desc),
		})
	}

	return transactions
}

// parseDate tries DD/MM/YYYY format (standard Indonesian bank statement).
func parseDate(s string) (time.Time, error) {
	return time.Parse("02/01/2006", s)
}

// parseIDRAmount normalises IDR amount strings.
// Handles: "55.000" → 55000, "1.250.000" → 1250000, "55.000,00" → 55000.
func parseIDRAmount(s string) (int64, error) {
	// Remove comma-decimal portion (e.g. ",00")
	if idx := strings.Index(s, ","); idx != -1 {
		s = s[:idx]
	}
	// Remove dot-thousands separators
	s = strings.ReplaceAll(s, ".", "")

	return strconv.ParseInt(s, 10, 64)
}

// guessCategory maps descriptions to categories using keyword matching.
func guessCategory(description string) string {
	desc := strings.ToLower(description)
	for _, rule := range categoryRules {
		for _, keyword := range rule.keywords {
			if strings.Contains(desc, keyword) {
				return rule.category
			}
		}
	}
	return "General"
}

type categoryRule struct {
	category string
	keywords []string
}

var categoryRules = []categoryRule{
	{"Food & Beverage", []string{"starbucks", "sbx", "mcdonald", "kfc", "burger", "pizza", "cafe", "coffee", "resto", "bakery"}},
	{"Transport", []string{"grab", "gojek", "pertamina", "shell", "toll", "parkir", "parking"}},
	{"Shopping", []string{"tokopedia", "shopee", "lazada", "blibli", "zalora"}},
	{"Entertainment", []string{"netflix", "spotify", "disney", "youtube", "bioskop", "cinema"}},
	{"Utilities", []string{"pln", "pdam", "telkom", "indosat", "xl", "tri"}},
	{"Health", []string{"apotek", "pharmacy", "hospital", "rumah sakit", "klinik"}},
}
