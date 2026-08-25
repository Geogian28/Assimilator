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
	"gopkg.in/yaml.v3"

	// Import the YAML library
	asslog "github.com/geogian28/Assimilator/assimilator_logger"
)

type AppConfig struct {
	IsServer              bool                  `toml:"is_server" env:"ASSIMILATOR_IS_SERVER"`
	IsAgent               bool                  `toml:"is_agent" env:"ASSIMILATOR_IS_AGENT"`
	GithubUsername        string                `toml:"Github_username" env:"ASSIMILATOR_GITHUB_USERNAME"`
	GithubToken           string                `toml:"Github_token" env:"ASSIMILATOR_GITHUB_TOKEN"`
	GithubRepo            string                `toml:"Github_repo" env:"ASSIMILATOR_GITHUB_REPO"`
	GithubBranch          string                `toml:"Github_branch" env:"ASSIMILATOR_GITHUB_BRANCH"`
	VerbosityLevel        int                   `toml:"verbosity_level" env:"ASSIMILATOR_VERBOSITY_LEVEL"`
	LogTypes              string                `toml:"log_types" env:"ASSIMILATOR_LOG_TYPES"`
	LogFileLocation       string                `toml:"log_file_location" env:"ASSIMILATOR_LOG_FILE_LOCATION"`
	RepoDir               string                `toml:"repo_dir" env:"ASSIMILATOR_REPO_DIR"`
	ServerIP              string                `toml:"server_ip" env:"ASSIMILATOR_SERVER_IP"`
	ServerPort            int                   `toml:"server_port" env:"ASSIMILATOR_SERVER_PORT"`
	Hostname              string                `toml:"-" env:"ASSIMILATOR_HOSTNAME"`
	packageMap            map[string]PackageMap `toml:"-" yaml:"package_map"`
	CacheDir              string                //`toml:"cache_dir" env:"ASSIMILATOR_CACHE_DIR"`
	version               string                `toml:"-"`
	commit                string                `toml:"-"`
	buildDate             string                `toml:"-"`
	distro                string                `toml:"-"`
	TormonAddress         string                `toml:"tormon_address" env:"ASSIMILATOR_TORMON_ADDRESS"`
	ConfigFilename        string                `toml:"-" env:"ASSIMILATOR_CONFIG_FILENAME"`
	RunAsUser             string                `toml:"-" env:"ASSIMILATOR_RUN_AS_USER"`
	CurrentUser           string                `toml:"-"`
	RunOnce               bool                  `toml:"-"`
	PackageUpdateInterval int64                 `toml:"package_update_interval" env:"ASSIMILATOR_PACKAGE_UPDATE_INTERVAL"`
	UpdateCheckInterval   int64                 `toml:"update_check_interval" env:"ASSIMILATOR_UPDATE_CHECK_INTERVAL"`
	TestMode              bool
}

var appConfig = AppConfig{
	IsAgent:               true,
	IsServer:              false,
	VerbosityLevel:        3,
	GithubToken:           "",
	GithubRepo:            "",
	GithubBranch:          "main",
	LogTypes:              "console file",
	LogFileLocation:       logFileLocation(),
	ServerIP:              "0.0.0.0",
	ServerPort:            2390,
	CacheDir:              userCacheDir(),
	CurrentUser:           runningUser(),
	RunAsUser:             runningUser(),
	PackageUpdateInterval: 600,
	UpdateCheckInterval:   60,
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
	Agent                 bool
	Server                bool
	GithubUsername        string
	GithubToken           string
	GithubRepo            string
	GithubBranch          string
	Verbosity             int
	LogTypes              string
	LogFileLocation       string
	RepoDir               string
	CacheDir              string
	ServerIP              string
	ServerPort            int
	Hostname              string
	ShowVersion           bool
	TormonAddress         string
	ConfigFilename        string
	RunAsUser             string
	RunOnce               bool
	PackageUpdateInterval int64
	UpdateCheckInterval   int64
	TestMode              bool
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
	return false
}

