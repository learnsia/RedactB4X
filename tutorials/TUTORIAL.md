# RedactB4X Tutorial

A step-by-step guide to finding and removing personal information from your documents.

---

## What This Tool Does

RedactB4X reads your documents, finds personal information (names, Social Security numbers, email addresses, and more), and replaces that information with safe placeholder tokens. You get a redacted version of your document that you can share without worrying about leaking private data.

![The RedactB4X dashboard showing the main interface](screenshots/02-dashboard.png)


### Important

Be sure you maintain provenance of the redacted data. Do not leave it outside of the current folders where you current keep confidential data. Once you are done using it, then delete it or store it safely.

With enough metadata, it could be possible to determine the redacted data.

---

## Step 1: Add Documents

You have three ways to get documents into the system.

### Option A: Drag and Drop

Open a file explorer window on your computer, select the files you want, and drag them onto the upload zone (the dashed-border area labeled "Upload Documents"). The zone will highlight when files are hovering over it.

Let go. A message will confirm how many files uploaded successfully.

### Option B: Click to Browse

Click inside the upload zone. A file picker window opens. Select one or more files, then click Open.

### Option C: Scan a Folder

If you have a whole folder of documents, click **Scan Directory** next to the category field. A dialog box appears.

- Type a folder path on the server (like `/home/user/documents`), or
- Click **Choose folder on this computer** and pick a folder from your machine.

Click **Scan**. The system imports every supported file it finds, preserving your folder structure. Duplicates are skipped automatically.

### What Files Work?

| Format | Notes |
|--------|-------|
| PDF (.pdf) | Converted to text automatically |
| Word (.docx) | Converted to text automatically |
| Excel (.xlsx, .xls) | Converted to text automatically |
| PowerPoint (.pptx) | Converted to text automatically |
| Text (.txt, .md, .csv, .json) | Used as-is |
| HTML (.html, .htm) | Converted to text automatically |
| EPUB (.epub) | Converted to text automatically |

---

## Step 2: Organize Your Documents

Your uploaded documents appear in the Document Library table on the right side of the dashboard.

### Create Folders

Click **New folder** above the library table. Type a folder path, using forward slashes to nest folders, like `HR/Recruiting`. Click Create.

### Filter and Search

- **Search bar**: type a keyword to filter documents by name, folder, or category.
- **Folder dropdown**: select a folder to show only documents in that folder and its sub-folders.

### Sort Through Pages

The library shows 10 documents at a time. Use the **Prev** and **Next** buttons to move between pages.

---

## Step 3: Process a Document (Find and Redact PII)

This is the main event. Processing runs the PII detector on a document and shows you everything it found.

### Single Document

1. Find the document in the library.
2. Click the **Process** button on that row.
3. A Pattern Picker modal appears. The system's built-in detection is always on. If you have saved custom rules (more on that later), you can check the ones you want to apply here.
4. Click **Process document**.

![The Pattern Picker modal where you select which rules to apply](screenshots/04-pattern-picker.png)

The Document Processing modal opens. You get three panels:

- **Left panel**: your original document text.
- **Right panel**: the redacted version, with personal information replaced by tokens like `[EMAIL-a1b2c3]` or `[SSN-d4e5f6]`.
- **PII Detection panel**: a list of every item the system found, showing the type (like "Email Address"), the original value, and the replacement token.

![Processing results showing original and redacted text side by side](screenshots/05-processing-results.png)

Review the results. You have two options:

- **Looks Correct -- Approve & Save**: click this if everything looks good. The document gets an "Approved" status badge.
- **Missed Something**: click this if the system missed personal information. The Pattern Lab opens so you can fix it.

---

## Step 4: Catch What Was Missed (Pattern Lab)

No detector is perfect. If the system missed a person's name, an address, or any other personal information, the Pattern Lab lets you tell it what to look for.

1. Click **Missed Something** on a processed document.
2. The Pattern Lab opens with your document text displayed.
3. **Highlight (select) the text** that was missed -- for example, drag your cursor over "Sarah Mitchell" in the document.
4. When you release, you see options:

   - **Save to library & apply similar**: the system figures out the type of text you highlighted (a name, a date, a phone number, etc.) and builds a rule that catches all similar values. For example, highlighting one person's name creates a rule that finds all names in the document. This is usually what you want.
   - **This document only (exact)**: redacts just the exact text you highlighted. Good for one-off items like a company name.

