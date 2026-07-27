package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/geogian28/Assimilator/proto"
)

type packageInfo struct {
	sourceDir        string
	cacheDir         string
	packageName      string
	packageTempPath  string
	packagePermPath  string
	checksum         string
	checksumTempPath string
	checksumPermPath string
	hostname         string
	size             int64
	name             string // the name of the package, but excluding the .tar.gz extension
	// localChecksum    string   // the checksum of the local package file
	serverChecksum string    // the checksum of the server's package file
	path           string    // the path to the local package including the .tar.gz extension
	extractDir     string    // the directory to extract the package into
	arguments      []string  // Any arguments that need to be passed to the package installer
	env            []string  // Any environment variables that need to be set
	runAsUser      string    // The user to run the package installer as
	ticketStatus   string    // The status of the package in Tormon
	ticketID       int       // The ID of the ticket in Tormon, if it exists
	action         string    // The action to perform on the package
	lastRunTime    time.Time // The last time the package was run
	updated        bool      // Whether the package has been updated
	updateInterval int64     // The interval at which the package should be updated
}

// Calculates the SHA256 checksum of the package
func calculateChecksum(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}
	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file at %s: %w", path, err)
	}
	defer file.Close()

	// Calculate the SHA256 checksum
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to copy file content to hash: %w", err)
	}
	hashInBytes := hash.Sum(nil)

	return hex.EncodeToString(hashInBytes), nil
}

// A single, unified function handles the entire lifecycle
func (p *packageInfo) ProcessPackage(a *AgentData) error {

	if err := p.ensurePackage(a); err != nil {
		return err
	}
	Trace("Successfully ensured ", p.name)

	p.checkLastRunTime()
	Info(p.printTimeSinceLastRun())
	// Check if no updates exist AND we are still within the cooldown window
	if !p.updated && time.Since(p.lastRunTime) < time.Duration(p.updateInterval)*time.Second {

		Info("No updates for ", p.name, " and not enough time has passed since the last run. Skipping.")
		return nil
	}

	if err := p.extractPackage(); err != nil {
		return err
	}
	Trace("Successfully extracted ", p.name)
	if err := p.executePackageScript(a); err != nil {
		return err
	}
	Trace("Successfully excuted script for", p.name)
	return nil
}

func (p *packageInfo) ensurePackage(a *AgentData) error {
	// 1. Check if the folder exists

	Debug("Checking if package folder exists: ", p.cacheDir)
	if !fileExists(p.cacheDir) {
		err := os.MkdirAll(p.cacheDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create cache folder: %w", err)
		}
	}

	// 2. Check if we have the file and if it matches the server
	Debug("Checking if package file exists: ", p.path)
	if fileExists(p.path) {
		var err error
		p.checksum, err = calculateChecksum(p.path)
		if err != nil {
			return err
		}
		if p.checksum == p.serverChecksum {
			Debug("Package ", p.name, " checksums match.")
			return nil
		}
	}

	// 3. If we are here, we either don't have it or it's old. Download it
	Debug("Downloading package: ", p.name)
	err := p.downloadPackage(a)
	if err != nil {
		a.failureReports[p.name] = fmt.Sprintf("error downloading %s package: %s", p.name, err)
		return fmt.Errorf("error downloading %s package: %s", p.name, err)
	}
	p.updated = true
	Debug("Downloaded package ", p.name, " successfully.")
	return nil
}

func (p *packageInfo) checkLastRunTime() {
	lastRunPath := filepath.Join(p.cacheDir, "lastRunTime.txt")

	if !fileExists(lastRunPath) || appConfig.RunOnce {
		p.lastRunTime = time.Time{}
		return
	}

	content, err := os.ReadFile(filepath.Join(p.cacheDir, "lastRunTime.txt"))
	if err != nil {
		Error("error opening lastRunTime.txt: ", err)
		p.lastRunTime = time.Time{}
		return
	}

	cleanContent := strings.TrimSpace(string(content))
	p.lastRunTime, err = time.Parse(time.RFC3339, string(cleanContent))
	if err != nil {
		Error("error parsing lastRunTime.txt: ", err)
		p.lastRunTime = time.Time{}
	}
}

