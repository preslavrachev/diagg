package analyzer

// ComponentMetrics tracks graph-theoretic metrics for layout optimization
type ComponentMetrics struct {
	InDegree    int // Number of components that depend on this component
	OutDegree   int // Number of components this component depends on
	TotalDegree int // InDegree + OutDegree
}

// ComponentRole classifies components by their connectivity pattern
type ComponentRole string

const (
	RoleHub      ComponentRole = "hub"     // High in-degree (many depend on it)
	RoleLeaf     ComponentRole = "leaf"    // High out-degree, low in-degree
	RoleCentral  ComponentRole = "central" // High total degree
	RoleOrdinary ComponentRole = "ordinary"
)

// CalculateMetrics computes connectivity metrics for all components
// AIDEV-NOTE: graph-metrics; calculates in/out degree for layout optimization
func CalculateMetrics(components []AnalyzedComponent) map[string]*ComponentMetrics {
	metrics := make(map[string]*ComponentMetrics)

	// Initialize metrics for all components
	for _, comp := range components {
		metrics[comp.Component.Name] = &ComponentMetrics{
			InDegree:  0,
			OutDegree: 0,
		}
	}

	// Count dependencies
	for _, comp := range components {
		sourceName := comp.Component.Name

		// Each dependency increases source's out-degree and target's in-degree
		for _, dep := range comp.Dependencies {
			if sourceMetrics, ok := metrics[sourceName]; ok {
				sourceMetrics.OutDegree++
			}

			if targetMetrics, ok := metrics[dep.TargetName]; ok {
				targetMetrics.InDegree++
			}
		}

		// Interface implementations also contribute to connectivity
		for _, impl := range comp.Implements {
			if sourceMetrics, ok := metrics[sourceName]; ok {
				sourceMetrics.OutDegree++ // Implementing an interface is a form of dependency
			}

			if targetMetrics, ok := metrics[impl.InterfaceName]; ok {
				targetMetrics.InDegree++
			}
		}
	}

	// Calculate total degree
	for _, m := range metrics {
		m.TotalDegree = m.InDegree + m.OutDegree
	}

	return metrics
}

// ClassifyRole determines the architectural role based on connectivity patterns
func ClassifyRole(metrics *ComponentMetrics, totalComponents int) ComponentRole {
	if totalComponents == 0 {
		return RoleOrdinary
	}

	// Hub: High in-degree (>= 20% of total components depend on it, min 3)
	hubThreshold := max(3, totalComponents/5)
	if metrics.InDegree >= hubThreshold {
		return RoleHub
	}

	// Leaf: High out-degree but low in-degree (depends on many, few depend on it)
	// Check this before central to avoid false positives
	if metrics.OutDegree >= 3 && metrics.InDegree <= 1 {
		return RoleLeaf
	}

	// Central: High total connectivity (>= 30% of possible connections, min 4)
	centralThreshold := max(4, (totalComponents*3)/10)
	if metrics.TotalDegree >= centralThreshold {
		return RoleCentral
	}

	return RoleOrdinary
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
