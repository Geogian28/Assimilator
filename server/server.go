package server

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	// Import go-git
	asslog "github.com/geogian28/Assimilator/assimilator_logger"
	config "github.com/geogian28/Assimilator/config"
	"github.com/go-git/go-git/v5"                    // Import go-git
	"github.com/go-git/go-git/v5/plumbing/transport" // Import go-git
	"google.golang.org/grpc"

	// For basic HTTP auth if needed
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	pb "github.com/geogian28/Assimilator/proto"
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

var DesiredState *config.DesiredState

type AssimilatorServer struct {
	pb.UnimplementedAssimilatorServer
}

type ServerVersion struct {
	Version   string
	Commit    string
	BuildDate string
}

var ServerVersionInfo *ServerVersion

// func newServerVersion(version, commit, buildDate string) *ServerVersion {
// 	return &ServerVersion{
// 		Version:   AppConfig.Version,
// 		Commit:    commit,
// 		BuildDate: buildDate,
// 	}
// }

// Clone the dotfiles repository
func cloneRepo(appConfig *config.AppConfig, repoDir string, auth *http.BasicAuth) error {
	cloneOptions := &git.CloneOptions{
		URL:      fmt.Sprintf("https://github.com/%s/%s.git", appConfig.GithubUsername, appConfig.GithubRepo),
		Auth:     auth,
		Progress: asslog.NewLogWriter(),
	}
	Debug(cloneOptions)
	_, err := git.PlainClone(repoDir, false, cloneOptions)
	return err
}

// Pull the dotfiles repository
func pullRepo(repoDir string, auth *http.BasicAuth) error {
	// Opens a git repository from the given path. It detects if the repository is bare or a normal one.
	// If the path doesn't contain a valid repository ErrRepositoryNotExists is returned
	Trace("Opening the local repo directory")
	r, err := git.PlainOpen(repoDir)
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			// check if directory exists
			if _, err := os.Stat(repoDir); os.IsNotExist(err) {
				Fatal(1, "Unable to pull repo. Local repo directory does not exist.")
			} else {
				Fatal(1, "Unable to pull repo. Local repo directory ("+repoDir+") exists but is not a git repository.")
			}
		}
		asslog.Unhandled("error opening repo with go-git: " + err.Error())
	}
	Trace("Opened the local repo directory without errors.")

	// Now that we have the repo opened, we can get the worktree
	// A worktree is, in simple terms, the directory of actual, visible files and folders that you can see and edit on your computer.
	Trace("Getting the local repo worktree")
	w, err := r.Worktree()
	if err != nil {
		if errors.Is(err, git.ErrIsBareRepository) {
			Fatal(1, "Unable to get worktree. Local repo directory ("+repoDir+") exists but is bare. Honestly the program should never get here since it should have pulled the repo already.")
		}
		if errors.Is(err, os.ErrPermission) {
			Fatal(1, "Permission denied. The program does not have the rights to read at least one worktree file.")
		}
		asslog.Unhandled("error getting worktree with go-git: " + err.Error())
	}
	Trace("Opened the local repo directory without errors.")

	// Pull changes
	Debug("Pulling changes...")
	err = w.Pull(&git.PullOptions{
		RemoteName: "origin",
		Auth:       auth,
		Progress:   asslog.NewLogWriter(),
	})
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			Debug("No changes made. Local repository already up to date.")
		} else if errors.Is(err, transport.ErrAuthenticationRequired) {
			Fatal(1, "Unable to pull changes. Authentication required. Please check your repository name, username and PAT.")
		} else if errors.Is(err, transport.ErrRepositoryNotFound) {
			Fatal(1, "Unable to pull changes. Repository not found. Please check your repository name, username and PAT.")
		} else if errors.Is(err, git.ErrUnstagedChanges) {
			Fatal(1, "Unable to pull changes. Local repository has unstaged changes. Please commit or stash them before pulling.")
		} else {
			asslog.Unhandled("error pulling changes with go-git: " + err.Error())
		}
	} else {
		Success("Changes pulled without errors.")
	}
	return nil
}

