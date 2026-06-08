package providers

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestGetRecommendationSystemPrompt(t *testing.T) {
	prompt := GetRecommendationSystemPrompt()
	if prompt == "" {
		t.Error("prompt should not be empty")
	}
	keywords := []string{"SRE", "Storage", "delete://", "Видалити", "Ukrainian", "action://"}
	for _, kw := range keywords {
		if !strings.Contains(prompt, kw) {
			t.Errorf("prompt should contain %q", kw)
		}
	}
}

func TestTestConnection_MissingAPIKey(t *testing.T) {
	types := []struct {
		name string
		cfg  ProviderConfig
	}{
		{"openai", ProviderConfig{Type: "openai"}},
		{"deepseek", ProviderConfig{Type: "deepseek"}},
		{"anthropic", ProviderConfig{Type: "anthropic"}},
		{"gemini", ProviderConfig{Type: "gemini"}},
	}
	for _, tt := range types {
		t.Run(tt.name, func(t *testing.T) {
			ok, msg := TestConnection(tt.cfg)
			if ok {
				t.Error("expected false")
			}
			if msg != "API key is missing." {
				t.Errorf("expected %q, got %q", "API key is missing.", msg)
			}
		})
	}
}

func TestTestConnection_UnknownType(t *testing.T) {
	ok, msg := TestConnection(ProviderConfig{Type: "nonexistent"})
	if ok {
		t.Error("expected false")
	}
	if msg != "Unknown provider type." {
		t.Errorf("expected %q, got %q", "Unknown provider type.", msg)
	}
}

func TestTestConnection_ProvidersWithoutKeyCheck(t *testing.T) {
	t.Run("custom", func(t *testing.T) {
		_, msg := TestConnection(ProviderConfig{Type: "custom", Model: "test"})
		if msg == "API key is missing." {
			t.Error("custom should not check API key")
		}
	})
	t.Run("ollama", func(t *testing.T) {
		_, msg := TestConnection(ProviderConfig{Type: "ollama", BaseURL: "http://127.0.0.1:1"})
		if msg == "API key is missing." {
			t.Error("ollama should not check API key")
		}
	})
	t.Run("lmstudio", func(t *testing.T) {
		_, msg := TestConnection(ProviderConfig{Type: "lmstudio", BaseURL: "http://127.0.0.1:1"})
		if msg == "API key is missing." {
			t.Error("lmstudio should not check API key")
		}
	})
}

func TestQueryAI_UnknownType(t *testing.T) {
	_, err := QueryAI(ProviderConfig{Type: "nonexistent"}, "system", "user")
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestQueryAIChat_UnknownType(t *testing.T) {
	_, err := QueryAIChat(ProviderConfig{Type: "nonexistent"}, "system", nil)
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestGetAvailableModels_MissingKey(t *testing.T) {
	types := []struct {
		name string
		cfg  ProviderConfig
	}{
		{"openai", ProviderConfig{Type: "openai"}},
		{"deepseek", ProviderConfig{Type: "deepseek"}},
		{"gemini", ProviderConfig{Type: "gemini"}},
		{"anthropic", ProviderConfig{Type: "anthropic"}},
	}
	for _, tt := range types {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetAvailableModels(tt.cfg)
			if err == nil {
				t.Error("expected error for missing API key")
			}
		})
	}
}

