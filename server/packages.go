package server

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	asslog "github.com/geogian28/Assimilator/assimilator_logger"
	config "github.com/geogian28/Assimilator/config"
)

// checksummap := make(map[string]map[string]PackageDetails)

type packageInfo struct {
	sourceDir        string
	cacheDir         string
	packageName      string
	packageTempPath  string
	packagePermPath  string
	checksum         string
	checksumTempPath string
	checksumPermPath string
	hostname         string
	size             int64
}

type PackagesMap map[string]map[string]*packageInfo

var packagesMap PackagesMap

func makePackages(appConfig *config.AppConfig) {
	repoDir := appConfig.RepoDir
	cacheDir := appConfig.CacheDir

	err := os.MkdirAll(cacheDir, 0750)
	if err != nil {
		asslog.Unhandled("error creating /var/cache directory: ", err)
	}
	Info("Making packages from repository: ", repoDir)

	// Make the PackagesMap
	packagesMap = make(PackagesMap)

	// Make packages for machine
	packagesMap["machine"] = make(map[string]*packageInfo)
	makePackagesFromPath(filepath.Join(repoDir, "machine"), filepath.Join(cacheDir, "machine"), "machine")

	// Make packages for user
	packagesMap["user"] = make(map[string]*packageInfo)
	makePackagesFromPath(filepath.Join(repoDir, "user"), cacheDir, "user")

}

func makePackagesFromPath(sourceDir string, cacheDir string, category string) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		asslog.Unhandled("error reading machine directory: ", err)
	}
	hostname, _ := os.Hostname()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// 1. Setup the struct that's used many times throughout this process
		sourceDir := filepath.Join(sourceDir, entry.Name())
		pkgInfo := &packageInfo{
			sourceDir:        sourceDir,
			cacheDir:         cacheDir,
			packageName:      entry.Name(),
			packageTempPath:  filepath.Join(cacheDir, entry.Name()+".tar.gz."+hostname),
			packagePermPath:  filepath.Join(cacheDir, entry.Name()+".tar.gz"),
			checksum:         "",
			checksumTempPath: filepath.Join(cacheDir, entry.Name()+".tar.gz.sha256"+hostname),
			checksumPermPath: filepath.Join(cacheDir, entry.Name()+".tar.gz.sha256"),
			hostname:         hostname,
		}

		// 2. Create the cache directory
		err := os.MkdirAll(pkgInfo.cacheDir, 0750)
		if err != nil {
			asslog.Unhandled("error creating /var/cache directory: ", err)
		}

		// 3. Make the temporary package. This will be moved to the permanent location later.
		err = makeTempPackage(pkgInfo)
		if err != nil {
			asslog.Unhandled("error making package: ", err)
		}

		// 4. Make the checksum from the created package.
		err = makeTempChecksum(pkgInfo)
		if err != nil {
			asslog.Unhandled("error making package: ", err)
		}

		// 5. Make the permanent package by moving the temporary package to the permanent location.
		makeTempFilesPermanent(pkgInfo)

		// 6. Add the package to the map so it can be found and referenced later
		packagesMap[category][pkgInfo.packageName] = pkgInfo
	}
}

