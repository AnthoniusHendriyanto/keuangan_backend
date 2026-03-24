package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run dump_text.go <path_to_pdf>")
	}

	filePath := os.Args[1]

	// Write PDF to temp file
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "pdf-dump-*")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	err = api.ExtractContentFile(filePath, tmpDir, nil, nil)
	if err != nil {
		log.Fatalf("Failed to extract content: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(tmpDir, "*"))
	fmt.Printf("Extracted %d content files (%d bytes PDF)\n\n", len(files), len(data))
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		fmt.Printf("=== %s ===\n", filepath.Base(f))
		fmt.Println(string(content))
		fmt.Println()
	}
}
