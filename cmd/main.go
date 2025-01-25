package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/casarotto/binance-bot/internal/config"
	traderbot "github.com/casarotto/binance-bot/internal/trader-bot"
	"github.com/casarotto/binance-bot/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Flags de linha de comando
	envPath := flag.String("env", ".env", "Caminho para o arquivo .env")
	flag.Parse()

	// Carregar configurações
	cfg, err := config.LoadFromEnv(*envPath)
	if err != nil {
		log.Printf(`
❌ Erro ao carregar configurações: %v

Por favor, crie um arquivo .env com o seguinte conteúdo:
BINANCE_API_KEY=sua_api_key_aqui
BINANCE_API_SECRET=seu_api_secret_aqui
INITIAL_FUNDS=100.0
USE_TESTNET=true

Você pode:
1. Criar o arquivo .env no diretório atual, ou
2. Especificar um caminho alternativo: ./bot -env=/caminho/para/.env
`, err)
		os.Exit(1)
	}

	// Criar diretório para histórico e logs
	historyDir := "history"
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		log.Fatal("Erro ao criar diretório de histórico:", err)
	}

	// Configurar logger
	logger, err := traderbot.NewLogger(historyDir)
	if err != nil {
		log.Fatal("Erro ao criar logger:", err)
	}
	defer logger.Close()

	// Arquivo de histórico
	historyFile := filepath.Join(historyDir, "trade_history.json")

	// Criar o trader
	trader := traderbot.NewBTCTrader(
		cfg.ApiKey,
		cfg.ApiSecret,
		cfg.InitialFunds,
		cfg.Testnet,
		historyFile,
	)

	// Configurar o logger do trader
	trader.SetLogger(logger)

	// Criar e iniciar o TUI para configuração inicial
	configModel := tui.NewConfigModel(trader)
	configProgram := tea.NewProgram(configModel)
	if err := configProgram.Start(); err != nil {
		logger.Fatal("Erro ao iniciar configuração:", err)
	}

	// Iniciar o trader em uma goroutine separada
	go func() {
		if err := trader.Start(); err != nil {
			logger.Fatal(err)
		}
	}()

	// Criar e iniciar o TUI principal
	model := tui.New(trader)
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Usar tela alternativa
		tea.WithMouseCellMotion(), // Habilitar suporte a mouse
	)
	
	if err := p.Start(); err != nil {
		logger.Fatal("Erro ao iniciar TUI:", err)
	}
}