func makeTempPackage(pkg *packageInfo) error {
	// create the output file (the ".tar.gz" file)
	tarball, err := os.Create(pkg.packageTempPath)
	if err != nil {
		return fmt.Errorf("error creating tarball: %s", err)
	}

	// create the compressor
	gzw := gzip.NewWriter(tarball)

	// 4. Create the tar writer
	tw := tar.NewWriter(gzw)

	filepath.Walk(pkg.sourceDir, func(file string, fi os.FileInfo, err error) error {
		Trace("filepath.Walk: currently looking at: ", file)
		// return any error
		if err != nil {
			Error("unable to walk directory: ", err)
			return fmt.Errorf("unable to walk directory: %s", err)
		}

		// return on non-regular files
		if !fi.Mode().IsRegular() {
			return nil
		}

		// create a new dir/file header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			Error("unable to create header: ", err)
			return fmt.Errorf("unable to create header: %s", err)
		}

		// update the name to correctly reflect the desired destination when untarring
		header.Name, err = filepath.Rel(pkg.sourceDir, file)
		if err != nil {
			Error("unable to get relative path for header.Name: ", err)
			return fmt.Errorf("unable to get relative path for header.Name: %s", err)
		}

		// write the header
		if err := tw.WriteHeader(header); err != nil {
			Error("unable to write header: ", err)
			return fmt.Errorf("unable to write header: %s", err)
		}

		// open files for taring
		f, err := os.Open(file)
		if err != nil {
			Error("unable to open file: ", err)
			return fmt.Errorf("unable to open file: %s", err)
		}

		// copy file data into tar writer
		if _, err := io.Copy(tw, f); err != nil {
			Error("unable to copy file data: ", err)
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
	Trace("closed the tarball for ", pkg.packageName)
	return nil
}

func makeTempChecksum(pkg *packageInfo) error {
	// Open the file
	file, err := os.Open(pkg.packageTempPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Calculate the SHA256 checksum
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to copy file content to hash: %w", err)
	}
	hashInBytes := hash.Sum(nil)
	pkg.checksum = hex.EncodeToString(hashInBytes)

	// Get the filesize while we're in here
	fileStat, _ := file.Stat()
	pkg.size = fileStat.Size()

	_, err = os.Create(pkg.checksumTempPath)
	os.WriteFile(pkg.checksumTempPath, []byte(pkg.checksum), 0644)
	return nil
}

func makeTempFilesPermanent(pkg *packageInfo) {
	// Rename the tarball and checksum
	err := os.Rename(pkg.packageTempPath, pkg.packagePermPath)
	if err != nil {
		Error("error renaming the tarball: ", err)
	}
	err = os.Rename(pkg.checksumTempPath, pkg.checksumPermPath)
	if err != nil {
		Error("error renaming the checksum: ", err)
		return
	}
	Trace("renamed the tarball sucessfully")
	Success("Package ", pkg.packageName, " was created successfully!")
}

// func collectChecksums(repoDir string) map[string]PackageDetails {
// 	fmt.Println("collectChecksums: repoDir: ", repoDir)
// 	checksumMap := make(map[string]PackageDetails)
// 	packageFolder := filepath.Join(repoDir, "packages")
// 	filepath.WalkDir(packageFolder, func(pkgFilePath string, pkgFileName fs.DirEntry, err error) error {
// 		Trace("filepath.WalkDir: currently looking at: ", pkgFilePath)
// 		if !strings.HasSuffix(pkgFileName.Name(), ".tar.gz") {
// 			return nil
// 		}
// 		packageName := strings.TrimSuffix(pkgFileName.Name(), ".tar.gz")
// 		// Open and read the checksum file
// 		checksumFile, err := os.Open(pkgFilePath + ".sha256")
// 		if err != nil {
// 			return fmt.Errorf("unable to read file %s: %s", pkgFileName.Name(), err)
// 		}
// 		scanner := bufio.NewScanner(checksumFile)
// 		scanner.Scan()
// 		checksum := scanner.Text()
// 		checksumFile.Close()

// 		// Get the package file size
// 		packageFile, _ := pkgFileName.Info()
// 		packageSize := packageFile.Size()

// 		// Add to map with the package path
// 		checksumMap[checksum] = PackageDetails{
// 			Name:     pkgFileName.Name(),
// 			FilePath: pkgFilePath,
// 			FileSize: packageSize,
// 		}
// 		Trace("added package to checksum map:")
// 		Trace("    packageName: ", packageName)
// 		Trace("    checksum: ", checksum)
// 		Trace("    filePath: ", pkgFilePath)
// 		Trace("    fileSize: ", packageSize)

// 		for _, machineConfig := range DesiredState.Machines {
// 			if _, okay := machineConfig.Packages[packageName]; okay {
// 				pkg := machineConfig.Packages[packageName]
// 				pkg.Checksum = checksum
// 				machineConfig.Packages[packageName] = pkg
// 				Debug("machineConfig.Packages[", packageName, "].Checksum: ", machineConfig.Packages[packageName].Checksum)
// 			}
// 		}
// 		for _, userConfig := range DesiredState.Users {
// 			if _, okay := userConfig.Packages[packageName]; okay {
// 				pkg := userConfig.Packages[packageName]
// 				pkg.Checksum = checksum
// 				userConfig.Packages[packageName] = pkg
// 				Debug("machineConfig.Packages[packageName].Checksum: ", userConfig.Packages[packageName].Checksum)
// 			}
// 		}
// 		return nil
// 	})
// 	if len(checksumMap) == 0 {
// 		Error("No checksums found in ", repoDir+"/packages")
// 		return nil
// 	}
// 	return checksumMap
// }

func syncChecksums() {
	Info("Syncing calculated checksums to DesiredState...")

	// 1. Sync Machine Packages
	for _, machineConfig := range DesiredState.Machines {
		Debug("machineConfig: ", machineConfig)
		for pkgName, pkgConfig := range machineConfig.Packages {
			Trace("syncing checksum for machineConfig.Packages[", pkgName, "]")
			// Look up the package in our generated map
			if info, ok := packagesMap["machine"][pkgName]; ok {
				Debug("Package ", pkgName, " found in repo")
				// Update the checksum in the config
				pkgConfig.Checksum = info.checksum
				// CRITICAL: Reassign the struct back to the map (Go map semantics)
				machineConfig.Packages[pkgName] = pkgConfig
			} else {
				Error("Package ", pkgName, " not found in repo")
				// Optional: Warn if a configured package wasn't found in the repo
				// Warning("Configured package not found in repo: ", pkgName)
			}
		}
	}

	// 2. Sync User Packages
	// for _, userConfig := range DesiredState.Users {
	// 	for pkgName, pkgConfig := range userConfig.Packages {
	// 		if info, ok := packagesMap["user"][pkgName]; ok {
	// 			pkgConfig.Checksum = info.checksum
	// 			userConfig.Packages[pkgName] = pkgConfig
	// 		}
	// 	}
	// }
}
