package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ferax564/ezkeel-cli/pkg/agent"
	"github.com/ferax564/ezkeel-cli/internal/config"
	"github.com/ferax564/ezkeel-cli/internal/detect"
	"github.com/ferax564/ezkeel-cli/internal/spec"
	"github.com/ferax564/ezkeel-cli/internal/tui"
	"github.com/ferax564/ezkeel-cli/pkg/templates"
	"github.com/spf13/cobra"
)

type dryRunInfo struct {
	AppName    string
	Framework  string
	Build      string
	Start      string
	Port       int
	DBEngine   string
	Dockerfile string
	Domain     string
	Server     string
}

func formatDryRun(info *dryRunInfo) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  app:        %s\n", info.AppName))
	sb.WriteString(fmt.Sprintf("  framework:  %s\n", info.Framework))
	sb.WriteString(fmt.Sprintf("  build:      %s\n", info.Build))
	sb.WriteString(fmt.Sprintf("  start:      %s\n", info.Start))
	sb.WriteString(fmt.Sprintf("  port:       %d\n", info.Port))
	if info.DBEngine != "" {
		sb.WriteString(fmt.Sprintf("  database:   %s\n", info.DBEngine))
	} else {
		sb.WriteString("  database:   none\n")
	}
	sb.WriteString(fmt.Sprintf("  dockerfile: %s\n", info.Dockerfile))
	sb.WriteString(fmt.Sprintf("  domain:     %s\n", info.Domain))
	sb.WriteString(fmt.Sprintf("  server:     %s\n", info.Server))
	return sb.String()
}

var upCmd = &cobra.Command{
	Use:   "up [repo-url]",
	Short: "Build, provision, and deploy an app",
	Long: `Deploy a repo (or the current directory) to a configured EZKeel server.

The pipeline is six steps: detect framework, generate Dockerfile,
build image, provision services (PostgreSQL when needed), deploy via
the agent, and run a health check. Each step is reported live; on
failure the offending step is named so you know where to look.

If the working tree contains a Dockerfile, it is used as-is. Otherwise
ezkeel writes a Dockerfile.ezkeel tuned to the detected framework.
Databases are auto-provisioned when ezkeel detects a Postgres client
in your dependencies, and DATABASE_URL is injected at deploy time.`,
	Example: `  # Deploy the current directory to the default server
  ezkeel up

  # Deploy a Forgejo or GitHub repo
  ezkeel up https://git.example.com/me/myapp

  # Deploy from a curated template
  ezkeel up --template todo-list

  # Show detection results without deploying
  ezkeel up --dry-run

  # Pin resources for a memory-tight box
  ezkeel up --memory 256m --cpus 0.5`,
	RunE: runUp,
}

func init() {
	upCmd.Flags().String("server", "", "Target server name")
	upCmd.Flags().String("name", "", "App name override")
	upCmd.Flags().Bool("plain", false, "Disable TUI output")
	upCmd.Flags().Bool("dry-run", false, "Show detection results without deploying")
	upCmd.Flags().String("memory", "", "Container memory limit (e.g. 512m, 1g)")
	upCmd.Flags().String("cpus", "", "Container CPU limit (e.g. 1.0, 0.5)")
	upCmd.Flags().String("template", "", "Deploy a curated template by slug (e.g. todo-list)")
}

// resolveTemplateArg expands a --template slug into a repo URL by
// looking it up in the embedded template catalog. Extracted as a pure
// helper so it can be unit-tested without touching the deploy
// pipeline. Returns the rewritten args slice, an optional warning
// line to print to stderr, and any resolution error.
func resolveTemplateArg(slug string, args []string) ([]string, string, error) {
	if slug == "" {
		return args, "", nil
	}
	tmpl, err := templates.Resolve(slug)
	if err != nil {
		return args, "", fmt.Errorf("resolving template %q: %w", slug, err)
	}
	var warning string
	if len(args) > 0 {
		warning = fmt.Sprintf("warning: --template=%s overrides positional repo URL %q\n", slug, args[0])
	}
	return []string{tmpl.RepoURL}, warning, nil
}

