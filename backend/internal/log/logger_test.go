package logger

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

func TestFatalStopsProcess(t *testing.T) {
	const childFlag = "MOH_LOGGER_FATAL_TEST_CHILD"
	if os.Getenv(childFlag) == "1" {
		NewLogger().Fatal("fatal startup failure")
		os.Exit(42) // Reaching this line means startup would incorrectly continue.
	}

	command := exec.Command(os.Args[0], "-test.run=^TestFatalStopsProcess$")
	command.Env = append(os.Environ(), childFlag+"=1")
	output, err := command.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("expected fatal logging to exit with code 1; got %v: %s", err, output)
	}
}
