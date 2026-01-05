package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	hfEndpoint = "https://router.huggingface.co/v1/chat/completions"
	hfModel    = "meta-llama/Llama-3.1-8B-Instruct"
)

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func transformWithHuggingFace(prompt string) (string, error) {
	raw, err := postChatCompletion(prompt)
	if err != nil {
		return "", err
	}

	var resp chatResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("huggingface api returned no choices")
	}
	translated := strings.TrimSpace(resp.Choices[0].Message.Content)
	if translated == "" {
		return "", fmt.Errorf("huggingface api returned empty translation")
	}
	return translated, nil
}

func postChatCompletion(prompt string) (string, error) {
	token, err := loadHuggingFaceToken()
	if err != nil {
		return "", err
	}

	payload := chatRequest{
		Model: hfModel,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hfEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("huggingface api status: %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	return string(respBody), nil
}

func loadHuggingFaceToken() (string, error) {
	if token := readEnv("HUGGINGFACE_TOKEN", "HF_TOKEN"); token != "" {
		return token, nil
	}

	if err := loadDotEnv(); err != nil {
		return "", err
	}

	if token := readEnv("HUGGINGFACE_TOKEN", "HF_TOKEN"); token != "" {
		return token, nil
	}

	return "", fmt.Errorf("missing Hugging Face token: set HUGGINGFACE_TOKEN or HF_TOKEN")
}
