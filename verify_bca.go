package main

import (
	"fmt"
	"os"
	"strings"
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

	// Dump raw pages for inspection
	pages, err := parser.DumpPages(f, password)
	if err != nil {
		fmt.Printf("DumpPages failed: %v\n", err)
		return
	}
	fmt.Printf("Total pages: %d\n", len(pages))

	for pi, page := range pages {
		fmt.Printf("\n===== PAGE %d (len=%d) =====\n", pi+1, len(page))
		count := 0
		for _, line := range strings.Split(page, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.Contains(line, "Td") || strings.Contains(line, "Tm") ||
				strings.Contains(line, "Tj") || strings.Contains(line, "BT") || strings.Contains(line, "ET") {
				fmt.Println(line)
				count++
				if count >= 60 {
					fmt.Println("... (truncated)")
					break
				}
			}
		}
	}
}
