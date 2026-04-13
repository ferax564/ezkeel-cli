package detect_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ferax564/ezkeel-cli/internal/detect"
)

func TestDetectDatabase_Prisma(t *testing.T) {
	dir := t.TempDir()
	prismaDir := filepath.Join(dir, "prisma")
	if err := os.MkdirAll(prismaDir, 0755); err != nil {
		t.Fatalf("failed to create prisma dir: %v", err)
	}
	schema := `datasource db {
  provider = "postgresql"
  url      = env("DATABASE_URL")
}
`
	if err := os.WriteFile(filepath.Join(prismaDir, "schema.prisma"), []byte(schema), 0644); err != nil {
		t.Fatalf("failed to write schema.prisma: %v", err)
	}

	result := detect.DetectDatabase(dir)
	if result.Engine != detect.DBPostgres {
		t.Errorf("Engine: got %q, want %q", result.Engine, detect.DBPostgres)
	}
	if result.Migrator != detect.MigratorPrisma {
		t.Errorf("Migrator: got %q, want %q", result.Migrator, detect.MigratorPrisma)
	}
	if result.MigrateCmd != "npx prisma migrate deploy" {
		t.Errorf("MigrateCmd: got %q, want %q", result.MigrateCmd, "npx prisma migrate deploy")
	}
}

func TestDetectDatabase_Drizzle(t *testing.T) {
	dir := t.TempDir()
	config := `import { defineConfig } from "drizzle-kit";

export default defineConfig({
  dialect: "postgresql",
  schema: "./src/db/schema.ts",
  out: "./drizzle",
});
`
	if err := os.WriteFile(filepath.Join(dir, "drizzle.config.ts"), []byte(config), 0644); err != nil {
		t.Fatalf("failed to write drizzle.config.ts: %v", err)
	}

	result := detect.DetectDatabase(dir)
	if result.Engine != detect.DBPostgres {
		t.Errorf("Engine: got %q, want %q", result.Engine, detect.DBPostgres)
	}
	if result.Migrator != detect.MigratorDrizzle {
		t.Errorf("Migrator: got %q, want %q", result.Migrator, detect.MigratorDrizzle)
	}
}

func TestDetectDatabase_PackageJSONPg(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies": {"express": "4.18.0", "pg": "8.11.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	result := detect.DetectDatabase(dir)
	if result.Engine != detect.DBPostgres {
		t.Errorf("Engine: got %q, want %q", result.Engine, detect.DBPostgres)
	}
}

func TestDetectDatabase_EnvExample(t *testing.T) {
	dir := t.TempDir()
	env := `NODE_ENV=development
DATABASE_URL=postgresql://user:pass@localhost:5432/mydb
PORT=3000
`
	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte(env), 0644); err != nil {
		t.Fatalf("failed to write .env.example: %v", err)
	}

	result := detect.DetectDatabase(dir)
	if result.Engine != detect.DBPostgres {
		t.Errorf("Engine: got %q, want %q", result.Engine, detect.DBPostgres)
	}
}

func TestDetectDatabase_SQLAlchemy(t *testing.T) {
	dir := t.TempDir()
	req := "sqlalchemy==2.0.0\npsycopg2-binary==2.9.9\nalembic==1.13.0\n"
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(req), 0644); err != nil {
		t.Fatalf("failed to write requirements.txt: %v", err)
	}

	result := detect.DetectDatabase(dir)
	if result.Engine != detect.DBPostgres {
		t.Errorf("Engine: got %q, want %q", result.Engine, detect.DBPostgres)
	}
	if result.Migrator != detect.MigratorAlembic {
		t.Errorf("Migrator: got %q, want %q", result.Migrator, detect.MigratorAlembic)
	}
}

func TestDetectDatabase_None(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies": {"express": "4.18.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	result := detect.DetectDatabase(dir)
	if result.Engine != detect.DBNone {
		t.Errorf("Engine: got %q, want %q (should be none)", result.Engine, detect.DBNone)
	}
}
