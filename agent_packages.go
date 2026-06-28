package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	pb "github.com/geogian28/Assimilator/proto"
)

// A single, unified function handles the entire lifecycle
func (a *AgentData) ProcessPackages(pkg *packageInfo) error {
	pkg.ticketStatus, pkg.ticketID = a.checkTormonStatus(pkg.name)

	if err := a.ensurePackage(pkg); err != nil {
		return err
	}
	if err := a.extractPackage(pkg); err != nil {
		return err
	}
	return a.executeInstallScript(pkg)
}

func (a *AgentData) ensurePackage(pkg *packageInfo) error {
	// 1. Check if the folder exists

	Debug("Checking if package folder exists: ", pkg.CacheDir)
	if !a.fileExists(pkg.CacheDir) {
		err := os.MkdirAll(pkg.CacheDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create cache folder: %w", err)
		}
	}

	// 2. Check if we have the file and if it matches the server
	Debug("Checking if package file exists: ", pkg.path)
	if a.fileExists(pkg.path) {
		var err error
		pkg.checksum, err = calculateChecksum(pkg.path)
		if err != nil {
			return err
		}
		if pkg.checksum == pkg.serverChecksum {
			Debug("Package ", pkg.name, " checksums match.")
			return nil
		}
	}

	// 3. If we are here, we either don't have it or it's old. Download it
	Debug("Downloading package: ", pkg.name)
	err := a.downloadPackage(pkg)
	if err != nil {
		Error("Error downloading package: ", err)
		return fmt.Errorf("error downloading %s package: %s", pkg.name, err)
	}
	Debug("Downloaded package ", pkg.name, " successfully.")
	return nil
}

func (a *AgentData) fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err == nil {
		return !info.IsDir()
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	// For other errors (e.g., permission denied), the file
	// might exist, but we can't access it.
	// You may want to handle these cases differently based on your application needs.
	return false
}

func (a *AgentData) downloadPackage(pkg *packageInfo) error {
	// 1. Initiate the request
	req := &pb.PackageRequest{
		Name:     pkg.name,
		Category: pkg.category,
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
	outFile, err := os.Create(pkg.path)
	if err != nil {
		Debug("failed to create cache file: ", err)
		return fmt.Errorf("failed to create cache file %s: %w", pkg.path, err)
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
			return fmt.Errorf("stream error while downloading %s: %w", pkg.name, err)
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

	Info(fmt.Sprintf("Successfully downloaded %s (%d bytes)", pkg.name, bytesReceived))
	return nil
}

func (a *AgentData) extractPackage(pkg *packageInfo) error {
	// 1. Create a predictable temp directory using pkgName
	//    We use /tmp/assimilator/<pkgName> (e.g. /tmp/assimilator/zsh)
	extractDir := filepath.Join(os.TempDir(), "assimilator", pkg.category, pkg.name)

	// 1. Clean up any previous run to ensure a fresh slate
	os.RemoveAll(extractDir)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	// 2. Extract the tarball INTO that directory
	//    -C tells tar to change directory before extracting
	_, _, err := a.commandRunner.Run("tar", "-xzf", pkg.path, "-C", extractDir)
	if err != nil {
		return fmt.Errorf("failed to extract %s: %w", pkg.name, err)
	}
	pkg.extractDir = extractDir
	return nil
}

func (a *AgentData) executeInstallScript(pkg *packageInfo) error {
	// 1. Ensure the script is executable
	if err := os.Chmod(filepath.Join(pkg.extractDir, fmt.Sprintf("%s.sh", pkg.action)), 0755); err != nil {
		return fmt.Errorf("failed to make script executable: %w", err)
	}

	// 2. Run the install script
	// Join the arguments array into a space-separated string (e.g. "--unattended --force")
	Trace("Arguments: ", pkg.arguments)
	argsString := strings.Join(pkg.arguments, " ")
	Trace("Args string: ", argsString)

	// If the user didnt specify a runAsUser, default to root, then look it up
	if pkg.runAsUser == "" {
		pkg.runAsUser = "root"
	}
	userData, uid, gid, err := userLookup(pkg.runAsUser)

	cmd := exec.Command("/bin/bash", pkg.extractDir+"/"+fmt.Sprintf("%s.sh", pkg.action), argsString)
	cmd.Dir = pkg.extractDir
	cmd.Env = append(
		[]string{
			fmt.Sprintf("HOME=%s", userData.HomeDir),
			fmt.Sprintf("USER=%s", userData.Username),
			fmt.Sprintf("ASSIMILATOR_HOME=%s", userData.HomeDir),
			fmt.Sprintf("ASSIMILATOR_USER=%s", userData.Username),
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		},
		pkg.env...,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uid,
			Gid: gid,
		},
	}

	// Only drop privileges if the target user is NOT root
	Trace("Running script ", "/tmp/assimilator/"+pkg.category+"/"+pkg.name+"/"+fmt.Sprintf("%s.sh", pkg.action), " as user:", pkg.runAsUser)
	// err = cmd.Run()
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Retrieve the actual exit status code (e.g., 1, 127, 2)
			code := exitErr.ExitCode()
			Error("Script failed with exit code:", code)
		} else {
			// The system couldn't even start the script
			Error("Failed to start script: %v\n", err)
		}
	} else {
		Success("Script ", "/tmp/assimilator/"+pkg.category+"/"+pkg.name+"/"+fmt.Sprintf("%s.sh", pkg.action), " ran successfully!")
	}
	Debug(string(output))
	// stdoutBytes, stderrBytes, err := a.commandRunner.Run("sh", "-c", installCmd)
	// stdout := string(stdoutBytes)
	// stderr := string(stderrBytes)

	// if err != nil {
	// 	// Combine the OS error, stdout, and stderr into one highly readable string block
	// 	fullLog := fmt.Sprintf("[FATAL] Script exited with error: %v\n\n=== STDOUT ===\n%s\n=== STDERR ===\n%s", err, string(stdout), string(stderr))

	// 	// Fire it off to the Tormon dashboard
	// 	if ticketStatus != "none" {
	// 		reportToTormon(pkg.name, "failure", fullLog)
	// 	}
	// 	Error("install script failed for ", pkg.name, ": ", err, "\n", stdout, "\n", stderr)
	// 	return fmt.Errorf("install script failed for %s: %s: %s", pkg.name, err, stderr)
	// }
	// if pendingStatus {
	// 	fullLog := fmt.Sprintf("[SUCCESS] Script install succeeded!\n\n=== STDOUT ===\n%s", string(stdout))
	// 	reportToTormon(pkg.name, "success", fullLog)
	// }
	return nil
}

