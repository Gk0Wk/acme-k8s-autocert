// Package utils 提供项目通用工具函数
package utils

import (
	"log"
	"os"
	"strings"
	"sync"
)

var (
	// IsDebugMode 标识是否启用调试模式
	IsDebugMode bool
	once        sync.Once
)

// InitLogger 初始化日志系统，检测环境变量并设置日志级别
func InitLogger() {
	once.Do(func() {
		debugEnv := os.Getenv("DEBUG_MODE")
		IsDebugMode = strings.ToLower(debugEnv) == "true"

		if IsDebugMode {
			log.Println("已启用调试模式")
			// 在调试模式下设置更详细的日志格式，包含文件名和行号
			log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
		}
	})
}

// DebugLog 打印调试日志，仅在DEBUG_MODE=true时输出
func DebugLog(format string, v ...interface{}) {
	if IsDebugMode {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// InfoLog 打印信息日志
func InfoLog(format string, v ...interface{}) {
	log.Printf("[INFO] "+format, v...)
}

// WarningLog 打印警告日志
func WarningLog(format string, v ...interface{}) {
	log.Printf("[WARNING] "+format, v...)
}

// ErrorLog 打印错误日志
func ErrorLog(format string, v ...interface{}) {
	log.Printf("[ERROR] "+format, v...)
}
