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

	asslog "github.com/geogian28/Assimilator/assimilator_logger"
	pb "github.com/geogian28/Assimilator/proto"
)

func (a *AgentData) ensurePackage(pkgName string, cachePath string, serverChecksum string) error {
	// 1. Check if we have the file and if it matches the server
	if a.fileExists(cachePath) {
		localChecksum, err := a.calculateSha256(cachePath)
		if err != nil {
			return err
		}
		if localChecksum == serverChecksum {
			return nil
		}
	}

	// 2. If we are here, we either don't have it, or it's old.
	// DOWNLOAD IT.
	err := a.downloadPackage(pkgName, serverChecksum, cachePath)
	if err != nil {
		return err
	}
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

func (a *AgentData) calculateSha256(cachePath string) (string, error) {
	// Open the file
	file, err := os.Open(cachePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Calculate the SHA256 checksum
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to copy file content to hash: %w", err)
	}
	hashInBytes := hash.Sum(nil)
	hashString := hex.EncodeToString(hashInBytes)
	return hashString, nil
}

func (a *AgentData) downloadPackage(pkgName string, checksum string, destPath string) error {
	asslog.Debug("Starting download for package: ", pkgName)

	// 1. Initiate the request
	req := &pb.PackageRequest{
		PackageName:     pkgName,
		PackageChecksum: checksum,
	}

	// 2. Open the stream
	stream, err := a.client.DownloadPackage(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to start download stream: %w", err)
	}

	// 3. Create the destination file
	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create cache file %s: %w", destPath, err)
	}
	defer outFile.Close()

	// 4. Receive chunks in a loop
	var bytesReceived int64
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			// End of stream means success
			break
		}
		if err != nil {
			return fmt.Errorf("stream error while downloading %s: %w", pkgName, err)
		}

		// (Optional) Progress logging
		if chunk.TotalSize > 0 {
			asslog.Trace(fmt.Sprintf("Download started. Size: %d bytes", chunk.TotalSize))
		}

		// Write bytes to disk
		n, err := outFile.Write(chunk.Content)
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
		bytesReceived += int64(n)
	}

	asslog.Info(fmt.Sprintf("Successfully downloaded %s (%d bytes)", pkgName, bytesReceived))
	return nil
}

func (a *AgentData) extractMachinePackage(pkgName string, cachePath string) error {
	destDir := filepath.Join(os.TempDir(), "assimilator", "machine", pkgName)
	return a.extractArchive(destDir, cachePath)
}

func (a *AgentData) extractUserPackage(pkgName string, cachePath string, username string) error {
	destDir := filepath.Join(os.TempDir(), "assimilator", "user", username, pkgName)
	return a.extractArchive(destDir, cachePath)
}

func (a *AgentData) extractArchive(pkgName string, cachePath string) error {
	extractDir := filepath.Join(os.TempDir(), "assimilator", pkgName)

	// 1. Clean up any previous run to ensure a fresh slate
	os.RemoveAll(extractDir)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	// 2. Extract the tarball INTO that directory
	//    -C tells tar to change directory before extracting
	_, _, err := a.commandRunner.Run("tar", "-xzf", cachePath, "-C", extractDir)
	if err != nil {
		return fmt.Errorf("failed to extract %s: %w", pkgName, err)
	}
	return nil
}
