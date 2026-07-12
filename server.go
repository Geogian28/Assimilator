package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Import go-git
	asslog "github.com/geogian28/Assimilator/assimilator_logger"
	"github.com/go-git/go-git/v5" // Import go-git
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport" // Import go-git
	"github.com/go-git/go-git/v5/storage/memory"
	"google.golang.org/grpc"

	// For basic HTTP auth if needed
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	pb "github.com/geogian28/Assimilator/proto"
)

type AssimilatorServer struct {
	pb.UnimplementedAssimilatorServer
	ServerVersion
	PackageDir   string
	desiredState *DesiredState
}

type ServerVersion struct {
	Version   string
	Commit    string
	BuildDate string
}

type PackageDetails struct {
	Name     string
	FilePath string
	FileSize int64
}

// Clone the dotfiles repository
func cloneRepo(repoDir string, auth *http.BasicAuth) error {
	targetBranch := appConfig.GithubBranch
	if targetBranch == "" {
		targetBranch = "main"
	}

	cloneOptions := &git.CloneOptions{
		URL:           fmt.Sprintf("https://Github.com/%s/%s.git", appConfig.GithubUsername, appConfig.GithubRepo),
		Auth:          auth,
		Progress:      asslog.NewLogWriter(),
		SingleBranch:  true,
		ReferenceName: plumbing.NewBranchReferenceName(targetBranch),
		Depth:         1,
	}
	// Debug(cloneOptions)
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
func cloneOrPullRepo() (string, error) {
	Info("Cloning or pulling repository...")
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

	// Clone or pull the repository
	Info("Cloning or pulling repository to ", repoDir)
	err := cloneRepo(repoDir, auth)
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
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		log.Fatalf("Failed to read directory: %v", err)
	}

	Info("Listing contents of '%s':\n", repoDir)
	Info("---------------------------------")

	// Loop through and print the names
	for _, entry := range entries {
		// You can check if it's a directory or a regular file
		suffix := ""
		if entry.IsDir() {
			suffix = "/"
		}

		Info("%s%s\n", entry.Name(), suffix)
	}

	filePath := repoDir + "/config.yaml"
	Info("Reading config file: ", filePath)

	// Read the entire file into memory
	content, err := os.ReadFile(filePath)
	if err != nil {
		Error("Error reading file: %v", err)
	}

	// os.ReadFile returns []byte, so we convert it to a string to print it
	Info(string(content))

	return repoDir, nil
}

func isUpdateAvailable(repoDir string) (bool, error) {
	Info("Checking for updates...")
	// 1. Open your local repo and get it's HEAD has to compare
	r, err := git.PlainOpen(repoDir)
	if err != nil {
		return false, fmt.Errorf("error opening local repo: %s", err)
	}
	localHeadRef, err := r.Head()
	if err != nil {
		return false, fmt.Errorf("error getting local repo HEAD: %s", err)
	}
	localHash := localHeadRef.Hash().String()

	// 2. Setup Authentication
	auth := &http.BasicAuth{ // Use BasicAuth for PAT
		Username: appConfig.GithubUsername,
		Password: appConfig.GithubToken,
	}

	// 3. Create an ephemeral remote (no local disk cloning required)
	remoteRepo := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{fmt.Sprintf("https://Github.com/%s/%s.git", appConfig.GithubUsername, appConfig.GithubRepo)},
	})

	// 4. List remote references (this hits the network but doesn't download files))
	refs, err := remoteRepo.List(&git.ListOptions{
		Auth: auth,
	})
	if err != nil {
		return false, fmt.Errorf("error listing remote refs: %s", err)
	}

	// 5. Find the remote HEAD hash
	Info("Remote repository refs:")
	var remoteHash string
	for _, ref := range refs {
		if ref.Name().String() == "refs/heads/"+appConfig.GithubBranch {
			remoteHash = ref.Hash().String()
			break
		}
	}

	// 6. Compare the hashes
	Trace("Local repository HEAD hash: ", localHash)
	Trace("Remote repository HEAD hash: ", remoteHash)
	if remoteHash != localHash {
		Trace("Update available!")
		return true, nil
	}
	return false, nil
}

// Start the server
func Server() {
	// Clone or pull the remote repository to the local one
	repoDir, err := cloneOrPullRepo()
	if err != nil {
		Trace("error cloning or pulling repository: ", err)
		Unhandled("error cloning or pulling repository: ", err)
	} else {
		Info("Repository cloned or pulled successfully")
	}

	// Load the desired state
	desiredState, err := LoadDesiredState(repoDir + "/config.yaml")
	if err != nil {
		asslog.Unhandled("unable to load desired state: ", err)
	}

	// Make packages for machine and sync them with the desired state
	makePackages()
	syncChecksums(desiredState)

	// Collect checksums (potentially unneeded)
	// collectChecksums(repoDir)
	// if err != nil {
	// 	asslog.Unhandled("error collecting checksums: ", err)
	// }

	// Start the server
	address := fmt.Sprintf("%s:%d", appConfig.ServerIP, appConfig.ServerPort)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		asslog.Unhandled("Failed to listen on address", address, ": ", err)
	}
	s := grpc.NewServer()
	pb.RegisterAssimilatorServer(s, &AssimilatorServer{
		ServerVersion: ServerVersion{
			Version:   appConfig.version,
			Commit:    appConfig.commit,
			BuildDate: appConfig.buildDate,
		},
		PackageDir:   "/var/cache/assimilator/packages",
		desiredState: desiredState,
	})
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

	var updateInterval time.Duration = 10
	ticker := time.NewTicker(updateInterval * time.Second)
	done := make(chan bool)
	go func(ticker *time.Ticker) {
		var attempts int = 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				updateAvailable, err := isUpdateAvailable(repoDir)

				if err != nil {
					Error("error checking for updates: ", err)
					attempts++
					if attempts >= 3 {
						Fatal(1, "Update check failed 3 times. Shutting down...")
					}
					continue
				}

				attempts = 0

				if updateAvailable {
					Info("Update available. Shutting down...")
					sigChan <- os.Interrupt
					return
				}

			}
			defer ticker.Stop()
		}
	}(ticker)
	// Wait for a signal
	<-sigChan
	Info("Received interrup signal. Gracefully stopping gRPC server...")
	close(done)
	// Graceful shutdown for gRPC server
	s.GracefulStop()
	Info("gRPC server stopped.")

}
