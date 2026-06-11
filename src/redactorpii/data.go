package redactorpii

type DemoScenario struct {
	ID                  string           `json:"id"`
	Company             CompanyInfo      `json:"company"`
	ComplianceFramework string           `json:"complianceFramework"`
	ComplianceVersion   string           `json:"complianceVersion"`
	KnownNames          []string         `json:"-"`
	Documents           []SampleDocument `json:"documents"`
}

type CompanyInfo struct {
	Name        string `json:"name"`
	Industry    string `json:"industry"`
	Size        string `json:"size"`
	Description string `json:"description"`
}

type SampleDocument struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Category      string `json:"category"`
	Folder        string `json:"folder,omitempty"`
	SourceRelPath string `json:"sourceRelPath,omitempty"`
	Content       string `json:"content"`
}

// CustomScenario builds a DemoScenario from user-defined AppConfig and uploaded documents.
func CustomScenario(cfg AppConfig, docs []SampleDocument) DemoScenario {
	framework := cfg.ComplianceFramework
	if framework == "" {
		framework = "Custom"
	}
	version := cfg.ComplianceVersion
	if version == "" {
		version = "v1.0"
	}
	return DemoScenario{
		ID: "custom",
		Company: CompanyInfo{
			Name:        cfg.Company,
			Industry:    cfg.Industry,
			Description: cfg.Description,
		},
		ComplianceFramework: framework,
		ComplianceVersion:   version,
		Documents:           docs,
	}
}
