package cmd

import "strings"

var genModule string

func parseModelRef(ref string) (module, model string) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ref
}
