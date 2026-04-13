// Package templates exposes the curated one-click app gallery.
//
// The manifest is embedded at build time from manifest.json and parsed
// once at package init. Callers look up a template by slug via Resolve,
// or enumerate the whole catalog via List. The package has zero
// dependencies on db / api / http so both the web handler and the
// ezkeel CLI can import it without pulling in the web stack.
package templates

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// Template is one curated starter app in the gallery.
type Template struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Icon          string `json:"icon"`
	RepoURL       string `json:"repo_url"`
	Framework     string `json:"framework"`
	NeedsDatabase bool   `json:"needs_database"`
	Port          int    `json:"port,omitempty"`
}

// ErrTemplateNotFound is returned by Resolve when the slug does not
// match any template in the manifest.
var ErrTemplateNotFound = errors.New("template not found")

//go:embed manifest.json
var manifestJSON []byte

// catalog is the parsed manifest, indexed by slug for O(1) lookup.
// Populated in init so tests and callers see a consistent view.
var catalog map[string]Template

// ordered holds the slugs in deterministic alphabetical order, used
// by List so the CLI and dashboard always render templates in the
// same sequence regardless of map iteration order.
var ordered []string

func init() {
	var all []Template
	if err := json.Unmarshal(manifestJSON, &all); err != nil {
		panic(fmt.Sprintf("templates: bad manifest.json: %v", err))
	}
	catalog = make(map[string]Template, len(all))
	ordered = make([]string, 0, len(all))
	for _, t := range all {
		if t.Slug == "" {
			panic("templates: manifest entry missing slug")
		}
		if _, dup := catalog[t.Slug]; dup {
			panic(fmt.Sprintf("templates: duplicate slug %q", t.Slug))
		}
		catalog[t.Slug] = t
		ordered = append(ordered, t.Slug)
	}
	sort.Strings(ordered)
}

// Resolve returns the template with the given slug, or
// ErrTemplateNotFound.
func Resolve(slug string) (*Template, error) {
	t, ok := catalog[slug]
	if !ok {
		return nil, ErrTemplateNotFound
	}
	return &t, nil
}

// List returns every template in deterministic slug order. The
// returned slice is a fresh copy — callers may mutate it without
// affecting the catalog.
func List() []Template {
	out := make([]Template, 0, len(ordered))
	for _, slug := range ordered {
		out = append(out, catalog[slug])
	}
	return out
}
