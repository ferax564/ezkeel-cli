package main

import "testing"

func TestRollbackCmd_RequiresAppName(t *testing.T) {
	if rollbackCmd.Args == nil {
		t.Error("rollbackCmd.Args should not be nil")
	}
}
