package agent

import (
	pb "github.com/geogian28/Assimilator/proto"
)

type FedoraManager struct {
	runner CommandRunner
}

func newFedoraManager() *FedoraManager {
	return &FedoraManager{runner: &LiveCommandRunner{}}
}

// RemovePackages uninstalls a list of packages
func (d *FedoraManager) RemovePackages(pkgs []string) error {
	return nil
}

// EnableService enables a systemd service
func (d *FedoraManager) EnableService(service string) error {
	return nil
}

// StartService starts a systemd service
func (d *FedoraManager) StartService(service string) error {
	return nil
}

func (d *FedoraManager) InstallPackages(packages map[string]*pb.PackageConfig) error {
	var err error
	for pkg := range packages {
		_, _, err = d.runner.Run("dnf", "list", "installed", pkg)
		if err == nil {
			Trace("Package already installed: ", pkg)
			continue
		}
		Trace("Installing package: ", pkg)
		stdout, stderr, err := d.runner.Run("dnf", "install", pkg, "-y")

		if err != nil {
			Error("Error installing package: ", err)
			Trace("stdout: ", string(stdout))
			Trace("stderr: ", string(stderr))
			continue
		}
		Debug("Installed package: ", pkg)
	}
	return nil
}

func (d *FedoraManager) UpdateCache() error {
	// sudo dnf upgrade --refresh
	Trace("Updating dnf cache...")

	_, _, err := d.runner.Run("dnf", "upgrade", "--refresh", "-y")
	if err != nil {
		Trace("dnf upgrade --refresh failed: ", err)
		return err
	}
	Trace("dnf cache updated.")
	return nil
}
