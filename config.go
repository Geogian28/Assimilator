package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/caarlos0/env/v11"
	toml "github.com/pelletier/go-toml/v2"

	// Import the YAML library
	asslog "github.com/geogian28/Assimilator/assimilator_logger"
)

type AppConfig struct {
	IsServer        bool                  `toml:"is_server" env:"ASSIMILATOR_IS_SERVER"`
	IsAgent         bool                  `toml:"is_agent" env:"ASSIMILATOR_IS_AGENT"`
	GithubUsername  string                `toml:"Github_username" env:"ASSIMILATOR_GITHUB_USERNAME"`
	GithubToken     string                `toml:"Github_token" env:"ASSIMILATOR_GITHUB_TOKEN"`
	GithubRepo      string                `toml:"Github_repo" env:"ASSIMILATOR_GITHUB_REPO"`
	GithubBranch    string                `toml:"Github_branch" env:"ASSIMILATOR_GITHUB_BRANCH"`
	VerbosityLevel  int                   `toml:"verbosity_level" env:"ASSIMILATOR_VERBOSITY_LEVEL"`
	LogTypes        string                `toml:"log_types" env:"ASSIMILATOR_LOG_TYPES"`
	LogFileLocation string                `toml:"log_file_location" env:"ASSIMILATOR_LOG_FILE_LOCATION"`
	RepoDir         string                `toml:"repo_dir" env:"ASSIMILATOR_REPO_DIR"`
	ServerIP        string                `toml:"server_ip" env:"ASSIMILATOR_SERVER_IP"`
	ServerPort      int                   `toml:"server_port" env:"ASSIMILATOR_SERVER_PORT"`
	Hostname        string                `toml:"-" env:"ASSIMILATOR_HOSTNAME"`
	packageMap      map[string]PackageMap `toml:"-" yaml:"package_map"`
	CacheDir        string                //`toml:"cache_dir" env:"ASSIMILATOR_CACHE_DIR"`
	version         string                `toml:"-"`
	commit          string                `toml:"-"`
	buildDate       string                `toml:"-"`
	distro          string                `toml:"-"`
	TormonAddress   string                `toml:"tormon_address" env:"ASSIMILATOR_TORMON_ADDRESS"`
	ConfigFilename  string                `toml:"-" env:"ASSIMILATOR_CONFIG_FILENAME"`
	RunAsUser       string                `toml:"-" env:"ASSIMILATOR_RUN_AS_USER"`
	CurrentUser     string
}

var appConfig = AppConfig{
	// IsAgent:         true,
	// IsServer:        false,
	GithubUsername:  "",
	GithubToken:     "",
	GithubRepo:      "",
	VerbosityLevel:  3,
	LogTypes:        "console file",
	LogFileLocation: logFileLocation(),
	ServerIP:        "0.0.0.0",
	ServerPort:      2390,
	CacheDir:        userCacheDir(),
	CurrentUser:     runningUser(),
	RunAsUser:       runningUser(),
}

type DesiredState struct {
	Profiles map[string]ProfileConfig `yaml:"profiles"`
	Machines map[string]MachineConfig `yaml:"machines"`
}

type ProfileConfig struct {
	AppConfig AppConfig                `yaml:"app_config"`
	Packages  map[string][]PackageStep `yaml:"packages"`
}

type MachineConfig struct {
	Global          AppConfig                `yaml:"app_config"`
	AppliedConfig   string                   `yaml:"applied_config"`
	AppliedProfiles []string                 `yaml:"applied_profiles"`
	Packages        map[string][]PackageStep `yaml:"packages"`
}

type PackageStep struct {
	Checksum  string   `yaml:"checksum,omitempty"`
	Action    string   `yaml:"action"`
	Arguments []string `yaml:"arguments,omitempty"`
	RunAsUser string   `yaml:"runasuser,omitempty"`
}

type PackageMap struct {
	Packages map[string][]PackageStep `yaml:"packages"`
}

type CliFlags struct {
	Agent           bool
	Server          bool
	GithubUsername  string
	GithubToken     string
	GithubRepo      string
	GithubBranch    string
	Verbosity       int
	LogTypes        string
	LogFileLocation string
	RepoDir         string
	CacheDir        string
	ServerIP        string
	ServerPort      int
	Hostname        string
	ShowVersion     bool
	TormonAddress   string
	ConfigFilename  string
	RunAsUser       string
}

