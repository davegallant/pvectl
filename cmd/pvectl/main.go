package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/davegallant/pvectl/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
