package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func TransformText(pattern, text string) (string, error) {
	prompt := buildPrompt(pattern, text)
	provider := strings.ToLower(strings.TrimSpace(getProvider()))
	switch provider {
	case "", "huggingface":
		return transformWithHuggingFace(prompt)
	default:
		return "", fmt.Errorf("unknown ai provider: %s", provider)
	}
}

func buildPrompt(pattern, text string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return text
	}
	if strings.Contains(pattern, "%s") {
		return fmt.Sprintf(pattern, text)
	}
	return pattern + text
}

func getProvider() string {
	if val := readEnv("AI_PROVIDER"); val != "" {
		return val
	}
	_ = loadDotEnv()
	return readEnv("AI_PROVIDER")
}

func readEnv(keys ...string) string {
	for _, key := range keys {
		if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

func loadDotEnv() error {
	root := findRepoRoot()
	envPath := filepath.Join(root, ".env")

	data, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if key == "" || val == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, val); err != nil {
				return err
			}
		}
	}

	return nil
}

func findRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}
