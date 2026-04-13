package agent

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
)

// Request is the envelope sent to ezkeel-agent over SSH stdin.
type Request struct {
	Type      CmdType           `json:"type"`
	Deploy    *DeployRequest    `json:"deploy,omitempty"`
	Stop      *StopRequest      `json:"stop,omitempty"`
	Logs      *LogsRequest      `json:"logs,omitempty"`
	DBCreate  *DBCreateRequest  `json:"db_create,omitempty"`
	DBMigrate *DBMigrateRequest `json:"db_migrate,omitempty"`
	DBBackup  *DBBackupRequest  `json:"db_backup,omitempty"`
	Rollback  *RollbackRequest  `json:"rollback,omitempty"`
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

// Response is the envelope returned by ezkeel-agent over SSH stdout.
type Response struct {
	OK      bool        `json:"ok"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
	Apps    []AppStatus `json:"apps,omitempty"`
	Logs    []string    `json:"logs,omitempty"`
}

// AppStatus describes the runtime state of a deployed application.
type AppStatus struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // running, stopped, failed
	Port      int    `json:"port"`
	URL       string `json:"url"`
	Framework string `json:"framework,omitempty"`
}