// func (a *AgentData) installUserPackage(pkgName string, username string) error {
// 	// // 1. Check if user exists and get their details
// 	// targetUser, err := user.Lookup(username)
// 	// if err != nil {
// 	// 	return fmt.Errorf("user %s not found: %w", username, err)
// 	// }
// 	// uid, err := strconv.ParseUint(targetUser.Uid, 10, 32)
// 	// if err != nil {
// 	// 	return fmt.Errorf("failed to parse UID for %s: %w", username, err)
// 	// }
// 	// gid, err := strconv.ParseUint(targetUser.Gid, 10, 32)
// 	// if err != nil {
// 	// 	return fmt.Errorf("failed to parse GID for %s: %w", username, err)
// 	// }

// 	// 2. Set the extract variable
// 	extractDir := filepath.Join(os.TempDir(), "assimilator", username, pkgName)

// 	// 3. Prepare the command
// 	scriptPath := filepath.Join(extractDir, "install.sh")
// 	cmd := exec.Command("/bin/bash", scriptPath)

// 	// 4. Prepare the environment
// 	cmd.Env = []string{
// 		fmt.Sprintf("HOME=%s", targetUser.HomeDir),
// 		fmt.Sprintf("USER=%s", targetUser.Username),
// 		fmt.Sprintf("ASSIMILATOR_HOME=%s", targetUser.HomeDir),
// 		fmt.Sprintf("ASSIMILATOR_USER=%s", targetUser.Username),
// 		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", // Basic path
// 	}

// 	// 5. Set the working directory
// 	cmd.Dir = extractDir

// 	// 6. Set the user
// 	cmd.SysProcAttr = &syscall.SysProcAttr{
// 		Credential: &syscall.Credential{
// 			Uid: uint32(uid),
// 			Gid: uint32(gid),
// 		},
// 	}

// 	// 7. Run
// 	output, err := cmd.CombinedOutput()
// 	if err != nil {
// 		return fmt.Errorf("install failed for %s: %s", pkgName, string(output))
// 	}

// 	return nil
// }

func userLookup(username string) (*user.User, uint32, uint32, error) {
	targetUser, err := user.Lookup(username)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("user %s not found: %w", username, err)
	}
	uid, err := strconv.ParseUint(targetUser.Uid, 10, 32)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to parse UID for %s: %w", username, err)
	}
	gid, err := strconv.ParseUint(targetUser.Gid, 10, 32)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to parse GID for %s: %w", username, err)
	}
	return targetUser, uint32(uid), uint32(gid), nil
}
