# Bank Statement Support

This document outlines which banks/products are supported via automatic PDF upload and which require a manual CSV/Excel export.

---

## ✅ Supported: PDF Upload (Automatic Parsing)

These banks have been tested and confirmed to work with the `/v1/statements/upload` endpoint. Simply upload the PDF and the backend will extract transactions automatically.

| # | Bank / Product | Account Type | Encryption | Notes |
|---|---|---|---|---|
| 1 | **BCA** | Credit Card / Debit (Standard & Xpresi) | None | Handles `TGL:` labels and CID-based hex `F2` encoding |
| 2 | **BRImo** | Savings / Debit e-Statement | None | Standard `Td`/`Tj` layout |
| 3 | **BRI Ovo U Card** | Credit Card | AES-128 (password required) | Uses `BT X Y Td` column layout; CR payments filtered out |
| 4 | **BRI Tokopedia Card** | Credit Card | AES-128 (password required) | Same layout as BRI Ovo U Card — shared BRI issuing system |
| 5 | **AEON** | Credit Card | AES-128 (password required) | Uses `DD Mon YYYY` date format |
| 6 | **DBS** | Credit Card | AES-128 (password required) | Uses `Tm`/`TJ` matrix layout; parsed natively via `pdfcpu` patch |
| 7 | **Seabank** | Digital Bank / Debit | Custom +28 Caesar Cipher | Uses heavy obfuscation (Octal escapes + Caesar Shift) |
| 8 | **Superbank** | Digital Bank / Debit | Custom +29 Caesar Cipher | Advanced coordinate grouping; defeats vector de-obfuscation |

> **Password-protected PDFs:** For encrypted statements (BRI co-branded, DBS), pass the `password` field in the form-data upload request alongside the PDF file.

---

## 🔮 Future Support

| Bank | Status | Notes |
|---|---|---|
| Mandiri | Not tested | Needs a sample statement to analyze |
| CIMB Niaga | Not tested | Needs a sample statement to analyze |
| Jenius / SMBC | Not tested | Likely uses vector PDF or similar obfuscation |

---

## Technical Notes

- **Why do some PDFs need passwords?** Banks encrypt PDF statements to prevent unauthorised access. BRI and DBS use AES-128 encryption. You must pass your PDF password as the `password` field in the upload form.
- **Advanced De-obfuscation (Seabank/Superbank):** These banks use non-standard font encodings (Caesar shifts) and octal escape sequences to obfuscate transaction data. Our parser uses custom decoders (+28 for Seabank, +29 for Superbank) to reconstruct the readable text from the raw PDF content streams.
- **BCA Xpresi & Credit:** The Xpresi format uses non-standard labels like `TGL:`. The **Credit Card** format uses CID-based hex encoding (Font F2) for characters, which our parser decodes using a custom ToUnicode mapping. It also handles the Indonesian `DD-MMM` date format (e.g., `13-MAR`).
- **The `pdfcpu` patch:** Our backend uses a patched local fork of the `pdfcpu` library (under `patches/pdfcpu/`) to handle AES-128 encrypted statements where the encryption dictionary uses bit-length (`128`) instead of the standard byte-length format. This patch lives in `patches/pdfcpu/pkg/pdfcpu/crypto.go` and `read.go`.
