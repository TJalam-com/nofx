package mcp

import (
	"net/http"
)

const (
	ProviderGrok       = "grok"
	DefaultGrokBaseURL = "https://api.x.ai/v1"
	DefaultGrokModel   = "grok-beta"
)

type GrokClient struct {
	*Client
}

// NewGrokClient creates Grok client (backward compatible)
//
// Deprecated: Recommend using NewGrokClientWithOptions for better flexibility
func NewGrokClient() AIClient {
	return NewGrokClientWithOptions()
}

// NewGrokClientWithOptions creates Grok client (supports options pattern)
//
// Usage examples:
//   // Basic usage
//   client := mcp.NewGrokClientWithOptions()
//
//   // Custom configuration
//   client := mcp.NewGrokClientWithOptions(
//       mcp.WithAPIKey("xai-xxx"),
//       mcp.WithLogger(customLogger),
//       mcp.WithTimeout(60*time.Second),
//   )
func NewGrokClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create Grok preset options
	grokOpts := []ClientOption{
		WithProvider(ProviderGrok),
		WithModel(DefaultGrokModel),
		WithBaseURL(DefaultGrokBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(grokOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create Grok client
	grokClient := &GrokClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to GrokClient (implement dynamic dispatch)
	baseClient.hooks = grokClient

	return grokClient
}

func (grokClient *GrokClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	grokClient.APIKey = apiKey

	if len(apiKey) > 8 {
		grokClient.logger.Infof("🔧 [MCP] Grok API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		grokClient.BaseURL = customURL
		grokClient.logger.Infof("🔧 [MCP] Grok using custom BaseURL: %s", customURL)
	} else {
		grokClient.logger.Infof("🔧 [MCP] Grok using default BaseURL: %s", grokClient.BaseURL)
	}
	if customModel != "" {
		grokClient.Model = customModel
		grokClient.logger.Infof("🔧 [MCP] Grok using custom Model: %s", customModel)
	} else {
		grokClient.logger.Infof("🔧 [MCP] Grok using default Model: %s", grokClient.Model)
	}
}

func (grokClient *GrokClient) setAuthHeader(reqHeaders http.Header) {
	grokClient.Client.setAuthHeader(reqHeaders)
}

