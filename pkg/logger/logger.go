package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// String 返回日志级别的字符串表示
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger 日志记录器接口
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	SetLevel(level LogLevel)
}

// DefaultLogger 默认日志记录器实现
type DefaultLogger struct {
	level  LogLevel
	logger *log.Logger
}

// NewLogger 创建新的日志记录器
func NewLogger(levelStr string) Logger {
	level := parseLogLevel(levelStr)
	return &DefaultLogger{
		level:  level,
		logger: log.New(os.Stdout, "", 0),
	}
}

// parseLogLevel 解析日志级别字符串
func parseLogLevel(levelStr string) LogLevel {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

// Debug 记录调试级别日志
func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	if l.level <= DEBUG {
		l.log(DEBUG, msg, args...)
	}
}

// Info 记录信息级别日志
func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	if l.level <= INFO {
		l.log(INFO, msg, args...)
	}
}

// Warn 记录警告级别日志
func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	if l.level <= WARN {
		l.log(WARN, msg, args...)
	}
}

// Error 记录错误级别日志
func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	if l.level <= ERROR {
		l.log(ERROR, msg, args...)
	}
}

// SetLevel 设置日志级别
func (l *DefaultLogger) SetLevel(level LogLevel) {
	l.level = level
}

// log 内部日志记录方法
func (l *DefaultLogger) log(level LogLevel, msg string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelStr := level.String()
	
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	
	logMsg := fmt.Sprintf("[%s] %s: %s", timestamp, levelStr, msg)
	l.logger.Println(logMsg)
}

// 全局日志记录器实例
var globalLogger Logger = NewLogger("info")

// SetGlobalLogger 设置全局日志记录器
func SetGlobalLogger(logger Logger) {
	globalLogger = logger
}

// Debug 全局调试日志
func Debug(msg string, args ...interface{}) {
	globalLogger.Debug(msg, args...)
}

// Info 全局信息日志
func Info(msg string, args ...interface{}) {
	globalLogger.Info(msg, args...)
}

// Warn 全局警告日志
func Warn(msg string, args ...interface{}) {
	globalLogger.Warn(msg, args...)
}

// Error 全局错误日志
func Error(msg string, args ...interface{}) {
	globalLogger.Error(msg, args...)
}

// SetLevel 设置全局日志级别
func SetLevel(level LogLevel) {
	globalLogger.SetLevel(level)
}
