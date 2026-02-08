package agent

// import (
// 	"fmt"
// 	"os"
// 	"sync"

// 	// "reflect"
// 	"testing"

// 	asslog "github.com/geogian28/Assimilator/assimilator_logger"
// )

// func TestMain(m *testing.M) {
// 	// 1. Setup: Initialize the logger before any tests run.
// 	asslog.StartLogger()

// 	// 2. Run the tests.
// 	exitCode := m.Run()

// 	// 3. Teardown (optional): Clean up resources after tests are done.
// 	os.Exit(exitCode)
// }

// // // TestApplyProfiles checks if profiles are correctly merged into machine configs.
// // func TestApplyProfiles(t *testing.T) {
// // 	initialPackages := map[string]*pb.PackageConfig{
// // 		"nano": {State: "present"},
// // 		"git":  {State: "present"},
// // 	}
// // }

// // func TestInstallPrograms(t *testing.T) {
// // 	t.Run("TestInstallPrograms", func(t *testing.T) {
// // 		// Create a mock command runner
// // 		mock := (&MockCommandRunner{
// // 			MockOutput: []byte(""),
// // 			MockError:  nil,
// // 		})

// // 		// Act
// // 		result := installPrograms(mock, validPackage)

// // 		// Assert
// // 	})

// // }

// // TestisValid tests the isValid function
// func TestIsValid(t *testing.T) {
// 	t.Run("TestIsValid", func(t *testing.T) {
// 		// Test case 1: Valid package
// 		validPackage := "nano"
// 		mock := (&MockCommandRunner{
// 			MockStdout: []byte("Package: nano\n"),
// 			MockStderr: []byte(""),
// 			MockError:  nil,
// 		})

// 		// Act: Call the isValid function
// 		result := isValid(mock, validPackage)

// 		// Assert: Check if the result is true
// 		if !result {
// 			t.Errorf("Expected isValid to return true for a valid package, but got false")
// 		}
// 	})

// 	t.Run("TestIsInvalid", func(t *testing.T) {
// 		// Test case 2: Invalid package
// 		invalidPackage := "invalid-package"
// 		mock := (&MockCommandRunner{
// 			MockStdout: []byte(""),
// 			MockStderr: []byte(""),
// 			MockError:  fmt.Errorf("package not found"),
// 		})

// 		// Act: Call the isValid function
// 		result := isValid(mock, invalidPackage)

// 		// Assert: Check if the result is false
// 		if result {
// 			t.Errorf("Expected isValid to return false for an invalid package, but got true")
// 		}
// 	})
// }

// func TestUpdateAptCache(t *testing.T) {
// 	var wg sync.WaitGroup
// 	wg.Add(1)
// 	t.Run("TestUpdateAptCache", func(t *testing.T) {
// 		// Test case 1: Valid package
// 		validPackage := "nano"
// 		mock := (&MockCommandRunner{
// 			MockStdout: []byte("Package: nano\n"),
// 			MockStderr: []byte(""),
// 			MockError:  nil,
// 		})

// 		// Act: Call the isValid function
// 		result := isValid(mock, validPackage)

// 		// Assert: Check if the result is true
// 		if !result {
// 			t.Errorf("Expected isValid to return true for a valid package, but got false")
// 		}
// 	})

// 	t.Run("TestUpdateAptCache", func(t *testing.T) {
// 		// Test case 2: Invalid package
// 		invalidPackage := "invalid-package"
// 		mock := (&MockCommandRunner{
// 			MockStdout: []byte(""),
// 			MockStderr: []byte("This is a STDERR!"),
// 			MockError:  fmt.Errorf("This is an error!"),
// 		})

// 		// Act: Call the isValid function
// 		result := isValid(mock, invalidPackage)

// 		// Assert: Check if the result is false
// 		if result {
// 			t.Errorf("Expected isValid to return false for an invalid package, but got true")
// 		}
// 	})
// }
