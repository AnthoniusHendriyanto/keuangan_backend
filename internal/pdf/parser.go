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

// Regex patterns for different statement formats

// Format A: DD/MM/YYYY DESCRIPTION 55.000 (dot-thousands, e.g. BCA)
var regexFormatA = regexp.MustCompile(`(\d{2}/\d{2}/\d{4})\s+(.+?)\s+([\d.]+(?:,\d{2})?)$`)

// Format B: DD Mon YYYY (from PDF content streams, e.g. AEON)
// After text reconstruction, lines look like: "12 Feb 2026 | 12 Feb 2026 | MERCHANT NAME | 160,000"
var regexFormatB = regexp.MustCompile(`(\d{1,2}\s+\w{3}\s+\d{4})\s+\d{1,2}\s+\w{3}\s+\d{4}\s+(.+?)\s+([\d,]+)\s*$`)

// Format C: raw Tj/TJ content stream extraction
var regexTjText = regexp.MustCompile(`\(([^)]*)\)\s*Tj`)

// Indonesian month names for parsing
var monthMap = map[string]time.Month{
	"Jan": time.January, "Feb": time.February, "Mar": time.March,
	"Apr": time.April, "Mei": time.May, "Jun": time.June,
	"Jul": time.July, "Agu": time.August, "Ags": time.August,
	"Sep": time.September, "Okt": time.October, "Oct": time.October,
	"Nov": time.November, "Des": time.December, "Dec": time.December,
}

// Parser handles extracting transactions from PDF files.
type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

// ParseStatement processes an Indonesian bank statement PDF.
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

// ParseStatementFromBytes is a convenience wrapper for byte slices.
func (p *Parser) ParseStatementFromBytes(data []byte) ([]model.ExtractedTransaction, error) {
	return p.ParseStatement(bytes.NewReader(data))
}

// extractText pulls raw content from the PDF.
func (p *Parser) extractText(rs io.ReadSeeker) (string, error) {
	data, err := io.ReadAll(rs)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidFile, err)
	}

	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return "", fmt.Errorf("%w: file does not start with PDF header", ErrInvalidFile)
	}

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

	tmpDir, err := os.MkdirTemp("", "pdf-extract-*")
	if err != nil {
		return "", fmt.Errorf("%w: could not create temp dir", ErrInvalidFile)
	}
	defer os.RemoveAll(tmpDir)

	err = api.ExtractContentFile(tmpPath, tmpDir, nil, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidFile, err)
	}

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

// extractTransactions tries multiple parsing strategies and returns the best result.
func (p *Parser) extractTransactions(content string) []model.ExtractedTransaction {
	// Strategy 1: Try Format A (plain text DD/MM/YYYY)
	if txs := p.extractFormatA(content); len(txs) > 0 {
		return txs
	}

	// Strategy 2: Try reconstructing text from PDF Tj operators (AEON-style)
	if txs := p.extractFromContentStream(content); len(txs) > 0 {
		return txs
	}

	return nil
}

// extractFormatA handles plain text statements with DD/MM/YYYY format.
func (p *Parser) extractFormatA(content string) []model.ExtractedTransaction {
	lines := strings.Split(content, "\n")
	var transactions []model.ExtractedTransaction
	idx := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		matches := regexFormatA.FindStringSubmatch(line)
		if len(matches) < 4 {
			continue
		}

		date, err := time.Parse("02/01/2006", matches[1])
		if err != nil {
			continue
		}

		amount, err := parseIDRAmount(matches[3])
		if err != nil {
			continue
		}

		transactions = append(transactions, model.ExtractedTransaction{
			TempID:          fmt.Sprintf("pdf-%d", idx),
			AmountIDR:       amount,
			TransactionDate: date,
			Description:     strings.TrimSpace(matches[2]),
			Category:        guessCategory(matches[2]),
		})
		idx++
	}

	return transactions
}