// This new struct will create the [config] table
type TomlConfigWrapper struct {
	Config AppConfig `toml:"config"`
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err == nil {
		return !info.IsDir()
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	// For other errors (e.g., permission denied), the file
	// might exist, but we can't access it.
	// You may want to handle these cases differently based on your application needs.
	return false
}

func ConfigFromFile() {
	// 1. Ensure folder exists:
	err := os.MkdirAll("/etc/assimilator", 0755)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrPermission):
			Error("Cannot make /etc/assimilator directory. Try running as root.")
			return
		default:
			asslog.Unhandled("Error creating assimilator directory: ", err)
		}
	}

	// 2. Ensure file exists
	if !fileExists("/etc/assimilator/config.toml") {
		Debug("Config file does not exist. Making one.")
		defaultConfig, err := toml.Marshal(TomlConfigWrapper{
			Config: appConfig,
		})
		if err != nil {
			Unhandled("Error marshalling default config: ", err)
		}
		err = os.WriteFile("/etc/assimilator/config.toml", []byte(defaultConfig), 0644)
		if err != nil {
			switch {
			case errors.Is(err, os.ErrPermission):
				Error("Received permission denied while creating config file. Try running as root.")
			default:
				Unhandled("Error creating config file: ", err)
			}
		}
	}

	// Load configs from /etc/assimilator
	configFile, err := os.ReadFile("/etc/assimilator/config.toml")
	if err != nil {
		Error("Failed to open config file: ", err)
		return
	}

	// 1. Initialize the wrapper WITH your existing defaults.
	//    This ensures that any field NOT in the file stays at its default value.
	wrapper := TomlConfigWrapper{
		Config: appConfig,
	}
	// 2. Unmarshal the file INTO the existing data.
	//    The unmarshaler acts as a "patch", only updating fields found in the text.
	err = toml.Unmarshal(configFile, &wrapper)
	if err != nil {
		Error("Failed to unmarshal config file: ", err)
		return
	}
	Debug("Loaded config from file.")
	if wrapper.Config.IsServer && wrapper.Config.IsAgent {
		Fatal(1, "Both 'server' and 'agent' enabled in config file. Cannot run as both agent and server.")
		return
	}
	if wrapper.Config.IsServer {
		appConfig.IsAgent = false
	}
	if wrapper.Config.IsAgent {
		appConfig.IsServer = false
	}
	appConfig = wrapper.Config
}

func ConfigFromEnv() {
	serverEnv := strings.ToLower(os.Getenv("ASSIMILATOR_IS_SERVER"))
	agentEnv := strings.ToLower(os.Getenv("ASSIMILATOR_IS_AGENT"))
	if agentEnv == "true" && serverEnv == "true" {
		Fatal(1, "Both 'server' and 'agent' enabled in environment variables. Cannot run as both agent and server.")
		return
	}
	if err := env.Parse(&appConfig); err != nil {
		Error("Failed to parse environment variables: ", err)
	}
}

func ConfigFromFlags(flags *CliFlags) {
	// Create a map to know which flags were set by the user.
	userSetFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		userSetFlags[f.Name] = true
	})

	// Now, conditionally update the config
	if flags.Server && flags.Agent {
		Fatal(1, "Cannot run as both agent and server.")
		return
	}
	if userSetFlags["server"] {
		if flags.Server {
			appConfig.IsAgent = false
		}
		appConfig.IsServer = flags.Server
	} else if userSetFlags["agent"] {
		if flags.Agent {
			appConfig.IsServer = false
		}
		appConfig.IsAgent = flags.Agent
	}
	if userSetFlags["Github_username"] {
		appConfig.GithubUsername = flags.GithubUsername
	}
	if userSetFlags["Github_token"] {
		appConfig.GithubToken = flags.GithubToken
	}
	if userSetFlags["Github_repo"] {
		appConfig.GithubRepo = flags.GithubRepo
	}
	if userSetFlags["Github_branch"] {
		appConfig.GithubBranch = flags.GithubBranch
	}
	if userSetFlags["verbosity"] {
		appConfig.VerbosityLevel = flags.Verbosity
	}
	if userSetFlags["log_types"] {
		appConfig.LogTypes = flags.LogTypes
	}
	if userSetFlags["log_file_location"] {
		appConfig.LogFileLocation = flags.LogFileLocation
	}
	if userSetFlags["repo_dir"] {
		appConfig.RepoDir = flags.RepoDir
	}
	if userSetFlags["cache_dir"] {
		appConfig.CacheDir = flags.CacheDir
	}
	if userSetFlags["server_ip"] {
		appConfig.ServerIP = flags.ServerIP
	}
	if userSetFlags["server_port"] {
		appConfig.ServerPort = flags.ServerPort
	}
	if userSetFlags["Hostname"] {
		appConfig.Hostname = flags.Hostname
	}
	if userSetFlags["tormon_address"] {
		appConfig.TormonAddress = flags.TormonAddress
	}
	if userSetFlags["config_filename"] {
		appConfig.CacheDir = flags.ConfigFilename
	}
	if userSetFlags["run_as_user"] {
		appConfig.RunAsUser = flags.RunAsUser
	}
}

