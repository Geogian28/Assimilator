package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"syscall"

	agent "github.com/geogian28/Assimilator/agent"
	asslog "github.com/geogian28/Assimilator/assimilator_logger"
	config "github.com/geogian28/Assimilator/config"
	server "github.com/geogian28/Assimilator/server"
)

var (
	VERSION = "development"
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

func isRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		asslog.Unhandled("Unable to get current user: ", err)
	}
	Debug("Current username: ", currentUser.Username)
	Debug("Current UID: ", currentUser.Uid)
	if currentUser.Uid == "0" {
		Debug("Running with sudo privileges.")
		return true
	}
	Debug("Running without sudo privileges.")
	return false
}

func elevatePrivileges() int {
	// Not running as root. Attempting escalation
	asslog.Info("Not running with root privileges. Attempting to escalate...")

	// 1. Check if 'sudo' command is available
	_, err := exec.LookPath("sudo") // LookPath checks if a command exists in PATH
	if err != nil {
		asslog.Unhandled("Error: This program requires root privileges, but 'sudo' command is not available.")
	}

	// 2. Get the current executable's path
	executable, err := os.Executable()
	if err != nil {
		asslog.Unhandled("Unable to get current executable path: ", err)
	}

	// 3. Prepare the sudo command to re-execute this program
	//    "$0" "$@" in bash corresponds to os.Args[0] and os.Args[1:] in Go.
	args := []string{executable}        // First argument is the executable itself
	args = append(args, os.Args[1:]...) // Append all original command-line arguments
	cmd := exec.Command("sudo", args...)

	// 4. Connect standard I/O to the new sudo process
	//    This makes 'sudo' prompt for password in the current terminal,
	//    and any output from the re-executed program goes to the current terminal.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 5. Run the sudo command
	err = cmd.Run() // cmd.Run() waits for the command to finish
	// 6. Handle the exit status of the sudo'd process
	if err != nil {
		// exec.ExitError is return if the command exited with a non-zero status
		if exitError, ok := err.(*exec.ExitError); ok {
			// Get the exit code
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				//asslog.Info(1, "[isRoot] Escalated process exited with status code: ", status.ExitStatus())
				os.Exit(status.ExitStatus())
			}
		}
		// If it's another kind of error (e.g., command not found, permission denied for sudo itself)
		asslog.Unhandled(fmt.Sprintf("Error during sudo re-execution: %v", err))
	}
	return 0
}

func checkForUpdates() {
	asslog.Info("Not yet implemented")
}

func main() {
	fmt.Println("===  Starting!  ===")
	asslog.StartLogger()
	defer asslog.Close()
	appConfig := config.ParseFlagsAndArgs()
	asslog.SetVerbosity(appConfig.VerbosityLevel)
	asslog.SetLogTypes(appConfig.LogTypes)
	if appConfig.TestMode {
		asslog.Info("Running in test mode. Not running as root.")
	} else if !isRoot() {
		elevatePrivileges()
		os.Exit(0)
	}

	// Checking for updates
	checkForUpdates()

	if appConfig.IsServer {
		asslog.Info("Running as server")
		server.Server(&appConfig)
	}

	if appConfig.IsAgent {
		asslog.Info("Running as agent")
		commandRunner := agent.LiveCommandRunner{}
		agent.Agent(&appConfig, &commandRunner)
	}
}