// extractFromContentStream parses PDF content stream Tj operators used by AEON and similar.
func (p *Parser) extractFromContentStream(content string) []model.ExtractedTransaction {
	// Extract all text strings from Tj operators
	tjMatches := regexTjText.FindAllStringSubmatch(content, -1)
	if len(tjMatches) == 0 {
		return nil
	}

	// Collect all text fragments
	var texts []string
	for _, m := range tjMatches {
		if len(m) >= 2 {
			texts = append(texts, strings.TrimSpace(m[1]))
		}
	}

	// Build transaction rows by identifying date patterns and grouping
	// AEON format repeats in groups: [TxDate] [BookDate] [Description] [Amount] [CR indicator]
	type rawRow struct {
		txDate string
		desc   string
		amount string
		isCR   bool
	}

	var rows []rawRow
	dateRegex := regexp.MustCompile(`^\d{1,2}\s+\w{3}\s+\d{4}$`)

	i := 0
	for i < len(texts) {
		text := texts[i]

		// Look for a transaction date
		if dateRegex.MatchString(text) {
			row := rawRow{txDate: text}

			// Next should be booking date (skip it)
			if i+1 < len(texts) && dateRegex.MatchString(texts[i+1]) {
				i += 2 // skip both dates
			} else {
				i++
			}

			// Next should be description
			if i < len(texts) {
				row.desc = texts[i]
				i++
			}

			// Next should be amount
			if i < len(texts) {
				row.amount = texts[i]
				i++
			}

			// Check for CR indicator (credit/payment)
			if i < len(texts) && strings.TrimSpace(texts[i]) == "CR" {
				row.isCR = true
				i++
			}

			rows = append(rows, row)
		} else {
			i++
		}
	}

	// Convert raw rows to transactions
	var transactions []model.ExtractedTransaction
	idx := 0
	for _, row := range rows {
		// Skip credit/payment rows
		if row.isCR {
			continue
		}

		// Skip empty descriptions or summary rows
		if row.desc == "" || strings.HasPrefix(row.desc, "SISA TAGIHAN") || strings.HasPrefix(row.desc, "Total") {
			continue
		}

		date, err := parseDateMonthName(row.txDate)
		if err != nil {
			continue
		}

		amount, err := parseIDRAmount(row.amount)
		if err != nil || amount <= 0 {
			continue
		}

		// Clean up description (remove excess spaces and location suffixes)
		desc := cleanDescription(row.desc)

		transactions = append(transactions, model.ExtractedTransaction{
			TempID:          fmt.Sprintf("pdf-%d", idx),
			AmountIDR:       amount,
			TransactionDate: date,
			Description:     desc,
			Category:        guessCategory(desc),
		})
		idx++
	}

	return transactions
}

// parseDateMonthName parses "12 Feb 2026" or "1 Mar 2026" format.
func parseDateMonthName(s string) (time.Time, error) {
	parts := strings.Fields(s)
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid date: %s", s)
	}

	day, err := strconv.Atoi(parts[0])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("invalid day: %s", parts[0])
	}

	month, ok := monthMap[parts[1]]
	if !ok {
		return time.Time{}, fmt.Errorf("unknown month: %s", parts[1])
	}

	year, err := strconv.Atoi(parts[2])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid year: %s", parts[2])
	}

	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC), nil
}

// parseDate tries DD/MM/YYYY format.
func parseDate(s string) (time.Time, error) {
	return time.Parse("02/01/2006", s)
}

// parseIDRAmount normalises IDR amount strings.
// Handles: "55.000" → 55000, "160,000" → 160000, "55.000,00" → 55000
func parseIDRAmount(s string) (int64, error) {
	s = strings.TrimSpace(s)

	// Detect format: if comma is followed by exactly 2 digits at end → decimal comma
	if regexp.MustCompile(`,\d{2}$`).MatchString(s) {
		// Format: "55.000,00" → remove decimal, then remove dots
		idx := strings.LastIndex(s, ",")
		s = s[:idx]
		s = strings.ReplaceAll(s, ".", "")
	} else {
		// Could be comma-thousands (AEON: "160,000") or dot-thousands (BCA: "55.000")
		s = strings.ReplaceAll(s, ",", "")
		s = strings.ReplaceAll(s, ".", "")
	}

	return strconv.ParseInt(s, 10, 64)
}

// cleanDescription cleans up merchant descriptions from statements.
func cleanDescription(desc string) string {
	// Remove trailing location codes like "IDN", "JAKARTA SLT", etc.
	desc = strings.TrimSpace(desc)
	// Collapse multiple spaces
	spaceRegex := regexp.MustCompile(`\s{2,}`)
	desc = spaceRegex.ReplaceAllString(desc, " ")
	return desc
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
	{"Food & Beverage", []string{"starbucks", "sbx", "mcdonald", "kfc", "burger", "pizza", "cafe", "coffee", "resto", "bakery", "kopi", "krispy", "kreme", "ippudo", "sichuan", "mie", "kari", "omurice", "kansai", "dine", "kaffeine", "teazzi", "captain"}},
	{"Transport", []string{"grab", "gojek", "pertamina", "shell", "toll", "parkir", "parking", "tiket.com", "tiket"}},
	{"Shopping", []string{"tokopedia", "shopee", "lazada", "blibli", "zalora", "giordano", "polo", "showroom"}},
	{"Entertainment", []string{"netflix", "spotify", "disney", "youtube", "bioskop", "cinema", "lounge", "lotte"}},
	{"Utilities", []string{"pln", "pdam", "telkom", "indosat", "xl", "tri"}},
	{"Health", []string{"apotek", "pharmacy", "hospital", "rumah sakit", "klinik"}},
}
