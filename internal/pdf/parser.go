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
	pages, err := p.extractPages(rs)
	if err != nil {
		return nil, err
	}

	// Try multi-page strategies first (AEON uses one composite content)
	combined := strings.Join(pages, "\n--- PAGE BREAK ---\n")

	// Strategy 1: Format A (plain text DD/MM/YYYY)
	if txs := p.extractFormatA(combined); len(txs) > 0 {
		return txs, nil
	}

	// Strategy 2: AEON content stream from combined pages
	if txs := p.extractFromContentStream(combined); len(txs) > 0 {
		return txs, nil
	}

	// Strategy 3: BCA Debit — process EACH PAGE independently to avoid Y-coordinate bleeding
	var allTxs []model.ExtractedTransaction
	globalIdx := 0
	for _, pageContent := range pages {
		txs := p.extractBCADebitPage(pageContent, globalIdx)
		allTxs = append(allTxs, txs...)
		globalIdx += len(txs)
	}
	if len(allTxs) > 0 {
		return allTxs, nil
	}

	return nil, ErrNoTransactions
}

// ParseStatementFromBytes is a convenience wrapper for byte slices.
func (p *Parser) ParseStatementFromBytes(data []byte) ([]model.ExtractedTransaction, error) {
	return p.ParseStatement(bytes.NewReader(data))
}

// extractPages pulls raw content from each page of the PDF.
func (p *Parser) extractPages(rs io.ReadSeeker) ([]string, error) {
	data, err := io.ReadAll(rs)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFile, err)
	}

	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return nil, fmt.Errorf("%w: file does not start with PDF header", ErrInvalidFile)
	}

	tmpFile, err := os.CreateTemp("", "statement-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("%w: could not create temp file", ErrInvalidFile)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("%w: could not write temp file", ErrInvalidFile)
	}
	tmpFile.Close()

	tmpDir, err := os.MkdirTemp("", "pdf-extract-*")
	if err != nil {
		return nil, fmt.Errorf("%w: could not create temp dir", ErrInvalidFile)
	}
	defer os.RemoveAll(tmpDir)

	err = api.ExtractContentFile(tmpPath, tmpDir, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFile, err)
	}

	files, err := filepath.Glob(filepath.Join(tmpDir, "*"))
	if err != nil || len(files) == 0 {
		return nil, ErrEmptyPDF
	}

	var pages []string
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(content))
		if text != "" {
			pages = append(pages, text)
		}
	}

	if len(pages) == 0 {
		return nil, ErrEmptyPDF
	}

	return pages, nil
}

