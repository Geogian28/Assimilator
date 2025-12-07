package main

// DistroManager defines the set of actions any supported
// Linux distribution must be able to perform.
type DistroManager interface {
	// AddAssimilatortoRepo adds the Assimilator repository
	AddRepo() error

	// UpdateCache refreshes the local package list
	UpdateCache() error

	// checkForUpdates checks for updates
	IsUpdateAvailable() (bool, error)

	// InstallPackages installs a list of packages
	InstallUpdate() error
}
