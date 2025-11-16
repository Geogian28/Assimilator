package main

import (
	"strings"
	"testing"
)

type MockCommandOutput struct {
	MockStdout []byte
	MockStderr []byte
	MockError  error
}

func (m *MockCommandRunner) GetMockOutput(name string, args ...string) ([]byte, []byte, error) {
	return nil, nil, nil
}

func TestParseDistro(t *testing.T) {

	// Arrange
	testCases := []struct {
		name       string
		input      string
		expectedOS string
		expectErr  bool
	}{
		{
			name:       "Debian",
			input:      "ID=debian",
			expectedOS: "Debian",
			expectErr:  false,
		},
		{
			name:       "Fedora from ID",
			input:      "ID=fedora\n",
			expectedOS: "Fedora",
			expectErr:  false,
		},
		{
			name:       "Arch from ID_LIKE",
			input:      "ID=manjaro\nID_LIKE=arch\n", // A good real-world case
			expectedOS: "Arch",
			expectErr:  false,
		},
		{
			name:       "Ubuntu is treated as Debian",
			input:      "ID=ubuntu\nID_LIKE=debian\n",
			expectedOS: "Debian",
			expectErr:  false,
		},
		{
			name:       "Unknown OS",
			input:      "ID=solaris\n",
			expectedOS: "",   // We don't expect a specific OS
			expectErr:  true, // We expect it to fail and return an error
		},
		{
			name:       "Empty File",
			input:      "",
			expectedOS: "",
			expectErr:  true,
		},
		{
			name:       "Buggy line (PRETTY_NAME)",
			input:      "PRETTY_NAME=\"My debian server\"\n", // This used to be a bug!
			expectedOS: "",
			expectErr:  true,
		},
	}

	for _, tc := range testCases {
		// t.Run gives each test its own name in the output
		t.Run(tc.name, func(t *testing.T) {

			// --- Arrange (for this one test) ---
			reader := strings.NewReader(tc.input)
			mockRunner := &MockCommandRunner{} // Not used, but parseDistro needs it

			// --- Act ---
			manager, err := parseDistro(mockRunner, reader)

			// --- Assert ---
			// First, check if we got an error when we shouldn't have, or vice-versa
			if tc.expectErr {
				if err == nil {
					// We expected an error, but didn't get one!
					t.Errorf("Expected an error, but got nil")
				}
				return // This test is done (it correctly failed)
			}
			if !tc.expectErr && err != nil {
				// We did NOT expect an error, but we got one!
				t.Errorf("Did not expect error, but got: %v", err)
				return
			}

			// Now, let's check the *type* of the manager
			var resultingOS string
			switch manager.(type) {
			case *DebianManager:
				resultingOS = "Debian"
			case *FedoraManager:
				resultingOS = "Fedora"
			case *ArchManager:
				resultingOS = "Arch"
			default:
				t.Fatalf("Got a manager, but it's an unknown type!")
			}

			// Finally, check if the OS we found matches what we expected
			if resultingOS != tc.expectedOS {
				t.Errorf("Expected OS %q, but got %q", tc.expectedOS, resultingOS)
			}
		})
	}
}
