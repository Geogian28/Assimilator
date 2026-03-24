package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/geogian28/Assimilator/proto"
)

type TicketStatusResponse struct {
	Status   string `json:"status"`
	TicketID int    `json:"ticket_id"`
}

func (a *AgentData) checkTormonStatus(packageName string) (string, int) {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("%s/api/status?hostname=%s&package_name=%s", appConfig.TormonAddress, appConfig.Hostname, packageName)

	resp, err := client.Get(url)
	if err != nil {
		return "none", 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "none", 0
	}

	var result TicketStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "none", 0
	}

	return result.Status, result.TicketID
}

func (a *AgentData) setupMachine(packages map[string]*pb.PackageConfig) error {
	if len(packages) == 0 {
		hostname, _ := os.Hostname()
		return fmt.Errorf("No packages in machine package list. Check config.yaml for %s", hostname)
	}

	if a.appConfig.CacheDir == "" {
		Fatal(1, "CacheDir is empty")
	}

	Debug("Listing machine packages:")
	for packageName := range packages {
		Debug("   - ", packageName)
	}
	for packageName, packageData := range packages {
		Trace("Installing machine package: ", packageName)
		ticketStatus, ticketID := a.checkTormonStatus(packageName)
		if ticketStatus == "open" {
			Info(fmt.Sprintf("skipping %s: open ticket exists in Tormon. Change status to 'pending' to retry.\n    Ticket: https://tormon/%d\n", packageName, ticketID))
			continue
		}
		pkg := &packageInfo{
			CacheDir:       filepath.Join(a.appConfig.CacheDir, "machine"),
			name:           packageName,
			category:       "machine",
			localChecksum:  "",
			serverChecksum: packageData.Checksum,
			path:           filepath.Join(a.appConfig.CacheDir, "machine", packageName+".tar.gz"),
			arguments:      packageData.Arguments,
		}
		Trace("packageData.Arguments: ", packageData.Arguments) //packageData.Arguments =
		Trace("pkg.arguments: ", pkg.arguments)

		// 1. Ensure the package exists and is up-to-date
		if pkg.serverChecksum == "" {
			Error("package ", pkg.name, " has no server checksum")
			continue
		}
		err := a.ensurePackage(pkg)
		if err != nil {
			Error("error installing package: ", err)
			continue
		}

		// 2. Extract package
		err = a.extractPackage(pkg)
		if err != nil {
			Error("error extracting machine package: ", err)
			continue
		}

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
	// Join the arguments array into a space-separated string (e.g. "--unattended --force")
	Trace("Arguments: ", pkg.arguments)
	argsString := strings.Join(pkg.arguments, " ")
	Trace("Args string: ", argsString)

	//    CRITICAL: We construct a command that CD's into the directory first.
	//    If we just ran `${extractDir}/install.sh`, the script's CWD would be the Agent's CWD,
	//    and commands like `cp ./.zshrc` would fail.
	installCmd := fmt.Sprintf("cd %s && ./install.sh %s", extractDir, argsString)
	Trace("installCmd: ", installCmd)

	//    Use sh -c to execute the compound command
	stdoutBytes, stderrBytes, err := a.commandRunner.Run("sh", "-c", installCmd)
	stdout := string(stdoutBytes)
	stderr := string(stderrBytes)

	if err != nil {
		// Combine the OS error, stdout, and stderr into one highly readable string block
		fullLog := fmt.Sprintf("[FATAL] Script exited with error: %v\n\n=== STDOUT ===\n%s\n=== STDERR ===\n%s", err, string(stdout), string(stderr))

		// Fire it off to the Tormon dashboard
		reportToTormon(pkg.name, fullLog)
		Error("install script failed for ", pkg.name, ": ", err, "\n", stdout, "\n", stderr)
		return fmt.Errorf("install script failed for %s: %s: %s", pkg.name, err, stderr)
	}

	return nil
}
