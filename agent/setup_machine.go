package agent

import (
	"fmt"
	"os"
	"path/filepath"

	pb "github.com/geogian28/Assimilator/proto"
)

func (a *AgentData) setupMachine(packages map[string]*pb.PackageConfig) error {
	for packageName, pkg := range packages {

		// 1. Ensure the package exists and is up-to-date
		packagePath := filepath.Join("/var/cache/assimilator/machine", packageName+".tar.gz")
		err := a.ensurePackage(packageName, packagePath, pkg.Checksum)
		if err != nil {
			Error("error installing package: ", err)
			continue
		}

		// 2. Extract and install
		err = a.installMachinePackage(packageName, packagePath)
		if err != nil {
			Error("error installing package: ", err)
			continue
		}
	}
	return nil
}

func (a *AgentData) installMachinePackage(pkgName string, cachePath string) error {
	// 1. Create a predictable temp directory using pkgName
	//    We use /tmp/assimilator/<pkgName> (e.g. /tmp/assimilator/zsh)
	extractDir := filepath.Join(os.TempDir(), "assimilator", pkgName)

	// 2. Clean up any previous run to ensure a fresh slate
	os.RemoveAll(extractDir)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	// 3. Extract the tarball INTO that directory
	//    -C tells tar to change directory before extracting
	_, _, err := a.commandRunner.Run("tar", "-xzf", cachePath, "-C", extractDir)
	if err != nil {
		return fmt.Errorf("failed to extract %s: %w", pkgName, err)
	}

	// 4. Run the install script
	//    CRITICAL: We construct a command that CD's into the directory first.
	//    If we just ran `${extractDir}/install.sh`, the script's CWD would be the Agent's CWD,
	//    and commands like `cp ./.zshrc` would fail.
	installCmd := fmt.Sprintf("cd %s && ./install.sh", extractDir)

	//    Use sh -c to execute the compound command
	_, _, err = a.commandRunner.Run("sh", "-c", installCmd)
	if err != nil {
		return fmt.Errorf("install script failed for %s: %w", pkgName, err)
	}

	return nil
}
