package mcp

import (
	"net/http"
)

const (
	ProviderOpenAI       = "openai"
	DefaultOpenAIBaseURL = "https://api.openai.com/v1"
	DefaultOpenAIModel    = "gpt-4o"
)

type OpenAIClient struct {
	*Client
}

// NewOpenAIClient creates OpenAI client (backward compatible)
//
// Deprecated: Recommend using NewOpenAIClientWithOptions for better flexibility
func NewOpenAIClient() AIClient {
	return NewOpenAIClientWithOptions()
}

// NewOpenAIClientWithOptions creates OpenAI client (supports options pattern)
//
// Usage examples:
//   // Basic usage
//   client := mcp.NewOpenAIClientWithOptions()
//
//   // Custom configuration
//   client := mcp.NewOpenAIClientWithOptions(
//       mcp.WithAPIKey("sk-xxx"),
//       mcp.WithLogger(customLogger),
//       mcp.WithTimeout(60*time.Second),
//   )
func NewOpenAIClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create OpenAI preset options
	openaiOpts := []ClientOption{
		WithProvider(ProviderOpenAI),
		WithModel(DefaultOpenAIModel),
		WithBaseURL(DefaultOpenAIBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(openaiOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create OpenAI client
	openaiClient := &OpenAIClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to OpenAIClient (implement dynamic dispatch)
	baseClient.hooks = openaiClient

	return openaiClient
}

func (openaiClient *OpenAIClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	openaiClient.APIKey = apiKey

	if len(apiKey) > 8 {
		openaiClient.logger.Infof("🔧 [MCP] OpenAI API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		openaiClient.BaseURL = customURL
		openaiClient.logger.Infof("🔧 [MCP] OpenAI using custom BaseURL: %s", customURL)
	} else {
		openaiClient.logger.Infof("🔧 [MCP] OpenAI using default BaseURL: %s", openaiClient.BaseURL)
	}
	if customModel != "" {
		openaiClient.Model = customModel
		openaiClient.logger.Infof("🔧 [MCP] OpenAI using custom Model: %s", customModel)
	} else {
		openaiClient.logger.Infof("🔧 [MCP] OpenAI using default Model: %s", openaiClient.Model)
	}
}

func (openaiClient *OpenAIClient) setAuthHeader(reqHeaders http.Header) {
	openaiClient.Client.setAuthHeader(reqHeaders)
}