// extractBCADebitPage parses BCA Tabungan/Debit statements page by page.
// Format: Td sets position, Tj sets text. Each transaction row appears by Y-coordinate.
// Columns: ~43 = date (DD/MM), ~88 = description, ~194 = detail lines, ~400 = amount, ~441 = DB/CR
func (p *Parser) extractBCADebitPage(content string, startIdx int) []model.ExtractedTransaction {
	// Match: X Y Td followed by (text) Tj
	// Example: "43.25 575.99 Td\n(26/02)Tj"
	tdTjRegex := regexp.MustCompile(`([\d.]+)\s+([\d.]+)\s+Td\s*\n(?:/\S+\s+Tw\s*\n)?(?:/\S+\s+\S+\s+\S+\s+\S+\s+Tc\s*\n)?(?:\S+\s+Tc\s*\n)?(?:\S+\s+Tw\s*\n)?\(([^)]*)\)\s*Tj`)

	type token struct {
		x, y float64
		text string
	}

	var tokens []token
	matches := tdTjRegex.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		x, _ := strconv.ParseFloat(m[1], 64)
		y, _ := strconv.ParseFloat(m[2], 64)
		tokens = append(tokens, token{x: x, y: y, text: strings.TrimSpace(m[3])})
	}

	if len(tokens) == 0 {
		return nil
	}

	// BCA short-date regex: exactly DD/MM with no year
	shortDateRe := regexp.MustCompile(`^\d{2}/\d{2}$`)
	// Amount regex: numeric with dots/commas
	amountRe := regexp.MustCompile(`^[\d,]+\.\d{2}$`)

	// Group tokens by Y (rows within ~2pt tolerance)
	type row struct {
		y      float64
		tokens []token
	}

	var rows []row
	const yTol = 2.0
	for _, t := range tokens {
		found := false
		for i := range rows {
			if abs64(rows[i].y-t.y) <= yTol {
				rows[i].tokens = append(rows[i].tokens, t)
				found = true
				break
			}
		}
		if !found {
			rows = append(rows, row{y: t.y, tokens: []token{t}})
		}
	}

	// Sort rows descending by Y (PDF origin is bottom-left, so higher Y = higher on page)
	// We want top-to-bottom order → sort descending
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].y < rows[j].y {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}

	// BCA transactions appear in blocks anchored by a date token at ~X=43
	// Column layout (approx): date=43, type=88, detail=194, amount=~398-415, db/cr=~441, balance=~530
	type txBlock struct {
		dateStr  string
		descParts []string
		amount   string
		isDB     bool
	}

	var blocks []txBlock
	var current *txBlock

	for _, row := range rows {
		// Find date column token (X≈43)
		var dateToken, typeToken, amtToken, dbToken string
		var detailTokens []string

		for _, t := range row.tokens {
			switch {
			case t.x < 70 && shortDateRe.MatchString(t.text):
				dateToken = t.text
			case t.x >= 70 && t.x < 180:
				typeToken = t.text
			case t.x >= 180 && t.x < 380:
				if t.text != "" {
					detailTokens = append(detailTokens, t.text)
				}
			case t.x >= 380 && t.x < 460 && amountRe.MatchString(t.text):
				amtToken = t.text
			case t.x >= 430 && t.x < 470 && (t.text == "DB" || t.text == "CR"):
				dbToken = t.text
			}
		}

		// New transaction block when we see a date
		if dateToken != "" {
			if current != nil {
				blocks = append(blocks, *current)
			}
			current = &txBlock{dateStr: dateToken, isDB: false}
			if typeToken != "" {
				current.descParts = append(current.descParts, typeToken)
			}
			for _, d := range detailTokens {
				current.descParts = append(current.descParts, d)
			}
			if amtToken != "" {
				current.amount = amtToken
			}
			if dbToken == "DB" {
				current.isDB = true
			}
		} else if current != nil {
			// Continuation row for current block
			if typeToken != "" {
				current.descParts = append(current.descParts, typeToken)
			}
			for _, d := range detailTokens {
				current.descParts = append(current.descParts, d)
			}
			if amtToken != "" && current.amount == "" {
				current.amount = amtToken
			}
			if dbToken == "DB" {
				current.isDB = true
			}
		}
	}
	if current != nil {
		blocks = append(blocks, *current)
	}

	// Parse statement year from header (look for FEBRUARI 2026 / MARET 2026 etc.)
	year := time.Now().Year()
	yearRe := regexp.MustCompile(`(?:JANUARI|FEBRUARI|MARET|APRIL|MEI|JUNI|JULI|AGUSTUS|SEPTEMBER|OKTOBER|NOVEMBER|DESEMBER)\s+(\d{4})`)
	if ym := yearRe.FindStringSubmatch(content); len(ym) > 1 {
		year, _ = strconv.Atoi(ym[1])
	}

	// Convert blocks to transactions
	var transactions []model.ExtractedTransaction
	idx := 0
	for _, b := range blocks {
		// Skip if no amount or not a debit (we only track spending)
		if b.amount == "" || !b.isDB {
			continue
		}

		// Parse date: DD/MM with year from header
		parts := strings.Split(b.dateStr, "/")
		if len(parts) != 2 {
			continue
		}
		day, err1 := strconv.Atoi(parts[0])
		month, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			continue
		}
		date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

		amount, err := parseIDRAmount(b.amount)
		if err != nil || amount <= 0 {
			continue
		}

		// Build description from parts, filtering noise
		var descParts []string
		for _, d := range b.descParts {
			d = strings.TrimSpace(d)
			// Skip reference/technical lines (TGL:, QR, addresses)
			if d == "" || strings.HasPrefix(d, "TGL:") || strings.HasPrefix(d, "QR ") ||
				strings.HasPrefix(d, "CUST NO.:") || strings.HasPrefix(d, "00000.00") ||
				d == "DB" || d == "CR" || len(d) < 2 {
				continue
			}
			descParts = append(descParts, d)
		}
		desc := strings.Join(descParts, " - ")
		desc = cleanDescription(desc)
		if desc == "" {
			continue
		}

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

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
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