// FormatLastRun Returns a human-readable relative time string.
// - < 1 minute: exact seconds
// - 1m to 5m: minutes and seconds
// - 5m to 3h: rounded to nearest minute
// - 3h to 3d: rounded to nearest hour
// - > 3d: rounded to nearest day
func (p *packageInfo) printTimeSinceLastRun() string {
	if p.lastRunTime.IsZero() {
		return fmt.Sprintf("Last run time for %s is never", p.name)
	}

	elapsed := time.Since(p.lastRunTime)

	// Guard against potential future clock skew
	if elapsed < 0 {
		return fmt.Sprintf("Last run time for %s is in the future", p.name)
	}

	// 1. Under 1 minute: Exact seconds
	if elapsed < time.Minute {
		secs := int(elapsed.Seconds())
		if secs == 1 {
			return fmt.Sprintf("Last run time for %s is %s (1 second ago)", p.name, p.lastRunTime.Format(time.RFC3339))
		}
		return fmt.Sprintf("Last run time for %s is (%d seconds ago)", p.name, secs)
	}

	// 2. 1 to 5 minutes: Minutes and seconds
	if elapsed < 5*time.Minute {
		mins := int(elapsed.Minutes())
		secs := int(elapsed.Seconds()) % 60

		minStr := "minute"
		if mins > 1 {
			minStr = "minutes"
		}

		if secs == 0 {
			return fmt.Sprintf("Last run time for %s is %s (%d %s ago)", p.name, p.lastRunTime.Format(time.RFC3339), mins, minStr)
		}

		secStr := "second"
		if secs > 1 {
			secStr = "seconds"
		}

		return fmt.Sprintf("Last run time for %s is %s (%d %s %d %s ago)", p.name, p.lastRunTime.Format(time.RFC3339), mins, minStr, secs, secStr)
	}

	// 3. 5 minutes to 3 hours: Round to whole minutes
	if elapsed < 3*time.Hour {
		mins := int(math.Round(elapsed.Minutes()))
		return fmt.Sprintf("Last run time for %s is %s (%d minutes ago)", p.name, p.lastRunTime.Format(time.RFC3339), mins)
	}

	// 4. 3 hours to 3 days: Round to whole hours
	if elapsed < 72*time.Hour {
		hours := int(math.Round(elapsed.Hours()))
		if hours == 1 {
			return fmt.Sprintf("Last run time for %s is %s (1 hour ago)", p.name, p.lastRunTime.Format(time.RFC3339))
		}
		return fmt.Sprintf("Last run time for %s is %s (%d hours ago)", p.name, p.lastRunTime.Format(time.RFC3339), hours)
	}

	// 5. Above 3 days: Round to whole days
	days := int(math.Round(elapsed.Hours() / 24))
	if days == 1 {
		return fmt.Sprintf("Last run time for %s is %s (1 day ago)", p.name, p.lastRunTime.Format(time.RFC3339))
	}
	return fmt.Sprintf("Last run time for %s is %s (%d days ago)", p.name, p.lastRunTime.Format(time.RFC3339), days)
}

func (p *packageInfo) downloadPackage(a *AgentData) error {
	// 1. Initiate the request
	req := &pb.PackageRequest{
		Name: p.name,
	}

	// 2. Open the stream
	Trace("Opening the stream")
	stream, err := a.client.DownloadPackage(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to start download stream: %w", err)
	}

	// 3. Create the destination file
	Trace("Creating the destinationfile")
	outFile, err := os.Create(p.path)
	if err != nil {
		return fmt.Errorf("failed to create cache file %s: %w", p.path, err)
	}
	defer outFile.Close()

	// 4. Receive chunks in a loop
	Trace("Receiving chunks in a loop")
	var bytesReceived int64
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			Trace("Received EOF")
			// End of stream means success
			break
		}
		if err != nil {
			return fmt.Errorf("stream error while downloading %s: %w", p.name, err)
		}

		// Write bytes to disk
		Trace("Writing bytes to disk")
		n, err := outFile.Write(chunk.Content)
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
		bytesReceived += int64(n)
	}

	Trace(fmt.Sprintf("Successfully downloaded %s (%d bytes)", p.name, bytesReceived))
	return nil
}

