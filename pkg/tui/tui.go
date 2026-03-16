package tui

import (
	"encoding/json"
	"strings"
)

func formatSelectionOutput(keys []string, outputJSON bool) (string, error) {
	if len(keys) == 0 {
		return "", nil
	}
	if outputJSON {
		b, err := json.Marshal(keys)
		if err != nil {
			return "", err
		}
		return string(b) + "\n", nil
	}
	return strings.Join(keys, "\n") + "\n", nil
}
