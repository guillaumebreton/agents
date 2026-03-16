package main

import (
	"notb.re/agents/cmd"

	// Register coding agent implementations.
	_ "notb.re/agents/internal/coding"
)

func main() {
	cmd.Execute()
}
