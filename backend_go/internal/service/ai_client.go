package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// V1: Original simple string-based messages (kept for backward compatibility)
// ---------------------------------------------------------------------------

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMRequest struct {
	Model       string       `json:"model"`
	Messages    []LLMMessage `json:"messages"`
	Temperature float64      `json:"temperature"`
}

type LLMResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *Usage `json:"usage,omitempty"`
}

type Usage struct {
	PromptTokens             int `json:"prompt_tokens"`
	CompletionTokens         int `json:"completion_tokens"`
	TotalTokens              int `json:"total_tokens"`
	CachedTokens             int `json:"cached_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
}

// ---------------------------------------------------------------------------
// V2: Cacheable content block support (OpenAI "content array" format)
// ---------------------------------------------------------------------------

// ContentBlock represents a single block in a content array.
// Can represent plain text or cacheable text.
type ContentBlock struct {
	Type         string `json:"type"` // "text" for plain text
	Text         string `json:"text"` // actual text content
	CacheControl *struct {
		Type string `json:"type"` // "ephemeral" for explicit cache
	} `json:"cache_control,omitempty"`
}

// LLMMessageV2 uses a JSON array content to support cache_control markers.
// Content is encoded as json.RawMessage so it can be either a plain string
// (backward compat) or an array of ContentBlocks.
type LLMMessageV2 struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type LLMRequestV2 struct {
	Model       string         `json:"model"`
	Messages    []LLMMessageV2 `json:"messages"`
	Temperature float64        `json:"temperature"`
}

// CacheUsage holds cache-related metrics returned by CallLLMV2.
type CacheUsage struct {
	PromptTokens             int `json:"prompt_tokens"`
	CompletionTokens         int `json:"completion_tokens"`
	TotalTokens              int `json:"total_tokens"`
	CachedTokens             int `json:"cached_tokens,omitempty"`               // tokens served from cache
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"` // tokens used to create the cache
}

// AIClient is the core LLM client. All existing callers continue to work via
// the V1 methods (CallLLM / CallLLMWithConfig).
type AIClient struct {
	APIKey   string
	Endpoint string
	Model    string
}

func NewAIClient(apiKey, endpoint, model string) *AIClient {
	if endpoint == "" {
		endpoint = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	if model == "" {
		// Qwen3 family (DashScope OpenAI-compatible); override via system settings ai_ingest_model
		model = "qwen3-235b-a22b"
	}
	return &AIClient{APIKey: apiKey, Endpoint: endpoint, Model: model}
}

// shouldUseSimpleChatFormat is true for DashScope / Qwen: their compatible API expects
// string message content, not OpenAI-style content arrays with cache_control (which otherwise returns 400).
func shouldUseSimpleChatFormat(endpoint, model string) bool {
	e := strings.ToLower(endpoint)
	m := strings.ToLower(model)
	if strings.Contains(e, "dashscope") || strings.Contains(e, "aliyuncs.com") {
		return true
	}
	return strings.Contains(m, "qwen")
}

func flattenMessageContentToString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str, nil
	}
	var blocks []ContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", fmt.Errorf("message content must be string or content block array: %w", err)
	}
	var parts []string
	for _, b := range blocks {
		if b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

func flattenV2MessagesToV1(messages []LLMMessageV2) ([]LLMMessage, error) {
	out := make([]LLMMessage, 0, len(messages))
	for _, m := range messages {
		text, err := flattenMessageContentToString(m.Content)
		if err != nil {
			return nil, fmt.Errorf("role %s: %w", m.Role, err)
		}
		out = append(out, LLMMessage{Role: m.Role, Content: text})
	}
	return out, nil
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// 捕获 unexpected EOF 和连接重置
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(err.Error(), "unexpected EOF") {
		return true
	}
	// 捕获网络层超时或连接重置
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// V1: Original non-caching methods (backward compatible)
// ---------------------------------------------------------------------------

func (c *AIClient) CallLLM(messages []LLMMessage, temperature float64) (string, error) {
	return c.CallLLMWithConfig(messages, temperature, c.Model, c.Endpoint, c.APIKey)
}

func (c *AIClient) CallLLMWithContext(ctx context.Context, messages []LLMMessage, temperature float64) (string, error) {
	return c.CallLLMWithConfigAndContext(ctx, messages, temperature, c.Model, c.Endpoint, c.APIKey)
}

func (c *AIClient) CallLLMWithConfig(messages []LLMMessage, temperature float64, model, endpoint, apiKey string) (string, error) {
	return c.CallLLMWithConfigAndContext(context.Background(), messages, temperature, model, endpoint, apiKey)
}

func (c *AIClient) CallLLMWithConfigAndContext(ctx context.Context, messages []LLMMessage, temperature float64, model, endpoint, apiKey string) (string, error) {
	if endpoint == "" {
		endpoint = c.Endpoint
	}
	if model == "" {
		model = c.Model
	}
	if apiKey == "" {
		apiKey = c.APIKey
	}

	reqBody := LLMRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	client := &http.Client{
		Timeout: 600 * time.Second, // 统一设置为 600s 防止大批量数据处理时请求截断
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/chat/completions", bytes.NewBuffer(jsonData))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if isRetryableError(err) && attempt < 3 {
				log.Printf("[AIClient] Retry %d/3 due to network error: %v", attempt, err)
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return "", err
		}

		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(body))
			// 对于 5xx 错误也重试
			if resp.StatusCode >= 500 && attempt < 3 {
				log.Printf("[AIClient] Retry %d/3 due to server error %d", attempt, resp.StatusCode)
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return "", lastErr
		}

		var llmResp LLMResponse
		if err := json.Unmarshal(body, &llmResp); err != nil {
			return "", err
		}

		if len(llmResp.Choices) > 0 {
			return llmResp.Choices[0].Message.Content, nil
		}
		return "", fmt.Errorf("no content returned from LLM")
	}

	return "", fmt.Errorf("LLM call failed after 3 attempts: %w", lastErr)
}

