package main

import (
	"fmt"
	"log"
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

	_, _, err := d.runner.Run("dnf", "clean", "all")
	if err != nil {
		fmt.Println("dnf clean all failed: ", err)
		return err
	}
	return nil
}

func (d *FedoraManager) IsUpdateAvailable() (bool, error) {
	// Get local version
	assimilatorVersion, err := getAssimilatorVersion(d.runner)
	if err != nil {
		return false, err
	}

	// Get cache version
	stdout, stderr, err := d.runner.Run("dnf", "repoquery", "--latest-limit=1", "--qf", "%{version}-%{release}", "assimilator")
	if err != nil {
		return false, fmt.Errorf("dnf list failed to find assimilator: %s", stderr)
	}
	if len(stdout) == 0 {
		return false, fmt.Errorf("dnf repoquery failed to find assimilator, but did not error out")
	}
	cachVerisonString := strings.TrimSpace(string(stdout))
	cacheVersion, err := version.NewVersion(cachVerisonString)
	if err != nil {
		return false, fmt.Errorf("error parsing version: %s", err)
	}

	// Convert to version then compare
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
