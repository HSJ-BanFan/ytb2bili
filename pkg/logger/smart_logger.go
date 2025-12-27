package logger

import (
	"go.uber.org/zap"
)

// SmartLogger 智能日志包装器，自动处理进度显示期间的日志输出
type SmartLogger struct {
	sugarLogger *zap.SugaredLogger
	progressMgr *ProgressLogManager
}

// NewSmartLogger 创建智能日志包装器
func NewSmartLogger(sugarLogger *zap.SugaredLogger) *SmartLogger {
	return &SmartLogger{
		sugarLogger: sugarLogger,
		progressMgr: GetProgressManager(),
	}
}

// Debug 输出 debug 级别日志
func (sl *SmartLogger) Debug(args ...interface{}) {
	sl.progressMgr.LogWithProgressCheck("debug", args...)
}

// Debugf 输出格式化 debug 日志
func (sl *SmartLogger) Debugf(template string, args ...interface{}) {
	sl.progressMgr.LogfWithProgressCheck("debug", template, args...)
}

// Info 输出 info 级别日志
func (sl *SmartLogger) Info(args ...interface{}) {
	sl.progressMgr.LogWithProgressCheck("info", args...)
}

// Infof 输出格式化 info 日志
func (sl *SmartLogger) Infof(template string, args ...interface{}) {
	sl.progressMgr.LogfWithProgressCheck("info", template, args...)
}

// Warn 输出 warn 级别日志
func (sl *SmartLogger) Warn(args ...interface{}) {
	sl.progressMgr.LogWithProgressCheck("warn", args...)
}

// Warnf 输出格式化 warn 日志
func (sl *SmartLogger) Warnf(template string, args ...interface{}) {
	sl.progressMgr.LogfWithProgressCheck("warn", template, args...)
}

// Error 输出 error 级别日志
func (sl *SmartLogger) Error(args ...interface{}) {
	sl.progressMgr.LogWithProgressCheck("error", args...)
}

// Errorf 输出格式化 error 日志
func (sl *SmartLogger) Errorf(template string, args ...interface{}) {
	sl.progressMgr.LogfWithProgressCheck("error", template, args...)
}

// Fatal 输出 fatal 级别日志并退出
func (sl *SmartLogger) Fatal(args ...interface{}) {
	sl.progressMgr.LogWithProgressCheck("error", args...)
	sl.sugarLogger.Fatal(args...)
}

// Fatalf 输出格式化 fatal 日志并退出
func (sl *SmartLogger) Fatalf(template string, args ...interface{}) {
	sl.progressMgr.LogfWithProgressCheck("error", template, args...)
	sl.sugarLogger.Fatalf(template, args...)
}

// With 添加字段到日志
func (sl *SmartLogger) With(args ...interface{}) *zap.SugaredLogger {
	return sl.sugarLogger.With(args...)
}
