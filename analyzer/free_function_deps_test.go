package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/preslavrachev/diagg/config"
	"github.com/preslavrachev/diagg/parser"
	"github.com/stretchr/testify/suite"
)

// AIDEV-NOTE: free-function-deps-bdd; reproduces the false-positive "unused component"
// gap from the diagg-unused-components handoff: extractDependenciesWithTypes only walks
// method bodies gated by isMethodOfComp (analyzer.go:428), so any type reachable only
// through a free function (package-level constructor caller, main, etc.) gets no inbound
// edge and reports in_degree == 0 even though it is genuinely used.

// FreeFunctionDepsTestSuite is a testify suite for testing dependency detection
// through non-method (free) function bodies.
type FreeFunctionDepsTestSuite struct {
	suite.Suite
	testdataDir      string
	parsedComponents []parser.Component
	pkgTypeInfo      *parser.PackageTypeInfo
	analyzed         []AnalyzedComponent
	analyzer         *Analyzer
}

func (s *FreeFunctionDepsTestSuite) SetupSuite() {
	s.analyzer = NewAnalyzer(config.New())
}

func (s *FreeFunctionDepsTestSuite) SetupTest() {
	s.parsedComponents = nil
	s.pkgTypeInfo = nil
	s.analyzed = nil
}

// Feature: Dependency Detection Through Free Functions
// Scenario: a constructor is only ever called from a plain (non-method) function

func (s *FreeFunctionDepsTestSuite) TestConstructorCalledFromFreeFunction() {
	s.
		givenRegistryOnlyConstructedByFreeFunction().
		whenAnalyzingWithTypeInfo().
		thenRegistryShouldHaveFreeFunctionReferencesOf(1)
}

// Feature: Dependency Detection Through Free Functions
// Scenario: a constructor referencing its own type (the universal `func NewX() *X {
// return &X{} }` shape) must not count as usage on its own - only calls from a
// different package should.

func (s *FreeFunctionDepsTestSuite) TestConstructorSelfReferenceIsNotCountedAsUsage() {
	s.
		givenRegistryWithOnlyItsOwnUncalledConstructor().
		whenAnalyzingWithTypeInfo().
		thenRegistryShouldHaveFreeFunctionReferencesOf(0)
}

// Feature: Dependency Detection Through Free Functions
// Scenario: a constructor with a (value, error) multi-return signature - the idiomatic
// Go shape - is still detected as usage when called from another package.

func (s *FreeFunctionDepsTestSuite) TestMultiReturnConstructorCalledFromFreeFunction() {
	s.
		givenRepoOnlyConstructedByMultiReturnFreeFunction().
		whenAnalyzingWithTypeInfo().
		thenRepoShouldHaveFreeFunctionReferencesOf(1)
}

// Feature: Dependency Detection Through Free Functions
// Scenario: two different free functions in the same package - one that constructs a
// type via a plain call, another that wires it into a different component - must still
// count as usage of the first type. Only a type's own constructor referencing itself
// should be excluded, not all same-package free-function references.

func (s *FreeFunctionDepsTestSuite) TestSamePackageFreeFunctionWiringIsCountedAsUsage() {
	s.
		givenRepoOnlyWiredByAnotherFreeFunctionInSamePackage().
		whenAnalyzingWithTypeInfo().
		thenRepoShouldHaveFreeFunctionReferencesOf(1)
}

// Feature: Dependency Detection Through Free Functions
// Scenario: a constructor is only ever called from main()

func (s *FreeFunctionDepsTestSuite) TestConstructorCalledFromMain() {
	s.
		givenDBOnlyConstructedByMain().
		whenAnalyzingWithTypeInfo().
		thenDBShouldHaveInDegreeOf(1)
}

// Feature: Dependency Detection Through Free Functions
// Scenario: a component genuinely referenced nowhere must still report zero in-degree,
// so a future fix for the above cases must not start over-reporting usage.

func (s *FreeFunctionDepsTestSuite) TestTrulyOrphanedComponentStaysZeroInDegree() {
	s.
		givenAnOrphanedComponentAndAnUnrelatedFreeFunction().
		whenAnalyzingWithTypeInfo().
		thenOrphanShouldHaveZeroInDegree()
}

