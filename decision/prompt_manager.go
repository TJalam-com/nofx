package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/config"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PromptTemplate system prompt template
type PromptTemplate struct {
	Name    string // Template name (filename without extension)
	Content string // Template content
}

// PromptManager prompt manager
type PromptManager struct {
	templates map[string]*PromptTemplate
	database  config.DatabaseInterface // Optional database interface
	mu        sync.RWMutex
}

var (
	// globalPromptManager global prompt manager
	globalPromptManager *PromptManager
	// promptsDir prompt folder path
	promptsDir = "prompts"
)

// init loads all prompt templates during package initialization
func init() {
	globalPromptManager = NewPromptManager()
	if err := globalPromptManager.LoadTemplates(promptsDir); err != nil {
		log.Printf("⚠️  Failed to load prompt templates: %v", err)
	} else {
		log.Printf("✓ Loaded %d system prompt templates", len(globalPromptManager.templates))
	}
}

// NewPromptManager creates a prompt manager
func NewPromptManager() *PromptManager {
	return &PromptManager{
		templates: make(map[string]*PromptTemplate),
		database:  nil,
	}
}

// SetDatabase sets database interface (for loading templates from database)
func (pm *PromptManager) SetDatabase(db config.DatabaseInterface) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.database = db
}

// LoadTemplates loads all prompt templates from specified directory
func (pm *PromptManager) LoadTemplates(dir string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("prompt directory does not exist: %s", dir)
	}

	// Scan all .txt files in directory
	files, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	if err != nil {
		return fmt.Errorf("failed to scan prompt directory: %w", err)
	}

	if len(files) == 0 {
		log.Printf("⚠️  No .txt files found in prompt directory %s", dir)
		return nil
	}

	// Load each template file
	for _, file := range files {
		// Read file content
		content, err := os.ReadFile(file)
		if err != nil {
			log.Printf("⚠️  Failed to read prompt file %s: %v", file, err)
			continue
		}

		// Extract filename (without extension) as template name
		fileName := filepath.Base(file)
		templateName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

		// Store template
		pm.templates[templateName] = &PromptTemplate{
			Name:    templateName,
			Content: string(content),
		}

		log.Printf("  📄 Loaded prompt template: %s (%s)", templateName, fileName)
	}

	return nil
}

// GetTemplate gets prompt template by name (ONLY from database - no filesystem fallback)
func (pm *PromptManager) GetTemplate(name string) (*PromptTemplate, error) {
	pm.mu.RLock()
	db := pm.database
	pm.mu.RUnlock()

	// ONLY load from database - templates must come from strategy-studio
	if db != nil {
		// Use default user to get system templates, or current user to get their own templates
		// Here we use "default" as fallback, actual usage should pass correct userID
		// But since this is a global function, temporarily use default
		templateConfig, err := db.GetPromptTemplate("default", name)
		if err == nil {
			// #region agent log
			// Log successful database template load
			logFile, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if logFile != nil {
				logData := map[string]interface{}{
					"location": "prompt_manager.go:115",
					"message": "Template loaded from database",
					"data": map[string]interface{}{
						"template_name": name,
						"template_id": templateConfig.ID,
						"is_system": templateConfig.IsSystem,
					},
					"timestamp": time.Now().UnixMilli(),
					"sessionId": "debug-session",
					"runId": "run1",
					"hypothesisId": "E",
				}
				json.NewEncoder(logFile).Encode(logData)
				logFile.Close()
			}
			// #endregion
			return &PromptTemplate{
				Name:    templateConfig.Name,
				Content: templateConfig.Content,
			}, nil
		}
		// If not found in database, return error (no filesystem fallback)
		return nil, fmt.Errorf("prompt template '%s' not found in database. Please ensure templates are loaded from strategy-studio (http://localhost:3000/strategy-studio): %w", name, err)
	}

	// Database not available - this should not happen in production
	return nil, fmt.Errorf("database not available - cannot load prompt template '%s'. Templates must be loaded from strategy-studio", name)
}

// GetAllTemplateNames gets all template names list (merge database and file system)
func (pm *PromptManager) GetAllTemplateNames() []string {
	pm.mu.RLock()
	db := pm.database
	fileNames := make(map[string]bool)
	for name := range pm.templates {
		fileNames[name] = true
	}
	pm.mu.RUnlock()

	// If database is available, get template names from database
	if db != nil {
		dbTemplates, err := db.GetPromptTemplates("default")
		if err == nil {
			names := make([]string, 0, len(dbTemplates)+len(fileNames))
			nameSet := make(map[string]bool)

			// First add database template names
			for _, dbTemplate := range dbTemplates {
				names = append(names, dbTemplate.ID)
				nameSet[dbTemplate.ID] = true
			}

			// Then add file template names (if not in database)
			for name := range fileNames {
				if !nameSet[name] {
					names = append(names, name)
				}
			}

			return names
		}
	}

	// Only return file template names
	names := make([]string, 0, len(fileNames))
	for name := range fileNames {
		names = append(names, name)
	}

	return names
}

// GetAllTemplates gets all templates (merge database and file system templates)
func (pm *PromptManager) GetAllTemplates() []*PromptTemplate {
	pm.mu.RLock()
	db := pm.database
	fileTemplates := make(map[string]*PromptTemplate)
	for k, v := range pm.templates {
		fileTemplates[k] = v
	}
	pm.mu.RUnlock()

	// If database is available, get templates from database
	if db != nil {
		dbTemplates, err := db.GetPromptTemplates("default")
		if err == nil {
			// Merge database templates and file templates (database priority)
			result := make([]*PromptTemplate, 0, len(dbTemplates)+len(fileTemplates))
			templateMap := make(map[string]bool)

			// First add database templates
			for _, dbTemplate := range dbTemplates {
				result = append(result, &PromptTemplate{
					Name:    dbTemplate.Name,
					Content: dbTemplate.Content,
				})
				templateMap[dbTemplate.ID] = true
			}

			// Then add file templates (if not in database)
			for name, fileTemplate := range fileTemplates {
				if !templateMap[name] {
					result = append(result, fileTemplate)
				}
			}

			return result
		}
	}

	// Only return file templates
	result := make([]*PromptTemplate, 0, len(fileTemplates))
	for _, template := range fileTemplates {
		result = append(result, template)
	}

	return result
}

// ReloadTemplates reloads all templates
func (pm *PromptManager) ReloadTemplates(dir string) error {
	pm.mu.Lock()
	pm.templates = make(map[string]*PromptTemplate)
	pm.mu.Unlock()

	return pm.LoadTemplates(dir)
}

// === Global functions (for external calls) ===

// GetPromptTemplate gets prompt template by name (global function)
func GetPromptTemplate(name string) (*PromptTemplate, error) {
	return globalPromptManager.GetTemplate(name)
}

// GetAllPromptTemplateNames gets all template names (global function)
func GetAllPromptTemplateNames() []string {
	return globalPromptManager.GetAllTemplateNames()
}

// GetAllPromptTemplates gets all templates (global function)
func GetAllPromptTemplates() []*PromptTemplate {
	return globalPromptManager.GetAllTemplates()
}

// ReloadPromptTemplates reloads all templates (global function)
func ReloadPromptTemplates() error {
	return globalPromptManager.ReloadTemplates(promptsDir)
}

// GetGlobalPromptManager gets global prompt manager (for setting database)
func GetGlobalPromptManager() *PromptManager {
	return globalPromptManager
}
