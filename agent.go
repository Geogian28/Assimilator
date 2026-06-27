package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	asslog "github.com/geogian28/Assimilator/assimilator_logger"
	assctl "github.com/geogian28/Assimilator/proto"
	pb "github.com/geogian28/Assimilator/proto"
	"github.com/hashicorp/go-version"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type AgentData struct {
	appConfig     *AppConfig
	client        pb.AssimilatorClient
	commandRunner CommandRunner
}

var agentData *AgentData

// Check the server for updates
func (a *AgentData) assimilationCheck(ctx context.Context) {
	machineConfig, err := getPackageInfoFromServer(ctx)
	if err == nil {
		if len(machineConfig.GetPackages()) == 0 {
			Error("No packages to install. Double-check config.yaml for ", agentData.appConfig.Hostname)
			return
		}

		// lists the packages to the logger
		go listPackages(machineConfig.GetPackages())
		machineConfig.GetPackages()
		// processes the packages
		for packageName, packageData := range machineConfig.GetPackages() {
			agentData.ProcessPackages(a.convertToPackageInfo(packageName, packageData))
		}
		return
	}

	errorStatus, ok := status.FromError(err)
	if !ok {
		Warning("failed to ping server: ", err)
		return
	}

	switch errorStatus.Code() {
	case codes.Unavailable:
		Warning("Assimilator server is unavailable (retrying at the next tick):\n      ", err.Error())
	case codes.NotFound:
		Error("Assimilator server could not find this machine's config:\n      ", err.Error())
	case codes.Canceled:
		Trace("Assimilator server request was canceled:\n      ", err.Error())
	default:
		Error("Assimilator server returned an unexpected error:\n      ", err.Error())
	}
}

func listPackages(packages map[string]*assctl.PackageConfig) {
	Debug("Number of machine packages: ", len(packages))
	Debug("Listing packages applied to this machine:")
	for packageName := range packages {
		Debug("   - ", packageName)
	}
}

// Get the machine config from the server
func getPackageInfoFromServer(ctx context.Context) (*pb.GetSpecificConfigResponse, error) {
	address := agentData.appConfig.ServerIP + ":" + fmt.Sprint(agentData.appConfig.ServerPort)
	Debug("Attempting to connect to server at ", address)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		Unhandled("Failed to start NewClient: ", err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	resp, err := getMachineConfig(ctx, conn)
	if err != nil {
		return nil, err
	}

	err = checkForVersionMismatch(resp)
	if err != nil {
		return nil, err
	}

	Info("Successfully got config for machine: ", agentData.appConfig.Hostname)
	return resp, nil
}

func (a *AgentData) convertToPackageInfo(packageName string, packageData *pb.PackageConfig) *packageInfo {
	ticketStatus, ticketID := a.checkTormonStatus(packageName)
	Trace("packageName : ", packageName, ", ticketStatus: ", ticketStatus, ", ticketID: ", ticketID)
	// var pendingStatus bool
	switch ticketStatus {
	case "open":
		Info(fmt.Sprintf("skipping %s: open ticket exists in Tormon. Change status to 'pending' to retry.\n    Ticket: https://tormon/%d\n", packageName, ticketID))
		return nil
	case "pending":
		Info("Tormon asked to retry deployment.")
		// pendingStatus = true
	case "none":
		Error("Tormon ticket not found. Continuing anyways deployment of ", packageName)
	}
	pkg := &packageInfo{
		CacheDir:       filepath.Join(a.appConfig.CacheDir, "machine"),
		name:           packageName,
		category:       "machine",
		checksum:       "",
		serverChecksum: packageData.Checksum,
		path:           filepath.Join(a.appConfig.CacheDir, "machine", packageName+".tar.gz"),
		arguments:      packageData.Arguments,
	}
	Trace("packageData.Arguments: ", packageData.Arguments) //packageData.Arguments =
	Trace("pkg.arguments: ", pkg.arguments)
	return pkg
}
func getMachineConfig(ctx context.Context, conn *grpc.ClientConn) (*pb.GetSpecificConfigResponse, error) {
	client := pb.NewAssimilatorClient(conn)
	agentData.client = client
	req := &pb.GetSpecificConfigRequest{MachineName: agentData.appConfig.Hostname}
	resp, err := client.GetSpecificConfig(ctx, req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			asslog.Trace("pingServer was canceled by shutdown signal.")
			return nil, err
		}
		return nil, err
	}
	return resp, nil
}

func checkForVersionMismatch(resp *pb.GetSpecificConfigResponse) error {
	Trace("setting respVersion to ", resp.Version.Version)
	respVersion, err := version.NewVersion(resp.Version.Version)
	Trace("setting configVersion to ", agentData.appConfig.version)
	configVersion, _ := version.NewVersion(agentData.appConfig.version)

	Trace("comparing ", configVersion, " to ", respVersion)
	if err == nil && configVersion.LessThan(respVersion) {
		Info("version mismatch. Server version: ", respVersion, " Local version: ", agentData.appConfig.version)
		Info("Restarting to update...")
		asslog.Close()
		os.Exit(0)
	}
	Info("Agent version (", agentData.appConfig.version, ") matches server version (", resp.Version.Version, ").")
	return nil
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

func Agent(commandRunner CommandRunner) {
	agentData = &AgentData{
		appConfig:     &appConfig,
		commandRunner: commandRunner,
	}

	// First, check for updates
	// selfupdate.CheckForUpdates(appConfig, commandRunner)

	Info("Agent starting up...")
	if appConfig.Hostname == "" {
		if appConfig.machineInfo.Node.Hostname != "" {
			appConfig.Hostname = appConfig.machineInfo.Node.Hostname
		} else {
			appConfig.Hostname = "uh-oh"
		}
	}
	Trace(appConfig.Hostname)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the ticker which activates the agent subroutines
	ticker := time.NewTicker(30 * time.Second)

	// Create a "done" channel to signal when we want to stop the pinger
	done := make(chan bool)

	// Start a goutine to run that check again at the specified interval
	go func(ctx context.Context) {
		Debug("Agent loop started.")
		// Run the first assimilation check
		agentData.assimilationCheck(ctx)

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				Trace("tick! ", time.Now())
				ticker.Stop()
				agentData.assimilationCheck(ctx)
				ticker = time.NewTicker(60 * time.Second)
			}
		}
	}(ctx)

	listenForShutdown(ticker, done, cancel)
	Debug("Agent shutting down...")
	// return nil
}
