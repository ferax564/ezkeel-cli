package environment

import (
	"reflect"
	"testing"
)

func TestBuildArgs(t *testing.T) {
	got := buildArgs("build", "/path/to/project")
	want := []string{"build", "--workspace-folder", "/path/to/project"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildArgs() = %v, want %v", got, want)
	}
}

func TestBuildArgsUp(t *testing.T) {
	got := buildArgs("up", "/my/project")
	want := []string{"up", "--workspace-folder", "/my/project"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildArgs() = %v, want %v", got, want)
	}
}

func TestExecArgs(t *testing.T) {
	got := execArgs("/path/to/project", "go", "test", "./...")
	want := []string{"exec", "--workspace-folder", "/path/to/project", "go", "test", "./..."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("execArgs() = %v, want %v", got, want)
	}
}
