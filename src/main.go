package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"redactb4x/redactorpii"
)

func main() {
	port := flag.String("port", "8090", "Port for the document management server")
	host := flag.String("host", "", "Network interface / bind address (empty or 0.0.0.0 = all interfaces; 127.0.0.1 = localhost only)")
	docsDir := flag.String("docs-dir", "", "Directory to scan for documents on startup")
	dataDir := flag.String("data-dir", ".", "Root directory that contains the data/ folder (default: current working directory)")
	flag.Parse()

	disk := redactorpii.NewDiskManager(*dataDir)
	if err := disk.Init(); err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	cfg := disk.LoadConfig()
	if cfg != nil {
		fmt.Printf("  Company: %s\n", cfg.Company)
		fmt.Printf("  Framework: %s %s\n", cfg.ComplianceFramework, cfg.ComplianceVersion)
	}

	if *docsDir != "" {
		fmt.Printf("  Scanning directory: %s\n", *docsDir)
		docs, err := redactorpii.ScanDirectory(*docsDir)
		if err != nil {
			log.Printf("  Warning: failed to scan directory: %v", err)
		} else {
			existing := disk.LoadDocumentIndex()
			existingSource := make(map[string]struct{})
			for _, d := range existing {
				if d.SourceRelPath != "" {
					existingSource[filepath.ToSlash(d.SourceRelPath)] = struct{}{}
				}
			}
			added := 0
			skipped := 0
			for _, doc := range docs.Documents {
				src := filepath.ToSlash(strings.TrimSpace(doc.SourceRelPath))
				if src != "" {
					if _, ok := existingSource[src]; ok {
						skipped++
						continue
					}
				}
				ext := strings.ToLower(filepath.Ext(doc.SourceRelPath))
				if ext == "" {
					ext = ".txt"
				}
				logicalName := doc.Title + ext
				stored, err := disk.StoreDocumentWithMeta(redactorpii.StoreDocumentInput{
					LogicalFilename: logicalName,
					Folder:          doc.Folder,
					Category:        doc.Category,
					Content:         doc.Content,
					SourceRelPath:   src,
				})
				if err != nil {
					continue
				}
				existing = append(existing, *stored)
				if src != "" {
					existingSource[src] = struct{}{}
				}
				added++
			}
			disk.SaveDocumentIndex(existing)
			fmt.Printf("  Added %d documents (%d skipped duplicates, %d total)\n", added, skipped, len(existing))
			if len(docs.Warnings) > 0 {
				log.Printf("  Scan warnings: %d permission/read issue(s)", len(docs.Warnings))
			}
		}
	}

	listenAddr := redactorpii.ListenAddr(*host, *port)

	srv := redactorpii.NewServerWithConfig(listenAddr, cfg, disk, *docsDir)

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          RedactB4X - Document Management System           ║")
	fmt.Println("║  Upload, redact & protect PII in your documents            ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	displayHost := redactorpii.DisplayHost(*host)
	fmt.Printf("  Open http://%s:%s in your browser\n", displayHost, *port)
	fmt.Println()
	log.Fatal(srv.Start())
}
