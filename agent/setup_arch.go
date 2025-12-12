package agent

import (
	pb "github.com/geogian28/Assimilator/proto"
)

type ArchManager struct {
	runner CommandRunner
}

func newArchManager() *ArchManager {
	return &ArchManager{runner: &LiveCommandRunner{}}
}

// RemovePackages uninstalls a list of packages
func (d *ArchManager) RemovePackages(pkgs []string) error {
	return nil
}

// EnableService enables a systemd service
func (d *ArchManager) EnableService(service string) error {
	return nil
}

// StartService starts a systemd service
func (d *ArchManager) StartService(service string) error {
	return nil
}

func (d *ArchManager) InstallPackages(packages map[string]*pb.PackageConfig) error {
	return nil
}

func (d *ArchManager) UpdateCache() error {
	return nil
}
