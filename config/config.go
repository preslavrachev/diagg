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
	Entrypoint string // Application entry point (main package)
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
	// PlantUML-specific styling
	HubTag        string // PlantUML tag definition for hub components
	CentralTag    string // PlantUML tag definition for central components
	LeafTag       string // PlantUML tag definition for leaf components
	EntrypointTag string // PlantUML tag definition for entrypoint components (main)

	// D3-specific styling
	D3 D3Styling
}

// D3Styling holds configuration for D3.js force-directed graph visualization.
type D3Styling struct {
	// Package color palette (will cycle through for multiple packages)
	PackageColors []string

	// Force simulation parameters
	LinkDistance      int     // Distance between connected nodes (default: 100)
	ChargeStrength    int     // Negative = repulsion, positive = attraction (default: -300)
	CollisionPadding  int     // Extra space around nodes to prevent overlap (default: 5)
	CenteringStrength float64 // Strength of forces pulling nodes toward center (default: 0.03)

	// Node sizing
	BaseNodeSize    int // Base size for nodes (default: 10)
	SizeScaleFactor int // Multiplier for connectivity-based sizing (default: 2)

	// Arrow configuration
	ArrowMarkerSize   int // Size of arrow markers (default: 6)
	ArrowHeadDistance int // Distance from node edge to arrow tip (default: 8)
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
			Entrypoint: "Application entry point",
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
				"ENTRYPOINT": "CLI",
				"COMPONENT":  "Go",
			},
		},

		Styling: VisualStyling{
			HubTag:        `AddComponentTag("hub", $borderThickness="5", $fontColor="#000000", $borderColor="darkBlue")`,
			CentralTag:    `AddComponentTag("central", $borderThickness="3", $fontColor="#000000", $borderColor="darkBlue")`,
			LeafTag:       `AddComponentTag("leaf", $borderThickness="1")`,
			EntrypointTag: `AddComponentTag("entrypoint", $shape="RoundedBoxShape()", $bgColor="red", $fontColor="white", $borderColor="darkred", $borderThickness="3")`,
			D3: D3Styling{
				PackageColors: []string{
					"#4285f4", // Blue
					"#ea4335", // Red
					"#34a853", // Green
					"#fbbc04", // Yellow
					"#9c27b0", // Purple
					"#00bcd4", // Cyan
					"#ff6d00", // Deep Orange
					"#795548", // Brown
					"#e91e63", // Pink
					"#009688", // Teal
					"#3f51b5", // Indigo
					"#8bc34a", // Light Green
					"#ff9800", // Orange
					"#607d8b", // Blue Grey
					"#f44336", // Deep Red
					"#2196f3", // Light Blue
					"#cddc39", // Lime
					"#673ab7", // Deep Purple
					"#00e676", // Bright Green
					"#ffc107", // Amber
					"#03a9f4", // Sky Blue
					"#ff5722", // Red Orange
					"#9e9e9e", // Grey
					"#ffeb3b", // Bright Yellow
					"#e040fb", // Bright Purple
					"#1de9b6", // Turquoise
					"#ffab40", // Light Orange
					"#536dfe", // Bright Indigo
					"#b2ff59", // Neon Green
					"#ff6e40", // Coral
				},
				LinkDistance:      150,  // Longer links to spread nodes out
				ChargeStrength:    -800, // Strong repulsion to prevent overlap
				CollisionPadding:  15,   // More spacing between nodes
				CenteringStrength: 0.05, // Stronger gravity toward center
				BaseNodeSize:      10,
				SizeScaleFactor:   2,
				ArrowMarkerSize:   6,
				ArrowHeadDistance: 8,
			},
		},

		Defaults: DefaultValues{
			DiagramTitle:      "Component Diagram",
			OutputFile:        "diagram.puml",
			PackageFallback:   "main",
			UnknownTechnology: "Go",
		},
	}
}
