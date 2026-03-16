package main

import (
	"fmt"
	"os"

	asslog "github.com/geogian28/Assimilator/assimilator_logger"
)

var (
	appVersion = "0.3.9"
	commit     = "none"
	buildDate  = "unknown"
)

var (
	Info      = asslog.Info
	Debug     = asslog.Debug
	Trace     = asslog.Trace
	Success   = asslog.Success
	Warning   = asslog.Warning
	Error     = asslog.Error
	Fatal     = asslog.Fatal
	Unhandled = asslog.Unhandled
)

func main() {
	asslog.StartLogger()
	defer asslog.Close()

	flags := ParseFlags()
	if flags.ShowVersion {
		fmt.Println("Version: ", appVersion)
		fmt.Println("Commit: ", commit)
		fmt.Println("Build Date: ", buildDate)
		os.Exit(0)
	}
	SetupAppConfig(flags)
	appConfig.version = appVersion
	appConfig.commit = commit
	appConfig.buildDate = buildDate
	Trace("Version: ", appVersion)
	Trace("Commit: ", commit)
	Trace("Build Date: ", buildDate)

	if appConfig.IsServer {
		asslog.Info("Running as server")
		Server()
	} else {
		asslog.Info("Running as agent")
		commandRunner := LiveCommandRunner{}
		Agent(&commandRunner)
	}
}
