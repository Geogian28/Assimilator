package agent

import (
	"context"
	"fmt"
	"os"
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

// const (
// 	// address = "127.0.0.1:2390"
// 	address = "10.42.1.66:2390"
// )

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

func pingServer(appConfig *config.AppConfig, commandRunner CommandRunner) error {
	address := appConfig.ServerIP + ":" + fmt.Sprint(appConfig.ServerPort)
	Debug("Attempting to connect to server at ", address)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		Unhandled("Failed to start NewClient: ", err)
	}
	defer conn.Close()
	client := pb.NewAssimilatorClient(conn)
	// Get the machine's config
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	specificConfigsReq := &pb.GetSpecificConfigRequest{MachineName: appConfig.Hostname}
	specificConfigsResp, err := client.GetSpecificConfig(ctx, specificConfigsReq)
	if err != nil {
		return err
	}

	Info("Successfully got config for machine: ", specificConfigsReq.MachineName)
	packages := specificConfigsResp.GetMachine().GetPackages()
	installPrograms(packages, commandRunner)
	os.Exit(0)
	return nil
}

func checkForUpdates(commandRunner CommandRunner) {
	asslog.Info("Checking for updates")
	updateAptCache(commandRunner)
	stdout, err := commandRunner.Run("apt-cache", "policy", "assimilator")
	Trace("")
	if err != nil {
		Trace("")
		asslog.Unhandled("Error checking for updates: ", err)
	}
	Trace("")
	if string(stdout) == "N: Unable to locate package assimilator" {
		Trace("")
		asslog.Info("Assimilator is not in local repo. Unable to update.")
		return
	}
	Trace("")
	lines := strings.Split((string(stdout)), "\n")
	Trace("Number of lines: ", len(lines))
	if len(lines) == 0 {
		asslog.Unhandled("Error parsing version: no lines returned")
	}
	Trace("")
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
	Trace("")
	if err != nil || cacheVersion == nil {
		asslog.Unhandled("Error parsing version: ", err)
	}
	Trace("")
	Info("Cache version: ", cacheVersion)
	Trace("")
	localVersion, err := version.NewVersion(config.VERSION)
	Trace("")
	if err != nil {
		Trace("")
		asslog.Unhandled("Error parsing version: ", err)
	}
	Trace("")
	Info("Local version: ", localVersion)
	if localVersion.LessThan(cacheVersion) {
		Info("Updating Assimilator...")
		updateAssimilator(commandRunner)
	}
	Info("Assimilator is up-to-date.")
}

func updateAssimilator(commandRunner CommandRunner) {
	_, err := commandRunner.Run("apt", "install", "--only-upgrade", "-y", "assimilator")
	if err != nil {
		asslog.Unhandled("Error updating assimilator: ", err)
	}
	Info("Assimilator updated. Restarting...")
	asslog.Close(0)
}

func Agent(appConfig *config.AppConfig, commandRunner CommandRunner) {
	// First, check for updates
	checkForUpdates(commandRunner)

	Info("Agent starting up...")
	// TODO: Get hostname
	if appConfig.Hostname == "" {

		appConfig.Hostname = "ubuntu-tester"
		Trace(appConfig.Hostname)
	}

	// Start the ticker which activates the agent subroutines
	ticker := time.NewTicker(10 * time.Second)

	// Create a "done" channel to signal when we want to stop the pinger
	done := make(chan bool)

	// Start a goutine to run the pinger
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				Trace("tick! ", time.Now())
				ticker.Stop()
				err := pingServer(appConfig, commandRunner)
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
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)

	<-shutdownSignal
	ticker.Stop()
	done <- true
	Debug("Agent shutting down...")
	// return nil
}
