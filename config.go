package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/caarlos0/env/v11"
	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3" // Import the YAML library

	asslog "github.com/geogian28/Assimilator/assimilator_logger"
)

// var (
// 	appversion   string = "0.3.11"
// 	commit    string
// 	buildDate string
// )

type DesiredState struct {
	Profiles map[string]ProfileConfig `yaml:"profiles"`
	Machines map[string]MachineConfig `yaml:"machines"`
	// Users    map[string]UserConfig    `yaml:"users"`
}

type AppConfig struct {
	IsServer        bool                  `toml:"is_server" env:"ASSIMILATOR_IS_SERVER"`
	IsAgent         bool                  `toml:"is_agent" env:"ASSIMILATOR_IS_AGENT"`
	GithubUsername  string                `toml:"Github_username" env:"ASSIMILATOR_GITHUB_USERNAME"`
	GithubToken     string                `toml:"Github_token" env:"ASSIMILATOR_GITHUB_TOKEN"`
	GithubRepo      string                `toml:"Github_repo" env:"ASSIMILATOR_GITHUB_REPO"`
	GithubBranch    string                `toml:"Github_branch" env:"ASSIMILATOR_GITHUB_BRANCH"`
	testMode        bool                  `toml:"-" env:"ASSIMILATOR_TEST_MODE"`
	VerbosityLevel  int                   `toml:"verbosity_level" env:"ASSIMILATOR_VERBOSITY_LEVEL"`
	LogTypes        string                `toml:"log_types" env:"ASSIMILATOR_LOG_TYPES"`
	LogFileLocation string                `toml:"log_file_location" env:"ASSIMILATOR_LOG_FILE_LOCATION"`
	RepoDir         string                `toml:"repo_dir" env:"ASSIMILATOR_REPO_DIR"`
	ServerIP        string                `toml:"server_ip" env:"ASSIMILATOR_SERVER_IP"`
	ServerPort      int                   `toml:"server_port" env:"ASSIMILATOR_SERVER_PORT"`
	Hostname        string                `toml:"Hostname" env:"ASSIMILATOR_HOSTNAME"`
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
	IsAgent:         false,
	IsServer:        false,
	GithubUsername:  "",
	GithubToken:     "",
	GithubRepo:      "",
	GithubBranch:    "main",
	testMode:        false,
	VerbosityLevel:  4,
	LogTypes:        "console",
	LogFileLocation: logFileLocation(),
	RepoDir:         "",
	ServerIP:        "0.0.0.0",
	ServerPort:      2390,
	Hostname:        "",
	CacheDir:        userCacheDir(),
	TormonAddress:   "",
	ConfigFilename:  "",
	CurrentUser:     runningUser(),
	RunAsUser:       runningUser(),
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
	} else {
		Debug("Loaded config from file.")
		appConfig = wrapper.Config
	}
}

func ConfigFromEnv() {
	fmt.Println(appConfig.IsServer)
	if err := env.Parse(&appConfig); err != nil {
		Error("Failed to parse environment variables: ", err)
	}
	fmt.Println(appConfig.IsServer)
}

func ConfigFromFlags(flags *CliFlags) {

	// Create a map to know which flags were set by the user.
	userSetFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		userSetFlags[f.Name] = true
	})

	// Now, conditionally update the config
	if userSetFlags["server"] {
		appConfig.IsServer = flags.Server
	}
	if userSetFlags["agent"] {
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
	if userSetFlags["test_mode"] {
		appConfig.testMode = flags.testMode
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
	Trace("testMode: ", appConfig.testMode)
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
		if appConfig.RunAsUser == "" {
			appConfig.RunAsUser = appConfig.CurrentUser
		}
		if appConfig.RunAsUser != "root" && appConfig.LogFileLocation == "/var/log/assimilator.log" {
			userHomeDir, _ := os.UserHomeDir()
			appConfig.LogFileLocation = userHomeDir + "/.cache/assimilator/assimilator.log"
		}

	case appConfig.testMode && appConfig.RepoDir == "":
		Trace("Test mode enabled, but repo directory not provided")
		Trace(`Setting repodir to "/tmp/assimilator-repo"`)
		appConfig.RepoDir = "/tmp/assimilator-repo"
	case !appConfig.testMode && appConfig.RepoDir == "":
		Fatal(1, "Repository directory not provided.")
	case appConfig.VerbosityLevel < 0:
		appConfig.VerbosityLevel = 0
	case appConfig.CacheDir == "":
		Fatal(1, "CacheDir is not set")
	default:
		Success("Configuration loaded successfully.")
	}
	asslog.SetVerbosity(appConfig.VerbosityLevel)
	asslog.SetLogTypes(logTypes(appConfig.LogTypes))
	asslog.SetLogFileLocation(appConfig.LogFileLocation)
}

type CliFlags struct {
	Agent           bool
	Server          bool
	GithubUsername  string
	GithubToken     string
	GithubRepo      string
	GithubBranch    string
	testMode        bool
	Verbosity       int
	LogTypes        string
	LogFileLocation string
	RepoDir         string
	ServerIP        string
	ServerPort      int
	Hostname        string
	ShowVersion     bool
	TormonAddress   string
	ConfigFilename  string
	RunAsUser       string
}

func ParseFlags() *CliFlags {
	flags := &CliFlags{}

	flag.BoolVar(&flags.Agent, "agent", true, "Run as agent")
	flag.BoolVar(&flags.Server, "server", false, "Run as server")
	flag.StringVar(&flags.GithubUsername, "Github_username", "", "GitHub username")
	flag.StringVar(&flags.GithubToken, "Github_token", "", "GitHub access token")
	flag.StringVar(&flags.GithubRepo, "Github_repo", "", "GitHub repository")
	flag.StringVar(&flags.GithubBranch, "Github_branch", "main", "GitHub branch. Useful for dev environments. Defaults to 'main'")
	flag.BoolVar(&flags.testMode, "test_mode", false, "Used when testing, do not use in production")
	flag.IntVar(&flags.Verbosity, "verbosity", 1, "Set verbosity level (0-Silent, 1=Info, 2=Debug, 3=Trace)")
	flag.StringVar(&flags.LogTypes, "log_types", "", "Set log output locations (console, file)")
	flag.StringVar(&flags.LogFileLocation, "log_file_location", "/var/lib/assimilator/assimilator.log", "Set log file location")
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

// LoadDesiredState reads the YAML file from the given path and unmarshals it into the AppConfig struct.
func LoadDesiredState(filePath string) (*DesiredState, error) {
	Trace("Reading config file: ", filePath)
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
	Debug("Available profiles: ", strings.Join(ProfileNames, ", "))

	for machineName, machineConfig := range desiredState.Machines {
		mergedPackages := make(map[string][]PackageStep)

		for _, profileName := range machineConfig.AppliedProfiles {
			profile, ok := desiredState.Profiles[profileName]
			if !ok {
				Error("Cannot apply profile: ", profileName, " to machine: ", machineName, ": profile not found: ")
				continue
			}

			Trace(fmt.Sprintf(`Copying packages from profile "%s" to machine: %s`, profileName, machineName))
			combinePackageSteps(mergedPackages, profile.Packages)
			// maps.Copy(machineConfig.Packages, profile.Packages)
		}

		Trace(fmt.Sprintf(`Applying specific overrides for machine: %s`, machineName))
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

func runningUser() string {
	runningUser, err := user.Current()
	if err != nil {
		Error("Failed to get current user: ", err)
		os.Exit(1)
	}
	return runningUser.Username
}

func userCacheDir() string {
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

	return filepath.Join(user.HomeDir, ".local/state/assimilator.log")
}
