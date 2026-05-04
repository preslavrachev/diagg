package generator

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/preslavrachev/diagg/config"
)

const (
	excalidrawSource = "https://github.com/preslavrachev/diagg"

	excalidrawPackageGap   = 80.0
	excalidrawComponentGap = 40.0
	excalidrawPackageWidth = 340.0
	excalidrawPackagePad   = 40.0
	excalidrawTitleHeight  = 52.0
	excalidrawNodeWidth    = 260.0
	excalidrawNodeHeight   = 84.0
)

// ExcalidrawGenerator generates importable Excalidraw JSON diagrams.
// Config is read-only after initialization.
type ExcalidrawGenerator struct {
	title  string
	config *config.Config
}

// NewExcalidrawGenerator creates a new Excalidraw generator.
func NewExcalidrawGenerator(title string, cfg *config.Config) *ExcalidrawGenerator {
	if title == "" {
		title = cfg.Defaults.DiagramTitle
	}
	return &ExcalidrawGenerator{
		title:  title,
		config: cfg,
	}
}

type excalidrawFile struct {
	Type     string              `json:"type"`
	Version  int                 `json:"version"`
	Source   string              `json:"source"`
	Elements []excalidrawElement `json:"elements"`
	AppState excalidrawAppState  `json:"appState"`
	Files    map[string]struct{} `json:"files"`
}

type excalidrawAppState struct {
	GridSize                   int    `json:"gridSize"`
	ViewBackgroundColor        string `json:"viewBackgroundColor"`
	CurrentItemStrokeColor     string `json:"currentItemStrokeColor"`
	CurrentItemBackgroundColor string `json:"currentItemBackgroundColor"`
}

type excalidrawElement struct {
	ID              string               `json:"id"`
	Type            string               `json:"type"`
	X               float64              `json:"x"`
	Y               float64              `json:"y"`
	Width           float64              `json:"width"`
	Height          float64              `json:"height"`
	Angle           float64              `json:"angle"`
	StrokeColor     string               `json:"strokeColor"`
	BackgroundColor string               `json:"backgroundColor"`
	FillStyle       string               `json:"fillStyle"`
	StrokeWidth     int                  `json:"strokeWidth"`
	StrokeStyle     string               `json:"strokeStyle"`
	Roughness       int                  `json:"roughness"`
	Opacity         int                  `json:"opacity"`
	GroupIDs        []string             `json:"groupIds"`
	FrameID         *string              `json:"frameId"`
	Roundness       *excalidrawRoundness `json:"roundness,omitempty"`
	Seed            int                  `json:"seed"`
	Version         int                  `json:"version"`
	VersionNonce    int                  `json:"versionNonce"`
	IsDeleted       bool                 `json:"isDeleted"`
	BoundElements   []excalidrawBinding  `json:"boundElements,omitempty"`
	Updated         int64                `json:"updated"`
	Link            *string              `json:"link"`
	Locked          bool                 `json:"locked"`

	// Text-only fields.
	Text          string  `json:"text,omitempty"`
	FontSize      int     `json:"fontSize,omitempty"`
	FontFamily    int     `json:"fontFamily,omitempty"`
	TextAlign     string  `json:"textAlign,omitempty"`
	VerticalAlign string  `json:"verticalAlign,omitempty"`
	ContainerID   *string `json:"containerId,omitempty"`
	OriginalText  string  `json:"originalText,omitempty"`
	LineHeight    float64 `json:"lineHeight,omitempty"`
	Baseline      int     `json:"baseline,omitempty"`

	// Arrow-only fields.
	Points             [][]float64           `json:"points,omitempty"`
	LastCommittedPoint *string               `json:"lastCommittedPoint"`
	StartBinding       *excalidrawEndBinding `json:"startBinding,omitempty"`
	EndBinding         *excalidrawEndBinding `json:"endBinding,omitempty"`
	StartArrowhead     *string               `json:"startArrowhead"`
	EndArrowhead       string                `json:"endArrowhead,omitempty"`
}

type excalidrawRoundness struct {
	Type int `json:"type"`
}

type excalidrawBinding struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type excalidrawEndBinding struct {
	ElementID string  `json:"elementId"`
	Focus     float64 `json:"focus"`
	Gap       float64 `json:"gap"`
}

type excalidrawPosition struct {
	x float64
	y float64
}

