package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	asslog "github.com/geogian28/Assimilator/assimilator_logger"
	config "github.com/geogian28/Assimilator/config"
	pb "github.com/geogian28/Assimilator/proto"
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
	return nil
}

func Agent(appConfig *config.AppConfig, commandRunner CommandRunner) {
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