// Feature: Dependency Detection Through Free Functions
// Scenario: free-function detection must still work when PackageTypeInfo only has the
// legacy LoadedPackages (by-name) map populated, not LoadedPackagesByPath - mirroring
// the fallback extractDependenciesWithTypes already relies on for method bodies.

func (s *FreeFunctionDepsTestSuite) TestConstructorCalledFromFreeFunction_LegacyPackageMapOnly() {
	s.
		givenRegistryOnlyConstructedByFreeFunction().
		whenAnalyzingWithOnlyLegacyPackageMap().
		thenRegistryShouldHaveFreeFunctionReferencesOf(1)
}

// Given steps

func (s *FreeFunctionDepsTestSuite) givenRepoOnlyConstructedByMultiReturnFreeFunction() *FreeFunctionDepsTestSuite {
	s.testdataDir = s.createTempTestdata(map[string]string{
		"storage/repo.go": `package storage
type Repo struct {
	dsn string
}
func NewRepo() (*Repo, error) {
	return &Repo{}, nil
}`,
		"bootstrap/setup.go": `package bootstrap
import "test/storage"
func Setup() (*storage.Repo, error) {
	repo, err := storage.NewRepo()
	return repo, err
}`,
		"go.mod": `module test
go 1.25`,
	})
	return s
}

func (s *FreeFunctionDepsTestSuite) givenRepoOnlyWiredByAnotherFreeFunctionInSamePackage() *FreeFunctionDepsTestSuite {
	s.testdataDir = s.createTempTestdata(map[string]string{
		"wiring/wiring.go": `package wiring
type Repo struct {
	dsn string
}
func NewRepo() *Repo {
	return &Repo{}
}
type Service struct {
	repo *Repo
}
func BuildService() *Service {
	repo := NewRepo()
	return &Service{repo: repo}
}`,
		"go.mod": `module test
go 1.25`,
	})
	return s
}

func (s *FreeFunctionDepsTestSuite) givenRegistryOnlyConstructedByFreeFunction() *FreeFunctionDepsTestSuite {
	s.testdataDir = s.createTempTestdata(map[string]string{
		"registry/registry.go": `package registry
type Registry struct {
	items map[string]string
}
func NewRegistry() *Registry {
	return &Registry{items: map[string]string{}}
}`,
		"bootstrap/setup.go": `package bootstrap
import "test/registry"
func Setup() *registry.Registry {
	r := registry.NewRegistry()
	return r
}`,
		"go.mod": `module test
go 1.25`,
	})
	return s
}

func (s *FreeFunctionDepsTestSuite) givenRegistryWithOnlyItsOwnUncalledConstructor() *FreeFunctionDepsTestSuite {
	s.testdataDir = s.createTempTestdata(map[string]string{
		"registry/registry.go": `package registry
type Registry struct {
	items map[string]string
}
func NewRegistry() *Registry {
	return &Registry{items: map[string]string{}}
}`,
		"bootstrap/setup.go": `package bootstrap
func Setup() string {
	return "noop"
}`,
		"go.mod": `module test
go 1.25`,
	})
	return s
}

func (s *FreeFunctionDepsTestSuite) givenDBOnlyConstructedByMain() *FreeFunctionDepsTestSuite {
	s.testdataDir = s.createTempTestdata(map[string]string{
		"database/db.go": `package database
type DB struct {
	dsn string
}
func New(dsn string) *DB {
	return &DB{dsn: dsn}
}`,
		"cmd/server/main.go": `package main
import "test/database"
func main() {
	db := database.New("dsn")
	_ = db
}`,
		"go.mod": `module test
go 1.25`,
	})
	return s
}

func (s *FreeFunctionDepsTestSuite) givenAnOrphanedComponentAndAnUnrelatedFreeFunction() *FreeFunctionDepsTestSuite {
	s.testdataDir = s.createTempTestdata(map[string]string{
		"stale/stale.go": `package stale
type Draft struct {
	title string
}`,
		"bootstrap/setup.go": `package bootstrap
func Setup() string {
	return "noop"
}`,
		"go.mod": `module test
go 1.25`,
	})
	return s
}

// When steps

