# RedactB4X

Find and redact personal information in documents before you share them.

![RedactB4X Dashboard](docs/images/02-dashboard.png)

Before sending your documents to AI, you should redact names, email addresses, phone numbers, social security numbers, medical record IDs.

RedactB4X scans your documents for personally identifiable information, highlights everything it finds, and lets you redact it with a click. You decide what gets shared and what gets covered up.

**It runs on your own machine or server. Your documents never leave your network.** Redaction is performed Go.

## Key Features

* **Drag-and-drop upload** — Drop a file, click to browse, or point it at a whole folder. Handles PDFs, Word docs, text files, HTML, and more.
* **Built-in PII patterns** — Names, emails, SSNs, credit cards, IP addresses, medical IDs. Runs automatically on every document.
* **Side-by-side review** — See the original and redacted versions next to each other. Every detected item is listed with its type and replacement token.
* **Custom pattern library** — Save your own detection rules. Build them once, reuse them forever. Regex or exact text match.

## Quick Start

```bash
./RedactB4X                        # Run HTTP server (default :8090)
./RedactB4X -port 3000             # Custom port
./RedactB4X -docs-dir /path        # Auto-scan documents on startup
```
