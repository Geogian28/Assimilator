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
	return nil
}

func (d *FedoraManager) UpdateCache() error {
	return nil
}
