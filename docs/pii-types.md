# PII Detection Types

19 built-in detection types. Each type uses pattern matching to find and redact personal information. No configuration needed. Runs on every document, every time.

## Personal Identifiers

| PII Type | Method | Example Match |
|----------|--------|---------------|
| Person Name (`PERSON`) | Line-level | `Patient Name: Sarah Mitchell` |
| Date of Birth (`DOB`) | Line-level | `Date of Birth: 01/15/1985` |
| Address (`ADDRESS`) | Line-level | `Home Address: 123 Main St, Springfield, IL 62701` |
| Emergency Contact (`CONTACT`) | Line-level | `Emergency Contact: Jane Mitchell` |
| Healthcare Provider (`PROVIDER`) | Line-level | `Treating Physician: Dr. James Park` |

## Contact & Network

| PII Type | Method | Example Match |
|----------|--------|---------------|
| Email Address (`EMAIL`) | Regex | `s.mitchell@apex.com` |
| Phone Number (`PHONE`) | Regex | `(555) 234-8901` |
| IP Address (`IP`) | Regex | `192.168.1.100` |
| Organization Contact (`ORG_CONTACT`) | Line-level | `Contact: Apex Medical Group` |

## Financial

| PII Type | Method | Example Match |
|----------|--------|---------------|
| Social Security Number (`SSN`) | Regex | `478-55-3201` |
| Credit Card Number (`CREDIT_CARD`) | Regex | `4111 1111 1111 1111` |
| Bank Routing Number (`ROUTING`) | Regex | `Routing 021000021` |
| Bank Account Number (`ACCOUNT`) | Regex | `Account Number: 1234567890` |
| Tax ID (`TAXID`) | Regex | `Tax ID: 12-3456789` |

## Healthcare

| PII Type | Method | Example Match |
|----------|--------|---------------|
| Insurance ID (`INSURANCE`) | Regex | `Policy ID: ABC-12345678` |
| NPI Number (`NPI`) | Regex | `NPI: 1234567890` |
| Medical Record Number (`MRN`) | Regex | `MRN: 00123456` |

## Case & Client Identifiers

| PII Type | Method | Example Match |
|----------|--------|---------------|
| Client/Patient ID (`CLIENT_ID`) | Line-level | `Patient ID: PT-2024-0042` |
| Case Number (`CASE_ID`) | Line-level | `Case Number: CV-2024-1138` |

---

**Regex** matches the pattern anywhere in text. **Line-level** requires a label (like "Patient Name:") and matches the value that follows.
