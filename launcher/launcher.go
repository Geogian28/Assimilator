package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func detectDistro(runner CommandRunner) (DistroManager, error) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		log.Fatal("Unable to determine OS: ", err)
	}
	defer file.Close()

	return parseDistro(runner, file)
}

func parseDistro(runner CommandRunner, reader io.Reader) (DistroManager, error) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") || strings.HasPrefix(line, "ID_LIKE=") {
			switch {
			case strings.Contains(line, "ubuntu") || strings.Contains(line, "debian"):
				return newDebianManager(runner), nil
			case strings.Contains(line, "fedora"):
				return newFedoraManager(runner), nil
			case strings.Contains(line, "arch"):
				return newArchManager(runner), nil
			}
		}
	}
	return nil, fmt.Errorf("unable to determine OS")
}

func updateAssimilator(runner CommandRunner) {
	log.Println("Checking for updates")

	// Detect distro
	distro, err := detectDistro(runner)
	if err != nil {
		log.Fatal("Unable to detect distro.")
	}

	// Add the assimilator repo - not implemented
	// err = distro.AddRepo()
	// if err != nil && err != fmt.Errorf("not implemented") {
	// 	log.Fatal("unable to add repo: ", err)
	// }

	// Update the cache
	err = distro.UpdateCache()
	if err != nil {
		log.Fatal("Unable to update cache: ", err)
	}

	// Check for updates
	updateIsAvailable, err := distro.IsUpdateAvailable()
	if err != nil {
		log.Fatal("Unable to check for updates: ", err)
	}

	// Install the update if needed
	if updateIsAvailable {
		distro.InstallUpdate()
	}
	fmt.Println("no update neededed")
}

func runAssimilator() {
	binaryPath, err := exec.LookPath("assimilator")
	if err != nil {
		log.Fatalf("Fatal: Could not find 'assimilator' binary in $PATH: %v", err)
	}
	log.Printf("Found 'assimilator' at %s. Executing...", binaryPath)
	newArgv := append([]string{binaryPath}, os.Args[1:]...)
	err = syscall.Exec(binaryPath, newArgv, os.Environ())
	if err != nil {
		log.Fatalf("Fatal: Failed to execute 'assimilator': %v", err)
	}
}

func main() {
	// Parse command-line flags for debugging
	// for _, arg := range os.Args[1:] {
	// 	if arg == "--test_mode" {
	// 		runAssimilator()
	// 	}

	// }

	// Create the command runner
	commandRunner := &LiveCommandRunner{}
	// Add the assimilator repo
	fmt.Println("Updating assimilator")
	updateAssimilator(commandRunner)

	// Run assimilator itself
	fmt.Println("Running assimilator")
	runAssimilator()
}
