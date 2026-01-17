package llm

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

type OllamaClient struct {
    BaseURL    string
    ModelName  string
    HTTPClient *http.Client
}

type GenerateRequest struct {
    Model     string `json:"model"`
    Prompt    string `json:"prompt"`
    Stream    bool   `json:"stream"`
    System    string `json:"system,omitempty"`
    Options   map[string]interface{} `json:"options,omitempty"`
}

type GenerateResponse struct {
    Model     string `json:"model"`
    Response  string `json:"response"`
    Done      bool   `json:"done"`
}

type ChatMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type ChatRequest struct {
    Model     string        `json:"model"`
    Messages  []ChatMessage `json:"messages"`
    Stream    bool          `json:"stream"`
    Options   map[string]interface{} `json:"options,omitempty"`
}

func NewOllamaClient(baseURL, modelName string) *OllamaClient {
    return &OllamaClient{
        BaseURL:   baseURL,
        ModelName: modelName,
        HTTPClient: &http.Client{
            Timeout: 120 * time.Second,
        },
    }
}

func (c *OllamaClient) Generate(prompt, systemPrompt string) (string, error) {
    reqBody := GenerateRequest{
        Model:  c.ModelName,
        Prompt: prompt,
        Stream: false,
        System: systemPrompt,
        Options: map[string]interface{}{
            "temperature": 0.2,
            "top_p":       0.9,
        },
    }

    jsonData, err := json.Marshal(reqBody)
    if err != nil {
        return "", fmt.Errorf("marshal request: %w", err)
    }

    resp, err := c.HTTPClient.Post(
        c.BaseURL+"/api/generate",
        "application/json",
        bytes.NewBuffer(jsonData),
    )
    if err != nil {
        return "", fmt.Errorf("HTTP request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
    }

    var genResp GenerateResponse
    if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
        return "", fmt.Errorf("decode response: %w", err)
    }

    return genResp.Response, nil
}

func (c *OllamaClient) Chat(messages []ChatMessage) (string, error) {
    reqBody := ChatRequest{
        Model:    c.ModelName,
        Messages: messages,
        Stream:   false,
        Options: map[string]interface{}{
            "temperature": 0.3,
            "top_p":       0.9,
            "num_ctx":     4096,
        },
    }

    jsonData, err := json.Marshal(reqBody)
    if err != nil {
        return "", fmt.Errorf("marshal request: %w", err)
    }

    resp, err := c.HTTPClient.Post(
        c.BaseURL+"/api/chat",
        "application/json",
        bytes.NewBuffer(jsonData),
    )
    if err != nil {
        return "", fmt.Errorf("HTTP request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
    }

    var chatResp struct {
        Message ChatMessage `json:"message"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
        return "", fmt.Errorf("decode response: %w", err)
    }

    return chatResp.Message.Content, nil
}