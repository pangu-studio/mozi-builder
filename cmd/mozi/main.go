// mozi is the CLI tool for the Model-Driven Development Platform.
// It provides commands for model initialization, validation, code generation, and diff analysis.
//
// Usage:
//
//	mozi init                      Initialize models directory
//	mozi validate                  Validate all model definitions
//	mozi gen --model User          Generate code for a specific model
//	mozi gen --all                 Generate code for all models
//	mozi diff --model User         Show changes since last generation
package main

import (
	"github.com/pangu-sutido/mozi-builder/cmd/mozi/cmd"
)

func main() {
	cmd.Execute()
}
