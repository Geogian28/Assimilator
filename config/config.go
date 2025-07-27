package config

import (
	"flag"
	"fmt"
	"maps"
	"os"
	"strings"

	"gopkg.in/yaml.v3" // Import the YAML library

	asslog "github.com/geogian28/Assimilator/assimilator_logger"
)

const (
	VERSION = "0.1.0"
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

type DesiredState struct {
	Global   AppConfig                `yaml:"global"`
	Profiles map[string]ConfigProfile `yaml:"profiles"`
	Machines map[string]MachineConfig `yaml:"machines"`
	Users    map[string]UserConfig    `yaml:"users"`
}

type AppConfig struct {
	IsServer       bool `yaml:"is_server"`
	IsAgent        bool `yaml:"is_agent"`
	MAAS           bool `yaml:"maas"`
	GithubUsername string
	GithubToken    string
	GithubRepo     string
	TestMode       bool `yaml:"test_mode"`
	VerbosityLevel int
	LogTypes       map[string]bool
	PackageMap     map[string]PackageMap `yaml:"package_map"`
	RepoDir        string
	ServerIP       string
	ServerPort     int
	Hostname       string
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

func (s *State) UnmarshalYAML(unmarshal func(any) error) error {
	var tempStr string
	// Unmarshall the YAML value into a temporary string variable.
	if err := unmarshal(&s); err != nil {
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

// processFlagsAndArgs processes the command line flags and returns the
// corresponding FlagsAndArgs structure.
func ParseFlagsAndArgs() AppConfig {
	serverPtr := flag.Bool("server", false, "Run as server")
	agentPtr := flag.Bool("agent", false, "Run as agent")
	githubUsernamePtr := flag.String("github_username", "", "GitHub username")
	githubTokenPtr := flag.String("github_token", "", "GitHub access token")
	githubRepoPtr := flag.String("github_repo", "", "GitHub repository")
	maasPtr := flag.Bool("maas", false, "Only MAAS should use this flag")
	testModePtr := flag.Bool("test_mode", false, "Used when testing, do not use in production")
	verbosityPtr := flag.Int("verbosity", 1, "Set verbosity level (0=Silent, 1=Info, 2=Debug, 3=Trace)")
	logTypesPtr := flag.String("log_types", "", "Set log output locations (console, file)")
	repoDirPtr := flag.String("repo_dir", "", "Set repository directory")
	serverIPPtr := flag.String("server_ip", "0.0.0.0", "Set server IP")
	serverPortPtr := flag.Int("server_port", 2390, "Set server port")
	hostnamePTR := flag.String("hostname", "", "Set hostname")

	flag.Parse() // Parse them once all are defined

	Trace("serverPtr: ", *serverPtr)
	Trace("agentPtr: ", *agentPtr)
	Trace("githubUsernamePtr: ", *githubUsernamePtr)
	Trace("githubTokenPtr: ", *githubTokenPtr)
	Trace("githubRepoPtr: ", *githubRepoPtr)
	Trace("maasPtr: ", *maasPtr)
	Trace("testModePtr: ", *testModePtr)
	Trace("verbosityPtr: ", *verbosityPtr)
	Trace("logTypesPtr: ", *logTypesPtr)
	Trace("repoDirPtr: ", *repoDirPtr)
	Trace("serverIPPtr: ", *serverIPPtr)
	Trace("serverPortPtr: ", *serverPortPtr)
	Trace("hostnamePTR: ", *hostnamePTR)

	if *verbosityPtr < 0 {
		*verbosityPtr = 0
	}
	if !*serverPtr && !*agentPtr {
		Fatal(1, "No flags provided.")
	}
	if *serverPtr && *agentPtr {
		Fatal(1, "Both server and agent flags provided. Cannot run as both.")
	}
	if *serverPtr && *githubUsernamePtr == "" {
		Fatal(1, "GitHub username not provided.")
	}
	if *serverPtr && *githubRepoPtr == "" {
		Fatal(1, "GitHub repo not provided.")
	}
	if *serverPtr && *githubTokenPtr == "" {
		Fatal(1, "GitHub token not provided.")
	}
	if *testModePtr && *repoDirPtr == "" {
		Trace("Test mode enabled, but repo directory not provided")
		Trace(`Setting repodir to "/tmp/assimilator-repo"`)
		*repoDirPtr = "/tmp/assimilator-repo"
	} else if *repoDirPtr == "" {
		Fatal(1, "Repository directory not provided.")
	}

	return AppConfig{
		IsServer:       *serverPtr,
		IsAgent:        *agentPtr,
		GithubUsername: *githubUsernamePtr,
		GithubToken:    *githubTokenPtr,
		GithubRepo:     *githubRepoPtr,
		MAAS:           *maasPtr,
		TestMode:       *testModePtr,
		VerbosityLevel: *verbosityPtr,
		LogTypes:       logTypes(logTypesPtr),
		RepoDir:        *repoDirPtr,
		ServerIP:       *serverIPPtr,
		ServerPort:     *serverPortPtr,
	}
}

func logTypes(logTypesPtr *string) map[string]bool {
	logTypes := strings.Split(*logTypesPtr, " ")
	if *logTypesPtr == "" {
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
			fmt.Println(allMap)
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
	desiredState = applyProfiles(&desiredState)
	return &desiredState, nil
}

func applyProfiles(desiredState *DesiredState) DesiredState {
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
	return *desiredState
}