func traceAppConfig() {
	Trace("agent: ", appConfig.IsAgent)
	Trace("server: ", appConfig.IsServer)
	Trace("GithubUsername: ", appConfig.GithubUsername)
	Trace("GithubToken: ", appConfig.GithubToken)
	Trace("GithubRepo: ", appConfig.GithubRepo)
	Trace("verbosity: ", appConfig.VerbosityLevel)
	Trace("logTypes: ", appConfig.LogTypes)
	Trace("logFileLocation: ", appConfig.LogFileLocation)
	Trace("repoDir: ", appConfig.RepoDir)
	Trace("ServerIP: ", appConfig.ServerIP)
	Trace("ServerPort: ", appConfig.ServerPort)
	Trace("Hostname: ", appConfig.Hostname)
	Trace("CacheDir: ", appConfig.CacheDir)
	Trace("TormonAdress: ", appConfig.TormonAddress)
	Trace("ConfigFilename: ", appConfig.ConfigFilename)
	Trace("RunAsUser: ", appConfig.RunAsUser)
}

// processFlagsAndArgs processes the command line flags and returns the
// corresponding FlagsAndArgs structure.
func SetupAppConfig(flags *CliFlags) {
	Trace("Loading config from file.")
	ConfigFromFile()
	traceAppConfig()

	Trace("Loading config from environment.")
	ConfigFromEnv()
	traceAppConfig()

	Trace("Loading config from flags.")
	ConfigFromFlags(flags)
	traceAppConfig()

	switch {
	case !appConfig.IsServer && !appConfig.IsAgent:
		Info(1, "Neither server nor agent flags provided. Assuming Agent")
		appConfig.IsAgent = true
	case appConfig.IsServer && appConfig.IsAgent:
		Fatal(1, "Both server and agent flags provided. Cannot run as both.")

	// Evaluate server flags
	case appConfig.IsServer:
		switch {
		case appConfig.GithubUsername == "":
			Fatal(1, "GitHub username not provided.")
		case appConfig.GithubRepo == "":
			Fatal(1, "GitHub repo not provided.")
		case appConfig.GithubToken == "":
			Fatal(1, "GitHub token not provided.")
		}

	// Evaluate agent flags
	case appConfig.IsAgent:
		switch {
		case appConfig.ServerIP == "":
			Fatal(1, "Server IP not provided.")
		case appConfig.ServerIP == "0.0.0.0":
			Fatal(1, "0.0.0.0 is not a valid server IP.")
		case appConfig.ServerPort <= 0 ||
			appConfig.ServerPort > 65535:
			Fatal(1, "Server port must be between 1 and 65535.")
		}
		// Evaluate misc flags
		if appConfig.Hostname == "" {
			var err error
			appConfig.Hostname, err = os.Hostname()
			if err != nil {
				Fatal(1, "Failed to get hostname from os.Hostname(): ", err)
			}
		}

	case appConfig.RepoDir == "":
		Fatal(1, "Repository directory not provided.")
	case appConfig.VerbosityLevel < 0:
		appConfig.VerbosityLevel = 0
	case appConfig.CacheDir == "":
		Fatal(1, "CacheDir is not set")
	}

	if appConfig.GithubBranch == "" {
		appConfig.GithubBranch = "main"
	}
	if appConfig.RunAsUser == "" {
		appConfig.RunAsUser = appConfig.CurrentUser
	}

	Success("Configuration loaded successfully.")
	asslog.SetVerbosity(appConfig.VerbosityLevel)
	asslog.SetLogTypes(logTypes(appConfig.LogTypes))
	asslog.SetLogFileLocation(appConfig.LogFileLocation)
}