func (p *packageInfo) extractPackage() error {
	// 0. Shared base temp directory: /tmp/assimilator
	baseTempPath := filepath.Join(os.TempDir(), "assimilator")
	if err := os.MkdirAll(baseTempPath, 0777); err != nil {
		return fmt.Errorf("failed to create base temp dir: %w", err)
	}

	if err := os.Chmod(baseTempPath, 01777); err != nil {
		return fmt.Errorf("failed to set sticky bit on base temp dir: %w", err)
	}

	// 1. Target directory: /tmp/assimilator/<user>/<pkgName>
	extractDir := filepath.Join(baseTempPath, appConfig.RunAsUser, p.name)

	// Clean up any previous run to ensure a fresh slate
	if err := os.RemoveAll(extractDir); err != nil {
		return fmt.Errorf("failed to clean extract dir: %w", err)
	}

	// Create the user-specific package directory with strictly private 0700 permissions
	if err := os.MkdirAll(extractDir, 0700); err != nil {
		return fmt.Errorf("failed to create extract dir: %w", err)
	}
	// Enforce 0700 explicitly in case the process umask masked out permissions
	if err := os.Chmod(extractDir, 0700); err != nil {
		return fmt.Errorf("failed to enforce 0700 on extract dir: %w", err)
	}

	// 2. Extract the tarball INTO that directory
	cmd := exec.Command("tar", "-xzf", p.path, "-C", extractDir)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error extracting package %s: %w, output: %s", p.name, err, string(output))
	}

	Trace(string(output))
	p.extractDir = extractDir
	return nil
}

func (p *packageInfo) executePackageScript(a *AgentData) error {
	Trace("Executing install script for ", p.name)
	// 1. Ensure the script is executable
	if err := os.Chmod(filepath.Join(p.extractDir, fmt.Sprintf("%s.sh", p.action)), 0755); err != nil {
		return fmt.Errorf("failed to make script executable: %w", err)
	}
	// 2. Run the install script
	// Join the arguments array into a space-separated string (e.g. "--unattended --force")
	Trace("Arguments: ", p.arguments)
	// If the package didnt specify a runAsUser, default to root, then look it up
	if p.runAsUser == "" {
		return fmt.Errorf("package %s did not specify a runAsUser. Exiting to expose error instead of applying bandaid", p.name)
	}
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("user.Current() error: %v", err)
	}

	commandToRun := p.extractDir + "/" + fmt.Sprintf("%s.sh", p.action)
	cmd := exec.Command(commandToRun, p.arguments...)
	cmd.Dir = p.extractDir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, p.env...)
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("USER=%s", currentUser.Username),
		fmt.Sprintf("HOME=%s", currentUser.HomeDir),
		fmt.Sprintf("ASSIMILATOR_HOME=%s", currentUser.HomeDir),
		fmt.Sprintf("ASSIMILATOR_USER=%s", currentUser.Username),
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	)

	Trace("Running script ", commandToRun, " as user: ", p.runAsUser)
	output, err := cmd.CombinedOutput()
	Debug("\n", string(output))

	if err != nil {
		a.failureReports[p.name] = string(output)
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			return fmt.Errorf("Script failed with exit code: %v", code)
		} else {
			// The system couldn't even start the script
			return fmt.Errorf("Failed to start script: %v\n", err)
		}
	}
	Trace("Script ", commandToRun, " ran successfully!")

	// 3. Update the last run time on disk
	if err := os.MkdirAll(p.cacheDir, 0755); err != nil {
		Error("failed to create cache directory: %w", err)
		return nil
	}

	lastRunPath := filepath.Join(p.cacheDir, "lastRunTime.txt")
	epochStr := fmt.Sprintf("%d", time.Now().Unix())
	if err := os.WriteFile(lastRunPath, []byte(epochStr), 0644); err != nil {
		Error("failed to write lastRunTime.txt: %w", err)
	}

	return nil
}
