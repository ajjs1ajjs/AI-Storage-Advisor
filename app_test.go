package main

import "testing"

func TestIsAllowedPackageCleanCommand(t *testing.T) {
	if !isAllowedPackageCleanCommand("npm cache clean --force") {
		t.Fatal("expected known package cleanup command to be allowed")
	}

	if isAllowedPackageCleanCommand("npm cache clean --force && rm -rf /") {
		t.Fatal("expected injected package cleanup command to be rejected")
	}
}