// ---------------------------------------------------------------------------
// V2: Cacheable message builders
// ---------------------------------------------------------------------------

// BuildCacheableSystemMessage returns a system-role message with cache_control
// marked as "ephemeral". Suitable for the stable system prompt.
func BuildCacheableSystemMessage(text string) LLMMessageV2 {
	block := ContentBlock{
		Type: "text",
		Text: text,
		CacheControl: &struct {
			Type string `json:"type"`
		}{Type: "ephemeral"},
	}
	raw, _ := json.Marshal([]ContentBlock{block})
	return LLMMessageV2{Role: "system", Content: raw}
}

// BuildCacheableUserBlock returns a user-role message with cache_control marked
// as "ephemeral". Suitable for stable project-level context (e.g. tender text,
// project profile, facts JSON).
func BuildCacheableUserBlock(text string) LLMMessageV2 {
	block := ContentBlock{
		Type: "text",
		Text: text,
		CacheControl: &struct {
			Type string `json:"type"`
		}{Type: "ephemeral"},
	}
	raw, _ := json.Marshal([]ContentBlock{block})
	return LLMMessageV2{Role: "user", Content: raw}
}

// BuildDynamicUserBlock returns a user-role message WITHOUT cache_control.
// Use for the dynamic tail portion of a user message (current question,
// instructions, etc.).
func BuildDynamicUserBlock(text string) LLMMessageV2 {
	block := ContentBlock{
		Type: "text",
		Text: text,
	}
	raw, _ := json.Marshal([]ContentBlock{block})
	return LLMMessageV2{Role: "user", Content: raw}
}

// BuildUserMessageWithCacheAndDynamic returns a single user message that
// contains a cacheable block followed by a dynamic block. This is useful
// when you want one user turn to have both a stable prefix and a changing tail.
func BuildUserMessageWithCacheAndDynamic(cacheText, dynamicText string) LLMMessageV2 {
	blocks := []ContentBlock{
		{
			Type: "text",
			Text: cacheText,
			CacheControl: &struct {
				Type string `json:"type"`
			}{Type: "ephemeral"},
		},
		{
			Type: "text",
			Text: dynamicText,
		},
	}
	raw, _ := json.Marshal(blocks)
	return LLMMessageV2{Role: "user", Content: raw}
}