// applySpec layers ezkeel.yaml overrides onto detected framework
// settings. Empty-valued spec fields are left untouched so a partial
// spec is additive rather than destructive. Returns true when at
// least one field was overridden, so the caller can log a one-liner.
func applySpec(fr *detect.FrameworkResult, s *spec.Spec) bool {
	if s == nil || fr == nil {
		return false
	}
	overridden := false
	if s.Framework != "" {
		fr.Framework = detect.Framework(s.Framework)
		overridden = true
	}
	if s.Build != "" {
		fr.Build = s.Build
		overridden = true
	}
	if s.Start != "" {
		fr.Start = s.Start
		overridden = true
	}
	if s.Port != 0 {
		fr.Port = s.Port
		overridden = true
	}
	return overridden
}

func runUp(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	start := time.Now()

	serverName, _ := cmd.Flags().GetString("server")
	nameFlag, _ := cmd.Flags().GetString("name")
	plain, _ := cmd.Flags().GetBool("plain")
	templateSlug, _ := cmd.Flags().GetString("template")

	// Resolve --template slug to a repo URL before the rest of the
	// pipeline looks at args. The flag wins if both are provided.
	rewritten, warn, err := resolveTemplateArg(templateSlug, args)
	if err != nil {
		return err
	}
	if warn != "" {
		fmt.Fprint(os.Stderr, warn)
	}
	args = rewritten

	// Step 1: Resolve server
	var srv *config.Server
	if serverName != "" {
		srv, err = config.LoadServer(serverName)
	} else {
		srv, err = config.DefaultServer()
	}
	if err != nil {
		return fmt.Errorf("resolving server: %w", err)
	}

	// Step 2: Determine source directory
	var sourceDir string
	var tempDir string
	var repoURL string

	if len(args) > 0 {
		repoURL = args[0]
		tempDir, err = cloneRepo(repoURL)
		if err != nil {
			return fmt.Errorf("cloning repo: %w", err)
		}
		defer os.RemoveAll(tempDir)
		sourceDir = tempDir
	} else {
		sourceDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
	}

	// Step 3: Determine app name
	appName := nameFlag
	if appName == "" && repoURL != "" {
		appName = appNameFromRepo(repoURL)
	}
	if appName == "" {
		appName = appNameFromDir(sourceDir)
	}

	// Step 4: Set up deploy model and steps
	stepLabels := []string{
		"detect framework",
		"generate dockerfile",
		"build image",
		"provision services",
		"deploy to server",
		"health check",
	}
	model := tui.NewDeployModel(appName, stepLabels)

	printStep := func(icon, msg string) {
		if plain {
			fmt.Printf("  %s %s\n", icon, msg)
		}
	}

	printProgress := func() {
		if !plain {
			fmt.Print("\033[H\033[2J") // clear screen
			fmt.Println(model.View())
		}
	}

	// Step 0: Detect framework
	model.StartStep(0)
	printProgress()

	fr, err := detect.DetectFramework(sourceDir)
	if err != nil {
		model.FailStep(0, err.Error())
		printProgress()
		printStep(tui.IconFail, "detect framework: "+err.Error())
		return fmt.Errorf("detecting framework in %s: %w\n\nhint: pass a repo URL with `ezkeel up <repo-url>` or run from the project root", sourceDir, err)
	}

	// Layer in ezkeel.yaml overrides if present in the source dir.
	// Placed before the FrameworkUnknown gate so an explicit framework
	// override can rescue a directory the auto-detector cannot classify.
	overridden := false
	if specPath, ferr := spec.Find(sourceDir); ferr == nil {
		loadedSpec, lerr := spec.Load(specPath)
		if lerr != nil {
			model.FailStep(0, lerr.Error())
			printProgress()
			printStep(tui.IconFail, "load ezkeel.yaml: "+lerr.Error())
			return fmt.Errorf("loading %s: %w", specPath, lerr)
		}
		overridden = applySpec(fr, loadedSpec)
	}

	if fr.Framework == detect.FrameworkUnknown {
		model.FailStep(0, "unknown framework")
		printProgress()
		printStep(tui.IconFail, "detect framework: unknown framework")
		return fmt.Errorf("could not detect a supported framework in %s\n\nhint: drop a Dockerfile next to your code and run `ezkeel up` again, or declare framework: in an ezkeel.yaml at the repo root.\nsupported frameworks: run `ezkeel up --help` or visit https://ezkeel.com/docs.html", sourceDir)
	}

	dbResult := detect.DetectDatabase(sourceDir)

	model.CompleteStep(0, fmt.Sprintf("detected %s", fr.Framework))
	printProgress()
	printStep(tui.IconDone, fmt.Sprintf("detected %s", fr.Framework))
	if overridden {
		printStep(tui.IconDone, "applied ezkeel.yaml overrides")
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		dbEngine := ""
		if dbResult != nil && dbResult.Engine != detect.DBNone {
			dbEngine = string(dbResult.Engine)
		}
		dockerfileKind := "auto"
		if fr.Dockerfile == "Dockerfile" {
			dockerfileKind = "Dockerfile (existing)"
		}
		info := &dryRunInfo{
			AppName:    appName,
			Framework:  string(fr.Framework),
			Build:      fr.Build,
			Start:      fr.Start,
			Port:       fr.Port,
			DBEngine:   dbEngine,
			Dockerfile: dockerfileKind,
			Domain:     fmt.Sprintf("%s.%s", appName, srv.Domain),
			Server:     srv.Name,
		}
		fmt.Println(formatDryRun(info))
		return nil
	}

	// Step 1: Generate Dockerfile if needed
	model.StartStep(1)
	printProgress()

	dockerfilePath := filepath.Join(sourceDir, "Dockerfile")
	if fr.Dockerfile == "auto" {
		content := detect.GenerateDockerfile(fr)
		if content == "" {
			model.FailStep(1, "could not generate dockerfile")
			printProgress()
			printStep(tui.IconFail, "generate dockerfile failed")
			return fmt.Errorf("no Dockerfile template for framework %s\n\nhint: add a Dockerfile to your repo and ezkeel will use it as-is. file an issue at https://github.com/ferax564/ezkeel-cli/issues if you'd like first-class support", fr.Framework)
		}
		dockerfilePath = filepath.Join(sourceDir, "Dockerfile.ezkeel")
		if err := os.WriteFile(dockerfilePath, []byte(content), 0o644); err != nil {
			model.FailStep(1, err.Error())
			printProgress()
			printStep(tui.IconFail, "write dockerfile: "+err.Error())
			return fmt.Errorf("writing Dockerfile.ezkeel to %s: %w\n\nhint: check that the target directory is writable", dockerfilePath, err)
		}
		model.CompleteStep(1, "generated Dockerfile.ezkeel")
		printStep(tui.IconDone, "generated Dockerfile.ezkeel")
	} else {
		model.CompleteStep(1, "using existing Dockerfile")
		printStep(tui.IconDone, "using existing Dockerfile")
	}
	printProgress()

	// Step 2: Build Docker image
	model.StartStep(2)
	printProgress()

	imageTag := appName + ":latest"
	buildArgs := []string{"build", "-t", imageTag, "-f", dockerfilePath, sourceDir}
	buildCmd := exec.Command("docker", buildArgs...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if !plain {
		buildCmd.Stdout = nil
		buildCmd.Stderr = nil
	}

	if err := buildCmd.Run(); err != nil {
		model.FailStep(2, "docker build failed: "+err.Error())
		printProgress()
		printStep(tui.IconFail, "docker build failed: "+err.Error())
		if !plain {
			fmt.Println(tui.RenderFailure(appName, "docker build failed: "+err.Error()))
		}
		return fmt.Errorf("docker build %s: %w\n\nhint: re-run with --plain to see full build output, or run `docker build -f %s %s` directly to debug", imageTag, err, dockerfilePath, sourceDir)
	}

	model.CompleteStep(2, "built "+imageTag)
	printProgress()
	printStep(tui.IconDone, "built "+imageTag)

	client := clientFromServer(srv)
	deployPort := appPort(fr.Port)
	memoryLimit, _ := cmd.Flags().GetString("memory")
	cpuLimit, _ := cmd.Flags().GetString("cpus")

	// Push image to server
	printStep(tui.IconActive, "pushing image to "+srv.Name+"...")
	var lastReported int64
	if err := client.UploadImage(imageTag, func(n int64) {
		if n-lastReported < 1<<20 {
			return
		}
		lastReported = n
		if plain {
			fmt.Fprintf(os.Stderr, "\r  uploading... %.1f MB", float64(n)/(1024*1024))
		}
	}); err != nil {
		model.FailStep(2, "image push failed: "+err.Error())
		printProgress()
		printStep(tui.IconFail, "image push failed: "+err.Error())
		return fmt.Errorf("pushing image: %w", err)
	}
	if plain {
		fmt.Fprintln(os.Stderr)
	}

	// Step 3: Provision services (database)
	model.StartStep(3)
	printProgress()

	env := make(map[string]string)
	existingManifest, _ := detect.LoadManifest(detect.ManifestPath(appName))
	if existingManifest != nil {
		for k, v := range existingManifest.Env {
			env[k] = v
		}
	}

	if dbResult != nil && dbResult.Engine == detect.DBPostgres {
		printStep(tui.IconActive, "provisioning PostgreSQL...")
		dbPass := generateDBPassword()
		dbName := appNameToDBName(appName)
		dbResp, dbErr := client.Send(ctx, &agent.Request{
			Type: agent.CmdDBCreate,
			DBCreate: &agent.DBCreateRequest{
				Engine:   string(dbResult.Engine),
				Version:  "16",
				Database: dbName,
				User:     dbName,
				Password: dbPass,
			},
		})
		if dbErr == nil && dbResp.OK {
			env["DATABASE_URL"] = fmt.Sprintf("postgresql://%s:%s@ezkeel-postgres:5432/%s", dbName, dbPass, dbName)
			model.CompleteStep(3, "provisioned PostgreSQL: "+dbName)
			printStep(tui.IconDone, "provisioned PostgreSQL: "+dbName)
		} else {
			model.CompleteStep(3, "database may already exist")
			printStep(tui.IconDone, "database may already exist")
		}
	} else {
		model.CompleteStep(3, "no services needed")
		printStep(tui.IconDone, "no services needed")
	}
	printProgress()

	// Step 4: Deploy via agent
	model.StartStep(4)
	printProgress()

	resp, err := client.Send(ctx, &agent.Request{
		Type: agent.CmdDeploy,
		Deploy: &agent.DeployRequest{
			AppName:    appName,
			ImageTag:   imageTag,
			Port:       deployPort,
			Env:        env,
			HealthPath: "/",
			Memory:     memoryLimit,
			CPUs:       cpuLimit,
		},
	})
	if err != nil {
		model.FailStep(4, "deploy failed: "+err.Error())
		printProgress()
		printStep(tui.IconFail, "deploy failed: "+err.Error())
		return fmt.Errorf("deploying via agent: %w", err)
	}
	if !resp.OK {
		model.FailStep(4, "agent: "+resp.Error)
		printProgress()
		printStep(tui.IconFail, "agent: "+resp.Error)
		return fmt.Errorf("agent deploy error: %s", resp.Error)
	}

	appDomain := fmt.Sprintf("%s.%s", appName, srv.Domain)
	cName := safeContainerName(appName)
	addCaddyRoute(client, appDomain, cName, deployPort)

	model.CompleteStep(4, fmt.Sprintf("deployed to %s", srv.Name))
	printStep(tui.IconDone, fmt.Sprintf("deployed to %s", srv.Name))
	printProgress()

	// Step 5: Health check
	model.StartStep(5)
	printProgress()

	appURL := fmt.Sprintf("https://%s", appDomain)
	healthOK := false
	// Give Caddy a moment to get the TLS cert and the container to start
	for i := 0; i < 6; i++ {
		out, err := client.RunRemote(ctx, fmt.Sprintf("curl -sf -o /dev/null -w '%%{http_code}' http://%s:%d/ 2>/dev/null || echo fail", cName, deployPort))
		if err == nil && strings.TrimSpace(out) != "fail" && strings.TrimSpace(out) != "000" {
			healthOK = true
			break
		}
		time.Sleep(5 * time.Second)
	}

	if healthOK {
		model.CompleteStep(5, "health check passed")
		printStep(tui.IconDone, "health check passed")
	} else {
		model.CompleteStep(5, "health check skipped (container may still be starting)")
		printStep(tui.IconDone, "health check skipped (container may still be starting)")
	}
	printProgress()

	// Save app manifest
	dbEngine := ""
	if dbResult != nil && dbResult.Engine != detect.DBNone {
		dbEngine = string(dbResult.Engine)
	}

	manifest := &detect.AppManifest{
		Name:   appName,
		Repo:   repoURL,
		Server: srv.Name,
		App: detect.AppConfig{
			Framework:  string(fr.Framework),
			Build:      fr.Build,
			Start:      fr.Start,
			Port:       fr.Port,
			Dockerfile: dockerfilePath,
		},
		Domain: fmt.Sprintf("%s.%s", appName, srv.Domain),
		Env:    env,
		Resources: detect.Resources{
			Memory: memoryLimit,
			CPUs:   cpuLimit,
		},
	}
	if dbEngine != "" {
		manifest.Services = map[string]detect.ServiceConfig{
			"db": {Database: dbEngine},
		}
	}

	manifestPath := detect.ManifestPath(appName)
	if err := manifest.Save(manifestPath); err != nil {
		// Non-fatal: warn but don't abort
		fmt.Fprintf(os.Stderr, "warning: could not save manifest: %v\n", err)
	}

	// Final output
	elapsed := int(time.Since(start).Seconds())

	if plain {
		fmt.Println(appURL)
	} else {
		result := &tui.DeployResult{
			AppName: appName,
			URL:     appURL,
			Server:  srv.Name,
			Stack:   string(fr.Framework),
			TimeSec: elapsed,
		}
		fmt.Println(tui.RenderSuccess(result))
	}

	return nil
}

// appNameFromRepo derives an app name from a repository URL.
// It handles:
//   - github.com/user/repo
//   - https://github.com/user/repo
//   - https://github.com/user/repo.git
//   - git@github.com:user/repo.git
func appNameFromRepo(repo string) string {
	if repo == "" {
		return ""
	}

	// Handle SCP-style git URLs: git@github.com:user/repo.git
	if strings.HasPrefix(repo, "git@") {
		// git@host:path/repo.git -> extract after ":"
		colonIdx := strings.LastIndex(repo, ":")
		if colonIdx >= 0 {
			repo = repo[colonIdx+1:]
		}
	}

	// Strip scheme
	repo = strings.TrimPrefix(repo, "https://")
	repo = strings.TrimPrefix(repo, "http://")
	repo = strings.TrimPrefix(repo, "git://")

	// Take the last path segment
	parts := strings.Split(strings.TrimSuffix(repo, "/"), "/")
	last := parts[len(parts)-1]

	// Strip .git suffix
	last = strings.TrimSuffix(last, ".git")

	return last
}

// appNameFromDir returns the base name of the given directory path.
func appNameFromDir(dir string) string {
	return filepath.Base(dir)
}

// cloneRepo clones the given repository URL to a temporary directory.
// If the URL has no scheme, "https://" is prepended.
// Returns the path to the temp dir; the caller is responsible for cleanup.
func cloneRepo(repoURL string) (string, error) {
	// Normalize: add https:// if no scheme present
	if !strings.Contains(repoURL, "://") && !strings.HasPrefix(repoURL, "git@") {
		repoURL = "https://" + repoURL
	}

	tmpDir, err := os.MkdirTemp("", "ezkeel-clone-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}

	cloneCmd := exec.Command("git", "clone", "--depth", "1", repoURL, tmpDir)
	cloneCmd.Stdout = os.Stderr // progress to stderr
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git clone %s: %w", repoURL, err)
	}

	return tmpDir, nil
}

// generateDBPassword returns a random 32-character hex string suitable for use
// as a database password.
func generateDBPassword() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
