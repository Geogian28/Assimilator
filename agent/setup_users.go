package agent

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	pb "github.com/geogian28/Assimilator/proto"
)

func (a *AgentData) setupUser(username string, user *pb.UserConfig) error {
	for packageName, packageData := range user.Packages {
		pkg := &packageInfo{
			cacheDir:       filepath.Join(a.appConfig.CacheDir, "machine"),
			name:           packageName,
			category:       "machine",
			localChecksum:  "",
			serverChecksum: packageData.Checksum,
			path:           filepath.Join(a.appConfig.CacheDir, "machine", packageName+".tar.gz"),
		}

		// 1. Ensure the package exists and is up-to-date
		if pkg.serverChecksum == "" {
			return fmt.Errorf("package %s has no server checksum", pkg.name)
		}
		err := a.ensurePackage(pkg)
		if err != nil {
			Error("error installing package: ", err)
			continue
		}

		// 2. Extract package
		err = a.extractPackage(pkg)
		if err != nil {
			Error("error installing package: ", err)
			continue
		}

		// 3. Install package
		if username == "_default" {
			realUsers, err := a.getValidUsers()
			if err != nil {
				Error("error getting valid users: ", err)
				continue
			}
			for _, realUser := range realUsers {
				err = a.installUserPackage(packageName, realUser)
				if err != nil {
					Error("error installing package '", packageName, "' for user '", realUser, "': ", err)
				}
			}
			continue
		}
		err = a.installUserPackage(packageName, username)
		if err != nil {
			Error("error installing package: ", err)
			continue
		}
	}
	return nil
}

// func (a *AgentData) extractUserPackage(pkgName string, cachePath string, username string) error {
// 	destDir := filepath.Join(os.TempDir(), "assimilator", "user", username, pkgName)
// 	return a.extractArchive(destDir, cachePath)
// }

func (a *AgentData) installUserPackage(pkgName string, username string) error {
	// 1. Check if user exists and get their details
	targetUser, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("user %s not found: %w", username, err)
	}
	uid, err := strconv.ParseUint(targetUser.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("failed to parse UID for %s: %w", username, err)
	}
	gid, err := strconv.ParseUint(targetUser.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("failed to parse GID for %s: %w", username, err)
	}

	// 2. Set the extract variable
	extractDir := filepath.Join(os.TempDir(), "assimilator", username, pkgName)

	// 3. Prepare the command
	scriptPath := filepath.Join(extractDir, "install.sh")
	cmd := exec.Command("/bin/bash", scriptPath)

	// 4. Prepare the environment
	cmd.Env = []string{
		fmt.Sprintf("HOME=%s", targetUser.HomeDir),
		fmt.Sprintf("USER=%s", targetUser.Username),
		fmt.Sprintf("ASSIMILATOR_HOME=%s", targetUser.HomeDir),
		fmt.Sprintf("ASSIMILATOR_USER=%s", targetUser.Username),
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", // Basic path
	}

	// 5. Set the working directory
	cmd.Dir = extractDir

	// 6. Set the user
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}

	// 7. Run
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install failed for %s: %s", pkgName, string(output))
	}

	return nil
}

func (a *AgentData) getValidUsers() ([]string, error) {
	// We use "getent passwd" to see both local and LDAP/FreeIPA users
	cmd := exec.Command("getent", "passwd")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run getent: %w", err)
	}

	var validUsers []string
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}

		username := parts[0]
		uidStr := parts[2]
		homeDir := parts[5]
		shell := parts[6]

		// 1. Filter by UID (Humans are usually >= 1000)
		uid, err := strconv.Atoi(uidStr)
		if err != nil {
			continue
		}

		// Skip system users and 'nobody'
		if uid < 1000 || uid == 65534 {
			continue
		}

		// 2. Filter by Shell (Optional but recommended)
		// Skip users that have /bin/false or /sbin/nologin
		if strings.Contains(shell, "nologin") || strings.Contains(shell, "false") {
			continue
		}

		// 3. Vital Check: Does the home directory actually exist?
		// We don't want to install configs if the user has never logged in
		// and the home dir hasn't been created yet.
		info, err := os.Stat(homeDir)
		if err != nil || !info.IsDir() {
			continue
		}

		validUsers = append(validUsers, username)
	}

	return validUsers, nil
}
