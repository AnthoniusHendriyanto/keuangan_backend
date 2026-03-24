package pdf

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{"valid date", "15/03/2026", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), false},
		{"end of year", "31/12/2025", time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), false},
		{"invalid format", "2026-03-15", time.Time{}, true},
		{"empty string", "", time.Time{}, true},
		{"partial date", "15/03", time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("parseDate(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseIDRAmount(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"simple thousands", "55.000", 55000, false},
		{"millions", "1.250.000", 1250000, false},
		{"no separator", "500", 500, false},
		{"with comma decimals", "55.000,00", 55000, false},
		{"large amount with comma", "12.500.000,50", 12500000, false},
		{"single digit", "5", 5, false},
		{"empty string", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIDRAmount(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIDRAmount(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseIDRAmount(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestGuessCategory(t *testing.T) {
	tests := []struct {
		desc     string
		expected string
	}{
		{"STARBUCKS COFFEE SUDIRMAN", "Food & Beverage"},
		{"SBX GRAND INDONESIA", "Food & Beverage"},
		{"MCDONALD TANGERANG", "Food & Beverage"},
		{"KFC BINTARO", "Food & Beverage"},
		{"GRAB CAR", "Transport"},
		{"GOJEK RIDE", "Transport"},
		{"PERTAMINA SPBU", "Transport"},
		{"SHELL FUEL", "Transport"},
		{"TOLL JAGORAWI", "Transport"},
		{"TOKOPEDIA MERCHANT", "Shopping"},
		{"SHOPEE PAY", "Shopping"},
		{"LAZADA ORDER", "Shopping"},
		{"NETFLIX SUBSCRIPTION", "Entertainment"},
		{"SPOTIFY PREMIUM", "Entertainment"},
		{"PLN PREPAID", "Utilities"},
		{"TELKOM INDIHOME", "Utilities"},
		{"APOTEK K24", "Health"},
		{"RUMAH SAKIT MMC", "Health"},
		{"RANDOM MERCHANT", "General"},
		{"UNKNOWN PAYEE", "General"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := guessCategory(tt.desc)
			if got != tt.expected {
				t.Errorf("guessCategory(%q) = %q, want %q", tt.desc, got, tt.expected)
			}
		})
	}
}

func TestExtractFromText(t *testing.T) {
	p := NewParser()

	// Provide a dummy PDF-like structure so extractFormatA won't fail if we bypass the file reader.
	// Actually, the easiest way to test the logic now without the PDF wrapper 
	// is to test the internal strategies directly for Format A.

	tests := []struct {
		name      string
		content   string
		wantCount int
		wantFirst struct {
			amountIDR int64
			desc      string
			category  string
		}
	}{
		{
			name:      "BCA style single row",
			content:   "15/03/2026 STARBUCKS COFFEE SUDIRMAN 55.000",
			wantCount: 1,
			wantFirst: struct {
				amountIDR int64
				desc      string
				category  string
			}{55000, "STARBUCKS COFFEE SUDIRMAN", "Food & Beverage"},
		},
		{
			name: "multiple rows",
			content: `KARTU KREDIT BCA - STATEMENT
15/03/2026 STARBUCKS COFFEE 55.000
16/03/2026 GRAB CAR JAKARTA 150.000
20/03/2026 TOKOPEDIA ORDER 1.250.000`,
			wantCount: 3,
			wantFirst: struct {
				amountIDR int64
				desc      string
				category  string
			}{55000, "STARBUCKS COFFEE", "Food & Beverage"},
		},
		{
			name: "rows with comma decimals",
			content: `15/03/2026 SBX SUDIRMAN 55.000,00
16/03/2026 PERTAMINA SPBU 350.000,00`,
			wantCount: 2,
			wantFirst: struct {
				amountIDR int64
				desc      string
				category  string
			}{55000, "SBX SUDIRMAN", "Food & Beverage"},
		},
		{
			name:      "no matching rows",
			content:   "This is a random document with no transaction data.",
			wantCount: 0,
		},
		{
			name:      "empty string",
			content:   "",
			wantCount: 0,
		},
		{
			name:      "invalid date skipped gracefully",
			content:   "99/99/9999 INVALID DATE ROW 10.000",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.extractFormatA(tt.content)
			if len(got) != tt.wantCount {
				t.Fatalf("extractFormatA() returned %d transactions, want %d", len(got), tt.wantCount)
			}
			if tt.wantCount > 0 {
				first := got[0]
				if first.AmountIDR != tt.wantFirst.amountIDR {
					t.Errorf("first.AmountIDR = %d, want %d", first.AmountIDR, tt.wantFirst.amountIDR)
				}
				if first.Description != tt.wantFirst.desc {
					t.Errorf("first.Description = %q, want %q", first.Description, tt.wantFirst.desc)
				}
				if first.Category != tt.wantFirst.category {
					t.Errorf("first.Category = %q, want %q", first.Category, tt.wantFirst.category)
				}
				if first.TempID != "pdf-0" {
					t.Errorf("first.TempID = %q, want %q", first.TempID, "pdf-0")
				}
			}
		})
	}
}

func TestExtractTransactions_MultipleRowsOrder(t *testing.T) {
	p := NewParser()
	content := `15/03/2026 GRAB CAR 150.000
20/03/2026 NETFLIX SUBSCRIPTION 199.000`

	got := p.extractFormatA(content)
	if len(got) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(got))
	}

	// First row
	if got[0].AmountIDR != 150000 {
		t.Errorf("row[0].AmountIDR = %d, want 150000", got[0].AmountIDR)
	}
	if got[0].Category != "Transport" {
		t.Errorf("row[0].Category = %q, want Transport", got[0].Category)
	}

	// Second row
	if got[1].AmountIDR != 199000 {
		t.Errorf("row[1].AmountIDR = %d, want 199000", got[1].AmountIDR)
	}
	if got[1].Category != "Entertainment" {
		t.Errorf("row[1].Category = %q, want Entertainment", got[1].Category)
	}
	if got[1].TempID != "pdf-1" {
		t.Errorf("row[1].TempID = %q, want pdf-1", got[1].TempID)
	}
}