func ParseFlags() *CliFlags {
	flags := &CliFlags{}

	flag.BoolVar(&flags.Agent, "agent", false, "Run as agent")
	flag.BoolVar(&flags.Server, "server", false, "Run as server")
	flag.StringVar(&flags.GithubUsername, "Github_username", "", "GitHub username")
	flag.StringVar(&flags.GithubToken, "Github_token", "", "GitHub access token")
	flag.StringVar(&flags.GithubRepo, "Github_repo", "", "GitHub repository")
	flag.StringVar(&flags.GithubBranch, "Github_branch", "main", "GitHub branch. Useful for dev environments. Defaults to 'main'")
	flag.IntVar(&flags.Verbosity, "verbosity", 1, "Set verbosity level (0-Silent, 1=Info, 2=Debug, 3=Trace)")
	flag.StringVar(&flags.LogTypes, "log_types", "", "Set log output locations (console, file)")
	flag.StringVar(&flags.LogFileLocation, "log_file_location", logFileLocation(), "Set log file location. Root defaults to '/var/log/assimilator.log' and non-root defaults to '~/.local/state/assimilator.log'")
	flag.StringVar(&flags.RepoDir, "repo_dir", "", "Set repository directory")
	flag.StringVar(&flags.ServerIP, "server_ip", "0.0.0.0", "Set server IP")
	flag.IntVar(&flags.ServerPort, "server_port", 2390, "Set server port")
	flag.StringVar(&flags.Hostname, "Hostname", "", "Set Hostname of the agent...")
	flag.BoolVar(&flags.ShowVersion, "version", false, "Show version information.")
	flag.StringVar(&flags.TormonAddress, "tormon_address", "", "If set, sends failures to Tormon")
	flag.StringVar(&flags.ConfigFilename, "config_filename", "", "Set the config filename. Defaults to config.yaml")

	flag.Parse() // Parse them once all are defined
	return flags
}

func logTypes(logTypesPtr string) map[string]bool {
	logTypes := strings.Split(logTypesPtr, " ")
	if logTypesPtr == "" {
		return map[string]bool{
			"console": true,
		}
	}
	logTypesMap := make(map[string]bool)
	for _, logType := range logTypes {
		if logType == "all" {
			allMap := make(map[string]bool, len(asslog.LogType))
			for logType := range asslog.LogType {
				allMap[logType] = true
			}
			// fmt.Println(allMap)
			return allMap
		}
		logType = strings.ToLower(logType)
		if _, ok := asslog.LogType[logType]; ok {
			logTypesMap[logType] = true
			continue
		}
		fmt.Println("Unknown log type: ", logType)
	}
	return logTypesMap
}

func runningUser() string {
	runningUser, err := user.Current()
	if err != nil {
		Error("Failed to get current user: ", err)
		os.Exit(1)
	}
	return runningUser.Username
}

func userCacheDir() string {
	user, err := user.Current()
	if err != nil {
		Error("Failed to get current user: ", err)
		os.Exit(1)
	}
	if user.Username == "root" {
		return "/var/cache/assimilator"
	}
	baseCacheDir, err := os.UserCacheDir()
	if err != nil {
		Error("Failed to get user cache directory: ", err)
		os.Exit(1)
	}
	return filepath.Join(baseCacheDir, "assimilator")
}

func logFileLocation() string {
	user, err := user.Current()
	if err != nil {
		Error("Failed to get current user: ", err)
		os.Exit(1)
	}
	if user.Username == "root" {
		return "/var/log/assimilator.log"
	}

	mkdir := exec.Command("mkdir", "-p", filepath.Join(user.HomeDir, ".local/state"))
	if err := mkdir.Run(); err != nil {
		Error("Failed to create log directory: ", err)
		os.Exit(1)
	}
	return filepath.Join(user.HomeDir, ".local/state/assimilator.log")
}