func (s *FreeFunctionDepsTestSuite) whenAnalyzingWithTypeInfo() *FreeFunctionDepsTestSuite {
	p := parser.NewParser()
	components, pkgTypeInfo, err := p.ParseDirectoryWithTypes(s.testdataDir)
	s.Require().NoError(err, "ParseDirectoryWithTypes should not fail")

	s.parsedComponents = components
	s.pkgTypeInfo = pkgTypeInfo
	s.analyzed = s.analyzer.AnalyzeWithTypes(components, pkgTypeInfo)

	return s
}

// whenAnalyzingWithOnlyLegacyPackageMap parses normally, then strips
// LoadedPackagesByPath so only the legacy by-name LoadedPackages map survives,
// reproducing a hand-built PackageTypeInfo that never populated the by-path map.
func (s *FreeFunctionDepsTestSuite) whenAnalyzingWithOnlyLegacyPackageMap() *FreeFunctionDepsTestSuite {
	p := parser.NewParser()
	components, pkgTypeInfo, err := p.ParseDirectoryWithTypes(s.testdataDir)
	s.Require().NoError(err, "ParseDirectoryWithTypes should not fail")

	pkgTypeInfo.LoadedPackagesByPath = nil

	s.parsedComponents = components
	s.pkgTypeInfo = pkgTypeInfo
	s.analyzed = s.analyzer.AnalyzeWithTypes(components, pkgTypeInfo)

	return s
}

// Then steps

func (s *FreeFunctionDepsTestSuite) thenRegistryShouldHaveFreeFunctionReferencesOf(want int) *FreeFunctionDepsTestSuite {
	registry := s.findComponent("Registry")
	s.Require().NotNil(registry, "Registry component should exist")

	s.Equal(want, registry.FreeFunctionReferences,
		"Registry's free-function reference count must count only external callers (e.g. bootstrap.Setup), not its own constructor referencing itself")

	// Free-function usage has no Dependency edge behind it, so it must never leak
	// into the graph metrics that drive diagram layout/role classification.
	s.Require().NotNil(registry.Metrics, "Registry should have metrics computed")
	s.Equal(0, registry.Metrics.InDegree,
		"Free-function references must not be folded into Metrics.InDegree")
	return s
}

func (s *FreeFunctionDepsTestSuite) thenRepoShouldHaveFreeFunctionReferencesOf(want int) *FreeFunctionDepsTestSuite {
	repo := s.findComponent("Repo")
	s.Require().NotNil(repo, "Repo component should exist")

	s.Equal(want, repo.FreeFunctionReferences,
		"Repo's free-function reference count should reflect genuine external/cross-function usage")
	return s
}

func (s *FreeFunctionDepsTestSuite) thenDBShouldHaveInDegreeOf(want int) *FreeFunctionDepsTestSuite {
	db := s.findComponent("DB")
	s.Require().NotNil(db, "DB component should exist")
	s.Require().NotNil(db.Metrics, "DB should have metrics computed")

	s.Equal(want, db.Metrics.InDegree,
		"DB is constructed by main(), which synthesizes a real Dependency edge (parser.extractMainComponent), so it should have exactly one inbound edge")
	return s
}

func (s *FreeFunctionDepsTestSuite) thenOrphanShouldHaveZeroInDegree() *FreeFunctionDepsTestSuite {
	draft := s.findComponent("Draft")
	s.Require().NotNil(draft, "Draft component should exist")
	s.Require().NotNil(draft.Metrics, "Draft should have metrics computed")

	s.Equal(0, draft.Metrics.InDegree,
		"Draft is referenced nowhere, so it must stay at zero in-degree even after free-function edges are added")
	s.Equal(0, draft.FreeFunctionReferences,
		"Draft is referenced nowhere, so it must stay at zero free-function references too")
	return s
}

// Helper methods

func (s *FreeFunctionDepsTestSuite) findComponent(name string) *AnalyzedComponent {
	for i := range s.analyzed {
		if s.analyzed[i].Component.Name == name {
			return &s.analyzed[i]
		}
	}
	return nil
}

func (s *FreeFunctionDepsTestSuite) createTempTestdata(files map[string]string) string {
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

// TestFreeFunctionDepsTestSuite is the entry point for running the suite
func TestFreeFunctionDepsTestSuite(t *testing.T) {
	suite.Run(t, new(FreeFunctionDepsTestSuite))
}