func TestGetAvailableModels_UnknownType(t *testing.T) {
	_, err := GetAvailableModels(ProviderConfig{Type: "nonexistent"})
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestGetAvailableModels_NoKeyCheckProviders(t *testing.T) {
	t.Run("ollama", func(t *testing.T) {
		_, err := GetAvailableModels(ProviderConfig{Type: "ollama", BaseURL: "http://127.0.0.1:1"})
		if err == nil {
			t.Error("expected error (no HTTP server)")
		}
	})
	t.Run("lmstudio", func(t *testing.T) {
		_, err := GetAvailableModels(ProviderConfig{Type: "lmstudio", BaseURL: "http://127.0.0.1:1"})
		if err == nil {
			t.Error("expected error (no HTTP server)")
		}
	})
	t.Run("custom", func(t *testing.T) {
		_, err := GetAvailableModels(ProviderConfig{Type: "custom", BaseURL: "http://127.0.0.1:1"})
		if err == nil {
			t.Error("expected error (no HTTP server)")
		}
	})
}

func TestProviderConfigJSONTags(t *testing.T) {
	cfg := ProviderConfig{
		Type:        "openai",
		APIKey:      "sk-test",
		BaseURL:     "https://example.com",
		Model:       "gpt-4",
		Temperature: 0.5,
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded ProviderConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Type != "openai" || decoded.Model != "gpt-4" || decoded.Temperature != 0.5 {
		t.Errorf("round-trip failed: got %+v", decoded)
	}
}

func TestProviderConfigOmitEmptyFields(t *testing.T) {
	t.Run("omit api_key", func(t *testing.T) {
		cfg := ProviderConfig{Type: "ollama", Model: "llama3", Temperature: 0.7}
		data, _ := json.Marshal(cfg)
		if strings.Contains(string(data), "api_key") {
			t.Errorf("api_key should be omitted when empty, got %s", string(data))
		}
	})
	t.Run("omit base_url", func(t *testing.T) {
		cfg := ProviderConfig{Type: "openai", Model: "gpt-4", Temperature: 0.7}
		data, _ := json.Marshal(cfg)
		if strings.Contains(string(data), "base_url") {
			t.Errorf("base_url should be omitted when empty, got %s", string(data))
		}
	})
}

func TestChatMessageJSONTags(t *testing.T) {
	msg := ChatMessage{Role: "user", Content: "hello"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded ChatMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Role != "user" || decoded.Content != "hello" {
		t.Errorf("round-trip failed: got %+v", decoded)
	}
}

func TestQueryAIResponseUnmarshal(t *testing.T) {
	raw := json.RawMessage(`{"choices":[{"message":{"content":"test response"}}]}`)
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Choices) == 0 {
		t.Fatal("expected at least one choice")
	}
	if result.Choices[0].Message.Content != "test response" {
		t.Errorf("expected %q, got %q", "test response", result.Choices[0].Message.Content)
	}
}

func TestEmptyQueryAIChoices(t *testing.T) {
	raw := json.RawMessage(`{"choices":[]}`)
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Choices) != 0 {
		t.Errorf("expected empty choices, got %d", len(result.Choices))
	}
}

func TestDefaultTemperature(t *testing.T) {
	if (ProviderConfig{Temperature: 0}).Temperature != 0 {
		t.Error("expected zero-value Temperature to be 0")
	}
}

func TestAllProviderTypesValid(t *testing.T) {
	valid := []string{"openai", "deepseek", "custom", "anthropic", "gemini", "ollama", "lmstudio"}
	for _, p := range valid {
		t.Run(p, func(t *testing.T) {
			cfg := ProviderConfig{Type: p}
			if p == "openai" || p == "deepseek" || p == "anthropic" || p == "gemini" {
				_, msg := TestConnection(cfg)
				if msg != "API key is missing." {
					t.Errorf("expected API key missing for %s, got %q", p, msg)
				}
			} else {
				_, msg := TestConnection(cfg)
				if msg == "API key is missing." || msg == "Unknown provider type." {
					t.Errorf("unexpected message for %s: %q", p, msg)
				}
			}
		})
	}
}

func TestGetAvailableModels_MissingKeyErrorMessages(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ProviderConfig
		partial string
	}{
		{"openai", ProviderConfig{Type: "openai"}, "OpenAI"},
		{"deepseek", ProviderConfig{Type: "deepseek"}, "DeepSeek"},
		{"anthropic", ProviderConfig{Type: "anthropic"}, "Anthropic"},
		{"gemini", ProviderConfig{Type: "gemini"}, "Gemini"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetAvailableModels(tt.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.partial) {
				t.Errorf("error %q should contain %q", err.Error(), tt.partial)
			}
		})
	}
}

