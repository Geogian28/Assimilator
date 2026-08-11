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

// TODO
//	- allow one-offs
//		- assimilator install <package name>
//	- allow not needing to be run as root

func main() {

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
	Trace(1, "Version: ", appVersion)
	Trace(2, "Commit: ", commit)
	Trace(2, "Build Date: ", buildDate)

	if appConfig.IsServer {
		Info(1, "Running as server")
		Server()
	} else {
		Info(1, "Running as agent")
		commandRunner := LiveCommandRunner{}
		Agent(&commandRunner)
	}
}
