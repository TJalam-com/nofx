package mcp

import (
	"net/http"
)

const (
	ProviderGemini       = "gemini"
	DefaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	DefaultGeminiModel   = "gemini-pro"
)

type GeminiClient struct {
	*Client
}

// NewGeminiClient creates Gemini client (backward compatible)
//
// Deprecated: Recommend using NewGeminiClientWithOptions for better flexibility
func NewGeminiClient() AIClient {
	return NewGeminiClientWithOptions()
}

// NewGeminiClientWithOptions creates Gemini client (supports options pattern)
//
// Usage examples:
//   // Basic usage
//   client := mcp.NewGeminiClientWithOptions()
//
//   // Custom configuration
//   client := mcp.NewGeminiClientWithOptions(
//       mcp.WithAPIKey("AIza..."),
//       mcp.WithLogger(customLogger),
//       mcp.WithTimeout(60*time.Second),
//   )
func NewGeminiClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create Gemini preset options
	geminiOpts := []ClientOption{
		WithProvider(ProviderGemini),
		WithModel(DefaultGeminiModel),
		WithBaseURL(DefaultGeminiBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(geminiOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create Gemini client
	geminiClient := &GeminiClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to GeminiClient (implement dynamic dispatch)
	baseClient.hooks = geminiClient

	return geminiClient
}

func (geminiClient *GeminiClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	geminiClient.APIKey = apiKey

	if len(apiKey) > 8 {
		geminiClient.logger.Infof("🔧 [MCP] Gemini API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		geminiClient.BaseURL = customURL
		geminiClient.logger.Infof("🔧 [MCP] Gemini using custom BaseURL: %s", customURL)
	} else {
		geminiClient.logger.Infof("🔧 [MCP] Gemini using default BaseURL: %s", geminiClient.BaseURL)
	}
	if customModel != "" {
		geminiClient.Model = customModel
		geminiClient.logger.Infof("🔧 [MCP] Gemini using custom Model: %s", customModel)
	} else {
		geminiClient.logger.Infof("🔧 [MCP] Gemini using default Model: %s", geminiClient.Model)
	}
}

func (geminiClient *GeminiClient) setAuthHeader(reqHeaders http.Header) {
	geminiClient.Client.setAuthHeader(reqHeaders)
}

