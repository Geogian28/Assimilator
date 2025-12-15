package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hashicorp/go-version"
)

type FedoraManager struct {
	runner CommandRunner
}

func newFedoraManager(runner CommandRunner) *FedoraManager {
	return &FedoraManager{runner: runner}
}

func (d *FedoraManager) AddRepo() error {
	return fmt.Errorf("not implemented")
}

func (d *FedoraManager) UpdateCache() error {

	_, _, err := d.runner.Run("dnf", "upgrade", "--refresh", "-y")
	if err != nil {
		fmt.Println("dnf upgrade --refresh failed: ", err)
		return err
	}
	return nil
}

func (d *FedoraManager) IsUpdateAvailable() (bool, error) {
	_, err := os.Stat("/usr/bin/assimilator")
	if os.IsNotExist(err) {
		return false, fmt.Errorf("assimilator binary not found. Assuming Assimilator needs an update")
	}

	binaryVersionString, _, err := d.runner.Run("/usr/bin/assimilator", "--version")
	if err != nil {
		return false, fmt.Errorf("error running assimilator --version: %s", err)
	}
	if string(binaryVersionString) == "" {
		return false, fmt.Errorf("error running assimilator --version: no output")
	}
	assimilatorVersion := strings.TrimSpace(string(binaryVersionString))
	fmt.Println("Assimilator version: ", assimilatorVersion)
	assimilatorVersion = strings.TrimSpace(strings.Split(assimilatorVersion, ":")[1])
	assimilatorVersion = strings.TrimSpace(strings.Split(assimilatorVersion, "\n")[0])
	stdout, _, err := d.runner.Run("dnf", "list", "installed", "assimilator")
	if err != nil {
		return false, fmt.Errorf("error checking for updates: %s", err)
	}
	words := strings.Fields(string(stdout))
	var cacheVersion *version.Version
	cacheVersion, err = version.NewVersion(words[1])
	lines := strings.Split((string(stdout)), "\n")
	if len(lines) == 0 {
		log.Fatal("Error parsing version: no lines returned")
	}
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

func (d *FedoraManager) InstallUpdate() error {
	_, _, err := d.runner.Run("dnf", "install", "assimilator", "-y")

	if err != nil {
		fmt.Println("Error installing package: ", err)
	}
	return nil
}
