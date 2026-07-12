package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
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
