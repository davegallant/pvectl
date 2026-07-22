package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// apiDataParams backs each `api` subcommand's repeatable --data key=value
// flag. A single package-level var is safe here since only one `api`
// subcommand runs per invocation — same pattern as ctConfigAppendLines
// (cmd/ct_config.go).
var apiDataParams []string

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Make a raw Proxmox API call — an escape hatch for endpoints pvectl has no dedicated command for",
}

var apiGetCmd = &cobra.Command{
	Use:         "get <path>",
	Short:       "GET a raw Proxmox API path (e.g. /nodes, /cluster/status)",
	Args:        requireArgs("path"),
	Annotations: mutationAnnotation(mutationSafe),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIRequest(http.MethodGet, args[0])
	},
}

var apiPostCmd = &cobra.Command{
	Use:   "post <path>",
	Short: "POST to a raw Proxmox API path",
	Args:  requireArgs("path"),
	// Unlike every other mutating command, `api post`/`put`/`delete` can
	// reach any endpoint — including ones with no dedicated pvectl command
	// specifically because they're destructive (delete a VM, wipe a
	// storage volume, ...). Same unbounded-blast-radius reasoning as `ct
	// exec`/`ct enter`/`qm enter` (see mutationDestructive's doc comment
	// in schema.go): the caller picked the path, so it gets the same tier
	// regardless of what that particular call happens to do.
	Annotations: mutationAnnotation(mutationDestructive),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIRequest(http.MethodPost, args[0])
	},
}

var apiPutCmd = &cobra.Command{
	Use:         "put <path>",
	Short:       "PUT to a raw Proxmox API path",
	Args:        requireArgs("path"),
	Annotations: mutationAnnotation(mutationDestructive),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIRequest(http.MethodPut, args[0])
	},
}

var apiDeleteCmd = &cobra.Command{
	Use:         "delete <path>",
	Short:       "DELETE a raw Proxmox API path",
	Args:        requireArgs("path"),
	Annotations: mutationAnnotation(mutationDestructive),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIRequest(http.MethodDelete, args[0])
	},
}

func init() {
	rootCmd.AddCommand(apiCmd)
	apiCmd.AddCommand(apiGetCmd, apiPostCmd, apiPutCmd, apiDeleteCmd)
	for _, c := range []*cobra.Command{apiGetCmd, apiPostCmd, apiPutCmd, apiDeleteCmd} {
		c.Flags().StringArrayVar(&apiDataParams, "data", nil, "a key=value parameter to send (repeatable)")
	}
}

// runAPIRequest issues method against path (normalized to start with "/")
// with apiDataParams as parameters, and prints the raw JSON response.
func runAPIRequest(method, path string) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	params, err := parseAPIData(apiDataParams)
	if err != nil {
		return err
	}

	client, err := loadClient()
	if err != nil {
		return friendlySetupError(err)
	}

	raw, err := client.RawRequest(context.Background(), method, path, params)
	if err != nil {
		return err
	}
	return printRawJSON(raw)
}

// parseAPIData parses each "key=value" entry in data into url.Values.
func parseAPIData(data []string) (url.Values, error) {
	params := url.Values{}
	for _, kv := range data {
		key, value, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --data %q: must be key=value", kv)
		}
		params.Add(key, value)
	}
	return params, nil
}

// printRawJSON indents and prints raw as-is. Unlike printJSON (which
// marshals an already-typed Go value), raw is Proxmox's own JSON reply —
// re-marshaling it through encoding/json would risk reformatting number
// literals, so it's indented in place instead. Prints nothing for a
// nil/empty reply (e.g. a DELETE with no response body).
func printRawJSON(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return fmt.Errorf("formatting response: %w", err)
	}
	buf.WriteByte('\n')
	_, err := os.Stdout.Write(buf.Bytes())
	return err
}
