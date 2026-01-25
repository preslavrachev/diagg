package analyzer

import (
	"testing"

	"github.com/preslavrachev/diagg/parser"
)

// AIDEV-NOTE: test-metrics; validates connectivity metrics calculation and role classification

func TestCalculateMetrics(t *testing.T) {
	tests := []struct {
		name       string
		components []AnalyzedComponent
		want       map[string]ComponentMetrics
	}{
		{
			name: "simple dependency chain",
			components: []AnalyzedComponent{
				{
					Component: parser.Component{Name: "HandlerA"},
					Dependencies: []Dependency{
						{TargetName: "ServiceB"},
					},
				},
				{
					Component: parser.Component{Name: "ServiceB"},
					Dependencies: []Dependency{
						{TargetName: "RepositoryC"},
					},
				},
				{
					Component:    parser.Component{Name: "RepositoryC"},
					Dependencies: []Dependency{},
				},
			},
			want: map[string]ComponentMetrics{
				"HandlerA":    {InDegree: 0, OutDegree: 1, TotalDegree: 1},
				"ServiceB":    {InDegree: 1, OutDegree: 1, TotalDegree: 2},
				"RepositoryC": {InDegree: 1, OutDegree: 0, TotalDegree: 1},
			},
		},
		{
			name: "hub component with multiple dependencies",
			components: []AnalyzedComponent{
				{
					Component: parser.Component{Name: "UserService"},
					Dependencies: []Dependency{
						{TargetName: "UserRepo"},
						{TargetName: "CacheService"},
					},
				},
				{
					Component: parser.Component{Name: "OrderService"},
					Dependencies: []Dependency{
						{TargetName: "UserService"},
						{TargetName: "OrderRepo"},
					},
				},
				{
					Component: parser.Component{Name: "PaymentService"},
					Dependencies: []Dependency{
						{TargetName: "UserService"},
					},
				},
				{
					Component: parser.Component{Name: "UserRepo"},
				},
				{
					Component: parser.Component{Name: "OrderRepo"},
				},
				{
					Component: parser.Component{Name: "CacheService"},
				},
			},
			want: map[string]ComponentMetrics{
				"UserService":    {InDegree: 2, OutDegree: 2, TotalDegree: 4}, // Hub: high in-degree
				"OrderService":   {InDegree: 0, OutDegree: 2, TotalDegree: 2},
				"PaymentService": {InDegree: 0, OutDegree: 1, TotalDegree: 1},
				"UserRepo":       {InDegree: 1, OutDegree: 0, TotalDegree: 1},
				"OrderRepo":      {InDegree: 1, OutDegree: 0, TotalDegree: 1},
				"CacheService":   {InDegree: 1, OutDegree: 0, TotalDegree: 1},
			},
		},
		{
			name: "interface implementations count as dependencies",
			components: []AnalyzedComponent{
				{
					Component: parser.Component{Name: "PostgresRepo"},
					Implements: []InterfaceImplementation{
						{InterfaceName: "UserRepository"},
					},
				},
				{
					Component:   parser.Component{Name: "UserRepository"},
					IsInterface: true,
				},
			},
			want: map[string]ComponentMetrics{
				"PostgresRepo":   {InDegree: 0, OutDegree: 1, TotalDegree: 1},
				"UserRepository": {InDegree: 1, OutDegree: 0, TotalDegree: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateMetrics(tt.components)

			for compName, wantMetrics := range tt.want {
				gotMetrics, ok := got[compName]
				if !ok {
					t.Errorf("missing metrics for component %s", compName)
					continue
				}

				if gotMetrics.InDegree != wantMetrics.InDegree {
					t.Errorf("%s.InDegree = %d, want %d", compName, gotMetrics.InDegree, wantMetrics.InDegree)
				}
				if gotMetrics.OutDegree != wantMetrics.OutDegree {
					t.Errorf("%s.OutDegree = %d, want %d", compName, gotMetrics.OutDegree, wantMetrics.OutDegree)
				}
				if gotMetrics.TotalDegree != wantMetrics.TotalDegree {
					t.Errorf("%s.TotalDegree = %d, want %d", compName, gotMetrics.TotalDegree, wantMetrics.TotalDegree)
				}
			}
		})
	}
}

func TestClassifyRole(t *testing.T) {
	tests := []struct {
		name            string
		metrics         ComponentMetrics
		totalComponents int
		want            ComponentRole
	}{
		{
			name:            "hub with high in-degree",
			metrics:         ComponentMetrics{InDegree: 5, OutDegree: 2, TotalDegree: 7},
			totalComponents: 10,
			want:            RoleHub,
		},
		{
			name:            "hub meets minimum threshold",
			metrics:         ComponentMetrics{InDegree: 3, OutDegree: 1, TotalDegree: 4},
			totalComponents: 5,
			want:            RoleHub,
		},
		{
			name:            "central with high total degree",
			metrics:         ComponentMetrics{InDegree: 2, OutDegree: 3, TotalDegree: 5},
			totalComponents: 10,
			want:            RoleCentral,
		},
		{
			name:            "leaf with high out-degree, low in-degree",
			metrics:         ComponentMetrics{InDegree: 1, OutDegree: 4, TotalDegree: 5},
			totalComponents: 10,
			want:            RoleLeaf,
		},
		{
			name:            "ordinary component",
			metrics:         ComponentMetrics{InDegree: 1, OutDegree: 1, TotalDegree: 2},
			totalComponents: 10,
			want:            RoleOrdinary,
		},
		{
			name:            "isolated component",
			metrics:         ComponentMetrics{InDegree: 0, OutDegree: 0, TotalDegree: 0},
			totalComponents: 10,
			want:            RoleOrdinary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyRole(&tt.metrics, tt.totalComponents)
			if got != tt.want {
				t.Errorf("ClassifyRole() = %v, want %v", got, tt.want)
			}
		})
	}
}
