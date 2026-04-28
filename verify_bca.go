package main

import (
	"fmt"
	"os"
	"keuangan_backend/internal/pdf"
)

func main() {
	filePath := `C:\Users\Administrator\Documents\18618881_10042026_1775911534281.pdf`
	password := "22071998"

	f, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Failed to open file: %v\n", err)
		return
	}
	defer f.Close()

	parser := &pdf.Parser{}
	transactions, err := parser.ParseStatement(f, password)
	if err != nil {
		fmt.Printf("ParseStatement failed: %v\n", err)
		return
	}

	fmt.Printf("Extracted %d transactions from BCA Credit Card:\n\n", len(transactions))
	for i, tx := range transactions {
		fmt.Printf("[%d] %s | Rp %d | %s\n    Desc: %s\n",
			i+1,
			tx.TransactionDate.Format("02/01/2006"),
			tx.AmountIDR,
			tx.Category,
			tx.Description)
	}
}
