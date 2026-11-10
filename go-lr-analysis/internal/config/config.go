package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultBaseURL = "https://intervals.icu"

type Config struct {
	APIKey    string
	AthleteID string
	BaseURL   string
}

func Load(explicitPath string) (Config, error) {
	fileValues := map[string]string{}
	path := explicitPath
	if path == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			candidate := filepath.Join(dir, "lr-analysis", "config.toml")
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
			}
		}
	}
	if path != "" {
		values, err := readSimpleTOML(path)
		if err != nil {
			return Config{}, err
		}
		fileValues = values
	}

	config := Config{
		APIKey:    firstNonEmpty(os.Getenv("INTERVALS_API_KEY"), fileValues["api_key"]),
		AthleteID: firstNonEmpty(os.Getenv("INTERVALS_ATHLETE_ID"), fileValues["athlete_id"]),
		BaseURL:   firstNonEmpty(os.Getenv("INTERVALS_BASE_URL"), fileValues["base_url"], defaultBaseURL),
	}
	if config.APIKey == "" {
		return Config{}, fmt.Errorf("missing API key: set INTERVALS_API_KEY or api_key in the config file")
	}
	if config.AthleteID == "" {
		return Config{}, fmt.Errorf("missing athlete ID: set INTERVALS_ATHLETE_ID or athlete_id in the config file")
	}
	if !safeID(config.AthleteID) {
		return Config{}, fmt.Errorf("athlete ID contains unsupported characters")
	}
	if !strings.HasPrefix(config.BaseURL, "https://") && !strings.HasPrefix(config.BaseURL, "http://localhost") {
		return Config{}, fmt.Errorf("base_url must use HTTPS (HTTP is allowed only for localhost testing)")
	}
	config.BaseURL = strings.TrimRight(config.BaseURL, "/")
	return config, nil
}

func readSimpleTOML(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("parse config file %s: invalid line %q", path, line)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
			return nil, fmt.Errorf("parse config file %s: %s must be a quoted string", path, key)
		}
		values[key] = strings.Trim(value, "\"")
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}
	return values, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func safeID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return false
		}
	}
	return true
}
