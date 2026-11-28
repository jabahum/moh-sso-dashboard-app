package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/moh-sso-dashboard/internal/identityreconcile"
)

func TestIdentitySyncCheckNetworkOriginNeedsNoDatabase(t *testing.T) {
	stdout, _, err := runCommand(t, newIdentitySyncCommand(), "check-network", "--origin", "http://alerts.example.test")
	if err == nil || !strings.Contains(err.Error(), "1 of 1 network checks failed") {
		t.Fatalf("err = %v", err)
	}
	var reports []identityreconcile.NetworkReport
	if jsonErr := json.NewDecoder(strings.NewReader(stdout)).Decode(&reports); jsonErr != nil || len(reports) != 1 {
		t.Fatalf("stdout = %s (%v)", stdout, jsonErr)
	}
	if reports[0].OK || reports[0].Stages[0].Name != identityreconcile.StageOrigin || reports[0].Stages[0].Status != identityreconcile.NetworkFail {
		t.Fatalf("report = %+v", reports[0])
	}
}

func TestIdentitySyncCheckNetworkRejectsBothTargets(t *testing.T) {
	_, _, err := runCommand(t, newIdentitySyncCommand(), "check-network", "--app", "alerts", "--origin", "https://alerts.example.test")
	if err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("err = %v", err)
	}
}
