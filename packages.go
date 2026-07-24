package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
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

// LoadDesiredState reads the YAML file from the given path and unmarshals it into the AppConfig struct.
func LoadDesiredState(filePath string) (*DesiredState, error) {
	Trace("Reading config file: ", filePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", filePath, err)
	}
	var desiredState DesiredState
	err = yaml.Unmarshal(data, &desiredState)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from '%s': %w", filePath, err)
	}

	// Apply profiles to machines and users
	applyProfiles(&desiredState)
	return &desiredState, nil
}

func applyProfiles(desiredState *DesiredState) {
	var ProfileNames []string
	for profileName := range desiredState.Profiles {
		ProfileNames = append(ProfileNames, profileName)
	}
	Debug("Available profiles: ", strings.Join(ProfileNames, ", "))

	for machineName, machineConfig := range desiredState.Machines {
		mergedPackages := make(map[string][]PackageStep)

		for _, profileName := range machineConfig.AppliedProfiles {
			profile, ok := desiredState.Profiles[profileName]
			if !ok {
				Error("Cannot apply profile: ", profileName, " to machine: ", machineName, ": profile not found: ")
				continue
			}

			Trace(fmt.Sprintf(`Copying packages from profile "%s" to machine: %s`, profileName, machineName))
			combinePackageSteps(mergedPackages, profile.Packages)
			// maps.Copy(machineConfig.Packages, profile.Packages)
		}

		Trace(fmt.Sprintf(`Applying specific overrides for machine: %s`, machineName))
		combinePackageSteps(mergedPackages, machineConfig.Packages)
		verifyPackages(mergedPackages)

		machineConfig.Packages = mergedPackages
		desiredState.Machines[machineName] = machineConfig
	}
}

func combinePackageSteps(target, source map[string][]PackageStep) {
	for pkgName, pkgSteps := range source {
		target[pkgName] = append(target[pkgName], pkgSteps...)
	}
}

func verifyPackages(packages map[string][]PackageStep) {
	for pkgName, pkgSteps := range packages {
		for i, pkgStep := range pkgSteps {
			if pkgStep.RunAsUser == "" {
				packages[pkgName][i].RunAsUser = "root"
			}
		}
	}
}
