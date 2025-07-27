package agent

import (
	"testing"
	"time"

	"github.com/geogian28/Assimilator/config"
)

type args struct {
	name   string
	config config.AppConfig
	cmd    LiveCommandRunner
}

// "google.golang.org/grpc/codes"
// "google.golang.org/grpc/status"

func TestAgent(t *testing.T) {
	//TODO: Test the agent
	testArgs := []args{
		{
			name: "should work",
			config: config.AppConfig{
				IsAgent:        true,
				Hostname:       "ubuntu-tester",
				ServerIP:       "borg-cube.brohome.net",
				ServerPort:     2390,
				VerbosityLevel: 6,
			},
			cmd: LiveCommandRunner{},
		},
		{
			name: "should return Unavailable",
			config: config.AppConfig{
				IsAgent:        true,
				Hostname:       "k8s-test",
				ServerIP:       "borg-cube.brohome.com",
				ServerPort:     2390,
				VerbosityLevel: 0,
			},
			cmd: LiveCommandRunner{},
		},
		{
			name: "should return NotFound",
			config: config.AppConfig{
				IsAgent:        true,
				Hostname:       "k8s-fail",
				ServerIP:       "borg-cube.brohome.net",
				ServerPort:     2390,
				VerbosityLevel: 0,
			},
			cmd: LiveCommandRunner{},
		},
	}
	for _, arg := range testArgs {
		t.Run(arg.name, func(t *testing.T) {
			Agent(&arg.config, &arg.cmd)
			// if results != nil {
			// 	t.Errorf("Agent() error = %v, wantErr %v", results, nil)
			// }
			time.Sleep(20000 * time.Millisecond)

		})
	}

}
