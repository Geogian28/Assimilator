package server

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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
	RepoDir     string
	ChecksumMap map[string]string
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

func makePackages(repoDir string) error {
	Info("Making packages from repository...")
	os.Mkdir(repoDir+"/packages", 0755)

	// Make packages for machine
	entries, err := os.ReadDir(repoDir + "/machine")
	if err != nil {
		asslog.Unhandled("error reading machine directory: ", err)
	}
	for _, folder := range entries {
		if folder.IsDir() {
			err = makePackage(repoDir, folder.Name())
			if err != nil {
				asslog.Unhandled("error making package: ", err)
			}
		}
	}

	// Make packages for users
	entries, err = os.ReadDir(repoDir + "/user")
	if err != nil {
		asslog.Unhandled("error reading user directory: ", err)
	}
	for _, folder := range entries {
		if folder.IsDir() {
			err = makePackage(repoDir, folder.Name())
			if err != nil {
				asslog.Unhandled("error making package: ", err)
			}
		}
	}
	return nil
}

func makePackage(sourceDir, packageName string) error {
	Info("Making package: " + packageName)
	// validate the input directory exists
	info, err := os.Stat(sourceDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("input directory is not a directory: %s", sourceDir)
	}

	// create the output file (the ".tar.gz" file)
	packageFolder := filepath.Join(sourceDir, "packages")
	hostname, _ := os.Hostname()
	Trace("hostname: ", hostname)
	packagePath := filepath.Join(packageFolder, packageName+".tar.gz."+hostname)
	Trace("packagePath: ", packagePath)
	packagePath, err = filepath.Abs(packagePath)
	Trace("absolute packagePath: ", packagePath)
	if err != nil {
		return fmt.Errorf("error creating absolute package path: %s", err)
	}
	tarball, err := os.Create(packagePath)
	if err != nil {
		return err
	}
	Trace("created the tarball")
	// create the compressor
	gzw := gzip.NewWriter(tarball)

	// 4. Create the tar writer
	tw := tar.NewWriter(gzw)

	filepath.Walk(sourceDir, func(file string, fi os.FileInfo, err error) error {
		// return any error
		if err != nil {
			return fmt.Errorf("unable to walk directory: %s", err)
		}

		// return on non-regular files
		if !fi.Mode().IsRegular() {
			return nil
		}

		if fi.IsDir() && fi.Name() == "packages" {
			return filepath.SkipDir
		}

		// create a new dir/file header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return fmt.Errorf("unable to create header: %s", err)
		}

		// update the name to correctly reflect the desired destination when untarring
		header.Name, err = filepath.Rel(sourceDir, file)
		if err != nil {
			return fmt.Errorf("unable to get relative path for header.Name: %s", err)
		}

		// write the header
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("unable to write header: %s", err)
		}

		// open files for taring
		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("unable to open file: %s", err)
		}

		// copy file data into tar writer
		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("unable to copy file data: %s", err)
		}

		// manually close here after each file operation;
		// defering would cause each file close operation to wait until all operations have completed.
		f.Close()

		return nil
	})

	// Close files to start finishing up
	tw.Close()
	gzw.Close()
	tarball.Close()
	Trace("closed the tarball for ", packageName)

	// Calculate the SHA256 checksum
	sha256, err := calculateSha256(packagePath)
	if err != nil {
		return fmt.Errorf("unable to calculate sha256 for package: %s", err)
	}
	checksumPath := filepath.Join(packageFolder, packageName+".tar.gz.sha256."+hostname)
	_, err = os.Create(checksumPath)
	os.WriteFile(checksumPath, []byte(sha256), 0644)

	// Rename the tarball and checksum
	newPackagePath := filepath.Join(packageFolder, packageName+".tar.gz")
	Trace("newPackagePath: ", newPackagePath)
	newPackagePath, err = filepath.Abs(newPackagePath)
	Trace("absolute newPackagePath: ", newPackagePath)
	newChecksumPath := filepath.Join(packageFolder, packageName+".tar.gz.sha256")
	if err != nil {
		return fmt.Errorf("error creating new absolute package path: %s", err)
	}
	err = os.Rename(packagePath, newPackagePath)
	if err != nil {
		return fmt.Errorf("error renaming the tarball: %s", err)
	}
	err = os.Rename(checksumPath, newChecksumPath)
	if err != nil {
		return fmt.Errorf("error renaming the checksum: %s", err)
	}
	Trace("renamed the tarball sucessfully")
	Success("Package ", packageName, " was created successfully!")

	return nil
}

func calculateSha256(filePath string) (string, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Calculate the SHA256 checksum
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to copy file content to hash: %w", err)
	}
	hashInBytes := hash.Sum(nil)
	hashString := hex.EncodeToString(hashInBytes)
	return hashString, nil
}

func collectChecksums(repoDir string) (map[string]string, error) {
	checksumMap := make(map[string]string)
	packageFolder := filepath.Join(repoDir, "packages")
	filepath.WalkDir(packageFolder, func(path string, info fs.DirEntry, err error) error {
		if strings.HasSuffix(info.Name(), ".sha256") {
			// Open the checksum file
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("unable to read file %s: %s", info.Name(), err)
			}

			// Read the checksum
			scanner := bufio.NewScanner(file)
			scanner.Scan()
			checksum := scanner.Text()

			// Add to map with the package path
			checksumMap[checksum] = strings.TrimSuffix(path, ".sha256")

			// Close the file
			file.Close()
		}
		return nil
	})
	if len(checksumMap) == 0 {
		return nil, fmt.Errorf("no checksums found in %s", repoDir+"/packages")
	}
	return checksumMap, nil
}

// Start the server
func Server(appConfig *config.AppConfig) {
	// Clone or pull the remote repository to the local one
	repoDir, err := cloneOrPullRepo(appConfig)
	if err != nil {
		Trace("error cloning or pulling repository: ", err)
		Unhandled("error cloning or pulling repository: ", err)
	} else {
		Info("Repository cloned or pulled successfully")
	}

	// Make packages for machine
	err = makePackages(repoDir)
	if err != nil {
		asslog.Unhandled("error making packages: ", err)
	}

	// Collect checksums
	checksumMap, err := collectChecksums(repoDir)
	if err != nil {
		asslog.Unhandled("error collecting checksums: ", err)
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
	pb.RegisterAssimilatorServer(s, &AssimilatorServer{
		RepoDir:     repoDir,
		ChecksumMap: checksumMap,
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
	// Wait for a signal
	<-sigChan
	Info("\nReceived interrup signal. Gracefully stopping gRPC server...")

	// Graceful shutdown for gRPC server
	s.GracefulStop()
	Info("gRPC server stopped.")

}
