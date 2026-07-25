package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
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
	serverChecksum string   // the checksum of the server's package file
	path           string   // the path to the local package including the .tar.gz extension
	extractDir     string   // the directory to extract the package into
	arguments      []string // Any arguments that need to be passed to the package installer
	env            []string // Any environment variables that need to be set
	runAsUser      string   // The user to run the package installer as
	ticketStatus   string   // The status of the package in Tormon
	ticketID       int      // The ID of the ticket in Tormon, if it exists
	action         string   // The action to perform on the package
	lastRunTime    int64    // The last time the package was run
	updated        bool     // Whether the package has been updated
	updateInterval int64    // The interval at which the package should be updated
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
	if p.updated || p.lastRunTime <= time.Now().Unix()-p.updateInterval {
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
		Error("Error downloading package: ", err)
		return fmt.Errorf("error downloading %s package: %s", p.name, err)
	}
	p.updated = true
	Debug("Downloaded package ", p.name, " successfully.")
	return nil
}

// func fileExists(filename string) bool {
// 	info, err := os.Stat(filename)
// 	if err == nil {
// 		return !info.IsDir()
// 	}
// 	if errors.Is(err, os.ErrNotExist) {
// 		return false
// 	}
// 	// For other errors (e.g., permission denied), the file
// 	// might exist, but we can't access it.
// 	// You may want to handle these cases differently based on your application needs.
// 	return false
// }

func (p *packageInfo) checkLastRunTime() {
	if !fileExists(filepath.Join(p.cacheDir, "lastRunTime.txt")) {
		p.lastRunTime = 0
		return
	}

	if appConfig.RunOnce {
		p.lastRunTime = 0
		return
	}

	content, err := os.ReadFile(filepath.Join(p.cacheDir, "lastRunTime.txt"))
	if err != nil {
		Error("error opening lastRunTime.txt: ", err)
		p.lastRunTime = 0
		return
	}

	p.lastRunTime, err = strconv.ParseInt(string(content), 10, 64)
	if err != nil {
		Error("error parsing lastRunTime.txt: ", err)
		p.lastRunTime = 0
	}
}

func (p *packageInfo) downloadPackage(a *AgentData) error {
	// 1. Initiate the request
	req := &pb.PackageRequest{
		Name: p.name,
	}

	// 2. Open the stream
	Debug("Opening the stream")
	stream, err := a.client.DownloadPackage(context.Background(), req)
	if err != nil {
		Debug("failed to start download stream: ", err)
		return fmt.Errorf("failed to start download stream: %w", err)
	}

	// 3. Create the destination file
	Debug("Creating the destinationfile")
	outFile, err := os.Create(p.path)
	if err != nil {
		Debug("failed to create cache file: ", err)
		return fmt.Errorf("failed to create cache file %s: %w", p.path, err)
	}
	defer outFile.Close()

	// 4. Receive chunks in a loop
	Debug("Receiving chunks in a loop")
	var bytesReceived int64
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			Trace("Received EOF")
			// End of stream means success
			break
		}
		if err != nil {
			Error("stream error while downloading", err)
			return fmt.Errorf("stream error while downloading %s: %w", p.name, err)
		}
		Debug("no 'io.EOF' or stream errors")

		// (Optional) Progress logging
		// if chunk.TotalSize > 0 {
		// 	asslog.Trace(fmt.Sprintf("Download started. Size: %d bytes", chunk.TotalSize))
		// }

		// Write bytes to disk
		Debug("Writing bytes to disk")
		n, err := outFile.Write(chunk.Content)
		if err != nil {
			Debug("failed to write to file:", err)
			return fmt.Errorf("failed to write to file: %w", err)
		}
		bytesReceived += int64(n)
	}

	Info(fmt.Sprintf("Successfully downloaded %s (%d bytes)", p.name, bytesReceived))
	return nil
}

func (p *packageInfo) extractPackage() error {
	// 0. Create a predictable temp directory using pkgName
	//    We use /tmp/assimilator/<user>/<pkgName> (e.g. /tmp/assimilator/zsh)
	tempPath := filepath.Join(os.TempDir(), "assimilator")
	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		if err := os.MkdirAll(tempPath, 0776); err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}
		if err := os.Chmod(tempPath, 0776); err != nil {
			return fmt.Errorf("failed to chmod temp dir: %w", err)
		}
	}

	extractDir := filepath.Join(os.TempDir(), "assimilator", appConfig.RunAsUser, p.name)
	// 0. Clean up any previous run to ensure a fresh slate
	os.RemoveAll(extractDir)
	if err := os.MkdirAll(extractDir, 0754); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	// 1. Extract the tarball INTO that directory
	//    -C tells tar to change directory before extracting
	arguments := []string{"tar", "-xzf", p.path, "-C", extractDir}
	cmd := exec.Command("tar", arguments...)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error extracting package: %s", err)
	}
	Trace(output)
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
	Trace()
	if err != nil {
		Trace()
		a.failureReports[p.name] = string(output)
		Trace()
		if exitErr, ok := err.(*exec.ExitError); ok {
			Trace()
			code := exitErr.ExitCode()
			Trace()
			return fmt.Errorf("Script failed with exit code: %v", code)
		} else {
			// The system couldn't even start the script
			Trace()
			return fmt.Errorf("Failed to start script: %v\n", err)
		}
	} else {
		Trace()
		Trace("Script ", commandToRun, " ran successfully!")
	}

	return nil
}