// Generate writes the Excalidraw scene JSON to the writer.
func (g *ExcalidrawGenerator) Generate(components []analyzer.AnalyzedComponent, w io.Writer) error {
	scene := excalidrawFile{
		Type:     "excalidraw",
		Version:  2,
		Source:   excalidrawSource,
		Elements: make([]excalidrawElement, 0, len(components)*3),
		AppState: excalidrawAppState{
			GridSize:                   20,
			ViewBackgroundColor:        "#ffffff",
			CurrentItemStrokeColor:     "#1e1e1e",
			CurrentItemBackgroundColor: "transparent",
		},
		Files: map[string]struct{}{},
	}

	layout := g.layoutComponents(components)
	packageBounds := g.packageBounds(components, layout)

	scene.Elements = append(scene.Elements, g.titleElement())

	packages := sortedPackageNames(packageBounds)
	for _, pkgName := range packages {
		bounds := packageBounds[pkgName]
		scene.Elements = append(scene.Elements, g.packageElements(pkgName, bounds)...)
	}

	sortedComponents := sortComponentsForExcalidraw(components)
	scene.Elements = append(scene.Elements, g.relationshipElements(sortedComponents, layout)...)

	for _, comp := range sortedComponents {
		pos := layout[comp.QualifiedName()]
		scene.Elements = append(scene.Elements, g.componentElements(comp, pos)...)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(scene); err != nil {
		return fmt.Errorf("encoding excalidraw scene: %w", err)
	}

	return nil
}

func (g *ExcalidrawGenerator) titleElement() excalidrawElement {
	return textElement(
		"diagram-title",
		g.title,
		40,
		24,
		900,
		44,
		28,
		"#1e1e1e",
		nil,
	)
}

func (g *ExcalidrawGenerator) layoutComponents(components []analyzer.AnalyzedComponent) map[string]excalidrawPosition {
	pkgMap := groupComponentsForExcalidraw(components, g.config.Defaults.PackageFallback)
	packages := sortedComponentPackageNames(pkgMap)

	positions := make(map[string]excalidrawPosition, len(components))
	x := 40.0
	for _, pkgName := range packages {
		pkgComponents := sortComponentsForExcalidraw(pkgMap[pkgName])
		y := 120.0 + excalidrawTitleHeight
		for _, comp := range pkgComponents {
			positions[comp.QualifiedName()] = excalidrawPosition{
				x: x + excalidrawPackagePad,
				y: y,
			}
			y += excalidrawNodeHeight + excalidrawComponentGap
		}
		x += excalidrawPackageWidth + excalidrawPackageGap
	}

	return positions
}

func (g *ExcalidrawGenerator) packageBounds(
	components []analyzer.AnalyzedComponent,
	layout map[string]excalidrawPosition,
) map[string]excalidrawPackageBounds {
	bounds := make(map[string]excalidrawPackageBounds)

	for _, comp := range components {
		pkgName := comp.Component.PackageName
		if pkgName == "" {
			pkgName = g.config.Defaults.PackageFallback
		}

		pos := layout[comp.QualifiedName()]
		current, ok := bounds[pkgName]
		if !ok {
			current = excalidrawPackageBounds{
				x:      pos.x - excalidrawPackagePad,
				y:      pos.y - excalidrawTitleHeight,
				width:  excalidrawPackageWidth,
				height: excalidrawTitleHeight + excalidrawNodeHeight + excalidrawPackagePad,
			}
		}

		bottom := pos.y + excalidrawNodeHeight + excalidrawPackagePad
		if bottom-current.y > current.height {
			current.height = bottom - current.y
		}
		bounds[pkgName] = current
	}

	return bounds
}

type excalidrawPackageBounds struct {
	x      float64
	y      float64
	width  float64
	height float64
}

func (g *ExcalidrawGenerator) packageElements(pkgName string, bounds excalidrawPackageBounds) []excalidrawElement {
	packageID := excalidrawID("package", pkgName)
	return []excalidrawElement{
		rectangleElement(
			packageID,
			bounds.x,
			bounds.y,
			bounds.width,
			bounds.height,
			"#495057",
			"#f8f9fa",
			1,
			"solid",
			nil,
		),
		textElement(
			excalidrawID("package-label", pkgName),
			pkgName,
			bounds.x+18,
			bounds.y+16,
			bounds.width-36,
			24,
			18,
			"#343a40",
			[]string{packageID},
		),
	}
}

func (g *ExcalidrawGenerator) componentElements(comp analyzer.AnalyzedComponent, pos excalidrawPosition) []excalidrawElement {
	componentID := excalidrawID("component", comp.QualifiedName())
	color := g.componentColor(comp)
	label := fmt.Sprintf("%s\n%s / %s", comp.Component.Name, comp.Type, comp.Technology)

	if comp.IsInterface {
		label = fmt.Sprintf("%s\ninterface / %s", comp.Component.Name, comp.Type)
	}

	return []excalidrawElement{
		rectangleElement(
			componentID,
			pos.x,
			pos.y,
			excalidrawNodeWidth,
			excalidrawNodeHeight,
			g.strokeColor(comp),
			color,
			g.strokeWidth(comp),
			"solid",
			&excalidrawRoundness{Type: 3},
		),
		textElement(
			excalidrawID("component-label", comp.QualifiedName()),
			label,
			pos.x+14,
			pos.y+14,
			excalidrawNodeWidth-28,
			excalidrawNodeHeight-28,
			16,
			"#1e1e1e",
			[]string{componentID},
		),
	}
}

func (g *ExcalidrawGenerator) relationshipElements(
	components []analyzer.AnalyzedComponent,
	layout map[string]excalidrawPosition,
) []excalidrawElement {
	var elements []excalidrawElement

	for _, comp := range components {
		sourcePos, ok := layout[comp.QualifiedName()]
		if !ok {
			continue
		}

		for _, dep := range comp.Dependencies {
			targetPos, ok := layout[dep.QualifiedTarget()]
			if !ok {
				continue
			}
			elements = append(elements, arrowElement(
				excalidrawID("dependency", comp.QualifiedName()+"-"+dep.QualifiedTarget()),
				comp.QualifiedName(),
				dep.QualifiedTarget(),
				sourcePos,
				targetPos,
				"solid",
				"#495057",
			))
		}

		for _, impl := range comp.Implements {
			targetQN := qualifiedNameForExcalidraw(impl.InterfacePackage, impl.InterfaceName)
			targetPos, ok := layout[targetQN]
			if !ok {
				continue
			}
			elements = append(elements, arrowElement(
				excalidrawID("implementation", comp.QualifiedName()+"-"+targetQN),
				comp.QualifiedName(),
				targetQN,
				sourcePos,
				targetPos,
				"dashed",
				"#7048e8",
			))
		}
	}

	return elements
}

func (g *ExcalidrawGenerator) componentColor(comp analyzer.AnalyzedComponent) string {
	if comp.Type == analyzer.TypeEntrypoint {
		return "#ffe3e3"
	}

	colors := g.config.Styling.D3.PackageColors
	if len(colors) == 0 {
		return "#e7f5ff"
	}

	pkgName := comp.Component.PackageName
	if pkgName == "" {
		pkgName = g.config.Defaults.PackageFallback
	}

	index := stableColorIndex(pkgName, len(colors))
	return lightenColor(colors[index])
}

func (g *ExcalidrawGenerator) strokeColor(comp analyzer.AnalyzedComponent) string {
	if comp.Type == analyzer.TypeEntrypoint {
		return "#c92a2a"
	}
	switch comp.Role {
	case analyzer.RoleHub:
		return "#1864ab"
	case analyzer.RoleCentral:
		return "#364fc7"
	default:
		return "#495057"
	}
}

func (g *ExcalidrawGenerator) strokeWidth(comp analyzer.AnalyzedComponent) int {
	if comp.Type == analyzer.TypeEntrypoint || comp.Role == analyzer.RoleHub {
		return 3
	}
	if comp.Role == analyzer.RoleCentral {
		return 2
	}
	return 1
}

func groupComponentsForExcalidraw(
	components []analyzer.AnalyzedComponent,
	fallbackPackage string,
) map[string][]analyzer.AnalyzedComponent {
	pkgMap := make(map[string][]analyzer.AnalyzedComponent)
	for _, comp := range components {
		pkgName := comp.Component.PackageName
		if pkgName == "" {
			pkgName = fallbackPackage
		}
		pkgMap[pkgName] = append(pkgMap[pkgName], comp)
	}
	return pkgMap
}

func sortedComponentPackageNames(pkgMap map[string][]analyzer.AnalyzedComponent) []string {
	packages := make([]string, 0, len(pkgMap))
	for pkgName := range pkgMap {
		packages = append(packages, pkgName)
	}
	sort.Strings(packages)
	return packages
}

func sortedPackageNames(bounds map[string]excalidrawPackageBounds) []string {
	packages := make([]string, 0, len(bounds))
	for pkgName := range bounds {
		packages = append(packages, pkgName)
	}
	sort.Strings(packages)
	return packages
}

func sortComponentsForExcalidraw(components []analyzer.AnalyzedComponent) []analyzer.AnalyzedComponent {
	sorted := make([]analyzer.AnalyzedComponent, len(components))
	copy(sorted, components)

	rolePriority := map[analyzer.ComponentRole]int{
		analyzer.RoleHub:      0,
		analyzer.RoleCentral:  1,
		analyzer.RoleOrdinary: 2,
		analyzer.RoleLeaf:     3,
	}

	sort.SliceStable(sorted, func(i, j int) bool {
		iPriority := rolePriority[sorted[i].Role]
		jPriority := rolePriority[sorted[j].Role]
		if iPriority != jPriority {
			return iPriority < jPriority
		}

		iDegree := 0
		jDegree := 0
		if sorted[i].Metrics != nil {
			iDegree = sorted[i].Metrics.TotalDegree
		}
		if sorted[j].Metrics != nil {
			jDegree = sorted[j].Metrics.TotalDegree
		}
		if iDegree != jDegree {
			return iDegree > jDegree
		}

		return sorted[i].QualifiedName() < sorted[j].QualifiedName()
	})

	return sorted
}

func rectangleElement(
	id string,
	x float64,
	y float64,
	width float64,
	height float64,
	strokeColor string,
	backgroundColor string,
	strokeWidth int,
	strokeStyle string,
	roundness *excalidrawRoundness,
) excalidrawElement {
	return baseElement(id, "rectangle", x, y, width, height, strokeColor, backgroundColor, strokeWidth, strokeStyle, nil, roundness)
}

func textElement(
	id string,
	text string,
	x float64,
	y float64,
	width float64,
	height float64,
	fontSize int,
	strokeColor string,
	groupIDs []string,
) excalidrawElement {
	elem := baseElement(id, "text", x, y, width, height, strokeColor, "transparent", 1, "solid", groupIDs, nil)
	elem.Text = text
	elem.OriginalText = text
	elem.FontSize = fontSize
	elem.FontFamily = 1
	elem.TextAlign = "left"
	elem.VerticalAlign = "top"
	elem.LineHeight = 1.25
	elem.Baseline = int(height) - 8
	return elem
}

func arrowElement(
	id string,
	sourceQN string,
	targetQN string,
	sourcePos excalidrawPosition,
	targetPos excalidrawPosition,
	strokeStyle string,
	strokeColor string,
) excalidrawElement {
	startX := sourcePos.x + excalidrawNodeWidth
	startY := sourcePos.y + excalidrawNodeHeight/2
	endX := targetPos.x
	endY := targetPos.y + excalidrawNodeHeight/2

	if targetPos.x < sourcePos.x {
		startX = sourcePos.x
		endX = targetPos.x + excalidrawNodeWidth
	}

	width := endX - startX
	height := endY - startY

	elem := baseElement(id, "arrow", startX, startY, width, height, strokeColor, "transparent", 2, strokeStyle, nil, nil)
	elem.Points = [][]float64{{0, 0}, {width, height}}
	elem.StartBinding = &excalidrawEndBinding{
		ElementID: excalidrawID("component", sourceQN),
		Focus:     0,
		Gap:       8,
	}
	elem.EndBinding = &excalidrawEndBinding{
		ElementID: excalidrawID("component", targetQN),
		Focus:     0,
		Gap:       8,
	}
	elem.EndArrowhead = "arrow"
	return elem
}

func baseElement(
	id string,
	elementType string,
	x float64,
	y float64,
	width float64,
	height float64,
	strokeColor string,
	backgroundColor string,
	strokeWidth int,
	strokeStyle string,
	groupIDs []string,
	roundness *excalidrawRoundness,
) excalidrawElement {
	return excalidrawElement{
		ID:                 id,
		Type:               elementType,
		X:                  x,
		Y:                  y,
		Width:              width,
		Height:             height,
		Angle:              0,
		StrokeColor:        strokeColor,
		BackgroundColor:    backgroundColor,
		FillStyle:          "solid",
		StrokeWidth:        strokeWidth,
		StrokeStyle:        strokeStyle,
		Roughness:          1,
		Opacity:            100,
		GroupIDs:           groupIDs,
		FrameID:            nil,
		Roundness:          roundness,
		Seed:               stableSeed(id),
		Version:            1,
		VersionNonce:       stableSeed(id + "-nonce"),
		IsDeleted:          false,
		Updated:            1,
		Link:               nil,
		Locked:             false,
		LastCommittedPoint: nil,
		StartArrowhead:     nil,
	}
}

func excalidrawID(prefix, value string) string {
	id := strings.ToLower(prefix + "-" + sanitizeID(value))
	id = strings.ReplaceAll(id, "_", "-")
	return id
}

func qualifiedNameForExcalidraw(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

func stableSeed(value string) int {
	seed := 17
	for _, r := range value {
		seed = (seed*31 + int(r)) % 2147483647
	}
	if seed == 0 {
		return 1
	}
	return seed
}

func stableColorIndex(value string, paletteSize int) int {
	if paletteSize == 0 {
		return 0
	}
	seed := stableSeed(value)
	return seed % paletteSize
}

func lightenColor(color string) string {
	switch strings.ToLower(color) {
	case "#4285f4":
		return "#e7f5ff"
	case "#ea4335":
		return "#fff5f5"
	case "#34a853":
		return "#ebfbee"
	case "#fbbc04":
		return "#fff9db"
	case "#9c27b0":
		return "#f8f0fc"
	case "#00bcd4":
		return "#e3fafc"
	case "#ff6d00":
		return "#fff4e6"
	default:
		return "#f8f9fa"
	}
}
