package mcp

import (
	"net/http"
)

const (
	ProviderKimi       = "kimi"
	DefaultKimiBaseURL = "https://api.moonshot.cn/v1"
	DefaultKimiModel   = "moonshot-v1-8k"
)

type KimiClient struct {
	*Client
}

// NewKimiClient creates Kimi client (backward compatible)
//
// Deprecated: Recommend using NewKimiClientWithOptions for better flexibility
func NewKimiClient() AIClient {
	return NewKimiClientWithOptions()
}

// NewKimiClientWithOptions creates Kimi client (supports options pattern)
//
// Usage examples:
//   // Basic usage
//   client := mcp.NewKimiClientWithOptions()
//
//   // Custom configuration
//   client := mcp.NewKimiClientWithOptions(
//       mcp.WithAPIKey("sk-xxx"),
//       mcp.WithLogger(customLogger),
//       mcp.WithTimeout(60*time.Second),
//   )
func NewKimiClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create Kimi preset options
	kimiOpts := []ClientOption{
		WithProvider(ProviderKimi),
		WithModel(DefaultKimiModel),
		WithBaseURL(DefaultKimiBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(kimiOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create Kimi client
	kimiClient := &KimiClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to KimiClient (implement dynamic dispatch)
	baseClient.hooks = kimiClient

	return kimiClient
}

func (kimiClient *KimiClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	kimiClient.APIKey = apiKey

	if len(apiKey) > 8 {
		kimiClient.logger.Infof("🔧 [MCP] Kimi API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		kimiClient.BaseURL = customURL
		kimiClient.logger.Infof("🔧 [MCP] Kimi using custom BaseURL: %s", customURL)
	} else {
		kimiClient.logger.Infof("🔧 [MCP] Kimi using default BaseURL: %s", kimiClient.BaseURL)
	}
	if customModel != "" {
		kimiClient.Model = customModel
		kimiClient.logger.Infof("🔧 [MCP] Kimi using custom Model: %s", customModel)
	} else {
		kimiClient.logger.Infof("🔧 [MCP] Kimi using default Model: %s", kimiClient.Model)
	}
}

func (kimiClient *KimiClient) setAuthHeader(reqHeaders http.Header) {
	kimiClient.Client.setAuthHeader(reqHeaders)
}

