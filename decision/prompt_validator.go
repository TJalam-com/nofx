package decision

import (
	"log"
	"strings"
)

// PromptType represents the type of prompt being validated
type PromptType string

const (
	PromptTypeStandard    PromptType = "standard"
	PromptTypeTradingView PromptType = "tradingview"
	PromptTypeParent      PromptType = "parent"
)

// ValidationResult represents the result of prompt validation
type ValidationResult struct {
	HasFormatSection bool
	HasXMLTags       bool
	HasJSONExample   bool
	HasFieldDesc     bool
	Warnings         []string
	Suggestions      []string
}

// ValidatePromptHasJSONFormat checks if a prompt includes proper JSON format requirements
func ValidatePromptHasJSONFormat(prompt string, promptType PromptType) ValidationResult {
	result := ValidationResult{
		Warnings:    []string{},
		Suggestions: []string{},
	}

	lowerPrompt := strings.ToLower(prompt)

	// Check for XML tags
	result.HasXMLTags = strings.Contains(lowerPrompt, "<reasoning>") && strings.Contains(lowerPrompt, "<decision>")
	if !result.HasXMLTags {
		result.Warnings = append(result.Warnings, "Missing XML tags <reasoning> and <decision>")
		result.Suggestions = append(result.Suggestions, "Add <reasoning> and <decision> tags to separate chain of thought from JSON")
	}

	// Check for JSON examples
	result.HasJSONExample = strings.Contains(lowerPrompt, "```json") || strings.Contains(lowerPrompt, "json")
	if !result.HasJSONExample {
		result.Warnings = append(result.Warnings, "Missing JSON code block example")
		result.Suggestions = append(result.Suggestions, "Add a JSON example wrapped in ```json code blocks")
	}

	// Check for format section header
	result.HasFormatSection = strings.Contains(lowerPrompt, "output format") || strings.Contains(lowerPrompt, "format requirements")
	if !result.HasFormatSection {
		result.Warnings = append(result.Warnings, "Missing 'Output Format' section")
		result.Suggestions = append(result.Suggestions, "Add an 'Output Format' section with clear requirements")
	}

	// Type-specific checks
	switch promptType {
	case PromptTypeTradingView:
		if !strings.Contains(lowerPrompt, "signal_decision") {
			result.Warnings = append(result.Warnings, "Missing TradingView-specific field: signal_decision")
			result.Suggestions = append(result.Suggestions, "Include signal_decision field (accept/reject/modify) in format requirements")
		}
		if !strings.Contains(lowerPrompt, "tradingview_signal_id") {
			result.Warnings = append(result.Warnings, "Missing TradingView-specific field: tradingview_signal_id")
			result.Suggestions = append(result.Suggestions, "Include tradingview_signal_id field in format requirements")
		}
	case PromptTypeParent:
		if !strings.Contains(lowerPrompt, "signal_decision") {
			result.Warnings = append(result.Warnings, "Missing parent signal-specific field: signal_decision")
			result.Suggestions = append(result.Suggestions, "Include signal_decision field (accept/reject/modify) in format requirements")
		}
		if !strings.Contains(lowerPrompt, "parent_signal_id") {
			result.Warnings = append(result.Warnings, "Missing parent signal-specific field: parent_signal_id")
			result.Suggestions = append(result.Suggestions, "Include parent_signal_id field in format requirements")
		}
	}

	// Check for field descriptions
	result.HasFieldDesc = strings.Contains(lowerPrompt, "field") && (strings.Contains(lowerPrompt, "description") || strings.Contains(lowerPrompt, "required"))
	if !result.HasFieldDesc {
		result.Warnings = append(result.Warnings, "Missing field descriptions")
		result.Suggestions = append(result.Suggestions, "Add field descriptions explaining required vs optional fields")
	}

	return result
}

// SuggestJSONFormat returns a format suggestion based on prompt type
func SuggestJSONFormat(promptType PromptType, accountEquity float64, btcEthLeverage, altcoinLeverage int) string {
	switch promptType {
	case PromptTypeTradingView:
		return GetTradingViewJSONFormat()
	case PromptTypeParent:
		return GetParentSignalJSONFormat()
	default:
		return GetStandardJSONFormat(accountEquity, btcEthLeverage, altcoinLeverage)
	}
}

// ValidateAndWarn validates a prompt and logs warnings if format is missing
func ValidateAndWarn(prompt string, promptType PromptType, accountEquity float64, btcEthLeverage, altcoinLeverage int) {
	result := ValidatePromptHasJSONFormat(prompt, promptType)

	if len(result.Warnings) > 0 {
		log.Printf("⚠️  Custom prompt validation found %d issue(s):", len(result.Warnings))
		for i, warning := range result.Warnings {
			log.Printf("  %d. %s", i+1, warning)
		}

		log.Printf("💡 Suggestions:")
		for i, suggestion := range result.Suggestions {
			log.Printf("  %d. %s", i+1, suggestion)
		}

		log.Printf("📋 Recommended format section:\n%s", SuggestJSONFormat(promptType, accountEquity, btcEthLeverage, altcoinLeverage))
	} else {
		log.Printf("✓ Custom prompt validation passed - format section detected")
	}
}

// DetectPromptType attempts to detect the prompt type based on content
func DetectPromptType(prompt string) PromptType {
	lowerPrompt := strings.ToLower(prompt)

	if strings.Contains(lowerPrompt, "tradingview") || strings.Contains(lowerPrompt, "webhook signal") {
		return PromptTypeTradingView
	}
	if strings.Contains(lowerPrompt, "parent trader") || strings.Contains(lowerPrompt, "parent signal") || strings.Contains(lowerPrompt, "followed trader") {
		return PromptTypeParent
	}

	return PromptTypeStandard
}

// EnsureFormatSection ensures a prompt has the appropriate format section, appending if missing
func EnsureFormatSection(prompt string, promptType PromptType, accountEquity float64, btcEthLeverage, altcoinLeverage int) string {
	// Check if format section already exists
	if HasJSONFormatSection(prompt) {
		// Validate it's complete
		result := ValidatePromptHasJSONFormat(prompt, promptType)
		if len(result.Warnings) == 0 {
			return prompt // Format section is complete, return as-is
		}
		// Format section exists but incomplete, log warning but don't modify
		log.Printf("⚠️  Prompt has format section but validation found issues - not auto-fixing to preserve custom content")
		return prompt
	}

	// No format section detected, append appropriate one
	log.Printf("📝 Auto-appending JSON format section for %s prompt type", promptType)
	formatSection := SuggestJSONFormat(promptType, accountEquity, btcEthLeverage, altcoinLeverage)

	var sb strings.Builder
	sb.WriteString(prompt)
	sb.WriteString("\n\n")
	sb.WriteString(formatSection)

	return sb.String()
}
