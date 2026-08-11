package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	asslog "github.com/geogian28/Assimilator/assimilator_logger"
)

// type PackagesMap map[string]*packageInfo

// var packagesMap PackagesMap

func makePackages() (map[string]*packageInfo, error) {
	err := os.MkdirAll(appConfig.CacheDir, 0750)
	if err != nil {
		return nil, fmt.Errorf("error creating %s: %v", appConfig.CacheDir, err)
	}
	Info(1, "Making packages from repository: ", appConfig.RepoDir)

	packages, err := createPackageInfo(appConfig.RepoDir, appConfig.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("error making packages: %v", err)
	}

	// Make packages for machine
	for _, p := range packages {
		err := p.createTarballs()
		if err != nil {
			Error(1, "error creating tarballs: ", err)
		}
	}

	return packages, nil
}

func createPackageInfo(sourceDir string, cacheDir string) (map[string]*packageInfo, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		if os.IsNotExist(err) {
			Error(1, "Directory does not exist: ", sourceDir)
		}
		asslog.Unhandled("error reading directory: ", err)
	}

	packages := make(map[string]*packageInfo)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// 1. Setup the struct that's used many times throughout this process
		sourceDir := filepath.Join(sourceDir, entry.Name())
		packages[entry.Name()] = &packageInfo{
			sourceDir:        sourceDir,
			cacheDir:         cacheDir,
			packageName:      entry.Name(),
			packageTempPath:  filepath.Join(cacheDir, entry.Name()+".tar.gz."+appConfig.Hostname),
			packagePermPath:  filepath.Join(cacheDir, entry.Name()+".tar.gz"),
			checksumTempPath: filepath.Join(cacheDir, entry.Name()+".tar.gz.sha256"+appConfig.Hostname),
			checksumPermPath: filepath.Join(cacheDir, entry.Name()+".tar.gz.sha256"),
			hostname:         appConfig.Hostname,
		}
	}
	return packages, nil
}

func (p *packageInfo) createTarballs() error {

	// 2. Create the cache directory
	err := os.MkdirAll(p.cacheDir, 0750)
	if err != nil {
		return fmt.Errorf("error creating %s directory: %v", p.packageName, err)
	}

	// 3. Make the temporary package. This will be moved to the permanent location later.
	err = p.makeTempPackage()
	if err != nil {
		return fmt.Errorf("error making %s package: %s", p.packageName, err)
	}

	// 4. Make the checksum from the created package.
	err = p.makeTempChecksum()
	if err != nil {
		return fmt.Errorf("failed to make %s package: %s", p.packageName, err)
	}

	// 5. Make the permanent package by moving the temporary package to the permanent location.
	err = p.makeTempFilesPermanent()
	if err != nil {
		return fmt.Errorf("failed to make %s package: %s", p.packageName, err)
	}

	return nil
}

func (p *packageInfo) makeTempPackage() error {
	// create the output file (the ".tar.gz" file)
	tarball, err := os.Create(p.packageTempPath)
	if err != nil {
		return fmt.Errorf("error creating tarball: %s", err)
	}

	// create the compressor
	gzw := gzip.NewWriter(tarball)

	// 4. Create the tar writer
	tw := tar.NewWriter(gzw)

	filepath.Walk(p.sourceDir, func(file string, fi os.FileInfo, err error) error {
		Trace(1, "filepath.Walk: currently looking at: ", file)
		// return any error
		if err != nil {
			Error(1, "unable to walk directory: ", err)
			return fmt.Errorf("unable to walk directory: %s", err)
		}

		// return on non-regular files
		if !fi.Mode().IsRegular() {
			return nil
		}

		// create a new dir/file header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			Error(1, "unable to create header: ", err)
			return fmt.Errorf("unable to create header: %s", err)
		}

		// update the name to correctly reflect the desired destination when untarring
		header.Name, err = filepath.Rel(p.sourceDir, file)
		if err != nil {
			Error(1, "unable to get relative path for header. Name: ", err)
			return fmt.Errorf("unable to get relative path for header. Name: %s", err)
		}

		// write the header
		if err := tw.WriteHeader(header); err != nil {
			Error(1, "unable to write header: ", err)
			return fmt.Errorf("unable to write header: %s", err)
		}

		// open files for taring
		f, err := os.Open(file)
		if err != nil {
			Error(1, "unable to open file: ", err)
			return fmt.Errorf("unable to open file: %s", err)
		}

		// copy file data into tar writer
		if _, err := io.Copy(tw, f); err != nil {
			Error(1, "unable to copy file data: ", err)
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
	Trace(1, "closed the tarball for ", p.packageName)
	return nil
}

func (p *packageInfo) makeTempChecksum() error {
	// Open the file
	file, err := os.Open(p.packageTempPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Calculate the SHA256 checksum
	p.checksum, err = calculateChecksum(p.packageTempPath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Get the filesize while we're in here
	fileStat, _ := file.Stat()
	p.size = fileStat.Size()

	_, err = os.Create(p.checksumTempPath)
	os.WriteFile(p.checksumTempPath, []byte(p.checksum), 0644)
	return nil
}

func (p *packageInfo) makeTempFilesPermanent() error {
	// Rename the tarball and checksum
	err := os.Rename(p.packageTempPath, p.packagePermPath)
	if err != nil {
		return fmt.Errorf("error renaming the tarball: %s", err)
	}
	err = os.Rename(p.checksumTempPath, p.checksumPermPath)
	if err != nil {
		return fmt.Errorf("error renaming the checksum: %s", err)
	}
	Trace(1, "renamed the tarball sucessfully")
	Success(1, "Package ", p.packageName, " was created successfully!")
	return nil
}

func syncChecksums(desiredState *DesiredState, packagesMap map[string]*packageInfo) {
	Info(1, "Syncing calculated checksums to DesiredState...")

	// 1. Sync Machine Packages
	for _, machineConfig := range desiredState.Machines {
		Debug(1, "machineConfig: ", machineConfig)
		for pkgName, pkgConfig := range machineConfig.Packages {
			Trace(1, "syncing checksum for machineConfig.Packages[", pkgName, "]")
			// Look up the package in our generated map
			if info, ok := packagesMap[pkgName]; ok {
				Debug(1, "Package ", pkgName, " found in repo")
				// Update the checksum in the config
				if len(pkgConfig) == 0 {
					Fatal(1, "Package ", pkgName, " not found in config")
				}
				pkgConfig[0].Checksum = info.checksum
				// CRITICAL: Reassign the struct back to the map (Go map semantics)
				machineConfig.Packages[pkgName] = pkgConfig
			} else {
				Warning(1, "Package ", pkgName, " not found in repo")
			}
		}
	}
}
