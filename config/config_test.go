package config

import (
	"os"
	"reflect"
	"testing"

	asslog "github.com/geogian28/Assimilator/assimilator_logger"
	"github.com/geogian28/Assimilator/utils"
)

func TestMain(m *testing.M) {
	// 1. Setup: Initialize the logger before any tests run.
	asslog.StartLogger()

	// 2. Run the tests.
	exitCode := m.Run()

	// 3. Teardown (optional): Clean up resources after tests are done.
	os.Exit(exitCode)
}

// TestApplyProfiles checks if profiles are correctly merged into machine configs.
func TestApplyProfiles(t *testing.T) {
	// 1. Arrange: Set up the initial state for our test.
	initialState := &DesiredState{
		// Define the available profiles
		Profiles: map[string]ConfigProfile{
			"base": {
				Packages: map[string]PackageConfig{
					"nano": {State: "present"},
					"git":  {State: "present"},
				},
				Services: map[string]ServiceConfig{
					"ssh": {State: true},
				},
			},
		},
		// Define the machine that will use the profile
		Machines: map[string]MachineConfig{
			"test-server": {
				// This machine has the "base" profile applied
				AppliedProfiles: []string{"base"},
				// It also has its own specific package
				Packages: map[string]PackageConfig{
					"htop": {State: "present"},
				},
			},
		},
	}

	// Define what we EXPECT the final state to look like after merging.
	expectedState := &DesiredState{
		Profiles: initialState.Profiles, // Profiles section doesn't change
		Machines: map[string]MachineConfig{
			"test-server": {
				AppliedProfiles: []string{"base"},
				// We expect the final map to contain packages from BOTH the
				// machine's original config AND the "base" profile.
				Packages: map[string]PackageConfig{
					"htop": {State: "present"}, // Original package
					"nano": {State: "present"}, // Merged from profile
					"git":  {State: "present"}, // Merged from profile
				},
				Services: map[string]ServiceConfig{
					"ssh": {State: true}, // Merged from profile
				},
			},
		},
	}

	// 2. Act: Run the function we are testing.
	resultState := applyProfiles(initialState)

	// 3. Assert: Check if the result matches our expectation.
	// reflect.DeepEqual is the standard way to compare complex structs in tests.
	if !reflect.DeepEqual(resultState, *expectedState) {
		// The t.Errorf function fails the test and prints a helpful message.
		t.Errorf("applyProfiles() did not produce the expected state.")

		// For debugging, you can pretty-print the difference
		t.Log("GOT:")
		utils.PrettyPrint(resultState)
		t.Log("WANTED:")
		utils.PrettyPrint(*expectedState)
	}
}
