package traderbot

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogEntry struct {
	Timestamp time.Time
	Message   string
}

type Logger struct {
	*log.Logger
	file      *os.File
	logs      []LogEntry
	maxLogs   int
	logsLock  sync.RWMutex
}

func NewLogger(historyDir string) (*Logger, error) {
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return nil, fmt.Errorf("erro ao criar diretório de logs: %v", err)
	}

	logFile := filepath.Join(historyDir, "bot.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar arquivo de log: %v", err)
	}

	return &Logger{
		Logger:   log.New(file, "", log.LstdFlags),
		file:     file,
		logs:     make([]LogEntry, 0),
		maxLogs:  100, // manter apenas os últimos 100 logs
	}, nil
}

func (l *Logger) Close() error {
	return l.file.Close()
}

// LogImportant registra apenas mensagens importantes
func (l *Logger) LogImportant(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.Logger.Printf(msg) // Salvar no arquivo

	l.logsLock.Lock()
	defer l.logsLock.Unlock()

	// Adicionar ao buffer circular
	l.logs = append(l.logs, LogEntry{
		Timestamp: time.Now(),
		Message:   msg,
	})

	// Manter apenas os últimos maxLogs
	if len(l.logs) > l.maxLogs {
		l.logs = l.logs[1:]
	}
}

// GetRecentLogs retorna os logs mais recentes
func (l *Logger) GetRecentLogs() []LogEntry {
	l.logsLock.RLock()
	defer l.logsLock.RUnlock()
	
	return l.logs
} 