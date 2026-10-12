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
	want := "-v 3f2a9c_anon:/app/cache -v ezkeel-vol.myapp.data:/data -v ezkeel-vol.myapp.uploads:/srv/up"
	if !r.called(want) {
		t.Errorf("declared volumes not mounted as %q; calls: %v", want, r.calls)
	}
	if r.called("abc123:/data") {
		t.Errorf("reattached volume at a declared path must be dropped; calls: %v", r.calls)
	}
}

func TestHandleDeploy_DeclaredVolumeAlreadyAttachedMountedOnce(t *testing.T) {
	r := &fakeRunner{respond: inspectResponder(runningID, "ezkeel-vol.myapp.data:/data\n")}
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
			if n := strings.Count(c, "ezkeel-vol.myapp.data:/data"); n != 1 {
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

func TestHandleDeploy_VolumeNamesDoNotCollideAcrossApps(t *testing.T) {
	// App "a" label "b-c" vs app "a-b" label "c": a '-'-joined scheme
	// gives both "ezkeel-a-b-c". They must get distinct volumes.
	mount := func(app, label string) string {
		r := &fakeRunner{respond: inspectResponder(runningID, "")}
		resp := handleDeploy(r, &agent.DeployRequest{
			AppName: app, ImageTag: app + ":latest",
			Volumes: []agent.VolumeMount{{Name: label, Path: "/data"}},
		})
		if !resp.OK {
			t.Fatal(resp.Error)
		}
		for _, c := range r.calls {
			if strings.HasPrefix(c, "docker run") {
				for _, f := range strings.Fields(c) {
					if strings.HasSuffix(f, ":/data") {
						return f
					}
				}
			}
		}
		t.Fatalf("no /data mount; calls: %v", r.calls)
		return ""
	}
	x, y := mount("a", "b-c"), mount("a-b", "c")
	if x == y {
		t.Fatalf("volume collision: both mount %q", x)
	}
	if x != "ezkeel-vol.a.b-c:/data" || y != "ezkeel-vol.a-b.c:/data" {
		t.Errorf("got %q and %q", x, y)
	}
}

func TestHandleDeploy_StaleDeclaredMountsDropped(t *testing.T) {
	// "data" moved from /old to /new and "gone" was undeclared: neither old
	// mount may survive; other apps' or anonymous volumes are kept.
	mounts := "ezkeel-vol.myapp.data:/old\nezkeel-vol.myapp.gone:/gone\n3f2a9c_anon:/cache\n"
	r := &fakeRunner{respond: inspectResponder(runningID, mounts)}
	resp := handleDeploy(r, &agent.DeployRequest{
		AppName: "myapp", ImageTag: "myapp:latest",
		Volumes: []agent.VolumeMount{{Name: "data", Path: "/new"}},
	})
	if !resp.OK {
		t.Fatal(resp.Error)
	}
	if !r.called("-v 3f2a9c_anon:/cache -v ezkeel-vol.myapp.data:/new") {
		t.Errorf("unexpected mounts; calls: %v", r.calls)
	}
	if r.called(":/old") || r.called("ezkeel-vol.myapp.gone") {
		t.Errorf("stale declared mount carried over; calls: %v", r.calls)
	}
}

func TestHandleDeploy_VolumesNeedUnsanitisedAppName(t *testing.T) {
	// "a.b" and "ab" would both map to ezkeel-vol.ab.data.
	r := &fakeRunner{respond: inspectResponder(runningID, "")}
	resp := handleDeploy(r, &agent.DeployRequest{
		AppName: "a.b", ImageTag: "a.b:latest",
		Volumes: []agent.VolumeMount{{Name: "data", Path: "/data"}},
	})
	if resp.OK {
		t.Fatal("app name that changes under sanitising accepted with volumes")
	}
	if len(r.calls) != 0 {
		t.Errorf("ran commands before rejecting: %v", r.calls)
	}
}
