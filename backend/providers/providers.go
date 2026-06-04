package providers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ProviderConfig struct {
	Type        string  `json:"type"` // "openai", "anthropic", "gemini", "deepseek", "custom", "ollama", "lmstudio"
	APIKey      string  `json:"api_key,omitempty"`
	BaseURL     string  `json:"base_url,omitempty"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
}

func GetRecommendationSystemPrompt() string {
	return "You are an SRE Storage Analytics assistant. Help the user optimize their disk space. " +
		"You MUST analyze the target Operating System and Connection Type provided in the request to adjust your recommendations accordingly (be extremely cautious with critical system folders like C:\\Windows on Windows or /boot, /etc, /var on Linux). " +
		"You MUST write your response entirely in Ukrainian language. " +
		"For each specific file path you recommend to delete, you MUST append a markdown link next to it in the exact format: " +
		"[Видалити](delete://<absolute_path>). For example: " +
		"'- C:\\Users\\Admin\\AppData\\Local\\Temp\\test.log (12.4 MB) - [Видалити](delete://C:/Users/Admin/AppData/Local/Temp/test.log)'. " +
		"CRITICAL: You MUST NOT generate delete:// links for system files, libraries (.dll, .so), application binaries (.exe), database files, configuration files, or active application data/models (like Ollama libraries, Chrome models, pagefile.sys, game files) even if they take up a lot of space, as deleting them can break the target operating system or applications. Only generate delete:// links for temporary files (Temp) and log files (Log) that are 100% safe to delete without breaking any software. " +
		"Analyze any SRE Metrics (Docker containers, volumes, Windows system folders like minidumps or IIS logs, and duplicate files space waste) provided in the request. Give detailed professional SRE recommendations on how to handle them. " +
		"IMPORTANT AUTOMATION RULES: " +
		"1. If you recommend to clear Docker system space, you MUST append this exact button link next to the recommendation: [🐳 Виконати Prune Docker](action://prune-docker). " +
		"2. If you recommend to clear Linux Journald logs, you MUST append this exact button link next to the recommendation: [🧹 Очистити Journald](action://vacuum-journald)."
}

func TestConnection(cfg ProviderConfig) (bool, string) {
	client := &http.Client{Timeout: 10 * time.Second}
	var req *http.Request
	var err error

	switch cfg.Type {
	case "openai":
		if cfg.APIKey == "" {
			return false, "API key is missing."
		}
		url := "https://api.openai.com/v1/chat/completions"
		payload := map[string]interface{}{
			"model":      cfg.Model,
			"messages":   []map[string]string{{"role": "user", "content": "Ping"}},
			"max_tokens": 5,
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		req.Header.Set("Content-Type", "application/json")

	case "deepseek":
		if cfg.APIKey == "" {
			return false, "API key is missing."
		}
		url := "https://api.deepseek.com/chat/completions"
		payload := map[string]interface{}{
			"model":      cfg.Model,
			"messages":   []map[string]string{{"role": "user", "content": "Ping"}},
			"max_tokens": 5,
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		req.Header.Set("Content-Type", "application/json")

	case "custom":
		baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
		url := baseURL + "/chat/completions"
		payload := map[string]interface{}{
			"model":      cfg.Model,
			"messages":   []map[string]string{{"role": "user", "content": "Ping"}},
			"max_tokens": 5,
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		if cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
		req.Header.Set("Content-Type", "application/json")

	case "anthropic":
		if cfg.APIKey == "" {
			return false, "API key is missing."
		}
		url := "https://api.anthropic.com/v1/messages"
		payload := map[string]interface{}{
			"model":      cfg.Model,
			"messages":   []map[string]string{{"role": "user", "content": "Ping"}},
			"max_tokens": 5,
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		req.Header.Set("x-api-key", cfg.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("Content-Type", "application/json")

	case "gemini":
		if cfg.APIKey == "" {
			return false, "API key is missing."
		}
		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", cfg.Model, cfg.APIKey)
		payload := map[string]interface{}{
			"contents": []map[string]interface{}{{
				"parts": []map[string]string{{"text": "Ping"}},
			}},
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

	case "ollama":
		baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		url := baseURL + "/api/tags"
		req, _ = http.NewRequest("GET", url, nil)

	case "lmstudio":
		baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
		if baseURL == "" {
			baseURL = "http://localhost:1234/v1"
		}
		url := baseURL + "/models"
		req, _ = http.NewRequest("GET", url, nil)

	default:
		return false, "Unknown provider type."
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("Connection failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		if cfg.Type == "ollama" {
			return true, "Connected to Ollama!"
		} else if cfg.Type == "lmstudio" {
			return true, "Connected to LM Studio!"
		}
		return true, "API connection successful!"
	}

	respBody, _ := io.ReadAll(resp.Body)
	return false, fmt.Sprintf("Error (HTTP %d): %s", resp.StatusCode, string(respBody))
}

func QueryAI(cfg ProviderConfig, system string, user string) (string, error) {
	client := &http.Client{Timeout: 90 * time.Second}
	var req *http.Request
	var err error

	temp := cfg.Temperature
	if temp == 0 {
		temp = 0.7
	}

	switch cfg.Type {
	case "openai", "deepseek", "custom":
		url := "https://api.openai.com/v1/chat/completions"
		if cfg.Type == "deepseek" {
			url = "https://api.deepseek.com/chat/completions"
		} else if cfg.Type == "custom" {
			url = strings.TrimSuffix(cfg.BaseURL, "/") + "/chat/completions"
		}

		payload := map[string]interface{}{
			"model": cfg.Model,
			"messages": []map[string]string{
				{"role": "system", "content": system},
				{"role": "user", "content": user},
			},
			"temperature": temp,
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		if cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
		req.Header.Set("Content-Type", "application/json")

	case "anthropic":
		url := "https://api.anthropic.com/v1/messages"
		payload := map[string]interface{}{
			"model":       cfg.Model,
			"system":      system,
			"messages":    []map[string]string{{"role": "user", "content": user}},
			"max_tokens":  4096,
			"temperature": temp,
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		req.Header.Set("x-api-key", cfg.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("Content-Type", "application/json")

	case "gemini":
		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", cfg.Model, cfg.APIKey)
		payload := map[string]interface{}{
			"contents": []map[string]interface{}{{
				"parts": []map[string]string{{"text": user}},
			}},
			"systemInstruction": map[string]interface{}{
				"parts": []map[string]string{{"text": system}},
			},
			"generationConfig": map[string]interface{}{
				"temperature": temp,
			},
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

	case "ollama":
		baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		url := baseURL + "/api/chat"
		payload := map[string]interface{}{
			"model": cfg.Model,
			"messages": []map[string]string{
				{"role": "system", "content": system},
				{"role": "user", "content": user},
			},
			"stream": false,
			"options": map[string]interface{}{
				"temperature": temp,
			},
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

	case "lmstudio":
		baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
		if baseURL == "" {
			baseURL = "http://localhost:1234/v1"
		}
		url := baseURL + "/chat/completions"
		payload := map[string]interface{}{
			"model": cfg.Model,
			"messages": []map[string]string{
				{"role": "system", "content": system},
				{"role": "user", "content": user},
			},
			"temperature": temp,
			"stream":      false,
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

	default:
		return "", errors.New("unknown provider type")
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API Error (HTTP %d): %s", resp.StatusCode, string(respBytes))
	}

	// Parse response content
	switch cfg.Type {
	case "openai", "deepseek", "custom", "lmstudio":
		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return "", err
		}
		if len(result.Choices) > 0 {
			return result.Choices[0].Message.Content, nil
		}

	case "anthropic":
		var result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return "", err
		}
		if len(result.Content) > 0 {
			return result.Content[0].Text, nil
		}

	case "gemini":
		var result struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return "", err
		}
		if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
			return result.Candidates[0].Content.Parts[0].Text, nil
		}

	case "ollama":
		var result struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return "", err
		}
		return result.Message.Content, nil
	}

	return "", errors.New("empty or unparseable response from provider")
}

type ChatMessage struct {
	Role    string `json:"role"` // "user", "assistant"
	Content string `json:"content"`
}

func QueryAIChat(cfg ProviderConfig, system string, history []ChatMessage) (string, error) {
	client := &http.Client{Timeout: 90 * time.Second}
	var req *http.Request
	var err error

	temp := cfg.Temperature
	if temp == 0 {
		temp = 0.7
	}

	switch cfg.Type {
	case "openai", "deepseek", "custom":
		url := "https://api.openai.com/v1/chat/completions"
		if cfg.Type == "deepseek" {
			url = "https://api.deepseek.com/chat/completions"
		} else if cfg.Type == "custom" {
			url = strings.TrimSuffix(cfg.BaseURL, "/") + "/chat/completions"
		}

		messages := []map[string]string{
			{"role": "system", "content": system},
		}
		for _, msg := range history {
			messages = append(messages, map[string]string{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}

		payload := map[string]interface{}{
			"model":       cfg.Model,
			"messages":    messages,
			"temperature": temp,
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		if cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
		req.Header.Set("Content-Type", "application/json")

	case "anthropic":
		url := "https://api.anthropic.com/v1/messages"
		messages := []map[string]string{}
		for _, msg := range history {
			messages = append(messages, map[string]string{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}
		payload := map[string]interface{}{
			"model":       cfg.Model,
			"system":      system,
			"messages":    messages,
			"max_tokens":  4096,
			"temperature": temp,
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		req.Header.Set("x-api-key", cfg.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("Content-Type", "application/json")

	case "gemini":
		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", cfg.Model, cfg.APIKey)
		contents := []map[string]interface{}{}
		for _, msg := range history {
			role := msg.Role
			if role == "assistant" {
				role = "model"
			}
			contents = append(contents, map[string]interface{}{
				"role": role,
				"parts": []map[string]string{
					{"text": msg.Content},
				},
			})
		}
		payload := map[string]interface{}{
			"contents": contents,
			"systemInstruction": map[string]interface{}{
				"parts": []map[string]string{{"text": system}},
			},
			"generationConfig": map[string]interface{}{
				"temperature": temp,
			},
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

	case "ollama":
		baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		url := baseURL + "/api/chat"
		messages := []map[string]string{
			{"role": "system", "content": system},
		}
		for _, msg := range history {
			messages = append(messages, map[string]string{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}
		payload := map[string]interface{}{
			"model":    cfg.Model,
			"messages": messages,
			"stream":   false,
			"options": map[string]interface{}{
				"temperature": temp,
			},
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

	case "lmstudio":
		baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
		if baseURL == "" {
			baseURL = "http://localhost:1234/v1"
		}
		url := baseURL + "/chat/completions"
		messages := []map[string]string{
			{"role": "system", "content": system},
		}
		for _, msg := range history {
			messages = append(messages, map[string]string{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}
		payload := map[string]interface{}{
			"model":       cfg.Model,
			"messages":    messages,
			"temperature": temp,
			"stream":      false,
		}
		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

	default:
		return "", errors.New("unknown provider type")
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API Error (HTTP %d): %s", resp.StatusCode, string(respBytes))
	}

	// Parse response content
	switch cfg.Type {
	case "openai", "deepseek", "custom", "lmstudio":
		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return "", err
		}
		if len(result.Choices) > 0 {
			return result.Choices[0].Message.Content, nil
		}

	case "anthropic":
		var result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return "", err
		}
		if len(result.Content) > 0 {
			return result.Content[0].Text, nil
		}

	case "gemini":
		var result struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return "", err
		}
		if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
			return result.Candidates[0].Content.Parts[0].Text, nil
		}

	case "ollama":
		var result struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return "", err
		}
		return result.Message.Content, nil
	}

	return "", errors.New("empty or unparseable response from provider")
}

func GetAvailableModels(cfg ProviderConfig) ([]string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	var req *http.Request

	switch cfg.Type {
	case "openai":
		if cfg.APIKey == "" {
			return nil, errors.New("OpenAI API key is missing")
		}
		url := "https://api.openai.com/v1/models"
		req, _ = http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	case "deepseek":
		if cfg.APIKey == "" {
			return nil, errors.New("DeepSeek API key is missing")
		}
		url := "https://api.deepseek.com/models"
		req, _ = http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	case "custom":
		url := strings.TrimSuffix(cfg.BaseURL, "/") + "/models"
		req, _ = http.NewRequest("GET", url, nil)
		if cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}

	case "anthropic":
		if cfg.APIKey == "" {
			return nil, errors.New("Anthropic API key is missing")
		}
		url := "https://api.anthropic.com/v1/models"
		req, _ = http.NewRequest("GET", url, nil)
		req.Header.Set("x-api-key", cfg.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")

	case "gemini":
		if cfg.APIKey == "" {
			return nil, errors.New("Gemini API key is missing")
		}
		url := "https://generativelanguage.googleapis.com/v1beta/models?key=" + cfg.APIKey
		req, _ = http.NewRequest("GET", url, nil)

	case "ollama":
		url := strings.TrimSuffix(cfg.BaseURL, "/") + "/api/tags"
		if cfg.BaseURL == "" {
			url = "http://localhost:11434/api/tags"
		}
		req, _ = http.NewRequest("GET", url, nil)

	case "lmstudio":
		url := strings.TrimSuffix(cfg.BaseURL, "/") + "/models"
		if cfg.BaseURL == "" {
			url = "http://localhost:1234/v1/models"
		}
		req, _ = http.NewRequest("GET", url, nil)

	default:
		return nil, errors.New("unknown provider type")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("models fetch failed (HTTP %d): %s", resp.StatusCode, string(respBytes))
	}

	models := make([]string, 0)

	switch cfg.Type {
	case "openai", "deepseek", "custom", "lmstudio":
		var result struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return nil, err
		}
		for _, m := range result.Data {
			models = append(models, m.ID)
		}

	case "anthropic":
		var result struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return nil, err
		}
		for _, m := range result.Data {
			models = append(models, m.ID)
		}

	case "gemini":
		var result struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return nil, err
		}
		for _, m := range result.Models {
			name := strings.TrimPrefix(m.Name, "models/")
			models = append(models, name)
		}

	case "ollama":
		var result struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return nil, err
		}
		for _, m := range result.Models {
			models = append(models, m.Name)
		}
	}

	return models, nil
}
