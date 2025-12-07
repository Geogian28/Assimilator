package main

import "fmt"

type ArchManager struct {
	runner CommandRunner
}

func newArchManager(runner CommandRunner) *ArchManager {
	return &ArchManager{runner: runner}
}

func (d *ArchManager) AddRepo() error {
	return fmt.Errorf("not implemented")
}

func (d *ArchManager) UpdateCache() error {
	return fmt.Errorf("not implemented")
}

func (d *ArchManager) IsUpdateAvailable() (bool, error) {
	return false, nil
}

func (d *ArchManager) InstallUpdate() error {
	return fmt.Errorf("not implemented")
}
