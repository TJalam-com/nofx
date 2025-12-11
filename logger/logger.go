package logger

import (
	"fmt"
	"io"
	"log"
	"nofx/config"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	// Log is the global logger instance
	Log *logrus.Logger

	// logFile holds the current log file handle
	logFile *os.File

	// telegramHook saves hook reference for graceful shutdown
	telegramHook *TelegramHook
)

// compactFormatter is a custom formatter for cleaner log output
type compactFormatter struct {
	logrus.TextFormatter
}

func (f *compactFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	level := strings.ToUpper(entry.Level.String())[0:4]

	// Skip frames to find actual caller (skip logrus + our wrapper functions)
	caller := ""
	for i := 3; i < 10; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Skip logrus internal and our logger.go
		if !strings.Contains(file, "logrus") && !strings.HasSuffix(file, "logger/logger.go") {
			// Get package name from path (e.g., "nofx/manager/trader_manager.go" -> "manager")
			dir := filepath.Dir(file)
			pkg := filepath.Base(dir)
			caller = fmt.Sprintf("%s/%s:%d", pkg, filepath.Base(file), line)
			break
		}
	}

	msg := fmt.Sprintf("[%s] %s %s\n", level, caller, entry.Message)
	return []byte(msg), nil
}

func init() {
	// Auto-initialize default logger to ensure it works before Init is called
	Log = logrus.New()
	Log.SetLevel(logrus.InfoLevel)
	Log.SetFormatter(&compactFormatter{})
	Log.SetOutput(os.Stdout)
}

// ============================================================================
// Initialization functions
// ============================================================================

// Init initializes the global logger
// If config is nil, uses default configuration (console output, info level)
func Init(cfg *Config) error {
	Log = logrus.New()

	// Use default values if no config provided
	if cfg == nil {
		cfg = &Config{Level: "info"}
	}

	// Set default values
	cfg.SetDefaults()

	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	Log.SetLevel(level)

	// Set compact formatter
	Log.SetFormatter(&compactFormatter{})

	// Setup log file output (write to both stdout and file)
	logDir := "data"
	if err := os.MkdirAll(logDir, 0755); err == nil {
		logFileName := filepath.Join(logDir, fmt.Sprintf("nofx_%s.log", time.Now().Format("2006-01-02")))
		f, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			logFile = f
			// Write to both stdout and file
			Log.SetOutput(io.MultiWriter(os.Stdout, f))
		} else {
			Log.SetOutput(os.Stdout)
		}
	} else {
		Log.SetOutput(os.Stdout)
	}

	Log.SetReportCaller(true)

	// Add Telegram Hook (optional)
	if cfg.Telegram != nil && cfg.Telegram.Enabled {
		if err := setupTelegramHook(cfg.Telegram); err != nil {
			Log.Warnf("Failed to initialize Telegram push, will continue using regular logging: %v", err)
		}
	}

	return nil
}

// setupTelegramHook sets up Telegram Hook
func setupTelegramHook(telegramCfg *TelegramConfig) error {
	hook, err := NewTelegramHook(telegramCfg)
	if err != nil {
		return err
	}

	Log.AddHook(hook)
	telegramHook = hook
	Log.Info("✅ Telegram log push enabled")
	return nil
}

// InitWithSimpleConfig initializes logger with simplified config
// Suitable for scenarios that only need basic functionality
func InitWithSimpleConfig(level string) error {
	return Init(&Config{Level: level})
}

// InitWithTelegram initializes logger with Telegram configuration
func InitWithTelegram(botToken string, chatID int64) error {
	return Init(&Config{
		Level: "info",
		Telegram: &TelegramConfig{
			Enabled:  true,
			BotToken: botToken,
			ChatID:   chatID,
		},
	})
}

