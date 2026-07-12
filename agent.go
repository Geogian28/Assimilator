package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	asslog "github.com/geogian28/Assimilator/assimilator_logger"
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
	// 1. Open the connection for the entire sync cycle here
	address := a.appConfig.ServerIP + ":" + fmt.Sprint(a.appConfig.ServerPort)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		Unhandled("Failed to start NewClient: ", err)
		return
	}
	defer conn.Close() // This will now stay open until all downloads finish

	// 2. Initialize the client for this cycle
	a.client = pb.NewAssimilatorClient(conn)

	// 3. Fetch the config
	machineConfig, err := a.getPackageInfoFromServer(ctx)
	if err == nil {
		if len(machineConfig.GetPackages()) == 0 {
			Error("No packages to install. Double-check config.yaml for ", a.appConfig.Hostname)
			return
		}

		go listPackages(machineConfig.GetPackages())

		// processes the packages
		for packageName, packageConfig := range machineConfig.GetPackages() {
			for _, packageStep := range packageConfig.PackageSteps {
				Trace(packageName, "'s packageStep.Runasuser: ", packageStep.Runasuser)
				Trace(packageName, "'s appConfig.RunAsUser: ", appConfig.RunAsUser)
				if packageStep.Runasuser == appConfig.RunAsUser {
					Debug(packageName, "'s packageStep.Runasuser: ", packageStep.Runasuser)
					Debug(packageName, "'s appConfig.RunAsUser: ", appConfig.RunAsUser)
					Info("Processing package: ", packageName)
					err := a.ProcessPackages(a.convertToPackageInfo(packageName, packageStep, packageConfig.Checksum))
					if err != nil {
						Error("Error processing package: ", err)
					} else {
						Info("Successfully processed package: ", packageName)
					}
				} else {
					Trace(packageName, "'s packageStep.Runasuser: ", packageStep.Runasuser)
					Trace(packageName, "'s appConfig.RunAsUser: ", appConfig.RunAsUser)
				}
			}
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

func listPackages(packages map[string]*pb.PackageConfig) {
	length := 0
	for _, packageConfig := range packages {
		length += len(packageConfig.PackageSteps)
	}
	Debug("There are ", length, " package configs across ,"+fmt.Sprint(len(packages))+" packages.")
	Debug("Listing packages applied to this machine:")
	for packageName, packageconfig := range packages {
		Debug("   - ", packageName)
		for _, packageData := range packageconfig.PackageSteps {
			Debug("      - ", packageData.Action)
		}
	}
}

// Get the machine config from the server
func (a *AgentData) getPackageInfoFromServer(ctx context.Context) (*pb.GetSpecificConfigResponse, error) {
	Debug("Attempting to fetch config from server...")

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	req := &pb.GetSpecificConfigRequest{MachineName: a.appConfig.Hostname}
	resp, err := a.client.GetSpecificConfig(ctx, req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			asslog.Trace("pingServer was canceled by shutdown signal.")
			return nil, err
		}
		return nil, err
	}

	err = checkForVersionMismatch(resp)
	if err != nil {
		return nil, err
	}

	Info("Successfully got config for machine: ", a.appConfig.Hostname)
	return resp, nil
}

func (a *AgentData) convertToPackageInfo(packageName string, packageData *pb.PackageSteps, checksum string) *packageInfo {
	ticketStatus, ticketID := a.checkTormonStatus(packageName)
	Trace("packageName : ", packageName, ", ticketStatus: ", ticketStatus, ", ticketID: ", ticketID)
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
		cacheDir:       a.appConfig.CacheDir,
		name:           packageName,
		checksum:       "",
		serverChecksum: checksum,
		path:           filepath.Join(a.appConfig.CacheDir, packageName+".tar.gz"),
		arguments:      packageData.Arguments,
		action:         packageData.GetAction(),
		runAsUser:      packageData.GetRunasuser(),
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

	Info("Agent starting up...")
	Trace(appConfig.Hostname)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the ticker which activates the agent subroutines
	ticker := time.NewTicker(30 * time.Second)

	// Create a "done" channel to signal when we want to stop the pinger
	done := make(chan bool)

	// Run the first assimilation check
	agentData.assimilationCheck(ctx)
	if appConfig.RunAsUser != "" && appConfig.RunAsUser != "root" {
		Info("Everything is updated. Shutting down.")
		ctx.Done()
		return
	}

	// Start a goutine to run that check again at the specified interval
	go func(ctx context.Context) {
		Debug("Agent loop started.")
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

func (a *AgentData) checkTormonStatus(packageName string) (string, int) {
	if appConfig.TormonAddress == "" {
		return "none", 0
	}
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("%s/api/status?hostname=%s&package_name=%s", appConfig.TormonAddress, appConfig.Hostname, packageName)

	resp, err := client.Get(url)
	if err != nil {
		return "none", 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "none", 0
	}

	var result TicketStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "none", 0
	}
	Trace("packageName : ", packageName, ", ticketStatus: ", result.Status, ", ticketID: ", result.TicketID)
	return result.Status, result.TicketID
}

type TicketStatusResponse struct {
	Status   string `json:"status"`
	TicketID int    `json:"ticket_id"`
}
