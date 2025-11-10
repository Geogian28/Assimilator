package main

import "fmt"

type FedoraManager struct {
	runner CommandRunner
}

func newFedoraManager(runner CommandRunner) *FedoraManager {
	return &FedoraManager{runner: runner}
}

func (d *FedoraManager) AddRepo() error {
	return fmt.Errorf("not implemented")
}

func (d *FedoraManager) UpdateCache() error {
	return fmt.Errorf("not implemented")
}

func (d *FedoraManager) CheckForUpdates() (bool, error) {
	return false, nil
}

func (d *FedoraManager) InstallUpdate() error {
	return fmt.Errorf("not implemented")
}