func TestQueryAIChat_UnknownTypeErrorMessage(t *testing.T) {
	_, err := QueryAIChat(ProviderConfig{Type: "unsupported"}, "sys", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "unknown provider type" {
		t.Errorf("expected 'unknown provider type', got %q", err.Error())
	}
}

func TestQueryAI_UnknownTypeErrorMessage(t *testing.T) {
	_, err := QueryAI(ProviderConfig{Type: "unsupported"}, "sys", "usr")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "unknown provider type" {
		t.Errorf("expected 'unknown provider type', got %q", err.Error())
	}
}

func TestGetAvailableModels_UnknownTypeErrorMessage(t *testing.T) {
	_, err := GetAvailableModels(ProviderConfig{Type: "unsupported"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "unknown provider type" {
		t.Errorf("expected 'unknown provider type', got %q", err.Error())
	}
}

func TestTestConnection_UnknownTypeErrorMessage(t *testing.T) {
	_, msg := TestConnection(ProviderConfig{Type: "unsupported"})
	if msg != "Unknown provider type." {
		t.Errorf("expected 'Unknown provider type.', got %q", msg)
	}
}

func TestTestConnection_MissingAPIKeyErrorMessage(t *testing.T) {
	for _, ptype := range []string{"openai", "deepseek", "anthropic", "gemini"} {
		t.Run(ptype, func(t *testing.T) {
			_, msg := TestConnection(ProviderConfig{Type: ptype})
			if msg != "API key is missing." {
				t.Errorf("expected 'API key is missing.', got %q", msg)
			}
		})
	}
}

// parseQueryAIResponse replicates the response-parsing logic from QueryAI and QueryAIChat.
// It accepts a mock JSON body and a provider type and extracts the response content.
func parseQueryAIResponse(respJSON []byte, providerType string) (string, error) {
	switch providerType {
	case "openai", "deepseek", "custom", "lmstudio":
		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(respJSON, &result); err != nil {
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
		if err := json.Unmarshal(respJSON, &result); err != nil {
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
		if err := json.Unmarshal(respJSON, &result); err != nil {
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
		if err := json.Unmarshal(respJSON, &result); err != nil {
			return "", err
		}
		return result.Message.Content, nil

	default:
		return "", errors.New("unknown provider type")
	}

	return "", errors.New("empty or unparseable response from provider")
}

// parseAvailableModelsResponse replicates the response-parsing logic from GetAvailableModels.
// It accepts a mock JSON body and a provider type and extracts the model list.
func parseAvailableModelsResponse(respJSON []byte, providerType string) ([]string, error) {
	models := make([]string, 0)

	switch providerType {
	case "openai", "deepseek", "custom", "lmstudio":
		var result struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respJSON, &result); err != nil {
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
		if err := json.Unmarshal(respJSON, &result); err != nil {
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
		if err := json.Unmarshal(respJSON, &result); err != nil {
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
		if err := json.Unmarshal(respJSON, &result); err != nil {
			return nil, err
		}
		for _, m := range result.Models {
			models = append(models, m.Name)
		}

	default:
		return nil, errors.New("unknown provider type")
	}

	return models, nil
}

func TestParseQueryAI_OpenAIStyle(t *testing.T) {
	mock := `{"choices":[{"message":{"content":"Hello from OpenAI"}}]}`
	content, err := parseQueryAIResponse([]byte(mock), "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "Hello from OpenAI" {
		t.Errorf("expected %q, got %q", "Hello from OpenAI", content)
	}
}

func TestParseQueryAI_DeepSeekStyle(t *testing.T) {
	mock := `{"choices":[{"message":{"content":"Hello from DeepSeek"}}]}`
	content, err := parseQueryAIResponse([]byte(mock), "deepseek")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "Hello from DeepSeek" {
		t.Errorf("expected %q, got %q", "Hello from DeepSeek", content)
	}
}

func TestParseQueryAI_CustomStyle(t *testing.T) {
	mock := `{"choices":[{"message":{"content":"Hello from Custom API"}}]}`
	content, err := parseQueryAIResponse([]byte(mock), "custom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "Hello from Custom API" {
		t.Errorf("expected %q, got %q", "Hello from Custom API", content)
	}
}

func TestParseQueryAI_LMStudioStyle(t *testing.T) {
	mock := `{"choices":[{"message":{"content":"Hello from LM Studio"}}]}`
	content, err := parseQueryAIResponse([]byte(mock), "lmstudio")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "Hello from LM Studio" {
		t.Errorf("expected %q, got %q", "Hello from LM Studio", content)
	}
}

func TestParseQueryAI_AnthropicStyle(t *testing.T) {
	mock := `{"content":[{"text":"Hello from Anthropic"}]}`
	content, err := parseQueryAIResponse([]byte(mock), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "Hello from Anthropic" {
		t.Errorf("expected %q, got %q", "Hello from Anthropic", content)
	}
}

func TestParseQueryAI_GeminiStyle(t *testing.T) {
	mock := `{"candidates":[{"content":{"parts":[{"text":"Hello from Gemini"}]}}]}`
	content, err := parseQueryAIResponse([]byte(mock), "gemini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "Hello from Gemini" {
		t.Errorf("expected %q, got %q", "Hello from Gemini", content)
	}
}

func TestParseQueryAI_OllamaStyle(t *testing.T) {
	mock := `{"message":{"content":"Hello from Ollama"}}`
	content, err := parseQueryAIResponse([]byte(mock), "ollama")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "Hello from Ollama" {
		t.Errorf("expected %q, got %q", "Hello from Ollama", content)
	}
}

func TestParseQueryAI_EmptyChoices(t *testing.T) {
	mock := `{"choices":[]}`
	_, err := parseQueryAIResponse([]byte(mock), "openai")
	if err == nil {
		t.Error("expected error for empty choices array")
	}
}

func TestParseQueryAI_EmptyAnthropicContent(t *testing.T) {
	mock := `{"content":[]}`
	_, err := parseQueryAIResponse([]byte(mock), "anthropic")
	if err == nil {
		t.Error("expected error for empty content array")
	}
}

func TestParseQueryAI_EmptyGeminiCandidates(t *testing.T) {
	mock := `{"candidates":[]}`
	_, err := parseQueryAIResponse([]byte(mock), "gemini")
	if err == nil {
		t.Error("expected error for empty candidates array")
	}
}

func TestParseQueryAI_GeminiMissingParts(t *testing.T) {
	mock := `{"candidates":[{"content":{"parts":[]}}]}`
	_, err := parseQueryAIResponse([]byte(mock), "gemini")
	if err == nil {
		t.Error("expected error for empty parts array")
	}
}

func TestParseQueryAI_OllamaEmptyContent(t *testing.T) {
	mock := `{"message":{"content":""}}`
	content, err := parseQueryAIResponse([]byte(mock), "ollama")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty string, got %q", content)
	}
}

func TestParseQueryAI_MalformedJSON(t *testing.T) {
	mock := `{invalid json}`
	_, err := parseQueryAIResponse([]byte(mock), "openai")
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestParseQueryAI_WrongProvider(t *testing.T) {
	mock := `{"choices":[{"message":{"content":"test"}}]}`
	_, err := parseQueryAIResponse([]byte(mock), "nonexistent")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestParseQueryAI_MissingChoicesField(t *testing.T) {
	mock := `{"foo":"bar"}`
	_, err := parseQueryAIResponse([]byte(mock), "openai")
	if err == nil {
		t.Error("expected error when choices field is missing")
	}
}

func TestParseQueryAI_MissingContentField(t *testing.T) {
	mock := `{"choices":[{"message":{"foo":"bar"}}]}`
	content, err := parseQueryAIResponse([]byte(mock), "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
}

func TestParseQueryAI_AnthropicMissingText(t *testing.T) {
	mock := `{"content":[{"foo":"bar"}]}`
	content, err := parseQueryAIResponse([]byte(mock), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty string, got %q", content)
	}
}

func TestParseQueryAI_GeminiMissingCandidateContent(t *testing.T) {
	mock := `{"candidates":[{"foo":"bar"}]}`
	_, err := parseQueryAIResponse([]byte(mock), "gemini")
	if err == nil {
		t.Error("expected error when candidate content is missing")
	}
}

func TestParseQueryAI_MultipleChoices(t *testing.T) {
	mock := `{"choices":[{"message":{"content":"first"}},{"message":{"content":"second"}}]}`
	content, err := parseQueryAIResponse([]byte(mock), "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "first" {
		t.Errorf("expected first choice content, got %q", content)
	}
}

func TestParseQueryAI_MultipleAnthropicContent(t *testing.T) {
	mock := `{"content":[{"text":"first"},{"text":"second"}]}`
	content, err := parseQueryAIResponse([]byte(mock), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "first" {
		t.Errorf("expected first content text, got %q", content)
	}
}

func TestParseQueryAI_MultipleGeminiCandidates(t *testing.T) {
	mock := `{"candidates":[{"content":{"parts":[{"text":"first"}]}},{"content":{"parts":[{"text":"second"}]}}]}`
	content, err := parseQueryAIResponse([]byte(mock), "gemini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "first" {
		t.Errorf("expected first candidate text, got %q", content)
	}
}

func TestParseQueryAI_ContentWithSpecialChars(t *testing.T) {
	mock := `{"choices":[{"message":{"content":"Line1\nLine2\nTab\there"}}]}`
	content, err := parseQueryAIResponse([]byte(mock), "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Line1\nLine2\nTab\there"
	if content != expected {
		t.Errorf("expected %q, got %q", expected, content)
	}
}

func TestParseQueryAI_ContentWithUnicode(t *testing.T) {
	mock := `{"choices":[{"message":{"content":"Привіт, це тест!"}}]}`
	content, err := parseQueryAIResponse([]byte(mock), "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Привіт, це тест!"
	if content != expected {
		t.Errorf("expected %q, got %q", expected, content)
	}
}

func TestParseQueryAI_AllProvidersRoundTrip(t *testing.T) {
	providers := []struct {
		name     string
		mock     string
		expected string
	}{
		{"openai", `{"choices":[{"message":{"content":"test"}}]}`, "test"},
		{"deepseek", `{"choices":[{"message":{"content":"test"}}]}`, "test"},
		{"custom", `{"choices":[{"message":{"content":"test"}}]}`, "test"},
		{"lmstudio", `{"choices":[{"message":{"content":"test"}}]}`, "test"},
		{"anthropic", `{"content":[{"text":"test"}]}`, "test"},
		{"gemini", `{"candidates":[{"content":{"parts":[{"text":"test"}]}}]}`, "test"},
		{"ollama", `{"message":{"content":"test"}}`, "test"},
	}
	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			content, err := parseQueryAIResponse([]byte(p.mock), p.name)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if content != p.expected {
				t.Errorf("expected %q, got %q", p.expected, content)
			}
		})
	}
}

func TestParseAvailableModels_OpenAIStyle(t *testing.T) {
	mock := `{"data":[{"id":"gpt-4"},{"id":"gpt-3.5-turbo"},{"id":"gpt-4-turbo"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_DeepSeekStyle(t *testing.T) {
	mock := `{"data":[{"id":"deepseek-chat"},{"id":"deepseek-coder"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "deepseek")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"deepseek-chat", "deepseek-coder"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_CustomStyle(t *testing.T) {
	mock := `{"data":[{"id":"custom-v1"},{"id":"custom-v2"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "custom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"custom-v1", "custom-v2"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_LMStudioStyle(t *testing.T) {
	mock := `{"data":[{"id":"local-model-1"},{"id":"local-model-2"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "lmstudio")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"local-model-1", "local-model-2"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_AnthropicStyle(t *testing.T) {
	mock := `{"data":[{"id":"claude-3-opus-20240229"},{"id":"claude-3-sonnet-20240229"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"claude-3-opus-20240229", "claude-3-sonnet-20240229"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_GeminiStyle(t *testing.T) {
	mock := `{"models":[{"name":"models/gemini-pro"},{"name":"models/gemini-ultra"},{"name":"models/gemini-nano"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "gemini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"gemini-pro", "gemini-ultra", "gemini-nano"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_GeminiNoPrefix(t *testing.T) {
	mock := `{"models":[{"name":"gemini-pro"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "gemini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"gemini-pro"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_OllamaStyle(t *testing.T) {
	mock := `{"models":[{"name":"llama3:latest"},{"name":"mistral:7b"},{"name":"codellama:34b"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "ollama")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"llama3:latest", "mistral:7b", "codellama:34b"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_OpenAIEmpty(t *testing.T) {
	mock := `{"data":[]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected empty slice, got %v", models)
	}
}

func TestParseAvailableModels_GeminiEmpty(t *testing.T) {
	mock := `{"models":[]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "gemini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected empty slice, got %v", models)
	}
}

func TestParseAvailableModels_OllamaEmpty(t *testing.T) {
	mock := `{"models":[]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "ollama")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected empty slice, got %v", models)
	}
}

func TestParseAvailableModels_MalformedJSON(t *testing.T) {
	mock := `{bad json}`
	_, err := parseAvailableModelsResponse([]byte(mock), "openai")
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestParseAvailableModels_UnknownProvider(t *testing.T) {
	mock := `{"data":[{"id":"test"}]}`
	_, err := parseAvailableModelsResponse([]byte(mock), "nonexistent")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestParseAvailableModels_MissingDataField(t *testing.T) {
	mock := `{"foo":"bar"}`
	models, err := parseAvailableModelsResponse([]byte(mock), "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected empty slice, got %v", models)
	}
}

func TestParseAvailableModels_MissingModelsField(t *testing.T) {
	mock := `{"foo":"bar"}`
	models, err := parseAvailableModelsResponse([]byte(mock), "gemini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected empty slice, got %v", models)
	}
}

func TestParseAvailableModels_OllamaMissingModels(t *testing.T) {
	mock := `{"foo":"bar"}`
	models, err := parseAvailableModelsResponse([]byte(mock), "ollama")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected empty slice, got %v", models)
	}
}

func TestParseAvailableModels_GeminiWithExtraFields(t *testing.T) {
	mock := `{"models":[{"name":"models/gemini-pro","version":"v1","displayName":"Gemini Pro"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "gemini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"gemini-pro"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_OpenAISingleModel(t *testing.T) {
	mock := `{"data":[{"id":"gpt-4"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"gpt-4"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_AnthropicSingleModel(t *testing.T) {
	mock := `{"data":[{"id":"claude-3-opus-20240229"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"claude-3-opus-20240229"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_OllamaSingleModel(t *testing.T) {
	mock := `{"models":[{"name":"llama3:8b"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "ollama")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"llama3:8b"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_LMStudioSingleModel(t *testing.T) {
	mock := `{"data":[{"id":"TheBloke/Llama-2-7B-GGUF"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "lmstudio")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"TheBloke/Llama-2-7B-GGUF"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_DeepSeekSingleModel(t *testing.T) {
	mock := `{"data":[{"id":"deepseek-chat"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "deepseek")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"deepseek-chat"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_CustomSingleModel(t *testing.T) {
	mock := `{"data":[{"id":"my-custom-model"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "custom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"my-custom-model"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_GeminiModelWithMultiplePrefixes(t *testing.T) {
	mock := `{"models":[{"name":"models/models/gemini-pro"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "gemini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"models/gemini-pro"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_GeminiNameWithoutModelsPrefix(t *testing.T) {
	mock := `{"models":[{"name":"gemini-pro-vision"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "gemini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"gemini-pro-vision"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_OpenAIDuplicateIDs(t *testing.T) {
	mock := `{"data":[{"id":"gpt-4"},{"id":"gpt-4"},{"id":"gpt-4-turbo"}]}`
	models, err := parseAvailableModelsResponse([]byte(mock), "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"gpt-4", "gpt-4", "gpt-4-turbo"}
	if !reflect.DeepEqual(models, expected) {
		t.Errorf("expected %v, got %v", expected, models)
	}
}

func TestParseAvailableModels_AllProvidersRoundTrip(t *testing.T) {
	providers := []struct {
		name     string
		mock     string
		expected []string
	}{
		{"openai", `{"data":[{"id":"m1"},{"id":"m2"}]}`, []string{"m1", "m2"}},
		{"deepseek", `{"data":[{"id":"m1"},{"id":"m2"}]}`, []string{"m1", "m2"}},
		{"custom", `{"data":[{"id":"m1"},{"id":"m2"}]}`, []string{"m1", "m2"}},
		{"lmstudio", `{"data":[{"id":"m1"},{"id":"m2"}]}`, []string{"m1", "m2"}},
		{"anthropic", `{"data":[{"id":"m1"},{"id":"m2"}]}`, []string{"m1", "m2"}},
		{"gemini", `{"models":[{"name":"models/m1"},{"name":"models/m2"}]}`, []string{"m1", "m2"}},
		{"ollama", `{"models":[{"name":"m1"},{"name":"m2"}]}`, []string{"m1", "m2"}},
	}
	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			models, err := parseAvailableModelsResponse([]byte(p.mock), p.name)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(models, p.expected) {
				t.Errorf("expected %v, got %v", p.expected, models)
			}
		})
	}
}
