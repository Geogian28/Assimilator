package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	pb "github.com/geogian28/Assimilator/proto"
)

type work struct {
	packageName       string
	installedPrograms map[string]bool
}

// Install programs
func installPrograms(packages map[string]*pb.PackageConfig, commandRunner CommandRunner) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// update apt cache
	err := updateAptCache(commandRunner)
	if err != nil {
		Error("Error updating apt cache. Will continue but packages will be out of date.")
		Debug(err)
	}

	// collect installed programs from "apt list" command

	installedPrograms, err := collectInstalledPrograms(commandRunner)
	if err != nil {
		Error("Error collecting installed programs. Will continue but packages will be out of date.")
		Debug(err)
	}

	// create apt workers
	maxWorkers := 10
	aptWorkers := make(chan work, maxWorkers)
	for range maxWorkers {
		go aptWorker(ctx, &wg, &mu, commandRunner, aptWorkers)
	}

	// install packages
	for packageName := range packages {
		wg.Add(1)
		aptWorkers <- work{
			installedPrograms: installedPrograms,
			packageName:       packageName,
		}
	}

	wg.Wait()
	close(aptWorkers)
	Info("installPrograms complete.")

	return nil
}

func updateAptCache(commandRunner CommandRunner) error {
	Trace("Updating apt cache...")

	lsbCommand, _, err := commandRunner.Run("cat", "/etc/os-release")
	if err != nil {
		return err
	}
	if string(lsbCommand) == "" {
		return fmt.Errorf("lsbCommand is empty")
	}
	for line := range strings.SplitSeq(string(lsbCommand), "\n") {
		if strings.Contains(strings.ToLower(line), "debian") {
			_, _, err := commandRunner.Run("apt", "update")
			if err != nil {
				return err
			}
			Trace("Apt cache updated.")
			return nil
		}
	}
	return fmt.Errorf("unsupported OS")
}

// collectInstalledPrograms runs apt list to get a list of the currently installed programs
func collectInstalledPrograms(commandRunner CommandRunner) (map[string]bool, error) {
	m := make(map[string]bool)
	Trace("Collecting installed programs...")
	stdout, _, err := commandRunner.Run("apt", "list", "--installed")
	if err != nil {
		return m, err
	}
	Trace("Installed programs collected.")
	for line := range strings.SplitSeq(string(stdout), "\n") {
		m[strings.Split(line, "/")[0]] = true
	}
	return m, nil
}

// update apt cache
func aptWorker(ctx context.Context, wg *sync.WaitGroup, mu *sync.Mutex, commandRunner CommandRunner, tasksChannel <-chan work) error {
	for {
		select {
		case <-ctx.Done():
			// The context was cancelled. Stop work and return.
			return nil
		case task, ok := <-tasksChannel:
			if !ok {
				// wg.Done()
				return nil
			} else {
				installAptPackage(wg, mu, commandRunner, task.packageName, task.installedPrograms)
			}
		}
	}
}

func installAptPackage(wg *sync.WaitGroup, mu *sync.Mutex, commandRunner CommandRunner, packageName string, installedPrograms map[string]bool) {
	defer wg.Done()

	// check if package is valid
	if !isValid(commandRunner, packageName) {
		Error(packageName, " is not a valid package")
		return
	}

	// Check if package is already installed
	if installedPrograms[packageName] {
		Info(packageName, " is already installed.")
		return
	}

	// install package
	installErr := func(commandRunner CommandRunner) error {
		mu.Lock()
		Debug("Installing package: ", packageName)
		defer mu.Unlock()
		_, _, err := commandRunner.Run("apt", "install", "-y", packageName)
		if err != nil {
			return err
		}
		return nil
	}(commandRunner)
	if installErr != nil {
		Error("Error installing package:", installErr)
		return
	}
	Info("Installation of ", packageName, " successful.")

	// configure package that was just installed
	err := ConfigureProgram(packageName)
	if err != nil {
		Error("Error configuring package:", err)
		return
	}
}

func ConfigureProgram(PackageName string) error {
	return nil
}

// Check if package is valid
func isValid(commandRunner CommandRunner, packageName string) bool {
	_, _, err := commandRunner.Run("apt-cache", "show", packageName)
	return err == nil
}
