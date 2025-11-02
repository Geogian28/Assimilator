package config

import (
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3" // Import the YAML library

	asslog "github.com/geogian28/Assimilator/assimilator_logger"
)

const (
	VERSION = "0.0.1"
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

type DesiredState struct {
	Global   AppConfig                `yaml:"global"`
	Profiles map[string]ConfigProfile `yaml:"profiles"`
	Machines map[string]MachineConfig `yaml:"machines"`
	Users    map[string]UserConfig    `yaml:"users"`
}

type AppConfig struct {
	IsServer        bool                  `toml:"is_server" env:"ASSIMILATOR_IS_SERVER"`
	IsAgent         bool                  `toml:"is_agent" env:"ASSIMILATOR_IS_AGENT"`
	MAAS            bool                  `toml:"maas" env:"ASSIMILATOR_MAAS"`
	GithubUsername  string                `toml:"github_username" env:"ASSIMILATOR_GITHUB_USERNAME"`
	GithubToken     string                `toml:"github_token" env:"ASSIMILATOR_GITHUB_TOKEN"`
	GithubRepo      string                `toml:"github_repo" env:"ASSIMILATOR_GITHUB_REPO"`
	TestMode        bool                  `toml:"test_mode" env:"ASSIMILATOR_TEST_MODE"`
	VerbosityLevel  int                   `toml:"verbosity_level" env:"ASSIMILATOR_VERBOSITY_LEVEL"`
	LogTypes        string                `toml:"log_types" env:"ASSIMILATOR_LOG_TYPES"`
	LogFileLocation string                `toml:"log_file_location" env:"ASSIMILATOR_LOG_FILE_LOCATION"`
	RepoDir         string                `toml:"repo_dir" env:"ASSIMILATOR_REPO_DIR"`
	ServerIP        string                `toml:"server_ip" env:"ASSIMILATOR_SERVER_IP"`
	ServerPort      int                   `toml:"server_port" env:"ASSIMILATOR_SERVER_PORT"`
	Hostname        string                `toml:"hostname" env:"ASSIMILATOR_HOSTNAME"`
	AptSources      string                `toml:"apt_sources" env:"ASSIMILATOR_APT_SOURCES"`
	PackageMap      map[string]PackageMap `yaml:"package_map"`
	Version         string
	Commit          string
	BuildDate       string
}

// Top-level config structure for the entire desired state
type ConfigProfile struct {
	Machines map[string]MachineConfig `yaml:"machines"`
	Users    map[string]UserConfig    `yaml:"users"`
	Packages map[string]PackageConfig `yaml:"packages"`
	Services map[string]ServiceConfig `yaml:"services"`
	Dotfiles map[string]Dotfiles      `yaml:"dotfiles"`
}

type MachineConfig struct {
	AppliedProfiles []string                 `yaml:"applied_profiles"`
	Packages        map[string]PackageConfig `yaml:"packages"`
	Services        map[string]ServiceConfig `yaml:"services"`
}

type UserConfig struct {
	AppliedProfiles []string            `yaml:"applied_profiles"`
	Dotfiles        map[string]Dotfiles `yaml:"dotfiles"`
}

type Dotfiles struct {
	DotfileLocation string       `yaml:"location"`
	Requires        Dependencies `yaml:"requires,omitempty"`
}

type Dependencies struct {
	Packages map[string]PackageConfig `yaml:"packages"`
	Files    map[string]ServiceConfig `yaml:"files,omitempty"`
}

type PackageConfig struct {
	State    string                  `yaml:"state"`
	Version  string                  `yaml:"version,omitempty"` // "omitempty" is good practice
	Branch   string                  `yaml:"branch,omitempty"`
	Requires map[string]Dependencies `yaml:"requires,omitempty"`
}

type PackageMap struct {
	Packages map[string]PackageConfig `yaml:"packages"`
}

type ServiceConfig struct {
	State   bool              `yaml:"enable"`
	Configs map[string]string `yaml:"config"`
}

type State string

const (
	StatePresent State = "present"
	StateAbsent  State = "absent"
)

// This new struct will create the [config] table
type TomlConfigWrapper struct {
	Config AppConfig `toml:"config"`
}

func (s *State) UnmarshalYAML(unmarshal func(any) error) error {
	var tempStr string
	// Unmarshall the YAML value into a temporary string variable.
	if err := unmarshal(&tempStr); err != nil {
		return err
	}

	// Create a new DesiredState from the string.
	state := State(tempStr)

	// Check if the value is one of our defined constants.
	switch state {
	case StatePresent, StateAbsent:
		// If it's valid, update the poinwer receiver
		*s = state
		return nil
	default:
		// If it's not a valid state, return an error.
		return fmt.Errorf("invalid package state: %q, must be one of [%q, %q]",
			tempStr, StatePresent, StateAbsent)
	}
}

func ConfigFromFile(appConfig *AppConfig) {
	// Load configs from /etc/assimilator
	configFile, err := os.ReadFile("/etc/assimilator/config.toml")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			Debug("Config file does not exist. Making one.")
			defaultConfig, err := toml.Marshal(TomlConfigWrapper{
				Config: *appConfig,
			})
			if err != nil {
				asslog.Unhandled("Error marshalling default config: ", err)
			}
			err = os.Mkdir("/etc/assimilator", 0755)
			if err != nil {
				switch {
				case errors.Is(err, os.ErrExist):
					Trace("Cannot make /etc/assimilator directory. It already exists.")
				case errors.Is(err, os.ErrPermission):
					Error("Cannot make /etc/assimilator directory. Try running as root.")
				default:
					asslog.Unhandled("Error creating assimilator directory: ", err)
				}
			}
			err = os.WriteFile("/etc/assimilator/config.toml", []byte(defaultConfig), 0644)
			if err != nil {
				switch {
				case errors.Is(err, os.ErrExist):
					Trace("Cannot make /etc/assimilator directory. It already exists.")
				case errors.Is(err, os.ErrPermission):
					Error("Received permission denied while creating config file. Try running as root.")
				default:
					asslog.Unhandled("Error creating config file: ", err)
				}
			}
			err = toml.Unmarshal(defaultConfig, &appConfig)
			if err != nil {
				switch err {
				case os.ErrPermission:
					Error("Cannot make config file /etc/assimilator/config.toml. Try running as root.")
				default:
					Error("Failed to open newly created config file: ", err)
				}
			}
		} else {
			Error("Failed to open config file: ", err)
		}
		return
	}

	err = toml.Unmarshal(configFile, &appConfig)
	if err != nil {
		Error("Failed to unmarshal config file: ", err)
	} else {
		Debug("Loaded config from file.")
	}

}

