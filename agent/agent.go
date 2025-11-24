package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
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
		// err = selfupdate.CheckForUpdates(appConfig, commandRunner)
		// if err != nil {
		// 	return err
		// }
	}
	Info("Agent matches server version.")

	Info("Successfully got config for machine: ", req.MachineName)
	packages := resp.GetMachine().GetPackages()
	installPrograms(packages, commandRunner)
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

func Agent(appConfig *config.AppConfig, commandRunner CommandRunner) {
	// First, check for updates
	// selfupdate.CheckForUpdates(appConfig, commandRunner)

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
