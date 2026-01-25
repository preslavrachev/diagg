package config

import "regexp"

// Config holds all configuration for diagram generation.
// IMPORTANT: After initialization, this struct is READ-ONLY.
// Components receive a pointer to this config but must never modify it.
// Thread-safe as long as no mutations occur post-init.
type Config struct {
	// Component type classification patterns
	Patterns ComponentPatterns

	// Human-readable descriptions for component types
	Descriptions ComponentDescriptions

	// Technology inference rules
	TechnologyRules TechnologyInference

	// Visual styling configuration
	Styling VisualStyling

	// Default values
	Defaults DefaultValues
}

// ComponentPatterns defines regex patterns for classifying components by name.
// Order matters: first match wins.
type ComponentPatterns struct {
	Service    *regexp.Regexp
	Repository *regexp.Regexp
	Handler    *regexp.Regexp
	Controller *regexp.Regexp
	Client     *regexp.Regexp
	Cache      *regexp.Regexp
	Gateway    *regexp.Regexp
	Middleware *regexp.Regexp
}

// ComponentDescriptions provides human-readable descriptions for each component type.
type ComponentDescriptions struct {
	Service    string
	Repository string
	Handler    string
	Controller string
	Client     string
	Cache      string
	Gateway    string
	Middleware string
	Unknown    string // Template: will be formatted with package name
}

// TechnologyInference holds rules for inferring technology from component structure.
type TechnologyInference struct {
	// Field name patterns that suggest specific technologies
	DatabasePatterns map[string]string // pattern -> technology name
	DefaultByType    map[string]string // component type -> default technology
}

// VisualStyling controls diagram appearance.
type VisualStyling struct {
	HubTag     string // PlantUML tag definition for hub components
	CentralTag string // PlantUML tag definition for central components
	LeafTag    string // PlantUML tag definition for leaf components
}

// DefaultValues provides sensible defaults for optional fields.
type DefaultValues struct {
	DiagramTitle      string
	OutputFile        string
	PackageFallback   string // Default package name when none is detected
	UnknownTechnology string // Technology label when inference fails
}

// New returns a Config with opinionated defaults for Go codebases.
// These defaults assume standard Go naming conventions:
// - *Service for business logic
// - *Repository/*Store for data access
// - *Handler for HTTP handlers
// - etc.
func New() *Config {
	return &Config{
		Patterns: ComponentPatterns{
			Service:    regexp.MustCompile(`(?i).*Service$`),
			Repository: regexp.MustCompile(`(?i).*Repository$|.*Repo$|.*Store$`),
			Handler:    regexp.MustCompile(`(?i).*Handler$`),
			Controller: regexp.MustCompile(`(?i).*Controller$`),
			Client:     regexp.MustCompile(`(?i).*Client$`),
			Cache:      regexp.MustCompile(`(?i).*Cache$`),
			Gateway:    regexp.MustCompile(`(?i).*Gateway$`),
			Middleware: regexp.MustCompile(`(?i).*Middleware$`),
		},

		Descriptions: ComponentDescriptions{
			Service:    "Business logic service",
			Repository: "Data access layer",
			Handler:    "HTTP request handler",
			Controller: "Request controller",
			Client:     "External service client",
			Cache:      "Caching layer",
			Gateway:    "External gateway",
			Middleware: "Middleware component",
			Unknown:    "%s component", // Will be formatted with package name
		},

		TechnologyRules: TechnologyInference{
			DatabasePatterns: map[string]string{
				"sql":   "SQL Database",
				"db":    "SQL Database",
				"mongo": "MongoDB",
				"redis": "Redis",
				"grpc":  "gRPC",
			},
			DefaultByType: map[string]string{
				"SERVICE":    "Go",
				"REPOSITORY": "Database",
				"HANDLER":    "HTTP",
				"CONTROLLER": "HTTP",
				"CLIENT":     "HTTP Client",
				"CACHE":      "Cache",
				"GATEWAY":    "External System",
				"MIDDLEWARE": "HTTP Middleware",
				"COMPONENT":  "Go",
			},
		},

		Styling: VisualStyling{
			HubTag:     `AddComponentTag("hub", $borderThickness="5", $fontColor="#000000", $borderColor="darkBlue")`,
			CentralTag: `AddComponentTag("central", $borderThickness="3", $fontColor="#000000", $borderColor="darkBlue")`,
			LeafTag:    `AddComponentTag("leaf", $borderThickness="1")`,
		},

		Defaults: DefaultValues{
			DiagramTitle:      "Component Diagram",
			OutputFile:        "diagram.puml",
			PackageFallback:   "main",
			UnknownTechnology: "Go",
		},
	}
}
