package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/preslavrachev/diagg/config"
	"github.com/preslavrachev/diagg/parser"
	"github.com/stretchr/testify/suite"
)

// AIDEV-NOTE: function-body-bdd-tests; BDD-style tests for function body dependency extraction

// FunctionBodyDepsTestSuite is a testify suite for testing function body dependency extraction
type FunctionBodyDepsTestSuite struct {
	suite.Suite
	testdataDir      string
	parsedComponents []parser.Component
	pkgTypeInfo      *parser.PackageTypeInfo
	analyzed         []AnalyzedComponent
	analyzer         *Analyzer
}

// SetupSuite runs once before all tests
func (s *FunctionBodyDepsTestSuite) SetupSuite() {
	s.analyzer = NewAnalyzer(config.New())
}

// SetupTest runs before each test
func (s *FunctionBodyDepsTestSuite) SetupTest() {
	// Reset state for each test
	s.parsedComponents = nil
	s.pkgTypeInfo = nil
	s.analyzed = nil
}

// Feature: Function Body Dependency Detection
// Scenario: Constructor function call creates dependency

func (s *FunctionBodyDepsTestSuite) TestConstructorFunctionCall() {
	s.
		givenComponentThatCallsConstructorFunction().
		whenAnalyzingWithTypeInfo().
		thenDependencyIsDetectedFromFunctionCallReturnType()
}

// Feature: Function Body Dependency Detection
// Scenario: Both composite literal and constructor function work

func (s *FunctionBodyDepsTestSuite) TestCompositeLiteralVsConstructor() {
	s.
		givenComponentsUsingBothInstantiationPatterns().
		whenAnalyzingWithTypeInfo().
		thenBothComponentsShowTheSameDependency()
}

// Feature: Function Body Dependency Detection
// Scenario: Standard library types are filtered out

func (s *FunctionBodyDepsTestSuite) TestStdlibTypesFiltered() {
	s.
		givenComponentUsingStandardLibraryTypes().
		whenAnalyzingWithTypeInfo().
		thenStandardLibraryDependenciesAreFiltered()
}

// Given steps - setup test data

func (s *FunctionBodyDepsTestSuite) givenComponentThatCallsConstructorFunction() *FunctionBodyDepsTestSuite {
	s.testdataDir = s.createTempTestdata(map[string]string{
		"markdown/parser.go": `package markdown
type StreamingParser struct {
	buffer string
}
func NewStreamingParser() *StreamingParser {
	return &StreamingParser{}
}`,
		"service/handler.go": `package service
import "test/markdown"
type Handler struct{}
func (h *Handler) Process() {
	parser := markdown.NewStreamingParser()
	_ = parser
}`,
		"go.mod": `module test
go 1.25`,
	})
	return s
}

func (s *FunctionBodyDepsTestSuite) givenComponentsUsingBothInstantiationPatterns() *FunctionBodyDepsTestSuite {
	s.testdataDir = s.createTempTestdata(map[string]string{
		"parser/types.go": `package parser
type Parser struct {
	name string
}
func NewParser() *Parser {
	return &Parser{}
}`,
		"servicea/svc.go": `package servicea
import "test/parser"
type ServiceA struct{}
func (s *ServiceA) Run() {
	p := &parser.Parser{}
	_ = p
}`,
		"serviceb/svc.go": `package serviceb
import "test/parser"
type ServiceB struct{}
func (s *ServiceB) Run() {
	p := parser.NewParser()
	_ = p
}`,
		"go.mod": `module test
go 1.25`,
	})
	return s
}

func (s *FunctionBodyDepsTestSuite) givenComponentUsingStandardLibraryTypes() *FunctionBodyDepsTestSuite {
	s.testdataDir = s.createTempTestdata(map[string]string{
		"service/handler.go": `package service
import (
	"context"
	"fmt"
)
type Handler struct{}
func (h *Handler) Process(ctx context.Context) error {
	fmt.Println("processing")
	return nil
}`,
		"go.mod": `module test
go 1.25`,
	})
	return s
}

