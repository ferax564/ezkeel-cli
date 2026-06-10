package agent

// CurrentProtocolVersion is the protocol version this build of the
// package speaks. Bump it whenever the wire format changes in a way an
// older peer cannot safely ignore (new command types, repurposed
// fields). Purely additive optional fields do NOT require a bump.
//
// Version history:
//
//	0 — every agent released before versioning existed. A missing
//	    protocol_version field unmarshals to 0, so pre-versioning
//	    binaries identify themselves by construction.
//	1 — adds protocol_version/agent_version to the envelopes, the
//	    "version" and "update" commands, and DBCreateRequest.ro_password.
//
// Compatibility rule: the client warns when an agent responds with an
// older version (the agent should be updated); the agent rejects
// requests carrying a NEWER version than it understands with a
// structured error instead of mis-handling them.
const CurrentProtocolVersion = 1

// CmdType identifies the kind of command sent to the remote agent.
type CmdType string

const (
	CmdDeploy    CmdType = "deploy"
	CmdStop      CmdType = "stop"
	CmdStatus    CmdType = "status"
	CmdLogs      CmdType = "logs"
	CmdDBCreate  CmdType = "db_create"
	CmdDBMigrate CmdType = "db_migrate"
	CmdDBBackup  CmdType = "db_backup"
	CmdRollback  CmdType = "rollback"
	// CmdVersion probes the agent's version and protocol version
	// without side effects. Agents predating protocol version 1
	// answer it with an "unknown command type" error — callers can
	// treat that error as "protocol version 0".
	CmdVersion CmdType = "version"
	// CmdUpdate makes the agent replace its own binary with a release
	// build (checksum-verified, atomic rename). Protocol version 1+.
	CmdUpdate CmdType = "update"
)

// Request is the envelope sent to ezkeel-agent over SSH stdin.
type Request struct {
	// ProtocolVersion is the protocol version the sender speaks.
	// Client.Send stamps it automatically. Requests from clients
	// predating versioning carry 0.
	ProtocolVersion int               `json:"protocol_version,omitempty"`
	Type            CmdType           `json:"type"`
	Deploy          *DeployRequest    `json:"deploy,omitempty"`
	Stop            *StopRequest      `json:"stop,omitempty"`
	Logs            *LogsRequest      `json:"logs,omitempty"`
	DBCreate        *DBCreateRequest  `json:"db_create,omitempty"`
	DBMigrate       *DBMigrateRequest `json:"db_migrate,omitempty"`
	DBBackup        *DBBackupRequest  `json:"db_backup,omitempty"`
	Rollback        *RollbackRequest  `json:"rollback,omitempty"`
	Update          *UpdateRequest    `json:"update,omitempty"`
}

// DeployRequest carries parameters for a deploy command.
type DeployRequest struct {
	AppName    string            `json:"app_name"`
	ImageTag   string            `json:"image_tag"`
	Port       int               `json:"port"`
	Env        map[string]string `json:"env,omitempty"`
	HealthPath string            `json:"health_path"`
	Memory     string            `json:"memory,omitempty"` // e.g. "512m", "1g"
	CPUs       string            `json:"cpus,omitempty"`   // e.g. "1.0", "0.5"
}

// StopRequest carries the target app name for a stop command.
type StopRequest struct {
	AppName string `json:"app_name"`
}

// LogsRequest carries parameters for a logs command.
type LogsRequest struct {
	AppName string `json:"app_name"`
	Lines   int    `json:"lines"`
}

// DBCreateRequest carries parameters for a database creation command.
type DBCreateRequest struct {
	Engine   string `json:"engine"`
	Version  string `json:"version"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password"`
	// ROPassword, when non-empty, makes the agent also provision a
	// read-only role named "<user>_ro" with this password: CONNECT on
	// the database, USAGE on schema public, SELECT on all current
	// tables, plus a default privilege so tables <user> creates later
	// are covered too. Empty keeps the pre-protocol-1 behaviour (no
	// read-only role).
	ROPassword string `json:"ro_password,omitempty"`
}

// DBMigrateRequest carries parameters for a database migration command.
type DBMigrateRequest struct {
	AppName    string `json:"app_name"`
	MigrateCmd string `json:"migrate_cmd"`
}

// DBBackupRequest carries parameters for a database backup command.
type DBBackupRequest struct {
	Database string `json:"database"`
}

// RollbackRequest carries parameters for a rollback command.
type RollbackRequest struct {
	AppName string `json:"app_name"`
	Port    int    `json:"port"`
	Memory  string `json:"memory,omitempty"`
	CPUs    string `json:"cpus,omitempty"`
}

// UpdateRequest carries parameters for an agent self-update command.
// All fields are optional: a zero-value request means "install the
// latest GitHub release for this OS/arch, verified against its
// SHA256SUMS asset".
type UpdateRequest struct {
	// Version pins a release tag (e.g. "v0.8.0"). Empty means latest.
	Version string `json:"version,omitempty"`
	// DownloadURL overrides the computed release asset URL entirely.
	// An "{ARCH}" placeholder, if present, is substituted with the
	// agent's runtime architecture (amd64/arm64). Takes precedence
	// over Version and over the agent-side EZKEEL_AGENT_DOWNLOAD_URL
	// environment variable.
	DownloadURL string `json:"download_url,omitempty"`
	// SHA256 is the expected hex digest of the new binary. When empty
	// the agent fetches SHA256SUMS from the same directory as the
	// binary and looks its asset up there. The update is refused when
	// no checksum can be obtained either way.
	SHA256 string `json:"sha256,omitempty"`
}

// Response is the envelope returned by ezkeel-agent over SSH stdout.
type Response struct {
	OK      bool        `json:"ok"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
	Apps    []AppStatus `json:"apps,omitempty"`
	Logs    []string    `json:"logs,omitempty"`
	// ProtocolVersion is the protocol version the agent speaks.
	// Responses from agents predating versioning carry 0.
	ProtocolVersion int `json:"protocol_version,omitempty"`
	// AgentVersion is the agent's build version (e.g. "0.7.0").
	// Empty on agents predating protocol version 1.
	AgentVersion string `json:"agent_version,omitempty"`
}

// AppStatus describes the runtime state of a deployed application.
type AppStatus struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // running, stopped, failed
	Port      int    `json:"port"`
	URL       string `json:"url"`
	Framework string `json:"framework,omitempty"`
}
