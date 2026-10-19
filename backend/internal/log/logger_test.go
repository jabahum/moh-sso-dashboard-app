package logger

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestFatalStopsExecution(t *testing.T) {
	if os.Getenv("MOH_LOG_FATAL_TEST") == "1" {
		log.Logger = zerolog.New(os.Stdout)
		NewLogger().Fatal("Failed to initialize databases: ", errors.New("migration unavailable"))
		// This models startup constructing services after a failed database init.
		_, _ = os.Stdout.WriteString("startup continued\n")
		return
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestFatalStopsExecution$")
	cmd.Env = append(os.Environ(), "MOH_LOG_FATAL_TEST=1")
	output, err := cmd.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %v; output: %s", err, output)
	}
	if strings.Contains(string(output), "startup continued") {
		t.Fatalf("execution continued after Fatal: %s", output)
	}
	var event struct {
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(output, &event); err != nil {
		t.Fatalf("expected one JSON log event: %v; output: %s", err, output)
	}
	if event.Level != "fatal" || event.Message != "Failed to initialize databases: migration unavailable" {
		t.Fatalf("fatal event lost the initialization error: %+v", event)
	}
}
