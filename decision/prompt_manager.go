package decision

import (
	"fmt"
	"log"
	"nofx/config"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PromptTemplate 系统提示词模板
type PromptTemplate struct {
	Name    string // 模板名称（文件名，不含扩展名）
	Content string // 模板内容
}

// PromptManager 提示词管理器
type PromptManager struct {
	templates map[string]*PromptTemplate
	database  config.DatabaseInterface // 可选的数据库接口
	mu        sync.RWMutex
}

var (
	// globalPromptManager 全局提示词管理器
	globalPromptManager *PromptManager
	// promptsDir 提示词文件夹路径
	promptsDir = "prompts"
)

// init 包初始化时加载所有提示词模板
func init() {
	globalPromptManager = NewPromptManager()
	if err := globalPromptManager.LoadTemplates(promptsDir); err != nil {
		log.Printf("⚠️  加载提示词模板失败: %v", err)
	} else {
		log.Printf("✓ 已加载 %d 个系统提示词模板", len(globalPromptManager.templates))
	}
}

// NewPromptManager 创建提示词管理器
func NewPromptManager() *PromptManager {
	return &PromptManager{
		templates: make(map[string]*PromptTemplate),
		database:  nil,
	}
}

// SetDatabase 设置数据库接口（用于从数据库加载模板）
func (pm *PromptManager) SetDatabase(db config.DatabaseInterface) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.database = db
}

// LoadTemplates 从指定目录加载所有提示词模板
func (pm *PromptManager) LoadTemplates(dir string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("提示词目录不存在: %s", dir)
	}

	// 扫描目录中的所有 .txt 文件
	files, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	if err != nil {
		return fmt.Errorf("扫描提示词目录失败: %w", err)
	}

	if len(files) == 0 {
		log.Printf("⚠️  提示词目录 %s 中没有找到 .txt 文件", dir)
		return nil
	}

	// 加载每个模板文件
	for _, file := range files {
		// 读取文件内容
		content, err := os.ReadFile(file)
		if err != nil {
			log.Printf("⚠️  读取提示词文件失败 %s: %v", file, err)
			continue
		}

		// 提取文件名（不含扩展名）作为模板名称
		fileName := filepath.Base(file)
		templateName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

		// 存储模板
		pm.templates[templateName] = &PromptTemplate{
			Name:    templateName,
			Content: string(content),
		}

		log.Printf("  📄 加载提示词模板: %s (%s)", templateName, fileName)
	}

	return nil
}

// GetTemplate 获取指定名称的提示词模板（优先从数据库获取，然后从文件）
func (pm *PromptManager) GetTemplate(name string) (*PromptTemplate, error) {
	pm.mu.RLock()
	db := pm.database
	pm.mu.RUnlock()

	// 优先从数据库获取（如果数据库可用）
	if db != nil {
		// 使用default用户获取系统模板，或当前用户获取自己的模板
		// 这里使用"default"作为fallback，实际使用时应该传入正确的userID
		// 但由于这是全局函数，暂时使用default
		templateConfig, err := db.GetPromptTemplate("default", name)
		if err == nil {
			return &PromptTemplate{
				Name:    templateConfig.Name,
				Content: templateConfig.Content,
			}, nil
		}
		// 如果数据库中没有找到，继续尝试文件系统
	}

	// 从文件系统获取（fallback）
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	template, exists := pm.templates[name]
	if !exists {
		return nil, fmt.Errorf("提示词模板不存在: %s", name)
	}

	return template, nil
}

// GetAllTemplateNames 获取所有模板名称列表（合并数据库和文件系统）
func (pm *PromptManager) GetAllTemplateNames() []string {
	pm.mu.RLock()
	db := pm.database
	fileNames := make(map[string]bool)
	for name := range pm.templates {
		fileNames[name] = true
	}
	pm.mu.RUnlock()

	// 如果数据库可用，从数据库获取模板名称
	if db != nil {
		dbTemplates, err := db.GetPromptTemplates("default")
		if err == nil {
			names := make([]string, 0, len(dbTemplates)+len(fileNames))
			nameSet := make(map[string]bool)

			// 先添加数据库模板名称
			for _, dbTemplate := range dbTemplates {
				names = append(names, dbTemplate.ID)
				nameSet[dbTemplate.ID] = true
			}

			// 再添加文件模板名称（如果数据库中没有）
			for name := range fileNames {
				if !nameSet[name] {
					names = append(names, name)
				}
			}

			return names
		}
	}

	// 只返回文件模板名称
	names := make([]string, 0, len(fileNames))
	for name := range fileNames {
		names = append(names, name)
	}

	return names
}

// GetAllTemplates 获取所有模板（合并数据库和文件系统的模板）
func (pm *PromptManager) GetAllTemplates() []*PromptTemplate {
	pm.mu.RLock()
	db := pm.database
	fileTemplates := make(map[string]*PromptTemplate)
	for k, v := range pm.templates {
		fileTemplates[k] = v
	}
	pm.mu.RUnlock()

	// 如果数据库可用，从数据库获取模板
	if db != nil {
		dbTemplates, err := db.GetPromptTemplates("default")
		if err == nil {
			// 合并数据库模板和文件模板（数据库优先）
			result := make([]*PromptTemplate, 0, len(dbTemplates)+len(fileTemplates))
			templateMap := make(map[string]bool)

			// 先添加数据库模板
			for _, dbTemplate := range dbTemplates {
				result = append(result, &PromptTemplate{
					Name:    dbTemplate.Name,
					Content: dbTemplate.Content,
				})
				templateMap[dbTemplate.ID] = true
			}

			// 再添加文件模板（如果数据库中没有）
			for name, fileTemplate := range fileTemplates {
				if !templateMap[name] {
					result = append(result, fileTemplate)
				}
			}

			return result
		}
	}

	// 只返回文件模板
	result := make([]*PromptTemplate, 0, len(fileTemplates))
	for _, template := range fileTemplates {
		result = append(result, template)
	}

	return result
}

// ReloadTemplates 重新加载所有模板
func (pm *PromptManager) ReloadTemplates(dir string) error {
	pm.mu.Lock()
	pm.templates = make(map[string]*PromptTemplate)
	pm.mu.Unlock()

	return pm.LoadTemplates(dir)
}

// === 全局函数（供外部调用）===

// GetPromptTemplate 获取指定名称的提示词模板（全局函数）
func GetPromptTemplate(name string) (*PromptTemplate, error) {
	return globalPromptManager.GetTemplate(name)
}

// GetAllPromptTemplateNames 获取所有模板名称（全局函数）
func GetAllPromptTemplateNames() []string {
	return globalPromptManager.GetAllTemplateNames()
}

// GetAllPromptTemplates 获取所有模板（全局函数）
func GetAllPromptTemplates() []*PromptTemplate {
	return globalPromptManager.GetAllTemplates()
}

// ReloadPromptTemplates 重新加载所有模板（全局函数）
func ReloadPromptTemplates() error {
	return globalPromptManager.ReloadTemplates(promptsDir)
}

// GetGlobalPromptManager 获取全局提示词管理器（用于设置数据库）
func GetGlobalPromptManager() *PromptManager {
	return globalPromptManager
}
