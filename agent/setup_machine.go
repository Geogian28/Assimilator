package agent

import (
	"fmt"
	"os"
	"path/filepath"

	pb "github.com/geogian28/Assimilator/proto"
)

func (a *AgentData) setupMachine(packages map[string]*pb.PackageConfig) error {
	if len(packages) == 0 {
		hostname, _ := os.Hostname()
		return fmt.Errorf("No packages in machine package list. Check config.yaml for %s", hostname)
	}
	// if a.appConfig.CacheDir == "" {
	// 	a.appConfig.CacheDir = "/var/cache/assimilator/packages"
	// }
	Debug("Listing machine packages:")
	for packageName := range packages {
		Debug("   - ", packageName)
	}
	for packageName, packageData := range packages {
		Trace("Installing machine package: ", packageName)
		pkg := &packageInfo{
			cacheDir:       filepath.Join(a.appConfig.CacheDir, "machine"),
			name:           packageName,
			category:       "machine",
			localChecksum:  "",
			serverChecksum: packageData.Checksum,
			path:           filepath.Join(a.appConfig.CacheDir, "machine", packageName+".tar.gz"),
		}
		Debug("successfully created &packageInfo")

		// 1. Ensure the package exists and is up-to-date
		if pkg.serverChecksum == "" {
			Error("package ", pkg.name, " has no server checksum")
			continue
		}
		Debug("Invoking a.ensurePackage(pkg) for ", pkg.name)
		err := a.ensurePackage(pkg)
		if err != nil {
			Error("error installing package: ", err)
			continue
		}
		Debug("Machine package ", pkg.name, " is ensured.")

		// 2. Extract package
		err = a.extractPackage(pkg)
		if err != nil {
			Error("error extracting machine package: ", err)
			continue
		}
		Debug("Machine package ", pkg.name, " is extracted.")

		// 3. Install package
		err = a.installMachinePackage(pkg)
		if err != nil {
			Error("error installing machine package: ", err)
			continue
		}
		Success("Machine package ", pkg.name, " was installed successfully!")
	}
	return nil
}

// func (a *AgentData) extractMachinePackage(pkgName string, cacheDir string) error {
// 	destDir := filepath.Join(os.TempDir(), "assimilator", "machine", pkgName)
// 	return a.extractPackage(destDir, cacheDir)
// }

func (a *AgentData) installMachinePackage(pkg *packageInfo) error {
	// 1. Create a predictable temp directory using pkgName
	//    We use /tmp/assimilator/<pkgName> (e.g. /tmp/assimilator/zsh)
	extractDir := filepath.Join(os.TempDir(), "assimilator", pkg.name)

	// 2. Clean up any previous run to ensure a fresh slate
	os.RemoveAll(extractDir)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	// 3. Extract the tarball INTO that directory
	//    -C tells tar to change directory before extracting
	_, _, err := a.commandRunner.Run("tar", "-xzf", pkg.path, "-C", extractDir)
	if err != nil {
		return fmt.Errorf("failed to extract %s: %w", pkg.name, err)
	}

	// 4. Ensure the script is executable
	if err := os.Chmod(filepath.Join(extractDir, "install.sh"), 0755); err != nil {
		return fmt.Errorf("failed to make script executable: %w", err)
	}

	// 5. Run the install script
	//    CRITICAL: We construct a command that CD's into the directory first.
	//    If we just ran `${extractDir}/install.sh`, the script's CWD would be the Agent's CWD,
	//    and commands like `cp ./.zshrc` would fail.
	installCmd := fmt.Sprintf("cd %s && ./install.sh", extractDir)

	//    Use sh -c to execute the compound command
	_, _, err = a.commandRunner.Run("sh", "-c", installCmd)
	if err != nil {
		return fmt.Errorf("install script failed for %s: %w", pkg.name, err)
	}

	return nil
}
