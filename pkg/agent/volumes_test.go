package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateVolumeMount(t *testing.T) {
	cases := []struct {
		name string
		v    VolumeMount
		ok   bool
	}{
		{"simple", VolumeMount{"data", "/data"}, true},
		{"nested", VolumeMount{"uploads-1", "/app/public/uploads"}, true},
		{"trailing slash", VolumeMount{"data", "/data/"}, true},
		{"32 char name", VolumeMount{strings.Repeat("a", 32), "/x"}, true},
		{"empty name", VolumeMount{"", "/data"}, false},
		{"33 char name", VolumeMount{strings.Repeat("a", 33), "/x"}, false},
		{"uppercase name", VolumeMount{"Data", "/data"}, false},
		{"underscore name", VolumeMount{"my_data", "/data"}, false},
		{"empty path", VolumeMount{"data", ""}, false},
		{"relative path", VolumeMount{"data", "data"}, false},
		{"root", VolumeMount{"data", "/"}, false},
		{"root double slash", VolumeMount{"data", "//"}, false},
		{"dotdot", VolumeMount{"data", "/data/../etc"}, false},
		{"trailing dotdot", VolumeMount{"data", "/data/.."}, false},
		{"colon", VolumeMount{"data", "/data:ro"}, false},
		{"comma", VolumeMount{"data", "/a,b"}, false},
		{"space", VolumeMount{"data", "/my data"}, false},
		{"newline", VolumeMount{"data", "/data\n-v /:/host"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateVolumeMount(tc.v)
			if (err == nil) != tc.ok {
				t.Fatalf("ValidateVolumeMount(%+v) = %v, want ok=%v", tc.v, err, tc.ok)
			}
		})
	}
}

func TestValidateVolumes(t *testing.T) {
	if err := ValidateVolumes(nil, 5); err != nil {
		t.Errorf("nil set: %v", err)
	}
	ok := []VolumeMount{{"data", "/data"}, {"cache", "/cache"}}
	if err := ValidateVolumes(ok, 5); err != nil {
		t.Errorf("valid set: %v", err)
	}
	if err := ValidateVolumes([]VolumeMount{{"a", "/a"}, {"a", "/b"}}, 5); err == nil {
		t.Error("duplicate name accepted")
	}
	if err := ValidateVolumes([]VolumeMount{{"a", "/data"}, {"b", "/data/"}}, 5); err == nil {
		t.Error("duplicate path (modulo trailing slash) accepted")
	}
	six := []VolumeMount{{"a", "/a"}, {"b", "/b"}, {"c", "/c"}, {"d", "/d"}, {"e", "/e"}, {"f", "/f"}}
	if err := ValidateVolumes(six, 5); err == nil {
		t.Error("more than max accepted")
	}
	if err := ValidateVolumes(six, 0); err != nil {
		t.Errorf("max<=0 should default to MaxVolumes: %v", err)
	}
}

func TestDeployRequest_VolumesWireFormat(t *testing.T) {
	// Omitted when empty so v1 agents see byte-identical deploy payloads.
	b, _ := json.Marshal(DeployRequest{AppName: "a"})
	if strings.Contains(string(b), "volumes") {
		t.Errorf("empty volumes serialized: %s", b)
	}
	b, _ = json.Marshal(DeployRequest{AppName: "a", Volumes: []VolumeMount{{"data", "/data"}}})
	if !strings.Contains(string(b), `"volumes":[{"name":"data","path":"/data"}]`) {
		t.Errorf("volumes wire format: %s", b)
	}
}

func TestRequiredProtocolVersion(t *testing.T) {
	if got := RequiredProtocolVersion(nil); got != MinProtocolVersion {
		t.Errorf("nil: %d", got)
	}
	if got := RequiredProtocolVersion(&Request{Type: CmdStatus}); got != MinProtocolVersion {
		t.Errorf("status: %d", got)
	}
	req := &Request{Type: CmdDeploy, Deploy: &DeployRequest{Volumes: []VolumeMount{{"d", "/d"}}}}
	if got := RequiredProtocolVersion(req); got != ProtocolVersionVolumes {
		t.Errorf("deploy with volumes: %d", got)
	}
	if ProtocolVersionVolumes > CurrentProtocolVersion {
		t.Error("ProtocolVersionVolumes must not exceed CurrentProtocolVersion")
	}
}
