// Package architecture checks production Go imports against explicit layer rules.
package architecture

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type Rule struct {
	Package  string   `json:"package"`
	Internal []string `json:"internal"`
	Stdlib   []string `json:"stdlib"`
	External []string `json:"external"`
}

type Manifest struct {
	SchemaVersion int    `json:"schema_version"`
	Module        string `json:"module"`
	Rules         []Rule `json:"rules"`
}

type Violation struct {
	File   string
	Import string
	Reason string
}

func (v Violation) String() string { return fmt.Sprintf("%s: %s: %s", v.File, v.Import, v.Reason) }

func hasPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func allowsPrefix(path string, allowed []string) bool {
	for _, prefix := range allowed {
		if hasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func standardImport(path string) bool {
	if path == "C" || !filepath.IsLocal(path) {
		return false
	}
	stat, err := os.Stat(filepath.Join(runtime.GOROOT(), "src", filepath.FromSlash(path)))
	return err == nil && stat.IsDir()
}

// Check scans every production .go file, including non-host build-tag variants.
// Tests are excluded because test doubles legitimately depend on several layers.
func Check(root, manifestPath string) ([]Violation, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(manifestPath) {
		manifestPath = filepath.Join(root, manifestPath)
	}
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var manifest Manifest
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, err
	}
	if manifest.SchemaVersion != 1 || manifest.Module == "" || len(manifest.Rules) == 0 {
		return nil, fmt.Errorf("invalid architecture manifest")
	}
	moduleFile, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	match := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindSubmatch(moduleFile)
	if len(match) != 2 || string(match[1]) != manifest.Module {
		return nil, fmt.Errorf("architecture module does not match go.mod")
	}
	seenRules := map[string]bool{}
	for _, rule := range manifest.Rules {
		if rule.Package == "" || seenRules[rule.Package] {
			return nil, fmt.Errorf("empty or duplicate rule: %q", rule.Package)
		}
		seenRules[rule.Package] = true
	}
	sort.Slice(manifest.Rules, func(i, j int) bool { return len(manifest.Rules[i].Package) > len(manifest.Rules[j].Package) })
	var violations []Violation
	graph := map[string][]string{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "node_modules" || entry.Name() == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		relative = filepath.ToSlash(relative)
		pkg := filepath.ToSlash(filepath.Dir(relative))
		var matched *Rule
		for i := range manifest.Rules {
			if hasPrefix(pkg, manifest.Rules[i].Package) {
				matched = &manifest.Rules[i]
				break
			}
		}
		if matched == nil {
			violations = append(violations, Violation{relative, pkg, "unregistered production package"})
			return nil
		}
		if _, ok := graph[pkg]; !ok {
			graph[pkg] = nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("%s: %w", relative, err)
		}
		for _, node := range parsed.Imports {
			importPath, err := strconv.Unquote(node.Path.Value)
			if err != nil {
				return err
			}
			if hasPrefix(importPath, manifest.Module) {
				dependency := strings.TrimPrefix(importPath, manifest.Module+"/")
				graph[pkg] = append(graph[pkg], dependency)
				if !allowsPrefix(dependency, matched.Internal) {
					violations = append(violations, Violation{relative, importPath, "forbidden internal dependency"})
				}
			} else if standardImport(importPath) {
				permitted := false
				for _, name := range matched.Stdlib {
					if name == "*" || name == importPath {
						permitted = true
						break
					}
				}
				if !permitted {
					violations = append(violations, Violation{relative, importPath, "standard-library effect is outside layer contract"})
				}
			} else if !allowsPrefix(importPath, matched.External) {
				violations = append(violations, Violation{relative, importPath, "external dependency is not approved for this layer"})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(graph))
	for key := range graph {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	state := map[string]int{}
	var visit func(string)
	visit = func(pkg string) {
		state[pkg] = 1
		for _, dependency := range graph[pkg] {
			if _, exists := graph[dependency]; !exists {
				continue
			}
			if state[dependency] == 1 {
				violations = append(violations, Violation{pkg, dependency, "production dependency cycle"})
			} else if state[dependency] == 0 {
				visit(dependency)
			}
		}
		state[pkg] = 2
	}
	for _, pkg := range keys {
		if state[pkg] == 0 {
			visit(pkg)
		}
	}
	sort.Slice(violations, func(i, j int) bool { return violations[i].String() < violations[j].String() })
	return violations, nil
}
