package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wapuugotchi/feed/app/ai"
)

// TransformTextByAi picks the active AI provider and returns the translation.
func TransformTextByAi(text string) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(getAIProvider()))
	switch provider {
	case "huggingface":
		return huggingface.TransformTextByHuggingFace(text)
	case "openai":
		return "", fmt.Errorf("openai provider not implemented")
	default:
		return "", fmt.Errorf("unknown ai provider: %s", provider)
	}
}

func getAIProvider() string {
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
