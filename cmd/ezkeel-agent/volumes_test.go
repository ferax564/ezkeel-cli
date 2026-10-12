package main

import (
	"strings"
	"testing"

	"github.com/ferax564/ezkeel-cli/pkg/agent"
)

func TestHandleDeploy_MountsDeclaredVolumes(t *testing.T) {
	// Reattached: an anonymous cache volume (kept), and the image's own
	// VOLUME at /data (superseded by the declared one at the same path).
	mounts := "3f2a9c_anon:/app/cache\nabc123:/data\n"
	r := &fakeRunner{respond: inspectResponder(runningID, mounts)}
	resp := handleDeploy(r, &agent.DeployRequest{
		AppName:  "myapp",
		ImageTag: "myapp:latest",
		Volumes:  []agent.VolumeMount{{Name: "data", Path: "/data"}, {Name: "uploads", Path: "/srv/up"}},
	})
	if !resp.OK {
		t.Fatal(resp.Error)
	}
	want := "-v 3f2a9c_anon:/app/cache -v ezkeel-myapp-data:/data -v ezkeel-myapp-uploads:/srv/up"
	if !r.called(want) {
		t.Errorf("declared volumes not mounted as %q; calls: %v", want, r.calls)
	}
	if r.called("abc123:/data") {
		t.Errorf("reattached volume at a declared path must be dropped; calls: %v", r.calls)
	}
}

func TestHandleDeploy_DeclaredVolumeAlreadyAttachedMountedOnce(t *testing.T) {
	r := &fakeRunner{respond: inspectResponder(runningID, "ezkeel-myapp-data:/data\n")}
	resp := handleDeploy(r, &agent.DeployRequest{
		AppName: "myapp", ImageTag: "myapp:latest",
		Volumes: []agent.VolumeMount{{Name: "data", Path: "/data"}},
	})
	if !resp.OK {
		t.Fatal(resp.Error)
	}
	ran := false
	for _, c := range r.calls {
		if strings.HasPrefix(c, "docker run") {
			ran = true
			if n := strings.Count(c, "ezkeel-myapp-data:/data"); n != 1 {
				t.Errorf("declared volume mounted %d times: %s", n, c)
			}
		}
	}
	if !ran {
		t.Fatalf("no docker run; calls: %v", r.calls)
	}
}

func TestHandleDeploy_InvalidVolumesRejectedBeforeSideEffects(t *testing.T) {
	bad := [][]agent.VolumeMount{
		{{Name: "data", Path: "/"}},
		{{Name: "data", Path: "relative"}},
		{{Name: "data", Path: "/a/../etc"}},
		{{Name: "data", Path: "/x:/host"}},
		{{Name: "Bad_Name", Path: "/data"}},
		{{Name: "a", Path: "/a"}, {Name: "a", Path: "/b"}},
	}
	for _, vols := range bad {
		r := &fakeRunner{respond: inspectResponder(runningID, "")}
		resp := handleDeploy(r, &agent.DeployRequest{AppName: "myapp", ImageTag: "myapp:latest", Volumes: vols})
		if resp.OK {
			t.Errorf("volumes %+v accepted", vols)
		}
		if len(r.calls) != 0 {
			t.Errorf("volumes %+v: ran commands before rejecting: %v", vols, r.calls)
		}
	}
}

func TestDispatch_AcceptsVolumesProtocol(t *testing.T) {
	r := &fakeRunner{respond: inspectResponder(runningID, "")}
	resp := dispatch(r, &agent.Request{
		ProtocolVersion: agent.ProtocolVersionVolumes,
		Type:            agent.CmdDeploy,
		Deploy: &agent.DeployRequest{
			AppName: "myapp", ImageTag: "myapp:latest",
			Volumes: []agent.VolumeMount{{Name: "data", Path: "/data"}},
		},
	})
	if !resp.OK {
		t.Fatalf("protocol %d deploy with volumes rejected: %s", agent.ProtocolVersionVolumes, resp.Error)
	}
}
