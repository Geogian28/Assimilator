package agent

import (
	"github.com/geogian28/Assimilator/config"
	pb "github.com/geogian28/Assimilator/proto"
)

type work struct {
	packageName       string
	installedPrograms map[string]bool
}

var installedAPackge bool = false

var Distro DistroManager

type DistroManager interface {
	// UpdateCache refreshes the local package list
	UpdateCache() error

	// InstallPackages installs a list of packages
	InstallPackages(map[string]*pb.PackageConfig) error

	// RemovePackages uninstalls a list of packages
	RemovePackages(pkgs []string) error

	// EnableService enables a systemd service
	EnableService(service string) error

	// StartService starts a systemd service
	StartService(service string) error
}

func setDistroManagerType(appConfig *config.AppConfig) {
	switch appConfig.Distro {
	case "debian":
		Distro = newDebianManager()
	case "fedora":
		Distro = newFedoraManager()
	case "arch":
		Distro = newArchManager()
	}
}

func setupMachine(packages map[string]*pb.PackageConfig) error {
	var err error

	// Update the cache (if applicable)
	err = Distro.UpdateCache()
	if err != nil {
		Error("unable to update cache: ", err)
		Info("Continuing without cache update.")
	}

	// Install packages
	err = Distro.InstallPackages(packages)
	if err != nil {
		return err
	}

	// Distro.RemovePackages(packages)

	// Distro.EnableService()

	// Distro.StartService()

	return nil

}
