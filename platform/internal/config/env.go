package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func LoadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open environment file: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid environment file line")
		}
		key = strings.TrimSpace(key)
		if key != "MOZI_DB" && key != "MOZI_PLATFORM_DB" {
			continue
		}
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, strings.Trim(strings.TrimSpace(value), `"'`)); err != nil {
				return fmt.Errorf("set %s: %w", key, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read environment file: %w", err)
	}
	return nil
}
