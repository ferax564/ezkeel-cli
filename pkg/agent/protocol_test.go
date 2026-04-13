package agent

import (
	"encoding/json"
	"testing"
)

func TestRequest_DeploySerialization(t *testing.T) {
	req := Request{
		Type: CmdDeploy,
		Deploy: &DeployRequest{
			AppName:    "myapp",
			ImageTag:   "registry.example.com/myapp:abc123",
			Port:       8080,
			Env:        map[string]string{"DATABASE_URL": "postgres://localhost/myapp"},
			HealthPath: "/healthz",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Request
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != CmdDeploy {
		t.Errorf("Type: got %q, want %q", got.Type, CmdDeploy)
	}
	if got.Deploy == nil {
		t.Fatal("Deploy is nil after round-trip")
	}
	if got.Deploy.AppName != "myapp" {
		t.Errorf("AppName: got %q, want %q", got.Deploy.AppName, "myapp")
	}
	if got.Deploy.ImageTag != "registry.example.com/myapp:abc123" {
		t.Errorf("ImageTag: got %q, want %q", got.Deploy.ImageTag, "registry.example.com/myapp:abc123")
	}
	if got.Deploy.Port != 8080 {
		t.Errorf("Port: got %d, want %d", got.Deploy.Port, 8080)
	}
	if got.Deploy.Env["DATABASE_URL"] != "postgres://localhost/myapp" {
		t.Errorf("Env[DATABASE_URL]: got %q", got.Deploy.Env["DATABASE_URL"])
	}
	if got.Deploy.HealthPath != "/healthz" {
		t.Errorf("HealthPath: got %q, want %q", got.Deploy.HealthPath, "/healthz")
	}
	if got.Stop != nil || got.Logs != nil || got.DBCreate != nil || got.DBMigrate != nil {
		t.Error("unexpected non-nil optional fields after round-trip")
	}
}

func TestRequest_DeployWithResources(t *testing.T) {
	req := Request{
		Type: CmdDeploy,
		Deploy: &DeployRequest{
			AppName:  "myapp",
			ImageTag: "myapp:latest",
			Port:     3000,
			Memory:   "512m",
			CPUs:     "1.0",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Request
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Deploy.Memory != "512m" {
		t.Errorf("Memory: got %q, want %q", got.Deploy.Memory, "512m")
	}
	if got.Deploy.CPUs != "1.0" {
		t.Errorf("CPUs: got %q, want %q", got.Deploy.CPUs, "1.0")
	}
}

func TestRequest_StatusSerialization(t *testing.T) {
	req := Request{Type: CmdStatus}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Request
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != CmdStatus {
		t.Errorf("Type: got %q, want %q", got.Type, CmdStatus)
	}
	if got.Deploy != nil || got.Stop != nil || got.Logs != nil {
		t.Error("unexpected non-nil optional fields in status request")
	}
}

func TestRequest_RollbackSerialization(t *testing.T) {
	req := Request{
		Type:     CmdRollback,
		Rollback: &RollbackRequest{AppName: "myapp", Port: 3000},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Request
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != CmdRollback {
		t.Errorf("Type: got %q, want %q", got.Type, CmdRollback)
	}
	if got.Rollback == nil {
		t.Fatal("Rollback is nil after round-trip")
	}
	if got.Rollback.AppName != "myapp" {
		t.Errorf("AppName: got %q, want %q", got.Rollback.AppName, "myapp")
	}
	if got.Rollback.Port != 3000 {
		t.Errorf("Port: got %d, want %d", got.Rollback.Port, 3000)
	}
}

func TestResponse_Serialization(t *testing.T) {
	resp := Response{
		OK:      true,
		Message: "2 apps running",
		Apps: []AppStatus{
			{Name: "api", Status: "running", Port: 8080, URL: "https://api.example.com", Framework: "go"},
			{Name: "web", Status: "stopped", Port: 3000, URL: "https://web.example.com"},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Response
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !got.OK {
		t.Error("OK: got false, want true")
	}
	if got.Message != "2 apps running" {
		t.Errorf("Message: got %q, want %q", got.Message, "2 apps running")
	}
	if len(got.Apps) != 2 {
		t.Fatalf("Apps length: got %d, want 2", len(got.Apps))
	}
	if got.Apps[0].Name != "api" {
		t.Errorf("Apps[0].Name: got %q, want %q", got.Apps[0].Name, "api")
	}
	if got.Apps[0].Status != "running" {
		t.Errorf("Apps[0].Status: got %q, want %q", got.Apps[0].Status, "running")
	}
	if got.Apps[0].Framework != "go" {
		t.Errorf("Apps[0].Framework: got %q, want %q", got.Apps[0].Framework, "go")
	}
	if got.Apps[1].Name != "web" {
		t.Errorf("Apps[1].Name: got %q, want %q", got.Apps[1].Name, "web")
	}
	if got.Apps[1].Status != "stopped" {
		t.Errorf("Apps[1].Status: got %q, want %q", got.Apps[1].Status, "stopped")
	}
	if got.Apps[1].Framework != "" {
		t.Errorf("Apps[1].Framework: got %q, want empty (omitempty)", got.Apps[1].Framework)
	}
}