func ConfigFromFile() {
	// 1. Ensure folder exists:
	err := os.MkdirAll("/etc/assimilator", 0755)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrPermission):
			Error(1, "Cannot make /etc/assimilator directory. Try running as root.")
			return
		default:
			asslog.Unhandled("Error creating assimilator directory: ", err)
		}
	}
	// 2. Ensure file exists
	if !fileExists("/etc/assimilator/config.toml") {
		Info(1, "Config file does not exist. Making one.")
		defaultConfig, err := toml.Marshal(TomlConfigWrapper{
			Config: AppConfig{
				IsServer:              false,
				IsAgent:               true,
				VerbosityLevel:        3,
				LogTypes:              "console file",
				ServerIP:              "0.0.0.0",
				ServerPort:            2390,
				PackageUpdateInterval: 600,
				UpdateCheckInterval:   60,
			},
		})
		if err != nil {
			Unhandled("Error marshalling default config: ", err)
		}
		err = os.WriteFile("/etc/assimilator/config.toml", []byte(defaultConfig), 0644)
		if err != nil {
			switch {
			case errors.Is(err, os.ErrPermission):
				Error(1, "Received permission denied while creating config file. Try running as root.")
			default:
				Unhandled("Error creating config file: ", err)
			}
		}
	}

	// Load configs from /etc/assimilator
	configFile, err := os.ReadFile("/etc/assimilator/config.toml")
	if err != nil {
		Error(1, "Failed to open config file: ", err)
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
		Error(1, "Failed to unmarshal config file: ", err)
		return
	}
	Trace(4, "Loaded config from file.")
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
	switch {
	case agentEnv == "true" && serverEnv == "true":
		Fatal(1, "Both 'server' and 'agent' enabled in environment variables. Cannot run as both agent and server.")
		return
	case agentEnv == "true":
		appConfig.IsAgent = true
		appConfig.IsServer = false
	case serverEnv == "true":
		appConfig.IsAgent = false
		appConfig.IsServer = true
	}
	if err := env.Parse(&appConfig); err != nil {
		Error(1, "Failed to parse environment variables: ", err)
	}
}

func ParseFlags() *CliFlags {
	flags := &CliFlags{}

	flag.BoolVar(&flags.Agent, "agent", true, "Run as agent")
	flag.BoolVar(&flags.Server, "server", false, "Run as server")
	flag.StringVar(&flags.GithubUsername, "Github_username", "", "GitHub username")
	flag.StringVar(&flags.GithubToken, "Github_token", "", "GitHub access token")
	flag.StringVar(&flags.GithubRepo, "Github_repo", "", "GitHub repository")
	flag.StringVar(&flags.GithubBranch, "Github_branch", "main", "GitHub branch. Useful for dev environments. Defaults to 'main'")
	flag.IntVar(&flags.Verbosity, "verbosity", 3, "Set verbosity level (0-Silent, 1=Info, 2=Debug, 3=Trace)")
	flag.StringVar(&flags.LogTypes, "log_types", "console file", "Set log output locations (console, file)")
	flag.StringVar(&flags.LogFileLocation, "log_file_location", logFileLocation(), "Set log file location. Root defaults to '/var/log/assimilator.log' and non-root defaults to '~/.local/state/assimilator.log'")
	flag.StringVar(&flags.RepoDir, "repo_dir", "", "Set repository directory")
	flag.StringVar(&flags.ServerIP, "server_ip", "0.0.0.0", "Set server IP")
	flag.IntVar(&flags.ServerPort, "server_port", 2390, "Set server port")
	flag.StringVar(&flags.Hostname, "hostname", "", "Set Hostname of the agent. Useful if you want to get another machine config")
	flag.BoolVar(&flags.ShowVersion, "version", false, "Show version information.")
	flag.StringVar(&flags.TormonAddress, "tormon_address", "", "If set, sends failures to Tormon")
	flag.StringVar(&flags.ConfigFilename, "config_filename", "", "Set the config filename. Defaults to config.yaml")
	flag.BoolVar(&flags.RunOnce, "runonce", false, "Run assimilator once and exit")
	flag.Int64Var(&flags.PackageUpdateInterval, "package_update_interval", 600, "Set how often the package should be reapplied even if there's been no changes from the server. This is the package update interval in seconds. 0 means always update.")
	flag.Int64Var(&flags.UpdateCheckInterval, "update_check_interval", 60, "How often the update check should be performed in seconds.")
	flag.BoolVar(&flags.TestMode, "test", false, "Test mode for development purposes")

	flag.Parse() // Parse them once all are defined
	return flags
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
		appConfig.ConfigFilename = flags.ConfigFilename
	}
	if userSetFlags["run_as_user"] {
		appConfig.RunAsUser = flags.RunAsUser
	}
	if userSetFlags["runonce"] {
		appConfig.RunOnce = flags.RunOnce
	}
	if userSetFlags["package_update_interval"] {
		appConfig.PackageUpdateInterval = int64(flags.PackageUpdateInterval)
	}
	if userSetFlags["test"] {
		appConfig.TestMode = flags.TestMode
	}

}

