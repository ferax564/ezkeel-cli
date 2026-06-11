package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ferax564/ezkeel-cli/internal/config"
	"github.com/ferax564/ezkeel-cli/internal/detect"
	"github.com/spf13/cobra"
)

func TestEmitJSON_NoHTMLEscaping(t *testing.T) {
	var buf bytes.Buffer
	if err := emitJSON(&buf, map[string]string{"url": "https://a?x=1&y=2"}); err != nil {
		t.Fatalf("emitJSON: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "&y=2") {
		t.Errorf("ampersand escaped: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("missing trailing newline: %q", got)
	}
}

func TestAppsToJSON(t *testing.T) {
	manifests := []*detect.AppManifest{
		{
			Name:   "blog",
			Server: "vps01",
			App:    detect.AppConfig{Framework: "nextjs", Port: 3000},
			Domain: "blog.deploy.example.com",
		},
		{
			Name:    "shop",
			Server:  "vps01",
			App:     detect.AppConfig{Framework: "express", Port: 8080},
			Domain:  "shop.deploy.example.com",
			Domains: []string{"shop.example.com"},
		},
	}

	got := appsToJSON(manifests)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].URL != "https://blog.deploy.example.com" {
		t.Errorf("URL = %q, want https:// prefix on the auto domain", got[0].URL)
	}
	if got[0].Domains == nil || len(got[0].Domains) != 0 {
		t.Errorf("Domains = %#v, want non-nil empty slice", got[0].Domains)
	}
	if got[1].Domains[0] != "shop.example.com" {
		t.Errorf("Domains = %v", got[1].Domains)
	}
	if got[1].Framework != "express" || got[1].Port != 8080 || got[1].Server != "vps01" {
		t.Errorf("manifest fields not carried over: %+v", got[1])
	}
}

func TestAppsToJSON_Empty(t *testing.T) {
	got := appsToJSON(nil)
	if got == nil || len(got) != 0 {
		t.Errorf("got %#v, want non-nil empty slice (marshals to [])", got)
	}
}

func TestServersToJSON_DefaultAndRootUser(t *testing.T) {
	servers := []*config.Server{
		{Name: "vps01", Host: "1.2.3.4", Domain: "deploy.example.com"},
		{Name: "vps02", Host: "5.6.7.8", User: "deploy", Domain: "apps.example.com"},
	}
	got := serversToJSON(servers)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if !got[0].Default || got[1].Default {
		t.Errorf("default flags = %v/%v, want first-only (mirrors config.DefaultServer)", got[0].Default, got[1].Default)
	}
	if got[0].User != "root" {
		t.Errorf("User = %q, want root default", got[0].User)
	}
	if got[1].User != "deploy" {
		t.Errorf("User = %q, want explicit user preserved", got[1].User)
	}
}

func TestAppDomains_AutoFirst(t *testing.T) {
	m := &detect.AppManifest{
		Domain:  "app.deploy.example.com",
		Domains: []string{"app.example.com", "www.app.example.com"},
	}
	got := appDomains(m)
	want := []string{"app.deploy.example.com", "app.example.com", "www.app.example.com"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("appDomains[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAppDomains_NeverNil(t *testing.T) {
	if got := appDomains(&detect.AppManifest{}); got == nil || len(got) != 0 {
		t.Errorf("got %#v, want non-nil empty slice", got)
	}
}

func TestDoctorReport_OKAggregation(t *testing.T) {
	allOK := doctorReport([]checkResult{
		{Name: "Local Docker", OK: true, Detail: "v27"},
		{Name: "SSH", OK: true, Detail: "1.2.3.4"},
	})
	if !allOK.OK {
		t.Errorf("OK = false, want true when every check passes")
	}
	if len(allOK.Checks) != 2 {
		t.Fatalf("Checks = %d, want 2", len(allOK.Checks))
	}

	oneFail := doctorReport([]checkResult{
		{Name: "Local Docker", OK: true, Detail: "v27"},
		{Name: "Agent", OK: false, Detail: "not installed"},
	})
	if oneFail.OK {
		t.Errorf("OK = true, want false when any check fails")
	}
}

func TestDoctorReport_Empty(t *testing.T) {
	got := doctorReport(nil)
	if !got.OK {
		t.Errorf("OK = false, want vacuous true")
	}
	if got.Checks == nil || len(got.Checks) != 0 {
		t.Errorf("Checks = %#v, want non-nil empty slice", got.Checks)
	}
}

// TestUpJSONEmitter_NDJSONShapes pins the exact NDJSON lines the
// emitter produces — these are the documented machine interface for
// `ezkeel up --json`, so any drift is a breaking change for agents.
func TestUpJSONEmitter_NDJSONShapes(t *testing.T) {
	var buf bytes.Buffer
	em := newUpJSONEmitter(&buf)

	em.step("detect", "start", "")
	em.step("build", "done", "built blog:latest")
	em.warning("migrations_not_run", "prisma", "npx prisma migrate deploy")
	em.result("blog", "https://blog.deploy.example.com", "nextjs", true)
	em.fail("docker build failed")

	want := []string{
		`{"event":"step","name":"detect","status":"start"}`,
		`{"event":"step","name":"build","status":"done","detail":"built blog:latest"}`,
		`{"event":"warning","kind":"migrations_not_run","migrator":"prisma","hint":"npx prisma migrate deploy"}`,
		`{"event":"result","ok":true,"name":"blog","url":"https://blog.deploy.example.com","framework":"nextjs","db":true}`,
		`{"event":"result","ok":false,"error":"docker build failed"}`,
	}
	got := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("lines = %d, want %d:\n%s", len(got), len(want), buf.String())
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d:\n  got  %s\n  want %s", i, got[i], want[i])
		}
	}
}

// TestUpJSONEmitter_NilSafe asserts a nil emitter is a no-op — runUp's
// call sites rely on that instead of guarding every emission.
func TestUpJSONEmitter_NilSafe(t *testing.T) {
	var em *upJSONEmitter
	em.step("detect", "start", "")
	em.warning("migrations_not_run", "prisma", "x")
	em.result("a", "b", "c", false)
	em.fail("boom")
}

// TestJSONFlagsRegistered guards the agent-facing contract: every
// read/list command documented as --json-capable actually has the flag.
func TestJSONFlagsRegistered(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"apps":        appsCmd,
		"logs":        logsCmd,
		"server list": serverListCmd,
		"domain list": domainListCmd,
		"doctor":      doctorCmd,
		"up":          upCmd,
	}
	for name, c := range cmds {
		if c.Flags().Lookup("json") == nil {
			t.Errorf("%s: missing --json flag", name)
		}
	}
	if upCmd.Flags().Lookup("env") == nil {
		t.Errorf("up: missing --env flag")
	}
}
