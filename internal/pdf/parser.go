package pdf

import (
	"fmt"
	"io"
	"keuangan_backend/internal/model"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpumodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// Parser handles extracting transactions from PDF files.
type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

// ParseStatement processes an Indonesian bank statement PDF.
func (p *Parser) ParseStatement(rs io.ReadSeeker) ([]model.ExtractedTransaction, error) {
	// Extract raw text from PDF
	var textBuilder strings.Builder
	err := api.ExtractTextStream(rs, nil, func(pageNr int, text string) error {
		textBuilder.WriteString(text)
		return nil
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to extract text: %w", err)
	}

	content := textBuilder.String()

	// Regex pattern for typical Indonesian CC statement rows:
	// Example: 15/03/2026 STARBUCKS COFFEE 55.000
	re := regexp.MustCompile(`(\d{2}/\d{2}/\d{4})\s+(.+?)\s+([\d.,]+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	var transactions []model.ExtractedTransaction
	for i, match := range matches {
		if len(match) < 4 {
			continue
		}

		dateStr := match[1]
		desc := strings.TrimSpace(match[2])
		amountStr := match[3]

		// Parse Date
		date, err := time.Parse("02/01/2006", dateStr)
		if err != nil {
			continue
		}

		// Parse Amount (handle dots/commas for IDR)
		cleanAmount := strings.ReplaceAll(amountStr, ".", "")
		cleanAmount = strings.ReplaceAll(cleanAmount, ",", "")
		amount, err := strconv.ParseInt(cleanAmount, 10, 64)
		if err != nil {
			continue
		}

		transactions = append(transactions, model.ExtractedTransaction{
			TempID:          fmt.Sprintf("pdf-%d", i),
			AmountIDR:       amount,
			TransactionDate: date,
			Description:     desc,
			Category:        p.guessCategory(desc),
		})
	}

	return transactions, nil
}

// guessCategory maps descriptions to categories.
func (p *Parser) guessCategory(description string) string {
	desc := strings.ToLower(description)
	switch {
	case strings.Contains(desc, "starbucks"), strings.Contains(desc, "sbx"), strings.Contains(desc, "mcdonald"), strings.Contains(desc, "kfc"):
		return "Food & Beverage"
	case strings.Contains(desc, "grab"), strings.Contains(desc, "gojek"), strings.Contains(desc, "pertamina"):
		return "Transport"
	case strings.Contains(desc, "tokopedia"), strings.Contains(desc, "shopee"):
		return "Shopping"
	case strings.Contains(desc, "netflix"), strings.Contains(desc, "spotify"):
		return "Entertainment"
	default:
		return "General"
	}
}
