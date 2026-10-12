package main

import (
	"bytes"
	"testing"
)

func TestRootCommandExposesAPIAndLocalCommands(t *testing.T) {
	t.Parallel()

	root := newRootCommand()
	for _, name := range []string{"serve", "classify", "export"} {
		if command, _, err := root.Find([]string{name}); err != nil || command == root {
			t.Fatalf("command %q is not registered: %v", name, err)
		}
	}

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("root help: %v", err)
	}
	if output.Len() == 0 {
		t.Fatal("root help did not produce output")
	}
}
