package cmd

import "context"

// commandContext is used by helpers called outside a Cobra RunE. It
// respects ExecuteContext while retaining a background fallback for
// direct unit calls that have no root command context installed.
func commandContext() context.Context {
	if ctx := rootCmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}
