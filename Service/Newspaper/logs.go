package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	LogLevelDebug   = 0
	LogLevelInfo    = 1
	LogLevelWarning = 2
	LogLevelError   = 3
	LogLevelFatal   = 4
)

type AppLog struct {
	ID      int       `json:"id"`
	Level   string    `json:"level"`
	Module  string    `json:"module"`
	Message string    `json:"message"`
	Details string    `json:"details"`
	LogTime time.Time `json:"log_time"`
}

type LogSystem struct {
	mu       sync.RWMutex
	logFile  *os.File
	logLevel int
	dbPath   string
}

var logger *LogSystem

func InitLogger() error {
	logger = &LogSystem{
		logLevel: LogLevelInfo,
	}

	logDir := filepath.Join("..", "..", "db", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile := filepath.Join(logDir, "app.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	logger.logFile = f

	logger.Info("system", "日志系统初始化完成")
	return nil
}

func (l *LogSystem) Debug(module, message string, args ...interface{}) {
	if l.logLevel > LogLevelDebug {
		return
	}
	l.writeLog("DEBUG", module, message, args...)
}

func (l *LogSystem) Info(module, message string, args ...interface{}) {
	if l.logLevel > LogLevelInfo {
		return
	}
	l.writeLog("INFO", module, message, args...)
}

func (l *LogSystem) Warning(module, message string, args ...interface{}) {
	if l.logLevel > LogLevelWarning {
		return
	}
	l.writeLog("WARNING", module, message, args...)
}

func (l *LogSystem) Error(module, message string, args ...interface{}) {
	if l.logLevel > LogLevelError {
		return
	}
	l.writeLog("ERROR", module, message, args...)
}

func (l *LogSystem) Fatal(module, message string, args ...interface{}) {
	l.writeLog("FATAL", module, message, args...)
}

func (l *LogSystem) writeLog(level, module, message string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	formattedMsg := fmt.Sprintf(message, args...)

	logEntry := AppLog{
		Level:   level,
		Module:  module,
		Message: formattedMsg,
		LogTime: time.Now(),
	}

	logText := fmt.Sprintf("[%s] [%s] [%s] %s\n", timestamp, level, module, formattedMsg)

	if l.logFile != nil {
		l.logFile.WriteString(logText)
	}

	fmt.Print(logText)

	if db != nil {
		details := ""
		if len(args) > 0 {
			if b, err := json.Marshal(args); err == nil {
				details = string(b)
			}
		}
		SaveLogToDB(logEntry.Level, logEntry.Module, logEntry.Message, details)
	}
}

func SaveLogToDB(level, module, message, details string) {
	if db == nil {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(
		"INSERT INTO app_logs (level, module, message, details, log_time) VALUES (?, ?, ?, ?, ?)",
		level, module, message, details, now,
	)
	if err != nil {
		fmt.Printf("Warning: failed to save log to DB: %v\n", err)
	}
}

func GetLogs(level string, module string, limit int, offset int) ([]AppLog, error) {
	query := "SELECT id, level, module, message, details, log_time FROM app_logs WHERE 1=1"
	args := []interface{}{}

	if level != "" && level != "all" {
		query += " AND level = ?"
		args = append(args, level)
	}

	if module != "" {
		query += " AND module LIKE ?"
		args = append(args, "%"+module+"%")
	}

	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []AppLog
	for rows.Next() {
		var l AppLog
		var details sql.NullString
		var logTime string
		if err := rows.Scan(&l.ID, &l.Level, &l.Module, &l.Message, &details, &logTime); err != nil {
			continue
		}
		if details.Valid {
			l.Details = details.String
		}
		l.LogTime, _ = time.Parse("2006-01-02 15:04:05", logTime)
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []AppLog{}
	}
	return logs, nil
}

func GetLogCount(level string, module string) (int, error) {
	query := "SELECT COUNT(*) FROM app_logs WHERE 1=1"
	args := []interface{}{}

	if level != "" && level != "all" {
		query += " AND level = ?"
		args = append(args, level)
	}

	if module != "" {
		query += " AND module LIKE ?"
		args = append(args, "%"+module+"%")
	}

	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	return count, err
}

func CleanLogs(olderThanDays int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -olderThanDays).Format("2006-01-02 15:04:05")
	result, err := db.Exec("DELETE FROM app_logs WHERE log_time < ?", cutoff)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func GetLogLevels() ([]string, error) {
	rows, err := db.Query("SELECT DISTINCT level FROM app_logs ORDER BY level")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var levels []string
	for rows.Next() {
		var level string
		if err := rows.Scan(&level); err != nil {
			continue
		}
		levels = append(levels, level)
	}
	if levels == nil {
		levels = []string{}
	}
	return levels, nil
}

func GetLogModules() ([]string, error) {
	rows, err := db.Query("SELECT DISTINCT module FROM app_logs ORDER BY module")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modules []string
	for rows.Next() {
		var module string
		if err := rows.Scan(&module); err != nil {
			continue
		}
		modules = append(modules, module)
	}
	if modules == nil {
		modules = []string{}
	}
	return modules, nil
}

func SetLogLevel(level int) {
	if logger != nil {
		logger.mu.Lock()
		defer logger.mu.Unlock()
		logger.logLevel = level
	}
}

func GetLogLevel() int {
	if logger != nil {
		logger.mu.RLock()
		defer logger.mu.RUnlock()
		return logger.logLevel
	}
	return LogLevelInfo
}

func GetLogLevelName(level int) string {
	switch level {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarning:
		return "WARNING"
	case LogLevelError:
		return "ERROR"
	case LogLevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

func ParseLogLevelName(name string) int {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "DEBUG":
		return LogLevelDebug
	case "INFO":
		return LogLevelInfo
	case "WARNING", "WARN":
		return LogLevelWarning
	case "ERROR":
		return LogLevelError
	case "FATAL":
		return LogLevelFatal
	default:
		return LogLevelInfo
	}
}
