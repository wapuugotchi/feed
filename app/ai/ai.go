package ai

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"wapuugotchi/feed/app/env"
)

const (
	githubEndpoint = "https://models.inference.ai.azure.com"
	githubModel    = "gpt-4o-mini"
)

// TransformText nimmt ein Prompt-Pattern und Text, baut den finalen Prompt und ruft GitHub Models auf.
func TransformText(pattern, text string) (string, error) {
	prompt := buildPrompt(pattern, text)
	return transformWithGitHub(prompt)
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

func transformWithGitHub(prompt string) (string, error) {
	token, err := loadGitHubToken()
	if err != nil {
		return "", err
	}

	cfg := openai.DefaultConfig(token)
	cfg.BaseURL = githubEndpoint
	client := openai.NewClientWithConfig(cfg)

	resp, err := client.CreateChatCompletion(context.Background(),
		openai.ChatCompletionRequest{
			Model: githubModel,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("github models api: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("github models api returned no choices")
	}
	result := strings.TrimSpace(resp.Choices[0].Message.Content)
	if result == "" {
		return "", fmt.Errorf("github models api returned empty response")
	}
	return result, nil
}

func loadGitHubToken() (string, error) {
	if token := env.ReadEnv("GH_MODELS_TOKEN"); token != "" {
		return token, nil
	}
	if err := env.LoadDotEnv(); err != nil {
		return "", err
	}
	if token := env.ReadEnv("GH_MODELS_TOKEN"); token != "" {
		return token, nil
	}
	return "", fmt.Errorf("missing GitHub token: set GITHUB_TOKEN or GH_MODELS_TOKEN")
}
