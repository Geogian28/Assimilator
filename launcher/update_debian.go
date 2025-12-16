package main

import (
	"fmt"
	"log"
	"strings"
	"time"

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
	assimilatorVersion, err := getAssimilatorVersion(d.runner)
	if err != nil {
		return false, err
	}
	stdout, _, err := d.runner.Run("apt-cache", "show", "--no-all-versions", "assimilator")
	fmt.Println(string(stdout))
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
		fmt.Println("line: ", line)
		if strings.Contains(line, "Version:") {
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
	fmt.Println("Local version: ", localVersion)
	fmt.Println("Cache version: ", cacheVersion)

	if localVersion.LessThan(cacheVersion) {
		log.Println("Assimilator update available.")
		return true, nil

	}
	log.Println("Assimilator is up-to-date.")
	return false, nil
}

func (d *DebianManager) InstallUpdate() error {
	fmt.Println("InstallUpdate: Installing assimilator")
	_, _, err := d.runner.Run("apt", "install", "-y", "assimilator")
	time.Sleep(60 * time.Second)
	return err
}