// When steps - perform actions

func (s *FunctionBodyDepsTestSuite) whenAnalyzingWithTypeInfo() *FunctionBodyDepsTestSuite {
	p := parser.NewParser()
	components, pkgTypeInfo, err := p.ParseDirectoryWithTypes(s.testdataDir)
	s.Require().NoError(err, "ParseDirectoryWithTypes should not fail")

	s.parsedComponents = components
	s.pkgTypeInfo = pkgTypeInfo
	s.analyzed = s.analyzer.AnalyzeWithTypes(components, pkgTypeInfo)

	return s
}

// Then steps - assertions

func (s *FunctionBodyDepsTestSuite) thenDependencyIsDetectedFromFunctionCallReturnType() *FunctionBodyDepsTestSuite {
	consumer := s.findComponent("Handler")
	provider := s.findComponent("StreamingParser")

	s.Require().NotNil(consumer, "Consumer component should exist")
	s.Require().NotNil(provider, "Provider component should exist")

	hasDep := s.componentDependsOn(consumer, provider.Component.Name)
	s.True(hasDep, "Component should show dependency on type returned from constructor function call")
	return s
}

func (s *FunctionBodyDepsTestSuite) thenBothComponentsShowTheSameDependency() *FunctionBodyDepsTestSuite {
	compositeUser := s.findComponent("ServiceA")
	constructorUser := s.findComponent("ServiceB")
	sharedDep := s.findComponent("Parser")

	s.Require().NotNil(compositeUser, "Component using composite literal should exist")
	s.Require().NotNil(constructorUser, "Component using constructor should exist")
	s.Require().NotNil(sharedDep, "Shared dependency should exist")

	hasDepA := s.componentDependsOn(compositeUser, sharedDep.Component.Name)
	hasDepB := s.componentDependsOn(constructorUser, sharedDep.Component.Name)

	s.True(hasDepA, "Component using &Type{} should show dependency")
	s.True(hasDepB, "Component using NewType() should show dependency")

	return s
}

func (s *FunctionBodyDepsTestSuite) thenStandardLibraryDependenciesAreFiltered() *FunctionBodyDepsTestSuite {
	component := s.findComponent("Handler")
	s.Require().NotNil(component, "Component should exist")

	stdlibTypes := []string{"Context", "Error"}
	for _, dep := range component.Dependencies {
		for _, stdlibType := range stdlibTypes {
			s.NotEqual(stdlibType, dep.TargetName,
				"Should not show dependency on stdlib type: %s", stdlibType)
		}
	}

	return s
}

// Helper methods

func (s *FunctionBodyDepsTestSuite) findComponent(name string) *AnalyzedComponent {
	for i := range s.analyzed {
		if s.analyzed[i].Component.Name == name {
			return &s.analyzed[i]
		}
	}
	return nil
}

func (s *FunctionBodyDepsTestSuite) componentDependsOn(component *AnalyzedComponent, targetName string) bool {
	for _, dep := range component.Dependencies {
		if dep.TargetName == targetName {
			return true
		}
	}
	return false
}

func (s *FunctionBodyDepsTestSuite) createTempTestdata(files map[string]string) string {
	dir, err := os.MkdirTemp("", "diagg-test-*")
	s.Require().NoError(err, "failed to create temp dir")

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
		s.Require().NoError(err, "failed to create parent dirs for %s", path)

		err = os.WriteFile(fullPath, []byte(content), 0o644)
		s.Require().NoError(err, "failed to write file %s", path)
	}

	s.T().Cleanup(func() {
		os.RemoveAll(dir)
	})

	return dir
}

// TestFunctionBodyDepsTestSuite is the entry point for running the suite
func TestFunctionBodyDepsTestSuite(t *testing.T) {
	suite.Run(t, new(FunctionBodyDepsTestSuite))
}
