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
	CacheDir         string
	packageName      string
	packageTempPath  string
	packagePermPath  string
	checksum         string
	checksumTempPath string
	checksumPermPath string
	hostname         string
	size             int64
	name             string // the name of the package, but excluding the .tar.gz extension
	category         string // the category of the package. Ex: machine or user
	// localChecksum    string   // the checksum of the local package file
	serverChecksum string   // the checksum of the server's package file
	path           string   // the path to the local package including the .tar.gz extension
	extractDir     string   // the directory to extract the package into
	arguments      []string // Any arguments that need to be passed to the package installer
	env            []string // Any environment variables that need to be set
	runAsUser      string   // The user to run the package installer as
	ticketStatus   string   // The status of the package in Tormon
	ticketID       int      // The ID of the ticket in Tormon, if it exists
}

// Calculates the SHA256 checksum of the package
func (pkg *packageInfo) calculateChecksum() error {
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
	pkg.checksum = hex.EncodeToString(hashInBytes)
	return nil
}
