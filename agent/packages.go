package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	pb "github.com/geogian28/Assimilator/proto"
)

type packageInfo struct {
	// the cache directory obtained from appconfig. Usually /var/cache/assimilator/packages
	cacheDir string
	// the name of the package, but excluding the .tar.gz extension
	name string
	// the category of the package. Ex: machine or user
	category string
	// the checksum of the local package file
	localChecksum string
	// the checksum of the server's package file
	serverChecksum string
	// the path to the local package including the .tar.gz extension
	path string
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
		err := a.calculateSha256(pkg)
		if err != nil {
			return err
		}
		if pkg.localChecksum == pkg.serverChecksum {
			Debug("Package ", pkg.name, " checksums match.")
			return nil
		}
	}

	// 3. If we are here, we either don't have it, or it's old.
	// DOWNLOAD IT.
	Debug("Downloading package: ", pkg.name)
	err := a.downloadPackage(pkg)
	if err != nil {
		Debug("Error downloading package: ", err)
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

func (a *AgentData) calculateSha256(pkg *packageInfo) error {
	// Open the file
	file, err := os.Open(pkg.path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Calculate the SHA256 checksum
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to copy file content to hash: %w", err)
	}
	hashInBytes := hash.Sum(nil)
	pkg.localChecksum = hex.EncodeToString(hashInBytes)
	return nil
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
			Debug("Received EOF")
			// End of stream means success
			break
		}
		if err != nil {
			Debug("stream error while downloading", err)
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
	return nil
}
