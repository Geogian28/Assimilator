package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
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
	appConfig      *AppConfig
	client         pb.AssimilatorClient
	commandRunner  CommandRunner
	failureReports map[string]string
}

var agentData *AgentData

// Check the server for updates
func (a *AgentData) assimilationCheck(ctx context.Context) {
	Info(2, "Starting assimilation check...")
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
	if err != nil {
		Error(1, "error getting package info from server: ", err)
		return
	}

	for packageName, packageConfig := range machineConfig {
		Trace(5, "Package: ", packageName, " checksum: ", packageConfig.Checksum)
	}

	// 4. Filter packages by user then sort them, and list them
	filteredNames, filteredPackages := PackagesForUser(machineConfig)
	// listPackages(filteredNames, machineConfig)

	// 5. Processes the packages
	a.failureReports = make(map[string]string, len(machineConfig))
	for _, packageName := range filteredNames {
		p := filteredPackages[packageName]
		err := p.ProcessPackage(a)
		if err != nil {
			a.failureReports[p.action+" "+packageName] = fmt.Sprintf("error processing %s package's %s action: %s ", packageName, p.action, err)
			Error(1, "error processing package: ", err)
		}
	}

	// printReports(filteredNames, a.failureReports)
	Info(2, "Completed assimilation check.")
}

func PackagesForUser(packages map[string]*pb.PackageConfig) ([]string, map[string]*packageInfo) {
	sortedNames := make([]string, 0, len(packages))
	filteredPackages := make(map[string]*packageInfo, len(packages))
	for packageName, packageConfig := range packages {
		for _, packageStep := range packageConfig.PackageSteps {
			if packageStep.Runasuser == appConfig.RunAsUser || packageStep.Runasuser == "_all" {
				Trace(4, packageName, "'s packageStep and appConfig 'RunAsUser' match: ", packageStep.Runasuser, " & ", appConfig.RunAsUser)
				sortedNames = append(sortedNames, packageName)
				filteredPackages[packageName] = convertToPackageInfo(
					packageName,
					packageStep,
					packageConfig.Checksum,
				)
			} else {
				Trace(5, packageName, "'s packageStep and appConfig 'RunAsUser' dont match: ", packageStep.Runasuser, " & ", appConfig.RunAsUser)
			}
		}
	}
	slices.Sort(sortedNames)
	if sortedNames == nil {
		Error(1, "No packages found for user: ", appConfig.RunAsUser)
		os.Exit(1)
	}
	return sortedNames, filteredPackages
}

// func printReports(namesSorted []string, failureReports map[string]string) {
// 	// successfulReports := []string{}
// 	// failedReports := []string{}
// 	Trace(5, "namesSorted length: ", len(namesSorted))
// 	// for _, packageName := range namesSorted {
// 	// 	Trace(5, "packageName: ", packageName)
// 	// 	if _, ok := failureReports[packageName]; ok {
// 	// 		failedReports = append(failedReports, packageName)
// 	// 	} else {
// 	// 		successfulReports = append(successfulReports, packageName)
// 	// 	}
// 	// }
// 	var builder strings.Builder
// 	var report string
// 	// if len(successfulReports) > 0 {
// 	// 	fmt.Fprintln(&builder, fmt.Sprintln("# These package actions succeeded:"))
// 	// 	for _, packageName := range successfulReports {
// 	// 		fmt.Fprintln(&builder, "  - ", packageName)
// 	// 	}
// 	// } else {
// 	// 	fmt.Fprintln(&builder, fmt.Sprintln("# No package actions succeeded..."))
// 	// }
// 	fmt.Fprintln(&builder, "\n# These package actions failed:")
// 	for packageName, report := range failureReports {
// 		fmt.Fprintln(&builder, "  - ", packageName)
// 		fmt.Fprintln(&builder, report)
// 	}
// 	report = builder.String()
// 	Info(2, "Results:\n", report, "\n")
// }

