package main

import (
	"bytes"
	"os/exec"
)

// --- 1. Define an Interface ---
// This interface describes what we need to do: run a command.
// By depending on this interface instead of the 'os/exec' package directly,
// we can swap out the implementation for a fake one in our tests.
type CommandRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

// --- 2. Create the REAL Implementation ---
// This is the implementation we will use in our actual application.
// It holds no data, it just has the method we need.
type LiveCommandRunner struct{}

// Run executes a real command on the operating system.
func (r *LiveCommandRunner) Run(name string, args ...string) ([]byte, error) {
	// This calls the real os/exec.Command
	cmd := exec.Command(name, args...)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	return outBuf.Bytes(), err
}

// --- 3. Create the MOCK Implementation for Tests ---
// This implementation is what we will use in our tests.
// It holds fake data that we can control.
type MockCommandRunner struct {
	// We can tell the mock what output and error it should return.
	MockStdout []byte
	MockError  error
}

// Run for the mock doesn't execute any real commands. It just returns
// the fake data we configured it with.
func (r *MockCommandRunner) Run(name string, args ...string) ([]byte, error) {
	return r.MockStdout, r.MockError
}
