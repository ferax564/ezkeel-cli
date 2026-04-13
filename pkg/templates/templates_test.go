package templates

import (
	"errors"
	"sort"
	"testing"
)

func TestResolve_Known(t *testing.T) {
	tmpl, err := Resolve("todo-list")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if tmpl.Slug != "todo-list" {
		t.Errorf("slug: got %q, want todo-list", tmpl.Slug)
	}
	if tmpl.Framework != "express" {
		t.Errorf("framework: got %q, want express", tmpl.Framework)
	}
	if !tmpl.NeedsDatabase {
		t.Error("needs_database: got false, want true")
	}
	if tmpl.Port != 3000 {
		t.Errorf("port: got %d, want 3000", tmpl.Port)
	}
	if tmpl.RepoURL == "" {
		t.Error("repo_url: got empty")
	}
}

func TestResolve_Unknown(t *testing.T) {
	_, err := Resolve("does-not-exist")
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Errorf("err: got %v, want ErrTemplateNotFound", err)
	}
}

func TestResolve_EmptySlug(t *testing.T) {
	_, err := Resolve("")
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Errorf("err: got %v, want ErrTemplateNotFound", err)
	}
}

func TestList_SortedBySlug(t *testing.T) {
	all := List()
	if len(all) == 0 {
		t.Fatal("List returned no templates — manifest must have >= 1 entry")
	}
	slugs := make([]string, len(all))
	for i, tmpl := range all {
		slugs[i] = tmpl.Slug
	}
	if !sort.StringsAreSorted(slugs) {
		t.Errorf("List not sorted by slug: got %v", slugs)
	}
}

func TestList_IsDefensiveCopy(t *testing.T) {
	first := List()
	second := List()
	if len(first) == 0 {
		t.Skip("no templates to mutate")
	}
	first[0].Slug = "mutated"
	if second[0].Slug == "mutated" {
		t.Error("List should return a defensive copy; catalog was mutated through returned slice")
	}
}
