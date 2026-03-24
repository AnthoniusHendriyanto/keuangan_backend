package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"keuangan_backend/internal/pdf"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run test_pdf.go <path_to_pdf>")
	}

	filePath := os.Args[1]
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	parser := pdf.NewParser()
	transactions, err := parser.ParseStatement(file, "")
	if err != nil {
		log.Fatalf("Parse error: %v", err)
	}

	fmt.Printf("Extracted %d transactions:\n\n", len(transactions))
	out, _ := json.MarshalIndent(transactions, "", "  ")
	fmt.Println(string(out))
}