// Clone or pull the repository
func cloneOrPullRepo(appConfig *config.AppConfig) (string, error) {
	repoDir := appConfig.RepoDir
	auth := &http.BasicAuth{ // Use BasicAuth for PAT
		Username: appConfig.GithubUsername,
		Password: appConfig.GithubToken,
	}
	Trace("appConfig.GithubUsername: ", appConfig.GithubUsername)
	Trace("appConfig.GithubToken: ", appConfig.GithubToken)

	// Create the repo temp directory
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		Debug(`Directory "` + repoDir + `") does not exist. Creating it.`)
		repoDirErr := os.Mkdir(repoDir, 0755)
		if repoDirErr != nil {
			switch repoDirErr {
			case os.ErrExist:
				Debug(fmt.Sprintf("Repository directory '%s' already exists. Proceeding.", repoDir))
			default:
				asslog.Unhandled("Error making the /tmp/assimilator-repo temp directory: ", repoDirErr)
			}
		}
	}
	// repoDirErr := os.Mkdir(repoDir, 0755)
	// if repoDirErr != nil {
	// 	if errors.Is(repoDirErr, os.ErrExist) {
	// 		Debug(fmt.Sprintf("Repository directory '%s' already exists. Proceeding.", repoDir))
	// 	} else {
	// 		asslog.Unhandled("Error making the /tmp/assimilator-repo temp directory: ", repoDirErr)
	// 	}
	// }

	// Clone or pull the repository
	Info("Cloning or pulling repository to ", repoDir)
	err := cloneRepo(appConfig, repoDir, auth)
	if err != nil {
		switch {
		case errors.Is(err, git.ErrRepositoryAlreadyExists):
			Debug("Repository already exists. Pulling...")
			pullRepo(repoDir, auth)
			return repoDir, nil
		case errors.Is(err, transport.ErrAuthenticationRequired):
			Fatal(1, "Unable to clone or pull repository. Authentication required. Please check your repository name, username and PAT.")
		default:
			asslog.Unhandled("Error cloning or pulling repository: ", err)
			return "", err
		}
	}
	return repoDir, nil
}

// Start the server
func Server(appConfig *config.AppConfig) {
	// Clone or pull the remote repository to the local one
	Info("Cloning or pulling repository...")
	repoDir, err := cloneOrPullRepo(appConfig)
	if err != nil {
		Trace("error cloning or pulling repository: ", err)
		Unhandled("error cloning or pulling repository: ", err)
	} else {
		Info("Repository cloned or pulled successfully")
	}

	// Load the desired state
	if appConfig.TestMode {
		Debug("test-mode not implemented")
	}

	DesiredState, err = config.LoadDesiredState(repoDir + "/config.yaml")
	if err != nil {
		asslog.Unhandled("unable to load desired state: ", err)
	}
	Trace("DesiredState.Machine:")
	Trace(DesiredState.Machines)

	// Start the server
	address := fmt.Sprintf("%s:%d", appConfig.ServerIP, appConfig.ServerPort)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		asslog.Unhandled("Failed to listen on address", address, ": ", err)
	}
	ServerVersionInfo = &ServerVersion{
		Version:   appConfig.Version,
		Commit:    appConfig.Commit,
		BuildDate: appConfig.BuildDate,
	}
	s := grpc.NewServer()
	pb.RegisterAssimilatorServer(s, &AssimilatorServer{})
	Info("Server listening on at ", lis.Addr())

	// Create a channel to receive OS signals
	// This is used to gracefully shutdown the server in case of SIGTERM (ctrl+c)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start gRPC server in a goroutine
	go func() {
		if err := s.Serve(lis); err != nil {
			asslog.Unhandled("Failed to serve: ", err)
		}
	}()
	// Wait for a signal
	<-sigChan
	Info("\nReceived interrup signal. Gracefully stopping gRPC server...")

	// Graceful shutdown for gRPC server
	s.GracefulStop()
	Info("gRPC server stopped.")

}
