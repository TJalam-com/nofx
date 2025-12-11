package mcp

import (
	"net/http"
)

const (
	ProviderClaude       = "claude"
	DefaultClaudeBaseURL = "https://api.anthropic.com/v1"
	DefaultClaudeModel   = "claude-3-5-sonnet-20241022"
)

type ClaudeClient struct {
	*Client
}

// NewClaudeClient creates Claude client (backward compatible)
//
// Deprecated: Recommend using NewClaudeClientWithOptions for better flexibility
func NewClaudeClient() AIClient {
	return NewClaudeClientWithOptions()
}

// NewClaudeClientWithOptions creates Claude client (supports options pattern)
//
// Usage examples:
//   // Basic usage
//   client := mcp.NewClaudeClientWithOptions()
//
//   // Custom configuration
//   client := mcp.NewClaudeClientWithOptions(
//       mcp.WithAPIKey("sk-ant-xxx"),
//       mcp.WithLogger(customLogger),
//       mcp.WithTimeout(60*time.Second),
//   )
func NewClaudeClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create Claude preset options
	claudeOpts := []ClientOption{
		WithProvider(ProviderClaude),
		WithModel(DefaultClaudeModel),
		WithBaseURL(DefaultClaudeBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(claudeOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create Claude client
	claudeClient := &ClaudeClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to ClaudeClient (implement dynamic dispatch)
	baseClient.hooks = claudeClient

	return claudeClient
}

func (claudeClient *ClaudeClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	claudeClient.APIKey = apiKey

	if len(apiKey) > 8 {
		claudeClient.logger.Infof("🔧 [MCP] Claude API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		claudeClient.BaseURL = customURL
		claudeClient.logger.Infof("🔧 [MCP] Claude using custom BaseURL: %s", customURL)
	} else {
		claudeClient.logger.Infof("🔧 [MCP] Claude using default BaseURL: %s", claudeClient.BaseURL)
	}
	if customModel != "" {
		claudeClient.Model = customModel
		claudeClient.logger.Infof("🔧 [MCP] Claude using custom Model: %s", customModel)
	} else {
		claudeClient.logger.Infof("🔧 [MCP] Claude using default Model: %s", claudeClient.Model)
	}
}

func (claudeClient *ClaudeClient) setAuthHeader(reqHeaders http.Header) {
	claudeClient.Client.setAuthHeader(reqHeaders)
}

