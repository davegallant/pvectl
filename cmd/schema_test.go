package cmd

import "testing"

func TestSchemaCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"schema"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("schema") error = %v`, err)
	}
	if found.Use != "schema" {
		t.Errorf(`Find("schema").Use = %q, want "schema"`, found.Use)
	}
}

func TestBuildSchemaIncludesGlobalFlagsAndCommands(t *testing.T) {
	s := buildSchema(rootCmd)

	if s.Name != "pvectl" {
		t.Errorf("buildSchema().Name = %q, want %q", s.Name, "pvectl")
	}

	var sawOutputFlag bool
	for _, f := range s.GlobalFlags {
		if f.Name == "output" {
			sawOutputFlag = true
		}
	}
	if !sawOutputFlag {
		t.Errorf("buildSchema().GlobalFlags = %+v, want it to include the --output flag", s.GlobalFlags)
	}

	var ct *schemaCommand
	for i := range s.Commands {
		if s.Commands[i].Name == "ct" {
			ct = &s.Commands[i]
		}
	}
	if ct == nil {
		t.Fatalf("buildSchema().Commands = %+v, want a \"ct\" entry", s.Commands)
	}

	var sawList bool
	for _, sub := range ct.Commands {
		if sub.Name == "list" {
			sawList = true
			var sawNodeFlag bool
			for _, f := range sub.Flags {
				if f.Name == "node" {
					sawNodeFlag = true
				}
			}
			if !sawNodeFlag {
				t.Errorf(`"ct list".Flags = %+v, want it to include "--node"`, sub.Flags)
			}
		}
	}
	if !sawList {
		t.Errorf(`"ct".Commands = %+v, want a "list" entry`, ct.Commands)
	}
}

func TestValidateOutputFormatRejectsUnknownValue(t *testing.T) {
	orig := outputFormat
	defer func() { outputFormat = orig }()

	outputFormat = "yaml"
	if err := validateOutputFormat(rootCmd, nil); err == nil {
		t.Error("validateOutputFormat() error = nil, want error for --output yaml")
	}
}

func TestValidateOutputFormatAcceptsTableAndJSON(t *testing.T) {
	orig := outputFormat
	defer func() { outputFormat = orig }()

	outputFormat = "table"
	if err := validateOutputFormat(rootCmd, nil); err != nil {
		t.Errorf(`validateOutputFormat() with --output "table" error = %v, want nil`, err)
	}
	if jsonOutput {
		t.Error("jsonOutput = true after --output table, want false")
	}

	outputFormat = "json"
	if err := validateOutputFormat(rootCmd, nil); err != nil {
		t.Errorf(`validateOutputFormat() with --output "json" error = %v, want nil`, err)
	}
	if !jsonOutput {
		t.Error("jsonOutput = false after --output json, want true")
	}
}

func TestValidateOutputFormatRejectsEmpty(t *testing.T) {
	orig := outputFormat
	defer func() { outputFormat = orig }()

	outputFormat = ""
	if err := validateOutputFormat(rootCmd, nil); err == nil {
		t.Error(`validateOutputFormat() error = nil, want error for --output ""`)
	}
}

func TestSchemaSubcommandsSkipsHelp(t *testing.T) {
	for _, c := range schemaSubcommands(rootCmd) {
		if c.Name == "help" {
			t.Errorf("schemaSubcommands(rootCmd) included cobra's auto-generated \"help\" command, want it skipped")
		}
	}
}
