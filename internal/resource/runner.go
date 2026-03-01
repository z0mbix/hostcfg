package resource

import (
	"context"
	"os/exec"
	"strings"

	"github.com/z0mbix/cliout"
)

var (
	// VerboseOutput is set by the executor to enable command logging
	VerboseOutput *cliout.Output
)

// RunCmd executes a command, logging it when verbose mode is enabled.
// Returns combined stdout+stderr output and any error.
func RunCmd(ctx context.Context, name string, args ...string) ([]byte, error) {
	if VerboseOutput != nil {
		VerboseOutput.Debugf("exec: %s %s", name, strings.Join(args, " "))
	}
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if VerboseOutput != nil && len(output) > 0 {
		VerboseOutput.Debugf("output: %s", strings.TrimSpace(string(output)))
	}
	return output, err
}

// RunCmdSilent executes a command, logging it when verbose mode is enabled.
// Returns only the error (discards output unless verbose).
func RunCmdSilent(ctx context.Context, name string, args ...string) error {
	if VerboseOutput != nil {
		VerboseOutput.Debugf("exec: %s %s", name, strings.Join(args, " "))
	}
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if VerboseOutput != nil && len(output) > 0 {
		VerboseOutput.Debugf("output: %s", strings.TrimSpace(string(output)))
	}
	return err
}
