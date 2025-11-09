package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	asslog "github.com/geogian28/Assimilator/assimilator_logger"
	config "github.com/geogian28/Assimilator/config"
	pb "github.com/geogian28/Assimilator/proto"
	"github.com/hashicorp/go-version"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var (
	Info      = asslog.Info
	Debug     = asslog.Debug
	Trace     = asslog.Trace
	Sucess    = asslog.Success
	Warning   = asslog.Warning
	Error     = asslog.Error
	Fatal     = asslog.Fatal
	Unhandled = asslog.Unhandled
)

func checkForUpdates(appConfig *config.AppConfig, commandRunner CommandRunner) error {
	Info("Checking for updates")
	var repoJustAdded = false

	for {
		updateAptCache(commandRunner)

		stdout, _, err := commandRunner.Run("apt-cache", "policy", "assimilator")
		Trace("stdout: ", string(stdout))
		Trace("err: ", err)
		if err != nil {
			Unhandled("Error checking for updates: ", err)
		}
		if string(stdout) == "" {
			if repoJustAdded {
				Error("Package 'assimilator' not found even after adding repository. Please check AptSources config.")
				return nil
			}

			if appConfig.AptSources != "" {
				Info("Assimilator package not found. Adding provided repository to sources.")
				addAssimilatortoRepo(appConfig)
				repoJustAdded = true
				continue
			}
			Error("Package 'assimilator' not found in apt cache. No repository was provided. Please add the repository to your sources and try again.")
			return nil
		}
		lines := strings.Split((string(stdout)), "\n")
		Trace("Number of lines: ", len(lines))
		if len(lines) == 0 {
			Unhandled("Error parsing version: no lines returned")
		}
		var cacheVersion *version.Version
		for _, line := range lines {
			if strings.Contains(line, "Candidate:") {
				// Get version
				versionString := strings.TrimSpace(strings.Split(line, ":")[1])
				Debug(`strings.TrimSpace(strings.Split(line, ":")[1]): `, versionString)
				cacheVersion, err = version.NewVersion(versionString)
				break
			}
		}
		if err != nil || cacheVersion == nil {
			Unhandled("Error parsing version: ", err)
		}
		Trace("")
		Info("Cache version: ", cacheVersion)
		localVersion, err := version.NewVersion(config.VERSION)
		if err != nil {
			Trace("")
			Unhandled("Error parsing version: ", err)
		}
		Info("Local version: ", localVersion)
		if localVersion.LessThan(cacheVersion) {
			Info("Updating Assimilator...")
			updateAssimilator(commandRunner)
		}
		Info("Assimilator is up-to-date.")
		break
	}
	return nil
}

func updateAssimilator(commandRunner CommandRunner) {
	Info("Starting detached update...")

	// The script:
	// 1. sleep 5: Give the main agent 5 seconds to shut down gracefully.
	// 2. apt update: Refresh package lists.
	// 3. apt install -y assimilator: Install the new version.
	updateScript := "sleep 5 && apt update && apt install -y assimilator"

	// We use os/exec directly to get the low-level control we need.
	// Your commandRunner probably waits, which we don't want.
	cmd := exec.Command("sh", "-c", updateScript)

	// --- This is the critical part ---
	// This detaches the new process from the agent's
	// standard input, output, and error. If we don't do this,
	// the OS might not see it as fully "disowned."
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	// --- End critical part ---

	// Start the command... and that's it. We DO NOT wait for it.
	err := cmd.Start()
	if err != nil {
		Error("Failed to start detached update process: ", err)
		// If we can't even START the command, don't shut down.
		return
	}

	// We've successfully launched the background process (e.g., PID 102).
	// Now, the agent (e.g., PID 100) can shut down safely.
	Info("Detached update process launched. Agent shutting down...")
	asslog.Close(0) // This is your graceful shutdown
}

func pingServer(ctx context.Context, appConfig *config.AppConfig, commandRunner CommandRunner) error {
	address := appConfig.ServerIP + ":" + fmt.Sprint(appConfig.ServerPort)
	Debug("Attempting to connect to server at ", address)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		Unhandled("Failed to start NewClient: ", err)
	}
	defer conn.Close()
	client := pb.NewAssimilatorClient(conn)
	// Get the machine's config
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req := &pb.GetSpecificConfigRequest{MachineName: appConfig.Hostname}
	resp, err := client.GetSpecificConfig(ctx, req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			asslog.Trace("pingServer was canceled by shutdown signal.")
			return nil
		}
		return err
	}

	respVersion, err := version.NewVersion(resp.Version.Version)
	configVersion, _ := version.NewVersion(config.VERSION)
	if err == nil && respVersion.LessThan(configVersion) {
		Info("Version mismatch. Server version: ", resp.Version, " Local version: ", config.VERSION)
		err = checkForUpdates(appConfig, commandRunner)
		if err != nil {
			return err
		}
	}
	Info("Agent matches server version.")

	Info("Successfully got config for machine: ", req.MachineName)
	packages := resp.GetMachine().GetPackages()
	installPrograms(packages, commandRunner)
	os.Exit(0)
	return nil
}

func addAssimilatortoRepo(appConfig *config.AppConfig) {
	filePath := "/etc/apt/sources.list.d/assimilator_repo.list"

	// Add a newline, just to be a good citizen
	content := []byte(appConfig.AptSources + "\n")

	// Write the file (0644 is standard read-write for owner, read-only for others)
	err := os.WriteFile(filePath, content, 0644)
	if err != nil {
		return
	}
}

func listenForShutdown(ticker *time.Ticker, done chan bool, cancel context.CancelFunc) {
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)

	// This line blocks the goroutine until a signal arrives
	<-shutdownSignal

	// Signal received, now clean up.
	asslog.Debug("Shutdown signal received, telling agent loop to stop...")
	ticker.Stop()
	cancel()
	done <- true
}

func Agent(appConfig *config.AppConfig, commandRunner CommandRunner) {
	// First, check for updates
	checkForUpdates(appConfig, commandRunner)

	Info("Agent starting up...")
	// TODO: Get hostname
	if appConfig.Hostname == "" {
		appConfig.Hostname = "ubuntu-tester"
		Trace(appConfig.Hostname)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the ticker which activates the agent subroutines
	ticker := time.NewTicker(10 * time.Second)

	// Create a "done" channel to signal when we want to stop the pinger
	done := make(chan bool)

	// Start a goutine to run the pinger
	go func(ctx context.Context) {
		Debug("Agent loop started.")
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				Trace("tick! ", time.Now())
				ticker.Stop()
				err := pingServer(ctx, appConfig, commandRunner)
				if err != nil {
					errorStatus, ok := status.FromError(err)
					if ok {
						switch errorStatus.Code() {
						case codes.Unavailable:
							Error("Assimilator server is unavailable:\n      ", err.Error())
							// return err
						case codes.NotFound:
							Error("Assimilator server could not find this machine's config:\n      ", err.Error())
							// return err
						case codes.Canceled:
							Error("Assimilator server request was canceled:\n      ", err.Error())
							// return err
						default:
							Error("Assimilator server returned an unexpected error:\n      ", err.Error())
							// return err
						}
					}
				}
				ticker = time.NewTicker(10 * time.Second)
			}
		}
	}(ctx)

	listenForShutdown(ticker, done, cancel)
	Debug("Agent shutting down...")
	// return nil
}