func traceAppConfig() {
	Trace(5, "Printing AppConfig:")
	Trace(5, "- agent: ", appConfig.IsAgent)
	Trace(5, "- server: ", appConfig.IsServer)
	Trace(5, "- GithubUsername: ", appConfig.GithubUsername)
	Trace(5, "- GithubToken: ", appConfig.GithubToken)
	Trace(5, "- GithubRepo: ", appConfig.GithubRepo)
	Trace(5, "- verbosity: ", appConfig.VerbosityLevel)
	Trace(5, "- logTypes: ", appConfig.LogTypes)
	Trace(5, "- logFileLocation: ", appConfig.LogFileLocation)
	Trace(5, "- repoDir: ", appConfig.RepoDir)
	Trace(5, "- ServerIP: ", appConfig.ServerIP)
	Trace(5, "- ServerPort: ", appConfig.ServerPort)
	Trace(5, "- Hostname: ", appConfig.Hostname)
	Trace(5, "- CacheDir: ", appConfig.CacheDir)
	Trace(5, "- TormonAdress: ", appConfig.TormonAddress)
	Trace(5, "- ConfigFilename: ", appConfig.ConfigFilename)
	Trace(5, "- RunAsUser: ", appConfig.RunAsUser)
	Trace(5, "- RunOnce: ", appConfig.RunOnce)
	Trace(5, "- PackageUpdateInterval: ", appConfig.PackageUpdateInterval)
	Trace(5, "- UpdateCheckInterval: ", appConfig.UpdateCheckInterval)
}

// processFlagsAndArgs processes the command line flags and returns the
// corresponding FlagsAndArgs structure.
func SetupAppConfig(flags *CliFlags) {
	Trace(5, "Loading config from file.")
	ConfigFromFile()
	traceAppConfig()

	Trace(5, "Loading config from environment.")
	ConfigFromEnv()
	traceAppConfig()

	Trace(5, "Loading config from flags.")
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
			if appConfig.Hostname == "" {
				Fatal(1, "Got hostname successfully, but it was empty... ¯\\_(ツ)_/¯")
			}
		}

	case appConfig.RepoDir == "":
		Fatal(1, "Repository directory not provided.")
	case appConfig.VerbosityLevel < 0:
		appConfig.VerbosityLevel = 0
	case appConfig.CacheDir == "":
		appConfig.CacheDir = userCacheDir()
	}

	if appConfig.Hostname == "" {
		Trace(5, "Hostname is blank. Attempting to get hostname from os.Hostname().")
		var err error
		appConfig.Hostname, err = os.Hostname()
		if err != nil {
			Fatal(1, "Failed to get hostname from os.Hostname(): ", err)
		}
		if appConfig.Hostname == "" {
			Fatal(1, "Got hostname successfully, but it was empty... ¯\\_(ツ)_/¯")
		} else {
			Trace(5, "Hostname finally set to: ", appConfig.Hostname)
		}
	}

	if appConfig.LogFileLocation == "/var/log/assimilator/assimilator.log" && appConfig.RunAsUser == "root" {
		appConfig.LogFileLocation = logFileLocation()
	}

	if appConfig.CacheDir == "" {
		appConfig.CacheDir = userCacheDir()
	}
	if appConfig.GithubBranch == "" {
		appConfig.GithubBranch = "main"
	}
	if appConfig.RunAsUser == "" {
		appConfig.RunAsUser = appConfig.CurrentUser
	}

	Success(1, "Configuration loaded successfully.")
	asslog.SetVerbosity(appConfig.VerbosityLevel)
	asslog.SetLogTypes(logTypes(appConfig.LogTypes))
	asslog.SetLogFileLocation(appConfig.LogFileLocation)
	traceAppConfig()
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
		Error(1, "Failed to get current user: ", err)
		os.Exit(1)
	}
	return runningUser.Username
}

