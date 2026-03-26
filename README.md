# Keuangan Backend

A production-ready Golang backend powering the **True Liability Tracker** mobile application. This service is designed to track automated credit card statement liabilities and manual cash spending, providing users with a clear "true" available balance. It features an advanced AI-driven Merge Wizard and a multi-strategy PDF parsing engine.

## Key Features

- **Multi-Strategy PDF Parsing**: Automatically extracts and normalises data from complex Indonesian bank statements (BCA Credit/Debit, AEON, BRImo, DBS, and Seabank) with multi-page handling and specialized decryption (AESV2 + Caesar Cipher).
- **AI Merge Wizard**: A smart reconciliation engine using fuzzy matching (Levenshtein distance) and temporal proximity to suggest merge candidates between user-entered manual transactions and imported PDF statement rows.
- **Supabase Integration**: Native `pgxpool` connection to Supabase PostgreSQL, leveraging Row-Level Security (RLS) and Supavisor Pooler (Port 6543) for isolated multi-tenant data management.
- **Data Integrity**: Enforces strict `int64` standardisation for all IDR amounts to eliminate floating-point precision errors.
- **Clean Architecture**: Decoupled, testable layers (Handler → Repository) built on a high-speed Fiber v3 routing foundation.

---

## Tech Stack

- **Language**: Go 1.26
- **Framework**: Fiber v3
- **Database**: PostgreSQL (via Supabase) with `pgx/v5`
- **Authentication**: JWT token validation parsing (`golang-jwt/v5`)
- **PDF Extraction**: PDFCPU (`github.com/pdfcpu/pdfcpu@v0.9.1`) & `dslipak/pdf`
- **Fuzzy Logic**: Golang Levenshtein algorithm

---

## Prerequisites