func listPackages(namesSorted []string, packages map[string]*pb.PackageConfig) {
	length := 0
	for _, packageConfig := range packages {
		length += len(packageConfig.PackageSteps)
	}

	Trace(4, "There are ", length, " package configs across ,"+fmt.Sprint(len(packages))+" packages.")
	Trace(4, "Listing packages applied to this machine:")
	// for packageName, packageconfig := range packages {
	for _, packageName := range namesSorted {
		Trace(4, "- ", packageName)
		for _, packageData := range packages[packageName].PackageSteps {
			if len(packageData.Arguments) > 0 {
				Trace(4, "  - ", packageData.Action, " as user ", packageData.Runasuser, " with arguments:")
				for _, argument := range packageData.Arguments {
					Trace(4, "    - ", argument)
				}
				continue
			}
			Trace(4, "   - ", packageData.Action, " as user ", packageData.Runasuser)
		}
	}
}

// Get the machine config from the server
func (a *AgentData) getPackageInfoFromServer(ctx context.Context) (map[string]*assctl.PackageConfig, error) {
	Trace(4, "Attempting to fetch config from server...")
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	req := &pb.GetSpecificConfigRequest{MachineName: a.appConfig.Hostname}
	resp, err := a.client.GetSpecificConfig(ctx, req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			asslog.Trace(5, "pingServer was canceled by shutdown signal.")
			return nil, err
		}
		errorStatus, ok := status.FromError(err)
		if !ok {
			Warning(1, "failed to ping server: ", err)
		}
		switch errorStatus.Code() {
		case codes.Unavailable:
			Warning(1, "assimilator server is unavailable (retrying at the next tick):\n      ", err.Error())
		case codes.NotFound:
			Error(1, "assimilator server could not find this machine's config:\n      ", err.Error())
		case codes.Canceled:
			Trace(5, "assimilator server request was canceled:\n      ", err.Error())
		default:
			Error(1, "assimilator server returned an unexpected error:\n      ", err.Error())
		}
		return nil, err
	}

	err = checkForVersionMismatch(resp)
	if err != nil {
		return nil, err
	}

	Info(2, "Successfully got config for machine: ", a.appConfig.Hostname)
	if len(resp.GetPackages()) == 0 {
		Error(1, "No packages to install. Double-check config.yaml for ", a.appConfig.Hostname)
		return nil, err
	}
	return resp.GetPackages(), nil
}

func convertToPackageInfo(packageName string, packageData *pb.PackageSteps, checksum string) *packageInfo {
	// ticketStatus, ticketID := checkTormonStatus(packageName)
	ticketStatus, ticketID := "notset", 0
	Trace(5, "packageName : ", packageName, ", ticketStatus: ", ticketStatus, ", ticketID: ", ticketID)
	switch ticketStatus {
	case "notset":
		break
	case "open":
		Info(2, fmt.Sprintf("skipping %s: open ticket exists in Tormon. Change status to 'pending' to retry.\n    Ticket: https://tormon/%d\n", packageName, ticketID))
		return nil
	case "pending":
		Info(2, "Tormon asked to retry deployment.")
		// pendingStatus = true
	case "none":
		Error(1, "Tormon ticket not found. Continuing anyways deployment of ", packageName)
	}
	packageCacheDir := filepath.Join(appConfig.CacheDir, packageName)
	runAsUser := packageData.GetRunasuser()
	if runAsUser == "_all" {
		runAsUser = appConfig.RunAsUser
	}

	pkg := &packageInfo{
		cacheDir:       packageCacheDir,
		name:           packageName,
		checksum:       "",
		serverChecksum: checksum,
		path:           filepath.Join(packageCacheDir, packageName+".tar.gz"),
		arguments:      packageData.Arguments,
		action:         packageData.GetAction(),
		runAsUser:      runAsUser,
		updateInterval: appConfig.PackageUpdateInterval,
	}
	return pkg
}

