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

	// "syscall"

	pb "github.com/geogian28/Assimilator/proto"
)

// A single, unified function handles the entire lifecycle
func (a *AgentData) ProcessPackages(pkg *packageInfo) error {
	if err := a.ensurePackage(pkg); err != nil {
		return err
	}
	Trace("Successfully ensured ", pkg.name)
	if err := a.extractPackage(pkg); err != nil {
		return err
	}
	Trace("Successfully extracted ", pkg.name)
	if err := a.executePackageScript(pkg); err != nil {
		return err
	}
	Trace("Successfully excuted script for", pkg.name)
	return nil
}

func (a *AgentData) ensurePackage(pkg *packageInfo) error {
	// 1. Check if the folder exists

	Debug("Checking if package folder exists: ", pkg.cacheDir)
	if !a.fileExists(pkg.cacheDir) {
		err := os.MkdirAll(pkg.cacheDir, 0755)
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
		Name: pkg.name,
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

	extractDir := filepath.Join(os.TempDir(), "assimilator", appConfig.RunAsUser, pkg.name)
	// 0. Clean up any previous run to ensure a fresh slate
	os.RemoveAll(extractDir)
	if err := os.MkdirAll(extractDir, 0754); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	// 1. Extract the tarball INTO that directory
	//    -C tells tar to change directory before extracting
	_, _, err := a.commandRunner.Run("tar", "-xzf", pkg.path, "-C", extractDir)
	if err != nil {
		return fmt.Errorf("failed to extract %s: %w", pkg.name, err)
	}
	pkg.extractDir = extractDir
	return nil
}

func (a *AgentData) executePackageScript(pkg *packageInfo) error {
	Trace("Executing install script for ", pkg.name)
	// 1. Ensure the script is executable
	if err := os.Chmod(filepath.Join(pkg.extractDir, fmt.Sprintf("%s.sh", pkg.action)), 0755); err != nil {
		return fmt.Errorf("failed to make script executable: %w", err)
	}
	// 2. Run the install script
	// Join the arguments array into a space-separated string (e.g. "--unattended --force")
	Trace("Arguments: ", pkg.arguments)
	// If the package didnt specify a runAsUser, default to root, then look it up
	if pkg.runAsUser == "" {
		return fmt.Errorf("package %s did not specify a runAsUser. Exiting to expose error instead of applying bandaid", pkg.name)
	}
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("user.Current() error: %v", err)
	}

	commandToRun := pkg.extractDir + "/" + fmt.Sprintf("%s.sh", pkg.action)
	cmd := exec.Command(commandToRun, pkg.arguments...)
	cmd.Dir = pkg.extractDir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, pkg.env...)
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("USER=%s", currentUser.Username),
		fmt.Sprintf("HOME=%s", currentUser.HomeDir),
		fmt.Sprintf("ASSIMILATOR_HOME=%s", currentUser.HomeDir),
		fmt.Sprintf("ASSIMILATOR_USER=%s", currentUser.Username),
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	)

	Trace("Running script ", commandToRun, " as user: ", pkg.runAsUser)
	output, err := cmd.CombinedOutput()
	Debug("\n", string(output))
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			return fmt.Errorf("Script failed with exit code: %v", code)
		} else {
			// The system couldn't even start the script
			return fmt.Errorf("Failed to start script: %v\n", err)
		}
	} else {
		Success("Script ", commandToRun, " ran successfully!")
	}
	return nil
}
