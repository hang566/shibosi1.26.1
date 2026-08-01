package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

// Logger 日志记录器
type Logger struct {
	infoLog  *log.Logger
	errLog   *log.Logger
	file     *os.File
}

// NewLogger 创建新的日志记录器
func NewLogger() *Logger {
	logDir := "logs"
	os.MkdirAll(logDir, 0755)

	logFile, err := os.OpenFile(
		fmt.Sprintf("%s/search_%s.log", logDir, time.Now().Format("2006-01-02")),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)
	if err != nil {
		log.Printf("Warning: cannot open log file: %v, using stdout only", err)
		return &Logger{
			infoLog: log.New(os.Stdout, "[INFO] ", log.LstdFlags|log.Lshortfile),
			errLog:  log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lshortfile),
		}
	}

	return &Logger{
		infoLog: log.New(logFile, "[INFO] ", log.LstdFlags),
		errLog:  log.New(logFile, "[ERROR] ", log.LstdFlags),
		file:    logFile,
	}
}

// Info 记录信息日志
func (l *Logger) Info(format string, v ...interface{}) {
	l.infoLog.Printf(format, v...)
}

// Error 记录错误日志
func (l *Logger) Error(format string, v ...interface{}) {
	l.errLog.Printf(format, v...)
}

// SearchLog 记录搜索日志
func (l *Logger) SearchLog(query string, engine string, resultCount int, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = fmt.Sprintf("error: %v", err)
	}
	l.infoLog.Printf("SEARCH query=%q engine=%s results=%d duration=%v status=%s",
		query, engine, resultCount, duration, status)
}

// Close 关闭日志文件
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}
