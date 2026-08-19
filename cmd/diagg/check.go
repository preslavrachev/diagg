package main

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/urfave/cli/v2"
)

var defaultRootBudgetIgnore = []string{
	"cmd", "config", "database", "web", "views",
	"auth", "jobs", "testutil", "httputil", "logging",
}

var defaultGenericNames = []string{
	"model", "core", "common", "utils", "helpers",
	"service", "manager", "business", "domain", "features",
}

func checkCommand() *cli.Command {
	return &cli.Command{
		Name:  "check",
		Usage: "Run structural checks against a Go module's package graph",
		Subcommands: []*cli.Command{
			rootBudgetCommand(),
			genericNamesCommand(),
		},
	}
}

func rootBudgetCommand() *cli.Command {
	return &cli.Command{
		Name:  "root-budget",
		Usage: "Fail if the module has more root-level product/domain packages than allowed",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "max",
				Value: 5,
				Usage: "Maximum number of root-level product/domain packages allowed",
			},
			&cli.StringFlag{
				Name:  "ignore",
				Value: strings.Join(defaultRootBudgetIgnore, ","),
				Usage: "Comma-separated root package names to exclude (infrastructure/support packages)",
			},
			&cli.BoolFlag{
				Name:  "warn-only",
				Usage: "Print the result but always exit 0",
			},
		},
		Action: runRootBudget,
	}
}

func runRootBudget(c *cli.Context) error {
	targetDir := "."
	if c.NArg() > 0 {
		targetDir = c.Args().Get(0)
	}

	graph, err := loadPackageGraph(targetDir)
	if err != nil {
		return err
	}

	ignore := toSet(splitCSV(c.String("ignore")))
	counted, ignored := filterRootBudget(graph, ignore)

	max := c.Int("max")

	fmt.Printf("Root-level product packages (%d, budget %d):\n", len(counted), max)
	for _, pkg := range counted {
		fmt.Printf("  - %s\n", pkg.Path)
	}

	fmt.Printf("\nIgnored infrastructure/support packages (%d):\n", len(ignored))
	for _, pkg := range ignored {
		fmt.Printf("  - %s\n", pkg.Path)
	}

	if len(counted) > max {
		fmt.Printf("\nFAIL: %d root-level product packages exceeds budget of %d\n", len(counted), max)
		if c.Bool("warn-only") {
			return nil
		}
		return cli.Exit("", 1)
	}

	fmt.Printf("\nOK: %d root-level product packages within budget of %d\n", len(counted), max)
	return nil
}

func genericNamesCommand() *cli.Command {
	return &cli.Command{
		Name:  "generic-names",
		Usage: "Flag packages with generic, low-information names",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "deny",
				Value: strings.Join(defaultGenericNames, ","),
				Usage: "Comma-separated package names to flag as generic",
			},
			&cli.BoolFlag{
				Name:  "warn-only",
				Usage: "Print the result but always exit 0",
			},
		},
		Action: runGenericNames,
	}
}

func runGenericNames(c *cli.Context) error {
	targetDir := "."
	if c.NArg() > 0 {
		targetDir = c.Args().Get(0)
	}

	graph, err := loadPackageGraph(targetDir)
	if err != nil {
		return err
	}

	deny := toSet(splitCSV(c.String("deny")))
	flagged := filterGenericNames(graph, deny)

	if len(flagged) == 0 {
		fmt.Println("OK: no packages matched the generic-name deny list")
		return nil
	}

	fmt.Printf("Flagged %d package(s) with generic names:\n", len(flagged))
	for _, pkg := range flagged {
		fmt.Printf("  - %s (in-degree=%d, out-degree=%d, total-degree=%d)\n",
			pkg.Path, pkg.InDegree, pkg.OutDegree, pkg.TotalDegree)
		if len(pkg.Imports) > 0 {
			fmt.Printf("      imports: %s\n", strings.Join(pkg.Imports, ", "))
		}
	}

	if c.Bool("warn-only") {
		return nil
	}
	return cli.Exit("", 1)
}

// filterRootBudget splits a package graph's root-level packages into those
// counted against the budget and those excluded by the ignore set.
func filterRootBudget(graph analyzer.PackageGraph, ignore map[string]bool) (counted, ignored []analyzer.PackageNode) {
	for _, pkg := range graph.Packages {
		if !pkg.RootLevel {
			continue
		}
		if ignore[rootSegmentName(graph.ModulePath, pkg.Path)] {
			ignored = append(ignored, pkg)
			continue
		}
		counted = append(counted, pkg)
	}
	sortPackagesByPath(counted)
	sortPackagesByPath(ignored)
	return counted, ignored
}

// filterGenericNames returns packages whose declared name or import-path
// directory segment matches the deny set, so a generic directory is still
// flagged even when its package identifier differs (e.g. dir "model" with
// "package datamodel").
func filterGenericNames(graph analyzer.PackageGraph, deny map[string]bool) []analyzer.PackageNode {
	var flagged []analyzer.PackageNode
	for _, pkg := range graph.Packages {
		if deny[pkg.Name] || deny[path.Base(pkg.Path)] {
			flagged = append(flagged, pkg)
		}
	}
	sortPackagesByPath(flagged)
	return flagged
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

// rootSegmentName returns the single path segment of a root-level package
// relative to the module root (e.g. "foo" for "module/foo").
func rootSegmentName(modulePath, pkgPath string) string {
	rel := strings.TrimPrefix(pkgPath, modulePath+"/")
	return rel
}

func sortPackagesByPath(pkgs []analyzer.PackageNode) {
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Path < pkgs[j].Path })
}
