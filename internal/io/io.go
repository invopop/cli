// Package io provides input/output helpers for the CLI.
package io

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ghodss/yaml"
)

// ReadInput reads data from a file path or stdin. If filePath is empty, reads from stdin.
func ReadInput(filePath string) (json.RawMessage, error) {
	var data []byte
	var err error

	if filePath != "" {
		data, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading file %s: %w", filePath, err)
		}
	} else {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no input data provided")
	}

	// Try to detect YAML and convert to JSON
	if !json.Valid(data) {
		jsonData, err := yaml.YAMLToJSON(data)
		if err != nil {
			return nil, fmt.Errorf("converting YAML to JSON: %w", err)
		}
		data = jsonData
	}

	return json.RawMessage(data), nil
}

// PrintJSON outputs a value as formatted JSON to stdout.
func PrintJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// PrintRawJSON outputs raw JSON data formatted to stdout.
func PrintRawJSON(data json.RawMessage) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		// Fall back to raw output
		_, err := os.Stdout.Write(data)
		return err
	}
	return PrintJSON(v)
}