- [Go 1.26+](https://go.dev/dl/) installed
- A running [Supabase](https://supabase.com/) project (PostgreSQL + Auth)
- Git

---

## Getting Started

### 1. Clone the Repository

```bash
git clone https://github.com/AnthoniusHendriyanto/keuangan_backend.git
cd keuangan_backend
```

### 2. Environment Setup

Copy the example environment file:

```bash
cp .env.example .env
```

Configure the following variables in your `.env` file:

| Variable | Description | Example |
| -------- | ----------- | ------- |
| `PORT` | The port the HTTP server binds to | `8080` |
| `SUPABASE_DB_URL` | PostgreSQL connection string | `postgres://postgres.[project]:[password]@aws-0-[region].pooler.supabase.com:6543/postgres` |
| `SUPABASE_JWT_SECRET` | Super-secret key provided by Supabase for verifying user sessions | `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpX...` |

### 3. Database Schema

Execute the following in your Supabase SQL Editor to set up the tables and RLS:

```sql
-- Credit Cards Table
CREATE TABLE public.credit_cards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    card_name TEXT NOT NULL,
    cutoff_day INTEGER NOT NULL CHECK (cutoff_day BETWEEN 1 AND 31),
    due_day INTEGER NOT NULL CHECK (due_day BETWEEN 1 AND 31),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Transactions Table
CREATE TABLE public.transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    amount_idr BIGINT NOT NULL,
    transaction_date TIMESTAMPTZ NOT NULL,
    description TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('Food & Beverage', 'Transport', 'Shopping', 'Bills', 'Entertainment', 'Groceries', 'Health', 'Education', 'Investment', 'Others', 'General', 'Utilities', 'Transfer')),
    type TEXT NOT NULL, -- 'MANUAL' or 'PDF_PARSED'
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'RECONCILED', 'DISPUTED')),
    payment_method TEXT NOT NULL, -- 'CREDIT_CARD', 'CASH', 'QR_BANK'
    credit_card_id UUID REFERENCES public.credit_cards(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Enable RLS
ALTER TABLE public.credit_cards ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.transactions ENABLE ROW LEVEL SECURITY;

-- RLS Policies
CREATE POLICY "Users can only select their own credit cards" ON public.credit_cards FOR SELECT USING (auth.uid() = user_id);
CREATE POLICY "Users can only insert their own credit cards" ON public.credit_cards FOR INSERT WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can only select their own transactions" ON public.transactions FOR SELECT USING (auth.uid() = user_id);
CREATE POLICY "Users can only update their own transactions" ON public.transactions FOR UPDATE USING (auth.uid() = user_id);
CREATE POLICY "Users can only insert their own transactions" ON public.transactions FOR INSERT WITH CHECK (auth.uid() = user_id);
```

### 4. Start Development Server

Run the main entry point:

```bash
go run ./cmd/server/main.go
```

The server will start on `http://localhost:8080`. Note: You must provide a valid Supabase `Authorization: Bearer <token>` to interact with the `/v1` endpoints.

---

## Architecture Overview

This project strictly adheres to **Clean Architecture** patterns.

### Directory Structure

```
├── cmd/
│   ├── server/       # Main Fiber API entry point
│   ├── dump_text/    # CLI tool for dumping raw PDF structure
│   └── test_pdf/     # CLI tool for dry-testing the PDF extraction engine
├── internal/
│   ├── handler/      # HTTP layer (Fiber parsing, payloads, AI Merge Wizard)
│   ├── middleware/   # Request interceptors (JWT extraction, standardisation)
│   ├── model/        # Domain structs and JSON contracts
│   ├── pdf/          # The multi-strategy parsing engine
│   └── repository/   # Data access layer (pgx Supabase wrappers)
```

### Request Lifecycle

1. Client sends a request with a Supabase JWT in the `Authorization` header.
2. `internal/middleware/auth.go` intercepts, validates the JWT signature, extracts the `sub` (user UUID), and injects it into Fiber's `c.Locals("userID")`.
3. `internal/handler/` validates the JSON payload or Multipart Form and invokes the relevant repository or library logic.
4. `internal/repository/` constructs parameterised SQL queries to safely interact with PostgreSQL. RLS guarantees multi-tenant boundaries.
5. Response is serialized back to the client.

### Advanced Component: The PDF Parser (`internal/pdf/`)

Because Indonesian banks use wildly different formats (Standard text vs custom Content Stream matrices), the parser implements a waterfall of extraction strategies on a per-page basis to avoid cross-page Y-coordinate bleeding. It also supports decrypting password-protected PDFs seamlessly.

- **Strategy 1 (Format A)**: Fallback standard DD/MM/YYYY plaintext regex capturing.
- **Strategy 2 (Format B - AEON)**: Global layout content stream Tj token reconstruction.
- **Strategy 3 (BCA Debit)**: Positional `Td` logic assembling rows by absolute Y-coordinates.
- **Strategy 4 (BRImo)**: Page-isolated positional extraction with multi-line description support.
- **Strategy 5 (DBS Credit)**: Positional reconstruction with `CR` (Credit) detection and strict Indonesian `DD/MM` date parsing.
- **Strategy 6 (BRI Tokopedia/Ovo)**: Positional extraction using `BT/Td` coordinate mapping.
- **Strategy 7 (Seabank Digital)**: Advanced reverse-engineering of +28 Caesar cipher and PDF octal escape obfuscations with 15pt fuzzy spatial grouping.

### Advanced Component: The AI Merge Wizard

When processing the `POST /v1/transactions/upload-statement` endpoint, the system doesn't just return rows. It executes the AI Merge Wizard:
1. Fetches all existing `PENDING` manual transactions for that user.
2. Cross-references the newly extracted PDF array against the DB list.
3. Calculates temporal proximity (`<= 3 days`) and checks Amount IDR equivalence.
4. If parameters align, it uses a string-distance algorithm (Levenshtein) to calculate confidence scores.
5. Emits `Merge Suggestions` in the API response payload, allowing the frontend client to prompt the user (e.g., *"Did your manual entry 'Starbucks' match the bank's 'SBX GRAND INDONESIA'?"*).

---

## Available Scripts

| Command | Description |
| ------- | ----------- |
| `go run ./cmd/server/main.go` | Boots the HTTP Fiber API server |
| `go test -v ./...` | Runs the full test suite |
| `go test -count=1 ./internal/pdf/` | Runs the PDF parsing validation suite natively (33+ table-driven tests) |
| `go run ./cmd/dump_text/main.go [path.pdf]` | Diagnostics: Dumps raw PDF text matrices into standard out |
| `go run ./cmd/test_pdf/main.go [path.pdf]` | Emulations: Runs the parser locally to see what JSON array is generated |

---

## API Guide & CURL Examples

All endpoints are prefixed with `/v1`. Authenticated routes require a Supabase JWT in the `Authorization: Bearer <TOKEN>` header.

### 1. Health & Connectivity
Check the liveness of the API and its connection to the Supabase database.
```bash
curl http://localhost:8080/v1/health
```

### 2. Credit Cards
**List Cards:**
```bash
curl -H "Authorization: Bearer <JWT>" http://localhost:8080/v1/credit-cards/
```

**Create Card:**
```bash
curl -X POST -H "Authorization: Bearer <JWT>" \
     -H "Content-Type: application/json" \
     -d '{"card_name": "BCA Everyday Card", "cutoff_day": 5, "due_day": 25}' \
     http://localhost:8080/v1/credit-cards/
```

### 3. Transactions
**List Transactions:**
```bash
curl -H "Authorization: Bearer <JWT>" "http://localhost:8080/v1/transactions/?start_date=2024-03-01T00:00:00Z"
```

**Record Manual Transaction:**
```bash
curl -X POST -H "Authorization: Bearer <JWT>" \
     -H "Content-Type: application/json" \
     -d '{
       "amount_idr": 45000,
       "transaction_date": "2024-03-24T12:00:00Z",
       "description": "Starbucks Coffee",
       "category": "Food & Beverage",
       "payment_method": "CASH"
     }' \
     http://localhost:8080/v1/transactions/
```

**Upload Statement (PDF):**
Uploads a statement and receives a JSON payload of extracted transactions + AI-suggested merges.
```bash
curl -X POST -H "Authorization: Bearer <JWT>" \
     -F "file=@/path/to/statement.pdf" \
     -F "password=optional_pw" \
     http://localhost:8080/v1/transactions/upload-statement
```

**Confirm Merge:**
Merges an extracted PDF transaction into an existing manual record to change status to `RECONCILED`.
```bash
curl -X POST -H "Authorization: Bearer <JWT>" \
     -H "Content-Type: application/json" \
     -d '{
       "existing_manual_transaction_id": "[EXISTING_UUID]",
       "pdf_override_data": {
         "temp_id": "pdf-0",
         "amount_idr": 45000,
         "transaction_date": "2024-03-24T12:05:00Z",
         "description": "SBX PIK AVENUE"
       }
     }' \
     http://localhost:8080/v1/transactions/merge
```

---

## Project Status

### ✅ Achieved
- **Hardened PDF Engine**: Successfully parsing complex layouts for BCA, BRI, AEON, DBS, and Seabank.
- **Native AESV2 Decryption**: Integrated a patched local fork of `pdfcpu` to natively handle encrypted DBS and BRI statements without intermediate workarounds.
- **Seabank De-obfuscation**: Defeated Seabank's proprietary +28 Caesar shift font encoding.
- **Positional Intelligence**: Overcame "coordinate bleeding" by page-isolated positional grouping.
- **Database Reconciliation**: AI Merge Wizard prototype identifies matches within a 3-day temporal window and exact amount matching.
- **Environment Stability**: Migrated to Supavisor Pooler to resolve DNS/connectivity issues in local development.

### 🚧 Still on Development
- **Mobile Frontend**: The Flutter application is currently being developed with a premium "Obsidian" design theme.
- **Native AESV2 Decryption**: Some DBS statements require a "Print to PDF" workaround due to library-level encryption limitations.
- **Advanced Categories**: Implementing machine learning classifiers for better merchant-to-category mapping.
- **Batch Processing**: Background workers for large batch statement uploads.
