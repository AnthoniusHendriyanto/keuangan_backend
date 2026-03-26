package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: dump_seabank <file.pdf>")
		os.Exit(1)
	}
	pdfPath := os.Args[1]

	// Use pdftotext to extract raw text (plain text mode)
	// Since we can't do pdftotext, let's extract actual text via pdfcpu's content stream info
	// and dump it alongside any recognizable sequences

	// First: try to grep for readable ASCII sequences inside the PDF binary
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		os.Exit(1)
	}

	// Extract all strings between ( and ) that look like Tj sequences
	re := regexp.MustCompile(`\(([^\x00-\x1f\x80-\xff]{4,})\)\s*Tj`)
	matches := re.FindAllStringSubmatch(string(data), -1)

	outPath := filepath.Join(filepath.Dir(pdfPath), "seabank_raw_strings.txt")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== Readable Tj strings from %s ===\n\n", filepath.Base(pdfPath)))
	for _, m := range matches {
		sb.WriteString(m[1] + "\n")
	}

	os.WriteFile(outPath, []byte(sb.String()), 0644)
	fmt.Println("Written to:", outPath)
	fmt.Printf("Found %d readable string sequences\n", len(matches))

	// Also try to find the ToUnicode CMap object using pdfcpu info
	cmd := exec.Command("go", "run", "./cmd/dump_dbs/main.go", pdfPath)
	out, _ := cmd.CombinedOutput()
	fmt.Println(string(out[:min(len(out), 200)]))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
