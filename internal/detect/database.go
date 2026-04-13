package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// DBEngine represents a detected database engine.
type DBEngine string

const (
	DBNone     DBEngine = ""
	DBPostgres DBEngine = "postgres"
	DBMySQL    DBEngine = "mysql"
	DBSQLite   DBEngine = "sqlite"
)

// Migrator represents a detected database migration tool.
type Migrator string

const (
	MigratorNone    Migrator = ""
	MigratorPrisma  Migrator = "prisma"
	MigratorDrizzle Migrator = "drizzle"
	MigratorAlembic Migrator = "alembic"
	MigratorDjango  Migrator = "django"
)

// DatabaseResult contains the detected database engine, migrator, and migration command.
type DatabaseResult struct {
	Engine     DBEngine
	Migrator   Migrator
	MigrateCmd string // e.g. "npx prisma migrate deploy"
}

// DetectDatabase scans dir and returns the best-matched DatabaseResult.
// Detection priority:
//  1. Prisma schema (prisma/schema.prisma)
//  2. Drizzle config (drizzle.config.ts / drizzle.config.js)
//  3. package.json deps (pg, mysql2)
//  4. Python deps (requirements.txt: sqlalchemy, psycopg2, asyncpg, alembic, django)
//  5. .env.example DATABASE_URL protocol
//  6. Nothing → DBNone
func DetectDatabase(dir string) *DatabaseResult {
	if result, ok := detectPrismaDB(dir); ok {
		return result
	}
	if result, ok := detectDrizzleDB(dir); ok {
		return result
	}
	if result, ok := detectNodeDB(dir); ok {
		return result
	}
	if result, ok := detectPythonDB(dir); ok {
		return result
	}
	if result, ok := detectEnvDB(dir); ok {
		return result
	}
	return &DatabaseResult{Engine: DBNone}
}

// detectPrismaDB reads prisma/schema.prisma and parses the provider field.
func detectPrismaDB(dir string) (*DatabaseResult, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "prisma", "schema.prisma"))
	if err != nil {
		return nil, false
	}

	content := string(data)
	engine := parsePrismaProvider(content)
	if engine == DBNone {
		return nil, false
	}

	return &DatabaseResult{
		Engine:     engine,
		Migrator:   MigratorPrisma,
		MigrateCmd: "npx prisma migrate deploy",
	}, true
}

// parsePrismaProvider extracts the database engine from a Prisma schema datasource block.
// It looks for lines like: provider = "postgresql"
func parsePrismaProvider(content string) DBEngine {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "provider") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "postgresql") || strings.Contains(lower, "postgres") {
			return DBPostgres
		}
		if strings.Contains(lower, "mysql") {
			return DBMySQL
		}
		if strings.Contains(lower, "sqlite") {
			return DBSQLite
		}
	}
	return DBNone
}

// detectDrizzleDB reads drizzle.config.ts or drizzle.config.js and parses the dialect.
func detectDrizzleDB(dir string) (*DatabaseResult, bool) {
	var content string
	for _, name := range []string{"drizzle.config.ts", "drizzle.config.js"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			content = string(data)
			break
		}
	}
	if content == "" {
		return nil, false
	}

	engine := parseDrizzleDialect(content)
	if engine == DBNone {
		return nil, false
	}

	return &DatabaseResult{
		Engine:   engine,
		Migrator: MigratorDrizzle,
	}, true
}

// parseDrizzleDialect extracts the database engine from a Drizzle config file.
// It looks for a dialect field like: dialect: "postgresql"
func parseDrizzleDialect(content string) DBEngine {
	lower := strings.ToLower(content)
	if strings.Contains(lower, "postgresql") || strings.Contains(lower, "postgres") {
		return DBPostgres
	}
	if strings.Contains(lower, "mysql") {
		return DBMySQL
	}
	if strings.Contains(lower, "sqlite") {
		return DBSQLite
	}
	return DBNone
}

// detectNodeDB checks package.json for Node.js database client dependencies.
func detectNodeDB(dir string) (*DatabaseResult, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil, false
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, false
	}

	if pkg.hasDep("pg") || pkg.hasDep("@neondatabase/serverless") || pkg.hasDep("postgres") {
		return &DatabaseResult{Engine: DBPostgres}, true
	}
	if pkg.hasDep("mysql2") || pkg.hasDep("mysql") {
		return &DatabaseResult{Engine: DBMySQL}, true
	}
	if pkg.hasDep("better-sqlite3") || pkg.hasDep("sqlite3") || pkg.hasDep("@libsql/client") {
		return &DatabaseResult{Engine: DBSQLite}, true
	}

	return nil, false
}

// detectPythonDB checks requirements.txt for Python database dependencies.
func detectPythonDB(dir string) (*DatabaseResult, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "requirements.txt"))
	if err != nil {
		return nil, false
	}

	content := strings.ToLower(string(data))

	// Determine migrator
	migrator := MigratorNone
	if strings.Contains(content, "alembic") {
		migrator = MigratorAlembic
	} else if strings.Contains(content, "django") {
		migrator = MigratorDjango
	}

	// Determine engine from adapter packages
	if strings.Contains(content, "psycopg2") || strings.Contains(content, "asyncpg") || strings.Contains(content, "psycopg") {
		return &DatabaseResult{Engine: DBPostgres, Migrator: migrator}, true
	}
	if strings.Contains(content, "mysqlclient") || strings.Contains(content, "pymysql") || strings.Contains(content, "aiomysql") {
		return &DatabaseResult{Engine: DBMySQL, Migrator: migrator}, true
	}

	// sqlalchemy alone without a specific adapter — can't determine engine precisely,
	// but we can still record it as detected if combined with django (which defaults to sqlite in dev)
	if strings.Contains(content, "sqlalchemy") {
		return &DatabaseResult{Engine: DBPostgres, Migrator: migrator}, true
	}
	if strings.Contains(content, "django") {
		return &DatabaseResult{Engine: DBSQLite, Migrator: MigratorDjango}, true
	}

	return nil, false
}

// detectEnvDB parses .env.example for a DATABASE_URL with a recognizable protocol.
func detectEnvDB(dir string) (*DatabaseResult, bool) {
	data, err := os.ReadFile(filepath.Join(dir, ".env.example"))
	if err != nil {
		return nil, false
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToUpper(line), "DATABASE_URL") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "postgresql://") || strings.Contains(lower, "postgres://") {
			return &DatabaseResult{Engine: DBPostgres}, true
		}
		if strings.Contains(lower, "mysql://") {
			return &DatabaseResult{Engine: DBMySQL}, true
		}
		if strings.Contains(lower, "sqlite://") || strings.Contains(lower, "sqlite3://") {
			return &DatabaseResult{Engine: DBSQLite}, true
		}
	}

	return nil, false
}
