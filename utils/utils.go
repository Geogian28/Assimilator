package utils

import (
	asslog "github.com/geogian28/Assimilator/assimilator_logger"
	"gopkg.in/yaml.v3"
)

// ANSI escape codes for text formatting
const (
	Overwrite = "\r"       // Carriage return to overwrite the current line
	Lgreen    = "\033[92m" // Light Green
	Lblack    = "\033[90m" // Light Black (or Dark Gray)
	Lred      = "\033[91m" // Light Red
	Reset     = "\033[0m"  // Reset to default color
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

// initialize Variables
const (
	//  test_mode bool = false
	AssimilatorDir string = "/etc/assimilator"
	AssimilatorLog string = "/var/log/assimilator.log"
	//  GITHUB_ACCESS_TOKEN string = "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"
	//  LOGROTATE_CONF string = "/etc/logrotate.d/assimilator"
	//  IS_FIRST_RUN string = "$ASSIMILATOR_DIR/assimilator_run" // Not yet implemented
)

func PrettyPrint(resp any) {
	prettyYAML, err := yaml.Marshal(resp)
	if err != nil {
		Error("Failed to generate JSON for display:", err)
		return
	}
	Info("Received configuration from server:\n", string(prettyYAML))
}
