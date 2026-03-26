# Bank Statement Support

This document outlines which banks/products are supported via automatic PDF upload and which require a manual CSV/Excel export.

---

## ✅ Supported: PDF Upload (Automatic Parsing)

These banks have been tested and confirmed to work with the `/v1/statements/upload` endpoint. Simply upload the PDF and the backend will extract transactions automatically.

| # | Bank / Product | Account Type | Encryption | Notes |
|---|---|---|---|---|
| 1 | **BCA** | Credit Card | None | Standard `Td`/`Tj` layout |
| 2 | **BRImo** | Savings / Debit e-Statement | None | Standard `Td`/`Tj` layout |
| 3 | **BRI Ovo U Card** | Credit Card | AES-128 (password required) | Uses `BT X Y Td` column layout; CR payments filtered out |
| 4 | **BRI Tokopedia Card** | Credit Card | AES-128 (password required) | Same layout as BRI Ovo U Card — shared BRI issuing system |
| 5 | **AEON** | Credit Card | AES-128 (password required) | Uses `DD Mon YYYY` date format |
| 6 | **DBS** | Credit Card | AES-128 (password required) | Uses `Tm`/`TJ` matrix layout; parsed natively via `pdfcpu` patch |
| 7 | **Seabank** | Digital Bank / Debit | Custom +28 Caesar Cipher | Uses heavy obfuscation (Octal escapes + Caesar Shift) |

> **Password-protected PDFs:** For encrypted statements (BRI co-branded, DBS), pass the `password` field in the form-data upload request alongside the PDF file.

---

## ⚠️ Unsupported: PDF Upload — Use CSV Export Instead

These banks export PDFs where all text has been converted to **vector path drawings** (outlined fonts). Standard PDF text extraction tools — including `pdfcpu`, `pdftotext` (Poppler), and regex-based parsers — cannot read vector-outlined text.

| # | Bank / Product | Reason | Workaround |
|---|---|---|---|
| 1 | **Superbank** (`000005060116-YYYY-MM-statement.pdf`) | All text is rendered as Bezier vector curves (`c`, `l`, `m` operators). Zero readable text bytes in the raw stream — confirmed via `pdftotext` (Poppler) and `pdfcpu`. | Export transactions as **CSV** from the Superbank app or website |

### How to Export from Superbank (CSV)
1. Open the **Superbank** app or web portal.
2. Navigate to **Transaction History**.
3. Tap/click **Export** or **Download**.
4. Select **CSV** format and choose the date range.
5. Upload the `.csv` file to our application's CSV upload endpoint (coming soon).

---

## 🔮 Future Support

| Bank | Status | Notes |
|---|---|---|
| Superbank (CSV upload) | Planned | Requires a new `/v1/statements/upload-csv` endpoint and parser |
| Mandiri | Not tested | Needs a sample statement to analyze |
| CIMB Niaga | Not tested | Needs a sample statement to analyze |
| Jenius / SMBC | Not tested | Likely uses vector PDF like Superbank |

---

## Technical Notes

- **Why do some PDFs need passwords?** Banks encrypt PDF statements to prevent unauthorised access. BRI and DBS use AES-128 encryption. You must pass your PDF password as the `password` field in the upload form.
- **Why can't we parse Superbank PDFs?** Superbank deliberately exports PDFs with "outlined" text (all characters converted to vector shapes). This is a security feature to prevent screen scrapers. The only options are OCR (expensive, unreliable) or using the bank's own data export feature (CSV).
- **The `pdfcpu` patch:** Our backend uses a patched local fork of the `pdfcpu` library (under `patches/pdfcpu/`) to handle AES-128 encrypted statements where the encryption dictionary uses bit-length (`128`) instead of the standard byte-length format. This patch lives in `patches/pdfcpu/pkg/pdfcpu/crypto.go` and `read.go`.