func userCacheDir() string {
	user, err := user.Current()
	if err != nil {
		Error(1, "Failed to get current user: ", err)
		os.Exit(1)
	}
	if user.Username == "root" {
		return "/var/cache/assimilator"
	}
	baseCacheDir, err := os.UserCacheDir()
	if err != nil {
		Error(1, "Failed to get user cache directory: ", err)
		os.Exit(1)
	}
	return filepath.Join(baseCacheDir, "assimilator")
}

func logFileLocation() string {
	user, err := user.Current()
	if err != nil {
		Error(1, "Failed to get current user: ", err)
		os.Exit(1)
	}
	if user.Username == "root" {
		return "/var/log/assimilator.log"
	}

	mkdir := exec.Command("mkdir", "-p", filepath.Join(user.HomeDir, ".local/state"))
	if err := mkdir.Run(); err != nil {
		Error(1, "Failed to create log directory: ", err)
		os.Exit(1)
	}
	return filepath.Join(user.HomeDir, ".local/state/assimilator.log")
}

// LoadDesiredState reads the YAML file from the given path and unmarshals it into the AppConfig struct.
func LoadDesiredState(filePath string) (*DesiredState, error) {
	Trace(5, "Reading config file: ", filePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", filePath, err)
	}
	var desiredState DesiredState
	err = yaml.Unmarshal(data, &desiredState)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from '%s': %w", filePath, err)
	}

	// Apply profiles to machines and users
	applyProfiles(&desiredState)
	return &desiredState, nil
}

func applyProfiles(desiredState *DesiredState) {
	var ProfileNames []string
	for profileName := range desiredState.Profiles {
		ProfileNames = append(ProfileNames, profileName)
	}
	Trace(4, "Available profiles: ", strings.Join(ProfileNames, ", "))

	for machineName, machineConfig := range desiredState.Machines {
		mergedPackages := make(map[string][]PackageStep)

		for _, profileName := range machineConfig.AppliedProfiles {
			profile, ok := desiredState.Profiles[profileName]
			if !ok {
				Error(1, "Cannot apply profile: ", profileName, " to machine: ", machineName, ": profile not found: ")
				continue
			}

			Trace(5, fmt.Sprintf(`Copying packages from profile "%s" to machine: %s`, profileName, machineName))
			combinePackageSteps(mergedPackages, profile.Packages)
			// maps.Copy(machineConfig.Packages, profile.Packages)
		}

		Trace(5, fmt.Sprintf(`Applying specific overrides for machine: %s`, machineName))
		combinePackageSteps(mergedPackages, machineConfig.Packages)
		verifyPackages(mergedPackages)

		machineConfig.Packages = mergedPackages
		desiredState.Machines[machineName] = machineConfig
	}
}

func combinePackageSteps(target, source map[string][]PackageStep) {
	for pkgName, pkgSteps := range source {
		target[pkgName] = append(target[pkgName], pkgSteps...)
	}
}

func verifyPackages(packages map[string][]PackageStep) {
	for pkgName, pkgSteps := range packages {
		for i, pkgStep := range pkgSteps {
			if pkgStep.RunAsUser == "" {
				packages[pkgName][i].RunAsUser = "root"
			}
		}
	}
}