// InitFromLogConfig initializes logger from config.LogConfig
func InitFromLogConfig(logConfig *config.LogConfig) error {
	if logConfig == nil {
		return InitWithSimpleConfig("info")
	}

	cfg := &Config{
		Level: logConfig.Level,
	}

	if cfg.Level == "" {
		cfg.Level = "info"
	}

	// If Telegram is enabled, add configuration
	if logConfig.Telegram != nil && logConfig.Telegram.Enabled {
		if botToken := logConfig.Telegram.BotToken; botToken != "" && logConfig.Telegram.ChatID != 0 {
			cfg.Telegram = &TelegramConfig{
				Enabled:  true,
				BotToken: botToken,
				ChatID:   logConfig.Telegram.ChatID,
				MinLevel: logConfig.Telegram.MinLevel,
			}
		}
	}

	return Init(cfg)
}

// InitFromParams initializes logger from parameters
// Suitable for scenarios that don't depend on config package
func InitFromParams(level string, telegramEnabled bool, botToken string, chatID int64) error {
	cfg := &Config{Level: level}

	if telegramEnabled && botToken != "" && chatID != 0 {
		cfg.Telegram = &TelegramConfig{
			Enabled:  true,
			BotToken: botToken,
			ChatID:   chatID,
		}
	}

	return Init(cfg)
}

// Shutdown gracefully shuts down logger (mainly for closing Telegram sender and log file)
func Shutdown() {
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	if telegramHook != nil {
		telegramHook.Stop()
		telegramHook = nil
	}
}

// ============================================================================
// Logging functions
// ============================================================================

// ensureLogger ensures Log is initialized, initializes with default config if nil
func ensureLogger() {
	if Log == nil {
		// Initialize with default config if not already initialized
		if err := InitWithSimpleConfig("info"); err != nil {
			// If initialization fails, Log will still be nil
			// This should not happen, but we handle it gracefully
			return
		}
	}
}

// WithFields creates logger entry with fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	ensureLogger()
	if Log == nil {
		// If Log is still nil after ensureLogger, return a no-op entry
		// This prevents panic but caller should check for nil
		return logrus.NewEntry(logrus.New())
	}
	return Log.WithFields(fields)
}

// WithField creates logger entry with single field
func WithField(key string, value interface{}) *logrus.Entry {
	ensureLogger()
	if Log == nil {
		// If Log is still nil after ensureLogger, return a no-op entry
		// This prevents panic but caller should check for nil
		return logrus.NewEntry(logrus.New())
	}
	return Log.WithField(key, value)
}

// add debug, info, warn
func Debug(args ...interface{}) {
	if Log == nil {
		log.Print(append([]interface{}{"[DEBUG] "}, args...)...)
		return
	}
	Log.Debug(args...)
}

func Info(args ...interface{}) {
	if Log == nil {
		log.Print(append([]interface{}{"[INFO] "}, args...)...)
		return
	}
	Log.Info(args...)
}

func Warn(args ...interface{}) {
	if Log == nil {
		log.Print(append([]interface{}{"[WARN] "}, args...)...)
		return
	}
	Log.Warn(args...)
}

func Debugf(format string, args ...interface{}) {
	if Log == nil {
		log.Printf("[DEBUG] "+format, args...)
		return
	}
	Log.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	if Log == nil {
		log.Printf("[INFO] "+format, args...)
		return
	}
	Log.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	if Log == nil {
		log.Printf("[WARN] "+format, args...)
		return
	}
	Log.Warnf(format, args...)
}

func Error(args ...interface{}) {
	if Log == nil {
		log.Print(append([]interface{}{"[ERROR] "}, args...)...)
		return
	}
	Log.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	if Log == nil {
		log.Printf("[ERROR] "+format, args...)
		return
	}
	Log.Errorf(format, args...)
}

func Fatal(args ...interface{}) {
	if Log == nil {
		log.Fatal(append([]interface{}{"[FATAL] "}, args...)...)
		return
	}
	Log.Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	if Log == nil {
		log.Fatalf("[FATAL] "+format, args...)
		return
	}
	Log.Fatalf(format, args...)
}

func Panic(args ...interface{}) {
	if Log == nil {
		log.Panic(append([]interface{}{"[PANIC] "}, args...)...)
		return
	}
	Log.Panic(args...)
}

func Panicf(format string, args ...interface{}) {
	if Log == nil {
		log.Panicf("[PANIC] "+format, args...)
		return
	}
	Log.Panicf(format, args...)
}