5. You can also use the **Advanced section** for more control:
   - **Suggested names**: the system compares your document against a list of known person names and shows clickable chips for any it missed. Click a name to redact it.
   - **Type exact text**: type any string to redact, like "Acme Corp".
   - **Technical regex**: enter a regular expression if you know what you are doing. The field auto-fills when a "similar" match is available.

After you add a rule, the document re-processes automatically. The PII list updates with the new findings.

---

## Step 5: Build Your Pattern Library

Every custom rule you create through the Pattern Lab gets saved to your Pattern Library. These rules stick around across sessions, so you only need to build them once.

### View Your Library

Click **Pattern library** in the sidebar on the left.

![The Pattern Library showing saved redaction rules](screenshots/08-pattern-library.png)

You will see a list of all your saved rules, each showing:

- **Label**: a name you gave it (like "SSN pattern").
- **Kind**: whether it is a regex (pattern) or literal (exact text).
- **Body**: the actual pattern or text.

### Add a Rule Manually

If you already know what pattern you need:

1. Click **Add a new pattern** in the Pattern Library.
2. Type a **Label** (like "Patient IDs").
3. Pick **regex** or **literal** from the dropdown.
4. Type the pattern or text.
5. Click **Test match** to see how many matches exist in the current document.
6. Click **Save to library**.

### Use Saved Rules When Processing

When you click Process on a document, the Pattern Picker shows all your saved rules with checkboxes. Check the ones you want to apply on top of the built-in detection.

---

## Step 6: Quick Redaction (Paste & Redact)

Sometimes you do not have a whole file. You just have a block of text and need it redacted fast.

1. Scroll to the **Paste & Redact** card on the dashboard.
2. Paste your text into the text area.
3. Click **Redact Now**.

![Paste & Redact area with sample text ready to redact](screenshots/06-paste-redact-filled.png)

The processing modal opens with the original and redacted versions side by side.

![Redaction results showing detected PII items](screenshots/07-redact-results.png)

Click **Copy Redacted Text** to grab the result.

Nothing gets saved to disk. This is for one-off, in-and-out redaction.

---

## Step 7: Download Your Redacted Documents

After processing and approving a document, you can download it.

### Single Document

Click the download button (down arrow) on a document row. Pick from:

- **Markdown**: downloads a `.md` file.
- **Original**: downloads the file in its original format (PDF, DOCX, etc.).
- **Redacted**: downloads the redacted version. The filename gets `-redacted` added to it.

### Multiple Documents

Check the boxes next to several documents. A selection toolbar appears. Click **Download Markdown** or **Download Redacted**. Multiple files come as a ZIP.

---

## Step 8: Batch Processing

Got a lot of documents? You can process them all at once.

1. Click **Process All Documents** in the Batch Actions bar.
2. The system runs PII detection on every document in the library and approves them automatically.
3. A status message tells you how many documents were processed and how many PII items were found.

This is the fastest way to redact an entire document set.

---

## Step 9: Customize Your View

### Toggle Dark Mode

Click the **Theme** button (moon icon) in the top-right corner to switch between light and dark mode. Your choice is saved for next time.

---

## What Types of Personal Information Get Caught?

The built-in detector finds these automatically:

| Type | Example |
|------|---------|
| Email addresses | `s.mitchell@apex.com` |
| Phone numbers | `(555) 234-8901` |
| Social Security numbers | `478-55-3201` |
| Credit card numbers | `4111-1111-1111-1111` |
| IP addresses | `192.168.1.50` |
| Bank routing numbers | `021000089` |
| Bank account numbers | `4455778899` |
| Tax IDs | `93-7654321` |
| Insurance policy IDs | `ABC-1234567` |
| Healthcare NPI numbers | `1234567890` |
| Medical record numbers | `12345678` |
| Person names | `Sarah Mitchell` |
| Addresses | `123 Main St` |
| Dates of birth | `01/15/1985` |
| Patient/client IDs | `98765` |
| Case numbers | `2024-001` |

You do not need to configure any of this. It runs on every document, every time.

---

## Tips

- **Start small.** Process one document first to see how the detector works before running batch processing on everything.
- **Use folders.** Organizing documents by department or project makes it easier to find things later.
- **Build your pattern library over time.** Each time you catch something the system missed, save the rule. Next time, it catches that type automatically.
- **Paste & Redact is great for emails.** Got a forwarded email with personal info? Paste it in, redact it, copy the result, and send it.
- **Check the redacted output.** No detector is perfect. Always review the redacted version before sharing it.
