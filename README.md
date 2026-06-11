# RedactB4X

A web-based document management system with PII redaction and pattern-based detection. Upload documents, detect and redact sensitive information, and build custom detection rules.

I borrowed from: https://github.com/noxone/regex-generator to create the automatic Regex buiilder.

## Features

- **Document Management** - Upload, organize, download, and delete documents with folder support
- **PII Redaction** - Automatically detect and redact emails, phone numbers, SSNs, credit cards, IP addresses, and more
- **Custom Patterns** - Build, test, and save your own PII detection patterns
- **Compliance Frameworks** - Configure compliance frameworks (HIPAA, GDPR, etc.) for context
- **Batch Processing** - Process multiple documents at once with progress tracking
- **Document Workflow** - Process, approve, and reject documents through a review pipeline
- **Library Folders** - Organize documents into a browsable folder structure
- **Security** - Rate limiting, security headers, request IDs, and panic recovery built in

## Quick Start

```bash
# Build
make build

# Run (default port 8090)
./RedactB4X

# Custom port
./RedactB4X -port 3000

# Auto-scan documents on startup
./RedactB4X -docs-dir /path/to/documents
```

Open `http://localhost:8090` in your browser.

## Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8090` | HTTP server port |
| `-host` | (all interfaces) | Bind address (`127.0.0.1` for localhost only) |
| `-docs-dir` | (none) | Directory to scan for documents on startup |
| `-data-dir` | `.` | Root directory containing the `data/` folder |

## Project Structure

```
redactorpii/          Core package: server, API, PII engine, storage
  handlers.go         All API routes and handlers
  redactor.go         PII detection and redaction engine
  disk.go             File and document persistence
  session.go          Session management
  pattern_library.go  Custom pattern storage and matching
  static/index.html   Embedded web frontend

internal/middleware/   Security headers, rate limiting, logging
internal/converter/   Document format conversion (PDF/Office → Markdown)
```

## Data

All data is stored under `data/` in the working directory:

```
data/
├── config.json              App configuration
├── converter.json           Document converter settings
├── documents/               Uploaded documents
├── reports/                 Generated compliance reports
└── state/                   Session, document index, library, patterns
```

## Running Tests

```bash
make test          # Run all tests
make vet           # Static analysis
```