func ConfigFromEnv(appConfig *AppConfig) {
	if err := env.Parse(appConfig); err != nil {
		Error("Failed to parse environment variables: ", err)
	}
}

func ConfigFromFlags(appConfig *AppConfig, flags *CliFlags) {

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
	if userSetFlags["github_username"] {
		appConfig.GithubUsername = flags.GithubUsername
	}
	if userSetFlags["github_token"] {
		appConfig.GithubToken = flags.GithubToken
	}
	if userSetFlags["github_repo"] {
		appConfig.GithubRepo = flags.GithubRepo
	}
	if userSetFlags["maas"] {
		appConfig.MAAS = flags.Maas
	}
	if userSetFlags["test_mode"] {
		appConfig.TestMode = flags.TestMode
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
	if userSetFlags["hostname"] {
		appConfig.Hostname = flags.Hostname
	}
	if userSetFlags["apt_sources"] {
		appConfig.AptSources = flags.AptSources
	}
}

func traceAppConfig(appConfig *AppConfig) {
	Trace("agent: ", appConfig.IsAgent)
	Trace("server: ", appConfig.IsServer)
	Trace("githubUsername: ", appConfig.GithubUsername)
	Trace("githubToken: ", appConfig.GithubToken)
	Trace("githubRepo: ", appConfig.GithubRepo)
	Trace("maas: ", appConfig.MAAS)
	Trace("testMode: ", appConfig.TestMode)
	Trace("verbosity: ", appConfig.VerbosityLevel)
	Trace("logTypes: ", appConfig.LogTypes)
	Trace("logFileLocation: ", appConfig.LogFileLocation)
	Trace("repoDir: ", appConfig.RepoDir)
	Trace("serverIP: ", appConfig.ServerIP)
	Trace("serverPort: ", appConfig.ServerPort)
	Trace("hostname: ", appConfig.Hostname)
	Trace("aptSources: ", appConfig.AptSources)
}

// processFlagsAndArgs processes the command line flags and returns the
// corresponding FlagsAndArgs structure.
func SetupAppConfig(version, commit, buildDate string, flags *CliFlags) AppConfig {
	appConfig := AppConfig{
		IsAgent:         false,
		IsServer:        false,
		GithubUsername:  "",
		GithubToken:     "",
		GithubRepo:      "",
		MAAS:            false,
		TestMode:        false,
		VerbosityLevel:  1,
		LogTypes:        "console",
		LogFileLocation: "/etc/assimilator/assimilator.log",
		RepoDir:         "",
		ServerIP:        "0.0.0.0",
		ServerPort:      2390,
		Hostname:        "",
		AptSources:      "",
		Version:         version,
		Commit:          commit,
		BuildDate:       buildDate,
	}

	Trace("Loading config from file.")
	ConfigFromFile(&appConfig)
	traceAppConfig(&appConfig)

	Trace("Loading config from environment.")
	ConfigFromEnv(&appConfig)
	traceAppConfig(&appConfig)

	Trace("Loading config from flags.")
	ConfigFromFlags(&appConfig, flags)
	traceAppConfig(&appConfig)

	switch {
	case !appConfig.IsServer && !appConfig.IsAgent:
		Fatal(1, "Neither server nor agent flags provided.")
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
	case appConfig.TestMode && appConfig.RepoDir == "":
		Trace("Test mode enabled, but repo directory not provided")
		Trace(`Setting repodir to "/tmp/assimilator-repo"`)
		appConfig.RepoDir = "/tmp/assimilator-repo"
	case !appConfig.TestMode && appConfig.RepoDir == "":
		Fatal(1, "Repository directory not provided.")
	case appConfig.VerbosityLevel < 0:
		appConfig.VerbosityLevel = 0
	default:
		Success("Configuration loaded successfully.")
	}
	asslog.SetVerbosity(appConfig.VerbosityLevel)
	asslog.SetLogTypes(logTypes(appConfig.LogTypes))
	return appConfig
}

type CliFlags struct {
	Agent           bool
	Server          bool
	GithubUsername  string
	GithubToken     string
	GithubRepo      string
	Maas            bool
	TestMode        bool
	Verbosity       int
	LogTypes        string
	LogFileLocation string
	RepoDir         string
	ServerIP        string
	ServerPort      int
	Hostname        string
	AptSources      string
	ShowVersion     bool
}

func ParseFlags() *CliFlags {
	flags := &CliFlags{}

	flag.BoolVar(&flags.Agent, "agent", false, "Run as agent")
	flag.BoolVar(&flags.Server, "server", false, "Run as server")
	flag.StringVar(&flags.GithubUsername, "github_username", "", "GitHub username")
	flag.StringVar(&flags.GithubToken, "github_token", "", "GitHub access token")
	flag.StringVar(&flags.GithubRepo, "github_repo", "", "GitHub repository")
	flag.BoolVar(&flags.Maas, "maas", false, "Only MAAS should use this flag")
	flag.BoolVar(&flags.TestMode, "test_mode", false, "Used when testing, do not use in production")
	flag.IntVar(&flags.Verbosity, "verbosity", 1, "Set verbosity level (0-Silent, 1=Info, 2=Debug, 3=Trace)")
	flag.StringVar(&flags.LogTypes, "log_types", "", "Set log output locations (console, file)")
	flag.StringVar(&flags.LogFileLocation, "log_file_location", "/var/lib/assimilator/assimilator.log", "Set log file location")
	flag.StringVar(&flags.RepoDir, "repo_dir", "", "Set repository directory")
	flag.StringVar(&flags.ServerIP, "server_ip", "0.0.0.0", "Set server IP")
	flag.IntVar(&flags.ServerPort, "server_port", 2390, "Set server port")
	flag.StringVar(&flags.Hostname, "hostname", "", "Set hostname of the agent...")
	flag.StringVar(&flags.AptSources, "apt_sources", "", "Set custom apt sources...")
	flag.BoolVar(&flags.ShowVersion, "version", false, "Show version information.")

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
	allProfileNames := strings.Join(ProfileNames, ", ")
	Debug("Available profiles: ", allProfileNames)

	// Take "applied_profiles" from machines and apply the actual profiles to machines
	for machineName, machineData := range desiredState.Machines {
		modifiedMachine := machineData
		for _, profileName := range machineData.AppliedProfiles {
			profile, ok := desiredState.Profiles[profileName]

			// Check if the profile exists. If not, skip
			if !ok {
				Error("Profile not found: ", profileName)
				continue
			}

			// Check if the profile has any packages. If not, skip
			if len(profile.Packages) > 0 {
				if modifiedMachine.Packages == nil {
					modifiedMachine.Packages = make(map[string]PackageConfig)
				}
				Trace(`Copying packages from profile "`, profileName, `" to machine: `, machineName)
				maps.Copy(modifiedMachine.Packages, profile.Packages)
			}
			// Check if the profile has any services. If not, skip
			if len(profile.Services) > 0 {
				if modifiedMachine.Services == nil {
					modifiedMachine.Services = make(map[string]ServiceConfig)
				}
				Trace(`Copying services from profile "`, profileName, `" to machine: `, machineName)
				maps.Copy(modifiedMachine.Services, profile.Services)
			}
		}
		desiredState.Machines[machineName] = modifiedMachine
	}

	// Take "applied_profiles" from users and apply the actual profiles to users
	for userName, userData := range desiredState.Users {
		modifiedUser := userData
		for _, profileName := range modifiedUser.AppliedProfiles {
			// Check if the profile exists. If not, skip
			profile, ok := desiredState.Profiles[profileName]
			if !ok {
				Error("Profile not found: ", profileName)
				continue
			}
			if len(profile.Dotfiles) > 0 {
				Trace(`Copying dotfiles from profile "`, profileName, `" to machine: `, userName)
				if modifiedUser.Dotfiles == nil {
					maps.Copy(modifiedUser.Dotfiles, profile.Dotfiles)
				}
			}
		}
		desiredState.Users[userName] = modifiedUser
	}
}