// func getMachineConfig(ctx context.Context, conn *grpc.ClientConn) (*pb.GetSpecificConfigResponse, error) {
// 	client := pb.NewAssimilatorClient(conn)
// 	agentData.client = client
// 	req := &pb.GetSpecificConfigRequest{MachineName: agentData.appConfig.Hostname}
// 	resp, err := client.GetSpecificConfig(ctx, req)
// 	if err != nil {
// 		if errors.Is(err, context.Canceled) {
// 			asslog.Trace(5, "pingServer was canceled by shutdown signal.")
// 			return nil, err
// 		}
// 		return nil, err
// 	}
// 	return resp, nil
// }

func checkForVersionMismatch(resp *pb.GetSpecificConfigResponse) error {
	Trace(5, "setting respVersion to ", resp.Version.Version)
	respVersion, err := version.NewVersion(resp.Version.Version)
	Trace(5, "setting configVersion to ", agentData.appConfig.version)
	configVersion, _ := version.NewVersion(agentData.appConfig.version)

	Trace(5, "comparing ", configVersion, " to ", respVersion)
	if err == nil && configVersion.LessThan(respVersion) {
		Info(2, "version mismatch. Server version: ", respVersion, " Local version: ", agentData.appConfig.version)
		if !appConfig.TestMode {
			Info(2, "Restarting to update...")
			asslog.Close()
			os.Exit(0)
		}
	}
	Info(2, "Agent version (", agentData.appConfig.version, ") matches server version (", resp.Version.Version, ").")
	return nil
}

// func checkTormonStatus(packageName string) (string, int) {
// 	Trace(5, "appConfig.TormonAddress: ", appConfig.TormonAddress)
// 	if appConfig.TormonAddress == "" {
// 		return "notset", 0
// 	}
// 	client := &http.Client{Timeout: 5 * time.Second}
// 	url := fmt.Sprintf("%s/api/status?hostname=%s&package_name=%s", appConfig.TormonAddress, appConfig.Hostname, packageName)

// 	resp, err := client.Get(url)
// 	if err != nil {
// 		return "none", 0
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		return "none", 0
// 	}

// 	var result TicketStatusResponse
// 	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// 		return "none", 0
// 	}
// 	Trace(5, "packageName : ", packageName, ", ticketStatus: ", result.Status, ", ticketID: ", result.TicketID)
// 	return result.Status, result.TicketID
// }

type TicketStatusResponse struct {
	Status   string `json:"status"`
	TicketID int    `json:"ticket_id"`
}

func Agent(commandRunner CommandRunner) {
	agentData = &AgentData{
		appConfig:     &appConfig,
		commandRunner: commandRunner,
	}

	Info(2, "Agent starting up...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the ticker which activates the agent subroutines
	ticker := time.NewTicker(time.Duration(appConfig.UpdateCheckInterval) * time.Second)

	// Create a "done" channel to signal when we want to stop the pinger
	done := make(chan bool)

	// Run the first assimilation check
	agentData.assimilationCheck(ctx)
	// if (appConfig.RunAsUser != "" && appConfig.RunAsUser != "root") || appConfig.RunOnce {
	if appConfig.RunOnce {
		Info(2, "Everything is updated. Shutting down.")
		ctx.Done()
		return
	}

	// Start a goutine to run that check again at the specified interval
	go func(ctx context.Context) {
		Trace(4, "Agent loop started.")
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				Trace(5, "tick! ", time.Now())
				ticker.Stop()
				agentData.assimilationCheck(ctx)
				ticker = time.NewTicker(time.Duration(appConfig.UpdateCheckInterval) * time.Second)
				Info(2, "Waiting ", time.Duration(appConfig.UpdateCheckInterval)*time.Second, " seconds before next check.")
			}
		}
	}(ctx)

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)

	// This line blocks the goroutine until a signal arrives
	<-shutdownSignal

	// Signal received, now clean up.
	Trace(4, "Shutdown signal received, telling agent loop to stop...")
	ticker.Stop()
	cancel()
	done <- true
	Trace(4, "Agent shutting down...")
}
