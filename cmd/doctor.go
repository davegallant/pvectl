package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/davegallant/pvectl/internal/config"
	"github.com/spf13/cobra"
)

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Hint   string `json:"hint,omitempty"`
}

type doctorReport struct {
	Checks []doctorCheck `json:"checks"`
}

func (r doctorReport) failedCount() int {
	count := 0
	for _, check := range r.Checks {
		if check.Status == "error" {
			count++
		}
	}
	return count
}

var doctorCmd = &cobra.Command{
	Use:         "doctor",
	Short:       "Check local setup, API connectivity, and effective Proxmox permissions",
	Annotations: mutationAnnotation(mutationSafe),
	Args:        cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		report := diagnose(cmd.Context())
		if jsonOutput {
			if err := printJSON(report); err != nil {
				return err
			}
		} else {
			for _, check := range report.Checks {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s — %s\n", check.Name, check.Status, check.Detail); err != nil {
					return fmt.Errorf("writing doctor report: %w", err)
				}
				if check.Hint != "" {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Hint: %s\n", check.Hint); err != nil {
						return fmt.Errorf("writing doctor report: %w", err)
					}
				}
			}
		}
		if n := report.failedCount(); n > 0 {
			return fmt.Errorf("doctor found %d failed check(s)", n)
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(doctorCmd) }

// diagnose deliberately performs only GET requests. The version check
// establishes reachability and authentication, but not token privileges.
// Subsequent probes remain independent so a denied permissions endpoint
// does not hide a separately useful resource-visibility result.
func diagnose(ctx context.Context) doctorReport {
	report := doctorReport{}
	add := func(name, status, detail, hint string) {
		report.Checks = append(report.Checks, doctorCheck{Name: name, Status: status, Detail: detail, Hint: hint})
	}
	path, err := config.Path()
	if err != nil {
		add("Config", "error", err.Error(), "Check your OS config directory")
		return report
	}
	add("Config path", "ok", path, "")
	cfg, err := config.Load()
	if err != nil {
		add("Config", "error", err.Error(), "Run 'pvectl setup' to configure this cluster")
		return report
	}
	backend := cfg.SecretBackend
	if backend == "" {
		backend = "keyring"
	}
	add("Secret backend", "ok", backend, "")
	switch {
	case strings.HasPrefix(cfg.Host, "https://") && cfg.InsecureSkipVerify:
		add("TLS", "warning", "certificate verification disabled", "Re-run setup without --insecure-skip-verify when the cluster certificate is trusted")
	case strings.HasPrefix(cfg.Host, "https://"):
		add("TLS", "warning", "certificate validation not checked", "A successful HTTPS API request will verify the certificate")
	default:
		add("TLS", "warning", "connection is not HTTPS", "Use an https:// Proxmox API URL")
	}
	secret, err := secretStoreFor(cfg).Get(cfg.Host)
	if err != nil {
		add("Credentials", "error", err.Error(), "Run 'pvectl setup' to store a usable token")
		return report
	}
	client := api.NewClient(cfg.Host, cfg.TokenID, secret, cfg.InsecureSkipVerify)
	client.SetDebug(debug)
	version, err := client.Version(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "TLS verify failed") {
			for i := range report.Checks {
				if report.Checks[i].Name == "TLS" {
					report.Checks[i].Status = "error"
					report.Checks[i].Detail = "certificate verification failed"
					report.Checks[i].Hint = "Trust the cluster certificate or re-run setup with --insecure-skip-verify for a self-signed cluster"
					break
				}
			}
		}
		add("API", "error", err.Error(), "Check the URL, certificate, and token; re-run 'pvectl setup' if needed")
		return report
	}
	if strings.HasPrefix(cfg.Host, "https://") && !cfg.InsecureSkipVerify {
		for i := range report.Checks {
			if report.Checks[i].Name == "TLS" {
				report.Checks[i].Status = "ok"
				report.Checks[i].Detail = "certificate verified"
				report.Checks[i].Hint = ""
				break
			}
		}
	}
	add("API", "ok", "Proxmox VE "+version, "")

	raw, err := client.RawRequest(ctx, http.MethodGet, "/access/permissions", nil)
	if err != nil {
		add("Permissions", "error", err.Error(), "Check the token ACL and Privilege Separation settings")
	} else {
		var envelope struct {
			Data map[string]map[string]json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			add("Permissions", "error", "invalid permissions response: "+err.Error(), "Check Proxmox API compatibility")
		} else if len(envelope.Data) == 0 {
			add("Permissions", "error", "no effective grants reported", "Check token ACL and Privilege Separation settings")
		} else {
			add("Permissions", "ok", fmt.Sprintf("effective grants reported on %d path(s)", len(envelope.Data)), "")
			for _, privilege := range []string{
				"VM.Audit", "VM.PowerMgmt", "VM.Config.Options", "VM.Config.Disk", "VM.Config.Network",
				"VM.Allocate", "VM.Backup", "VM.Snapshot", "VM.Clone", "VM.Migrate", "VM.Console",
				"Datastore.Audit", "Datastore.AllocateSpace", "Datastore.AllocateTemplate",
				"Sys.Audit", "Sys.PowerMgmt",
			} {
				var paths []string
				for path, grants := range envelope.Data {
					if value, ok := grants[privilege]; ok && len(value) > 0 && string(value) != "null" {
						// Proxmox's value is a propagation flag, not an
						// allow/deny boolean: 0 grants exactly this path.
						if string(value) == "0" || string(value) == "false" {
							path += " (not propagated)"
						}
						paths = append(paths, path)
					}
				}
				if len(paths) == 0 {
					add(privilege, "warning", "not granted on any reported path", "This may be intentional; grant only the paths and roles your workflow needs")
				} else {
					sort.Strings(paths)
					add(privilege, "ok", "granted on "+strings.Join(paths, ", "), "Grants are path-specific; inspect the exact target path before an operation")
				}
			}
		}
	}

	resources, err := client.ClusterResources(ctx)
	if err != nil {
		add("Guests", "error", err.Error(), "Check resource-view permissions on the relevant VM and node paths")
	} else {
		total := resources.Containers.Total + resources.VMs.Total
		if total == 0 {
			add("Guests", "warning", "0 visible guests; this does not prove the cluster is empty", "Check VM.Audit grants and Privilege Separation if guests were expected")
		} else {
			add("Guests", "ok", fmt.Sprintf("%d visible guest(s): %d CT, %d VM", total, resources.Containers.Total, resources.VMs.Total), "")
		}
	}
	return report
}
