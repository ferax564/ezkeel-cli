package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/ferax564/ezkeel-cli/internal/detect"
	"github.com/ferax564/ezkeel-cli/internal/spec"
	"github.com/ferax564/ezkeel-cli/pkg/templates"
)

func TestAppNameFromRepo(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"github.com/user/my-app", "my-app"},
		{"github.com/user/my-app.git", "my-app"},
		{"https://github.com/user/my-app", "my-app"},
		{"https://github.com/user/my-app.git", "my-app"},
		{"git@github.com:user/my-app.git", "my-app"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := appNameFromRepo(tt.input)
			if got != tt.want {
				t.Errorf("appNameFromRepo(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveTemplateArg_Empty(t *testing.T) {
	args := []string{"https://example.com/repo.git"}
	out, warn, err := resolveTemplateArg("", args)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if warn != "" {
		t.Errorf("warn: got %q, want empty", warn)
	}
	if len(out) != 1 || out[0] != args[0] {
		t.Errorf("args: got %v, want %v", out, args)
	}
}

func TestResolveTemplateArg_Happy(t *testing.T) {
	// Pick a template that's guaranteed to exist in the embedded manifest.
	all := templates.List()
	if len(all) == 0 {
		t.Skip("no templates in manifest")
	}
	slug := all[0].Slug

	out, warn, err := resolveTemplateArg(slug, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if warn != "" {
		t.Errorf("warn: got %q, want empty when no positional arg", warn)
	}
	if len(out) != 1 || !strings.Contains(out[0], ".git") {
		t.Errorf("args: got %v, want one repo URL", out)
	}
}

func TestResolveTemplateArg_Unknown(t *testing.T) {
	_, _, err := resolveTemplateArg("does-not-exist", nil)
	if !errors.Is(err, templates.ErrTemplateNotFound) {
		t.Errorf("err: got %v, want ErrTemplateNotFound", err)
	}
}

func TestResolveTemplateArg_FlagOverridesPositional(t *testing.T) {
	all := templates.List()
	if len(all) == 0 {
		t.Skip("no templates in manifest")
	}
	slug := all[0].Slug

	out, warn, err := resolveTemplateArg(slug, []string{"https://example.com/other.git"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if warn == "" {
		t.Error("expected warning when both flag and positional are given")
	}
	if !strings.Contains(warn, slug) {
		t.Errorf("warning should mention the slug %q: got %q", slug, warn)
	}
	if len(out) != 1 || out[0] == "https://example.com/other.git" {
		t.Errorf("args: got %v, expected the template URL to win", out)
	}
}

func TestAppNameFromDir(t *testing.T) {
	got := appNameFromDir("/home/user/projects/my-cool-app")
	want := "my-cool-app"
	if got != want {
		t.Errorf("appNameFromDir(%q) = %q, want %q", "/home/user/projects/my-cool-app", got, want)
	}
}

func TestApplySpecOverridesPort(t *testing.T) {
	fr := &detect.FrameworkResult{
		Framework:  detect.FrameworkExpress,
		Build:      "npm ci",
		Start:      "node server.js",
		Port:       3000,
		Dockerfile: "auto",
	}
	s := &spec.Spec{Name: "x", Port: 8080, Start: "node custom.js"}

	if !applySpec(fr, s) {
		t.Errorf("applySpec returned false despite overrides")
	}

	if fr.Port != 8080 {
		t.Errorf("Port = %d, want 8080", fr.Port)
	}
	if fr.Start != "node custom.js" {
		t.Errorf("Start = %q, want override", fr.Start)
	}
	if fr.Build != "npm ci" {
		t.Errorf("Build = %q, want untouched", fr.Build)
	}
}

func TestApplySpecNilNoop(t *testing.T) {
	fr := &detect.FrameworkResult{Framework: detect.FrameworkExpress, Port: 3000}
	if applySpec(fr, nil) {
		t.Errorf("applySpec(nil) returned true")
	}
	if fr.Port != 3000 {
		t.Errorf("Port mutated to %d", fr.Port)
	}
}

func TestApplySpec_ReportsOverridden(t *testing.T) {
	fr := &detect.FrameworkResult{Framework: detect.FrameworkExpress, Port: 3000}
	if applySpec(fr, &spec.Spec{Name: "x"}) {
		t.Errorf("applySpec returned true for empty-override spec")
	}
	if !applySpec(fr, &spec.Spec{Name: "x", Port: 8080}) {
		t.Errorf("applySpec returned false despite Port override")
	}
}

func TestApplySpec_SetsDockerfileAutoWhenSpecRescues(t *testing.T) {
	fr := &detect.FrameworkResult{Framework: detect.FrameworkUnknown, Dockerfile: ""}
	applySpec(fr, &spec.Spec{Name: "x", Framework: "rust"})
	if fr.Framework != detect.Framework("rust") {
		t.Errorf("Framework = %q, want rust", fr.Framework)
	}
	if fr.Dockerfile != "auto" {
		t.Errorf("Dockerfile = %q, want auto", fr.Dockerfile)
	}
}

func TestApplySpec_PreservesExistingDockerfile(t *testing.T) {
	fr := &detect.FrameworkResult{Framework: detect.FrameworkExpress, Dockerfile: "Dockerfile"}
	applySpec(fr, &spec.Spec{Name: "x", Framework: "rust"})
	// Spec overrode framework but the repo already had a Dockerfile —
	// don't clobber it to "auto".
	if fr.Dockerfile != "Dockerfile" {
		t.Errorf("Dockerfile = %q, want Dockerfile preserved", fr.Dockerfile)
	}
}

func TestApplySpec_FrameworkOnlyFillsDefaults(t *testing.T) {
	// When auto-detect returned FrameworkUnknown and the spec rescues
	// with `framework: rust` only, applySpec must populate Build/Start/
	// Port from the canonical defaults — otherwise the generated
	// Dockerfile emits EXPOSE 0 / CMD [].
	fr := &detect.FrameworkResult{Framework: detect.FrameworkUnknown}
	if !applySpec(fr, &spec.Spec{Name: "x", Framework: "rust"}) {
		t.Errorf("applySpec returned false despite framework override")
	}
	if fr.Build != "cargo build --release" {
		t.Errorf("Build = %q, want cargo build --release", fr.Build)
	}
	if fr.Start != "./app" {
		t.Errorf("Start = %q, want ./app", fr.Start)
	}
	if fr.Port != 8080 {
		t.Errorf("Port = %d, want 8080", fr.Port)
	}
	if fr.Dockerfile != "auto" {
		t.Errorf("Dockerfile = %q, want auto", fr.Dockerfile)
	}
}

func TestApplySpec_ExplicitOverridesDefaults(t *testing.T) {
	// Explicit spec values must beat the framework-defaults table.
	fr := &detect.FrameworkResult{Framework: detect.FrameworkUnknown}
	applySpec(fr, &spec.Spec{Name: "x", Framework: "rust", Port: 9999})
	if fr.Port != 9999 {
		t.Errorf("Port = %d, want 9999 (explicit override)", fr.Port)
	}
	// Start was not explicitly set, so the default carries through.
	// (Default Start for Rust references the runner-stage path /app/app,
	// not the builder-stage target/release/app.)
	if fr.Start != "./app" {
		t.Errorf("Start = %q, want default carry-over", fr.Start)
	}
}

func TestApplySpec_FrameworkAugmentPreservesDetectedFields(t *testing.T) {
	// If auto-detect populated fields and the spec just restates the
	// framework, applySpec must NOT clobber the detected Build/Start/
	// Port from the defaults table.
	fr := &detect.FrameworkResult{
		Framework: detect.FrameworkExpress,
		Build:     "npm ci && npm run build",
		Start:     "node ./dist/main.js",
		Port:      4000,
	}
	applySpec(fr, &spec.Spec{Name: "x", Framework: "express"})
	if fr.Build != "npm ci && npm run build" {
		t.Errorf("Build clobbered: %q", fr.Build)
	}
	if fr.Start != "node ./dist/main.js" {
		t.Errorf("Start clobbered: %q", fr.Start)
	}
	if fr.Port != 4000 {
		t.Errorf("Port clobbered: %d", fr.Port)
	}
}

func TestApplySpec_FrameworkSwitchResetsDefaults(t *testing.T) {
	// Auto-detect found Vite; spec overrides to express. Vite's start
	// command must NOT carry over — otherwise the Express container
	// ships with "npx serve dist" and dies on first launch.
	fr := &detect.FrameworkResult{
		Framework:  detect.FrameworkVite,
		Build:      "npm run build",
		Start:      "npx serve dist",
		Port:       5173,
		Dockerfile: "auto",
	}
	applySpec(fr, &spec.Spec{Name: "x", Framework: "express"})

	if fr.Framework != detect.FrameworkExpress {
		t.Errorf("Framework = %q, want express", fr.Framework)
	}
	if fr.Start != "node index.js" {
		t.Errorf("Start = %q, want node index.js (express default, NOT vite carry-over)", fr.Start)
	}
	if fr.Port != 3000 {
		t.Errorf("Port = %d, want 3000 (express default)", fr.Port)
	}
}

func TestApplySpec_SwitchWithExplicitStartUsesExplicit(t *testing.T) {
	// Switch case + explicit Start in spec — explicit value wins over
	// both detected and switched defaults.
	fr := &detect.FrameworkResult{
		Framework:  detect.FrameworkVite,
		Start:      "npx serve dist",
		Port:       5173,
		Dockerfile: "auto",
	}
	applySpec(fr, &spec.Spec{Name: "x", Framework: "express", Start: "node server.js"})

	if fr.Start != "node server.js" {
		t.Errorf("Start = %q, want explicit spec value", fr.Start)
	}
	if fr.Port != 3000 {
		t.Errorf("Port = %d, want express default after switch", fr.Port)
	}
}

func TestApplyServicesFromSpec_OverridesDetect(t *testing.T) {
	detected := &detect.DatabaseResult{Engine: detect.DBNone}
	got, err := applyServicesFromSpec(detected, &spec.Spec{
		Name:     "x",
		Services: map[string]spec.Service{"db": {Engine: "postgres"}},
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Engine != detect.DBPostgres {
		t.Errorf("Engine = %q, want postgres", got.Engine)
	}
}

func TestApplyServicesFromSpec_CarriesVersion(t *testing.T) {
	// Spec-declared Postgres 15 must thread through DBCreateRequest;
	// otherwise the deploy step always provisions PG 16 regardless.
	detected := &detect.DatabaseResult{Engine: detect.DBNone}
	got, err := applyServicesFromSpec(detected, &spec.Spec{
		Name:     "x",
		Services: map[string]spec.Service{"db": {Engine: "postgres", Version: "15"}},
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Engine != detect.DBPostgres {
		t.Errorf("Engine = %q, want postgres", got.Engine)
	}
	if got.Version != "15" {
		t.Errorf("Version = %q, want 15", got.Version)
	}
}

func TestApplyServicesFromSpec_NilSpecNoop(t *testing.T) {
	detected := &detect.DatabaseResult{Engine: detect.DBPostgres}
	got, err := applyServicesFromSpec(detected, nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != detected {
		t.Errorf("expected unchanged pointer for nil spec")
	}
}

func TestNormalizeDBEngine_Aliases(t *testing.T) {
	cases := []struct {
		in   string
		want detect.DBEngine
	}{
		{"postgres", detect.DBPostgres},
		{"postgresql", detect.DBPostgres},
		{"PostgreSQL", detect.DBPostgres},
		{"pg", detect.DBPostgres},
		{"mysql", detect.DBMySQL},
		{"mariadb", detect.DBMySQL},
	}
	for _, c := range cases {
		got, err := normalizeDBEngine(c.in)
		if err != nil {
			t.Errorf("%q: unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeDBEngine_Rejects(t *testing.T) {
	rejected := []string{"sqlite", "mongodb", "oracle", "", "redis"}
	for _, in := range rejected {
		if _, err := normalizeDBEngine(in); err == nil {
			t.Errorf("%q: expected error, got nil", in)
		}
	}
}

func TestApplyServicesFromSpec_Postgresql(t *testing.T) {
	// `postgresql` (with the trailing -ql) is a common alias from
	// people who type the full name. Without normalization the cast
	// to detect.DBEngine yields "postgresql", which never matches
	// DBPostgres == "postgres" — DB provisioning silently skips and
	// the app starts without DATABASE_URL.
	detected := &detect.DatabaseResult{Engine: detect.DBNone}
	got, err := applyServicesFromSpec(detected, &spec.Spec{
		Name:     "x",
		Services: map[string]spec.Service{"db": {Engine: "postgresql"}},
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Engine != detect.DBPostgres {
		t.Errorf("Engine = %q, want postgres (alias normalized)", got.Engine)
	}
}

func TestApplyServicesFromSpec_RejectsUnknown(t *testing.T) {
	detected := &detect.DatabaseResult{Engine: detect.DBPostgres}
	_, err := applyServicesFromSpec(detected, &spec.Spec{
		Name:     "x",
		Services: map[string]spec.Service{"db": {Engine: "sqlite"}},
	})
	if err == nil {
		t.Fatalf("expected error for unsupported engine")
	}
	if !strings.Contains(err.Error(), "sqlite") {
		t.Errorf("error should name the unsupported engine: %v", err)
	}
}

func TestValidateSpecFramework_RejectsCommonTypos(t *testing.T) {
	// `framework: nodejs` is a common copy-paste from the comment in
	// templates/ezkeel.yaml. Without validation, applySpec sets a
	// non-empty Framework and the FrameworkUnknown gate inside runUp
	// silently passes — failure surfaces much later in dockerfile
	// generation as "no Dockerfile template for framework nodejs".
	typos := []string{"nodejs", "node", "Node", "rust-lang", "express.js"}
	for _, name := range typos {
		err := validateSpecFramework(&spec.Spec{Name: "x", Framework: name})
		if err == nil {
			t.Errorf("validateSpecFramework(%q) returned nil, want unsupported-framework error", name)
		}
	}
}

func TestValidateSpecFramework_AcceptsKnown(t *testing.T) {
	known := []string{"go", "rust", "express", "nextjs", "fastapi"}
	for _, name := range known {
		if err := validateSpecFramework(&spec.Spec{Name: "x", Framework: name}); err != nil {
			t.Errorf("validateSpecFramework(%q) = %v, want nil", name, err)
		}
	}
}

func TestValidateSpecFramework_NilAndEmpty(t *testing.T) {
	if err := validateSpecFramework(nil); err != nil {
		t.Errorf("validateSpecFramework(nil) = %v, want nil", err)
	}
	if err := validateSpecFramework(&spec.Spec{Name: "x"}); err != nil {
		t.Errorf("validateSpecFramework(empty framework) = %v, want nil (auto-detect path)", err)
	}
}

func TestResolveAppName_FlagWins(t *testing.T) {
	got := resolveAppName("flag-name", "https://x.com/y/repo", "/dir", &spec.Spec{Name: "spec-name"})
	if got != "flag-name" {
		t.Errorf("got %q, want flag-name", got)
	}
}

func TestResolveAppName_SpecBeatsRepo(t *testing.T) {
	got := resolveAppName("", "https://x.com/y/repo", "/dir", &spec.Spec{Name: "spec-name"})
	if got != "spec-name" {
		t.Errorf("got %q, want spec-name", got)
	}
}

func TestResolveAppName_RepoBeatsDir(t *testing.T) {
	got := resolveAppName("", "https://x.com/y/repo-name", "/path/dir-name", nil)
	if got != "repo-name" {
		t.Errorf("got %q, want repo-name", got)
	}
}

func TestResolveAppName_DirFallback(t *testing.T) {
	got := resolveAppName("", "", "/path/to/myapp", nil)
	if got != "myapp" {
		t.Errorf("got %q, want myapp", got)
	}
}

func TestResolveAppName_NilSpec(t *testing.T) {
	got := resolveAppName("", "https://x.com/y/repo", "/dir", nil)
	if got != "repo" {
		t.Errorf("got %q, want repo", got)
	}
}

func TestResolveAppName_EmptySpecNameFallsThrough(t *testing.T) {
	// A Spec with empty Name should fall through to the repo URL,
	// not return "". (Defensive — spec.Load enforces Name != "" but
	// callers might construct Spec literals.)
	got := resolveAppName("", "https://x.com/y/repo", "/dir", &spec.Spec{})
	if got != "repo" {
		t.Errorf("got %q, want repo", got)
	}
}

func TestResolveResources_FlagWins(t *testing.T) {
	m, c := resolveResources("1g", "2.0", &spec.Spec{
		Resources: spec.Resources{Memory: "512m", CPUs: "1.0"},
	})
	if m != "1g" || c != "2.0" {
		t.Errorf("got mem=%q cpu=%q; want flag values 1g/2.0", m, c)
	}
}

func TestResolveResources_SpecFillsBlanks(t *testing.T) {
	m, c := resolveResources("", "", &spec.Spec{
		Resources: spec.Resources{Memory: "512m", CPUs: "1.0"},
	})
	if m != "512m" || c != "1.0" {
		t.Errorf("got mem=%q cpu=%q; want spec values 512m/1.0", m, c)
	}
}

func TestResolveResources_PartialFlag(t *testing.T) {
	m, c := resolveResources("2g", "", &spec.Spec{
		Resources: spec.Resources{Memory: "512m", CPUs: "1.0"},
	})
	if m != "2g" || c != "1.0" {
		t.Errorf("got mem=%q cpu=%q; want 2g/1.0 (flag wins for memory, spec fills cpus)", m, c)
	}
}

func TestResolveResources_NilSpec(t *testing.T) {
	m, c := resolveResources("", "", nil)
	if m != "" || c != "" {
		t.Errorf("got mem=%q cpu=%q; want empty", m, c)
	}
}

func TestResolveResources_NilSpecPreservesFlags(t *testing.T) {
	m, c := resolveResources("1g", "0.5", nil)
	if m != "1g" || c != "0.5" {
		t.Errorf("got mem=%q cpu=%q; want flag pass-through 1g/0.5", m, c)
	}
}

func TestResolveResources_EmptySpecResources(t *testing.T) {
	// Spec with no resources block should behave like nil for resources.
	m, c := resolveResources("", "", &spec.Spec{Name: "x"})
	if m != "" || c != "" {
		t.Errorf("got mem=%q cpu=%q; want empty (spec has no resources)", m, c)
	}
}

func TestFormatDryRun(t *testing.T) {
	info := dryRunInfo{
		AppName:    "my-app",
		Framework:  "nextjs",
		Build:      "npm run build",
		Start:      "node .next/standalone/server.js",
		Port:       3000,
		DBEngine:   "postgresql",
		Dockerfile: "auto",
		Domain:     "my-app.deploy.example.com",
		Server:     "vps01",
	}

	output := formatDryRun(&info)

	if !strings.Contains(output, "my-app") {
		t.Error("expected app name in output")
	}
	if !strings.Contains(output, "nextjs") {
		t.Error("expected framework in output")
	}
	if !strings.Contains(output, "postgresql") {
		t.Error("expected database in output")
	}
	if !strings.Contains(output, "3000") {
		t.Error("expected port in output")
	}
	if !strings.Contains(output, "my-app.deploy.example.com") {
		t.Error("expected domain in output")
	}
}
