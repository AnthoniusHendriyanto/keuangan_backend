package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jung-kurt/gofpdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func main() {
	// 1. Generate the unencrypted mock PDF with DBS-style Tm and TJ layout
	pdf := gofpdf.New("P", "pt", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Courier", "", 10)

	// Write basic text to mimic the content without perfectly replicating the Tm/TJ raw operators
	// The primary goal of this mock is to reproduce the AESV2 Length:128 encryption bug for the GitHub issue.
	
	pdf.Text(74.8, 119.3, "03/02/2026")

	// Transaction 1
	pdf.Text(67.4, 402.3, "02/05")
	pdf.Text(242.0, 402.3, "MOCK RESTAURANT JAKARTA")
	pdf.Text(518.5, 402.3, "150,000")

	// Transaction 2 (Credit)
	pdf.Text(67.4, 420.3, "02/10")
	pdf.Text(242.0, 420.3, "PAYMENT RECEIVED THANK YOU")
	pdf.Text(518.5, 420.3, "2,000,000CR")

	// Transaction 3
	pdf.Text(67.4, 440.3, "02/15")
	pdf.Text(242.0, 440.3, "MOCK INSTALLMENT (1/12)")
	pdf.Text(518.5, 440.3, "300,000")

	unencryptedPath := "mock_dbs_unencrypted.pdf"
	if err := pdf.OutputFileAndClose(unencryptedPath); err != nil {
		log.Fatalf("Failed to generate PDF: %v", err)
	}
	fmt.Println("Generated unencrypted mock PDF:", unencryptedPath)

	// 2. Encrypt the file using AES-128 (AESV2)
	encryptedPath := "mock_dbs_encrypted.pdf"
	
	conf := model.NewDefaultConfiguration()
	conf.UserPW = "password123"
	conf.OwnerPW = "owner123"
	conf.EncryptUsingAES = true
	conf.EncryptKeyLength = 128

	err := api.EncryptFile(unencryptedPath, encryptedPath, conf)
	if err != nil {
		log.Fatalf("Failed to encrypt PDF: %v", err)
	}

	fmt.Println("Successfully encrypted mock PDF to:", encryptedPath)
	os.Remove(unencryptedPath)
}
