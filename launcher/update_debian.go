package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hashicorp/go-version"
)

type DebianManager struct {
	runner CommandRunner
}

func newDebianManager(runner CommandRunner) *DebianManager {
	return &DebianManager{runner: runner}
}

func (d *DebianManager) AddRepo() error {
	return nil
}

func (d *DebianManager) UpdateCache() error {
	_, _, err := d.runner.Run("apt", "update")
	return err
}

// IsUpdateAvailable compares the local version against the cached version.
// It returns true if a newer version is available.
func (d *DebianManager) IsUpdateAvailable() (bool, error) {
	_, err := os.Stat("/usr/bin/assimilator")
	if os.IsNotExist(err) {
		return false, fmt.Errorf("assimilator binary not found. Assuming Assimilator needs an update")
	}

	binaryVersionString, _, err := d.runner.Run("/usr/bin/assimilator", "--version")
	if err != nil {
		return false, fmt.Errorf("error running assimilator --version: %s", err)
	}
	assimilatorVersion := strings.TrimSpace(string(binaryVersionString))
	assimilatorVersion = strings.TrimSpace(strings.Split(assimilatorVersion, ":")[1])
	assimilatorVersion = strings.TrimSpace(strings.Split(assimilatorVersion, "\n")[0])
	stdout, _, err := d.runner.Run("apt-cache", "policy", "assimilator")
	if err != nil {
		return false, fmt.Errorf("error checking for updates: %s", err)
	}
	if string(stdout) == "" {
		return false, fmt.Errorf("package 'assimilator' not found in apt cache. Please add the repository to your sources and try again")
	}
	lines := strings.Split((string(stdout)), "\n")
	if len(lines) == 0 {
		log.Fatal("Error parsing version: no lines returned")
	}
	var cacheVersion *version.Version
	for _, line := range lines {
		if strings.Contains(line, "Candidate:") {
			// Get version
			versionString := strings.TrimSpace(strings.Split(line, ":")[1])
			cacheVersion, err = version.NewVersion(versionString)
			break
		}
	}
	if err != nil || cacheVersion == nil {
		log.Fatal("Error parsing version: ", err)
	}
	localVersion, err := version.NewVersion(assimilatorVersion)
	if err != nil {
		log.Fatal("Error parsing version: ", err)
	}
	if localVersion.LessThan(cacheVersion) {
		return true, nil

	}
	log.Println("Assimilator is up-to-date.")
	return false, nil
}

func (d *DebianManager) InstallUpdate() error {
	_, _, err := d.runner.Run("apt", "install", "-y", "assimilator")
	return err
}