// ---------------------------------------------------------------------------
// V2: Cacheable LLM call methods
// ---------------------------------------------------------------------------

// CallLLMV2 sends a cacheable message list and returns the response along
// with cache metrics. Use this when you want explicit context cache support.
func (c *AIClient) CallLLMV2(messages []LLMMessageV2, temperature float64) (string, *CacheUsage, error) {
	return c.CallLLMV2WithConfig(messages, temperature, c.Model, c.Endpoint, c.APIKey)
}

func (c *AIClient) CallLLMV2WithContext(ctx context.Context, messages []LLMMessageV2, temperature float64) (string, *CacheUsage, error) {
	return c.CallLLMV2WithConfigAndContext(ctx, messages, temperature, c.Model, c.Endpoint, c.APIKey)
}

func (c *AIClient) CallLLMV2WithConfig(
	messages []LLMMessageV2,
	temperature float64,
	model, endpoint, apiKey string,
) (string, *CacheUsage, error) {
	return c.CallLLMV2WithConfigAndContext(context.Background(), messages, temperature, model, endpoint, apiKey)
}

func (c *AIClient) CallLLMV2WithConfigAndContext(
	ctx context.Context,
	messages []LLMMessageV2,
	temperature float64,
	model, endpoint, apiKey string,
) (string, *CacheUsage, error) {
	if endpoint == "" {
		endpoint = c.Endpoint
	}
	if model == "" {
		model = c.Model
	}
	if apiKey == "" {
		apiKey = c.APIKey
	}

	// DashScope / Qwen: use plain string messages (same semantics as V2, no prompt cache metrics).
	if shouldUseSimpleChatFormat(endpoint, model) {
		flat, err := flattenV2MessagesToV1(messages)
		if err != nil {
			return "", nil, err
		}
		text, err := c.CallLLMWithConfigAndContext(ctx, flat, temperature, model, endpoint, apiKey)
		return text, nil, err
	}

	start := time.Now()

	reqBody := LLMRequestV2{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 600 * time.Second, // 统一设置为 600s 防止大批量数据处理时请求截断
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/chat/completions", bytes.NewBuffer(jsonData))
		if err != nil {
			return "", nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if isRetryableError(err) && attempt < 3 {
				log.Printf("[AIClient] (V2) Retry %d/3 due to network error: %v", attempt, err)
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return "", nil, err
		}

		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(body))
			if resp.StatusCode >= 500 && attempt < 3 {
				log.Printf("[AIClient] (V2) Retry %d/3 due to server error %d", attempt, resp.StatusCode)
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return "", nil, lastErr
		}

		var llmResp LLMResponse
		if err := json.Unmarshal(body, &llmResp); err != nil {
			return "", nil, err
		}

		latencyMs := time.Since(start).Milliseconds()

		if len(llmResp.Choices) == 0 {
			return "", nil, fmt.Errorf("no content returned from LLM")
		}

		cu := &CacheUsage{}
		if llmResp.Usage != nil {
			cu.PromptTokens = llmResp.Usage.PromptTokens
			cu.CompletionTokens = llmResp.Usage.CompletionTokens
			cu.TotalTokens = llmResp.Usage.TotalTokens
			cu.CachedTokens = llmResp.Usage.CachedTokens
			cu.CacheCreationInputTokens = llmResp.Usage.CacheCreationInputTokens
		}

		mode := "none"
		if cu.CachedTokens > 0 && cu.CacheCreationInputTokens > 0 {
			mode = "explicit"
		} else if cu.CachedTokens > 0 {
			mode = "implicit"
		}

		log.Printf("[AIClient] cache_mode=%s prompt_tokens=%d cached_tokens=%d cache_creation_input_tokens=%d completion_tokens=%d latency_ms=%d",
			mode, cu.PromptTokens, cu.CachedTokens, cu.CacheCreationInputTokens, cu.CompletionTokens, latencyMs)

		return llmResp.Choices[0].Message.Content, cu, nil
	}

	return "", nil, fmt.Errorf("LLM call failed after 3 attempts: %w", lastErr)
}
