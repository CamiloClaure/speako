// Package arch holds the one test that makes the architecture real.
//
// Folder names do not enforce anything. This does: it reads the actual import
// graph and fails the build when a layer reaches somewhere it must not.
package arch_test

import (
	"encoding/json"
	"io"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const module = "github.com/camiloclaure/speako/api"

// layer describes what a group of packages is allowed to import, beyond the
// standard library (which is always allowed).
type layer struct {
	name    string
	prefix  string
	allowed []string
	// forbidden lists standard-library packages the layer must not touch.
	// "stdlib only" is not enough for the domain: net/http is stdlib, and a
	// business rule that knows a status code has already leaked.
	forbidden []string
	reason    string
}

// Ordered most-specific first: a package is matched against the first prefix that fits.
var layers = []layer{
	{
		name:   "domain",
		prefix: module + "/internal/domain",
		allowed: []string{
			module + "/internal/domain",
		},
		forbidden: []string{
			"net/http", "net", "net/url", "database/sql",
			"context", "encoding/json", "encoding/xml",
			"os", "log", "log/slog", "io/fs", "embed",
		},
		reason: "business rules must not know that any technology exists",
	},
	{
		name:   "app",
		prefix: module + "/internal/app",
		allowed: []string{
			module + "/internal/domain",
			module + "/internal/app",
		},
		forbidden: []string{
			"net/http", "database/sql", "encoding/json", "os", "log",
		},
		reason: "use cases talk to ports, never to concrete adapters or transport",
	},
	{
		name:   "http",
		prefix: module + "/internal/http",
		allowed: []string{
			module + "/internal/domain",
			module + "/internal/app",
			module + "/internal/http",
		},
		reason: "transport receives ports already wired by cmd; it must not construct adapters",
	},
	{
		name:   "adapters",
		prefix: module + "/internal/adapters",
		allowed: []string{
			module + "/internal/domain",
			module + "/internal/app",
			module + "/internal/adapters",
		},
		reason: "an adapter implements a port; it has no business knowing about transport",
	},
	{
		name:   "config",
		prefix: module + "/internal/config",
		allowed: []string{
			module + "/internal/config",
		},
		reason: "config is read at boot and passed down; it depends on nothing internal",
	},
}

// thirdPartyAllowed lists layers that may import modules outside the standard
// library. domain, app and config may not - that is the whole point.
var thirdPartyAllowed = map[string]bool{
	"http":     true,
	"adapters": true,
}

type pkg struct {
	ImportPath  string
	Standard    bool
	Imports     []string
	TestImports []string
}

func TestDependencyRule(t *testing.T) {
	for _, p := range listPackages(t) {
		l, ok := layerFor(p.ImportPath)
		if !ok {
			continue // cmd/ and the repo root wire everything together on purpose
		}
		for _, imp := range append(append([]string{}, p.Imports...), p.TestImports...) {
			if imp == p.ImportPath {
				continue
			}
			if slices.Contains(l.forbidden, imp) {
				t.Errorf("%s (%s layer) imports forbidden stdlib %q\n  rule: %s", p.ImportPath, l.name, imp, l.reason)
				continue
			}
			if isStdlib(imp) {
				continue
			}
			if !strings.HasPrefix(imp, module+"/") {
				if thirdPartyAllowed[l.name] {
					continue
				}
				t.Errorf("%s (%s layer) imports third-party %q\n  rule: %s", p.ImportPath, l.name, imp, l.reason)
				continue
			}
			if !allowedBy(l, imp) {
				t.Errorf("%s (%s layer) imports %q\n  rule: %s", p.ImportPath, l.name, imp, l.reason)
			}
		}
	}
}

func listPackages(t *testing.T) []pkg {
	t.Helper()
	// Module-wide pattern, not "./...", so the check is independent of where it runs from.
	out, err := exec.Command("go", "list", "-json", module+"/...").Output()
	if err != nil {
		t.Fatalf("go list failed: %v", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	var pkgs []pkg
	for {
		var p pkg
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("decoding go list output: %v", err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}

func layerFor(importPath string) (layer, bool) {
	for _, l := range layers {
		if importPath == l.prefix || strings.HasPrefix(importPath, l.prefix+"/") {
			return l, true
		}
	}
	return layer{}, false
}

func allowedBy(l layer, imp string) bool {
	for _, a := range l.allowed {
		if imp == a || strings.HasPrefix(imp, a+"/") {
			return true
		}
	}
	return false
}

// isStdlib reports whether an import path belongs to the standard library.
// Every non-stdlib module path has a dot in its first segment (a domain name).
func isStdlib(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}
