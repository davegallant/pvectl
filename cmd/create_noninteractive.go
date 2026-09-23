package cmd

import (
	"fmt"
	"sort"
	"strings"
)

func validateNonInteractiveCreate(inputs map[string]string) error {
	var missing []string
	for name, value := range inputs {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, "--"+name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("--non-interactive requires %s", strings.Join(missing, ", "))
}

func validateNonInteractivePassword(value string) error {
	if value == "-" {
		return fmt.Errorf("--cipassword - requires a terminal prompt and cannot be used with --non-interactive")
	}
	return nil
}
