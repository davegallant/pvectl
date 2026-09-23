package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/config"
	"github.com/davegallant/pvectl/internal/secrets"
)

type failingDoctorWriter struct{}

func (failingDoctorWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func doctorTestConfig(t *testing.T, host, secret string) {
	t.Helper()
	oldDir, oldStore := config.ConfigDir, fileStore
	dir := t.TempDir()
	config.ConfigDir = func() (string, error) { return dir, nil }
	store := secrets.NewFakeStore()
	fileStore = store
	t.Cleanup(func() { config.ConfigDir, fileStore = oldDir, oldStore })
	if err := config.Save(&config.Config{Host: host, TokenID: "test@pve!doctor", SecretBackend: "file"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(host, secret); err != nil {
		t.Fatal(err)
	}
}

func doctorCheckByName(t *testing.T, report doctorReport, name string) doctorCheck {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("missing %s check: %+v", name, report.Checks)
	return doctorCheck{}
}

func TestDoctorReportsEffectiveGrantsWithoutMutating(t *testing.T) {
	var paths []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
		}
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/api2/json/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"version": "9.2"}})
		case "/api2/json/access/permissions":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"/vms/101": map[string]any{"VM.Audit": 0, "VM.PowerMgmt": 1}, "/storage/local": map[string]any{"Datastore.Audit": nil, "Datastore.AllocateTemplate": 1}}})
		case "/api2/json/cluster/resources":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"type": "qemu", "vmid": 101, "status": "running"}}})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer s.Close()
	doctorTestConfig(t, s.URL, "top-secret")

	report := diagnose(context.Background())
	if got := doctorCheckByName(t, report, "API"); got.Status != "ok" || !strings.Contains(got.Detail, "9.2") {
		t.Errorf("API check = %+v", got)
	}
	if got := doctorCheckByName(t, report, "VM.Audit"); got.Status != "ok" || !strings.Contains(got.Detail, "/vms/101") || !strings.Contains(got.Detail, "not propagated") {
		t.Errorf("VM.Audit check = %+v", got)
	}
	if got := doctorCheckByName(t, report, "Datastore.Audit"); got.Status != "warning" {
		t.Errorf("null grant treated as granted: %+v", got)
	}
	if got := doctorCheckByName(t, report, "Datastore.AllocateTemplate"); got.Status != "ok" || !strings.Contains(got.Detail, "/storage/local") {
		t.Errorf("template download grant = %+v", got)
	}
	if got := doctorCheckByName(t, report, "Guests"); got.Status != "ok" || !strings.Contains(got.Detail, "1 visible") {
		t.Errorf("Guests check = %+v", got)
	}
	if len(paths) != 3 {
		t.Errorf("requests = %v, want 3 read-only probes", paths)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "top-secret") {
		t.Fatal("report leaked token secret")
	}
}

func TestDoctorDoesNotInterpretEmptyGuestListAsEmptyCluster(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"version": "9.2"}})
		case "/api2/json/access/permissions":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		case "/api2/json/cluster/resources":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer s.Close()
	doctorTestConfig(t, s.URL, "secret")
	report := diagnose(context.Background())
	if got := doctorCheckByName(t, report, "Permissions"); got.Status != "error" || !strings.Contains(got.Hint, "Privilege Separation") {
		t.Errorf("Permissions check = %+v", got)
	}
	if got := doctorCheckByName(t, report, "Guests"); got.Status != "warning" || !strings.Contains(got.Detail, "does not prove") {
		t.Errorf("Guests check = %+v", got)
	}
}

func TestDoctorReportsPermissionProbeFailure(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"version": "9.2"}})
		case "/api2/json/access/permissions":
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "Permission check failed"})
		case "/api2/json/cluster/resources":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer s.Close()
	doctorTestConfig(t, s.URL, "secret")
	report := diagnose(context.Background())
	if got := doctorCheckByName(t, report, "Permissions"); got.Status != "error" || !strings.Contains(got.Hint, "ACL") {
		t.Errorf("Permissions check = %+v", got)
	}
	if got := doctorCheckByName(t, report, "Guests"); got.Status != "warning" {
		t.Errorf("Guests check = %+v", got)
	}
}

func TestDoctorReportsTLSVerificationFailure(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("TLS failure should prevent requests reaching handler")
	}))
	defer s.Close()
	doctorTestConfig(t, s.URL, "secret")
	report := diagnose(context.Background())
	if got := doctorCheckByName(t, report, "TLS"); got.Status != "error" || !strings.Contains(got.Hint, "certificate") {
		t.Errorf("TLS check = %+v", got)
	}
}

func TestDoctorDoesNotClaimUnattemptedTLSVerification(t *testing.T) {
	oldDir, oldStore := config.ConfigDir, fileStore
	dir := t.TempDir()
	config.ConfigDir = func() (string, error) { return dir, nil }
	fileStore = secrets.NewFakeStore()
	t.Cleanup(func() { config.ConfigDir, fileStore = oldDir, oldStore })
	if err := config.Save(&config.Config{Host: "https://pve.example.com:8006", TokenID: "test@pve!doctor", SecretBackend: "file"}); err != nil {
		t.Fatal(err)
	}
	report := diagnose(context.Background())
	if got := doctorCheckByName(t, report, "TLS"); got.Status == "ok" || !strings.Contains(got.Detail, "not checked") {
		t.Errorf("TLS check = %+v", got)
	}
}

func TestDoctorMissingConfigSuggestsSetupWithoutNetwork(t *testing.T) {
	oldDir := config.ConfigDir
	dir := t.TempDir()
	config.ConfigDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { config.ConfigDir = oldDir })
	report := diagnose(context.Background())
	if got := doctorCheckByName(t, report, "Config"); got.Status != "error" || !strings.Contains(got.Hint, "pvectl setup") {
		t.Errorf("Config check = %+v", got)
	}
}

func TestDoctorPropagatesOutputFailure(t *testing.T) {
	oldDir := config.ConfigDir
	dir := t.TempDir()
	config.ConfigDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { config.ConfigDir = oldDir })
	jsonOutput = false
	doctorCmd.SetOut(failingDoctorWriter{})
	t.Cleanup(func() { doctorCmd.SetOut(nil) })
	if err := doctorCmd.RunE(doctorCmd, nil); err == nil || !strings.Contains(err.Error(), "output unavailable") {
		t.Errorf("RunE() error = %v, want output failure", err)
	}
}
