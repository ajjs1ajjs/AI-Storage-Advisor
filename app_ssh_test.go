package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestClearPackageCache_RejectsInvalidCommand(t *testing.T) {
	a := &App{}
	err := a.ClearPackageCache("", 0, "rm -rf /", "")
	if err == nil {
		t.Fatal("expected error for invalid command")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("expected 'not allowed' error, got: %v", err)
	}
}

func TestClearPackageCache_RejectsEmptyCommand(t *testing.T) {
	a := &App{}
	err := a.ClearPackageCache("", 0, "", "")
	if err == nil {
		t.Fatal("expected error for empty command")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("expected 'not allowed' error, got: %v", err)
	}
}

func TestVacuumJournaldLogs_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("test only applicable on Windows")
	}
	a := &App{}
	err := a.VacuumJournaldLogs("", 0)
	if err == nil {
		t.Fatal("expected error on Windows")
	}
	if !strings.Contains(err.Error(), "not applicable to Windows") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClearContainerLogs_EmptyContainerID(t *testing.T) {
	a := &App{}
	err := a.ClearContainerLogs("", 0, "")
	if err == nil {
		t.Fatal("expected error for empty container ID")
	}
}

func TestPruneDockerSystem_Local(t *testing.T) {
	a := &App{}
	// Non-SSH path runs docker system prune locally;
	// should return immediately even without Docker installed.
	err := a.PruneDockerSystem("", 0)
	if err != nil {
		t.Logf("PruneDockerSystem failed as expected without Docker: %v", err)
	}
}
