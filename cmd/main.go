package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	traderbot "github.com/casarotto/binance-bot/internal/trader-bot"
	"github.com/joho/godotenv"
)

func main() {
	// Flags de linha de comando
	envPath := flag.String("env", ".env", "Caminho para o arquivo .env")
	flag.Parse()

	// Tentar carregar o .env
	if err := godotenv.Load(*envPath); err != nil {
		log.Printf(`
❌ Erro ao carregar arquivo .env: %v

Por favor, crie um arquivo .env com o seguinte conteúdo:
BINANCE_API_KEY=sua_api_key_aqui
BINANCE_API_SECRET=seu_api_secret_aqui

Você pode:
1. Criar o arquivo .env no diretório atual, ou
2. Especificar um caminho alternativo: ./bot -env=/caminho/para/.env
`, err)
		os.Exit(1)
	}

	// Verificar se as variáveis necessárias existem
	apiKey := os.Getenv("BINANCE_API_KEY")
	apiSecret := os.Getenv("BINANCE_API_SECRET")

	if apiKey == "" || apiSecret == "" {
		log.Fatal(`
❌ Credenciais da Binance não encontradas no arquivo .env

O arquivo .env deve conter:
BINANCE_API_KEY=sua_api_key_aqui
BINANCE_API_SECRET=seu_api_secret_aqui
`)
	}
	
	// Criar diretório para histórico se não existir
	historyDir := "history"
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		log.Fatal("Erro ao criar diretório de histórico:", err)
	}

	// Arquivo de histórico
	historyFile := filepath.Join(historyDir, "trade_history.json")

	// Criar e iniciar o trader
	trader := traderbot.NewBTCTrader(
		apiKey,
		apiSecret,
		1000.0,  // Fundos iniciais
		true,    // Usar testnet
		historyFile,
	)

	fmt.Println(`
🤖 Bot de Trading Iniciado!
📊 Histórico será salvo em: history/trade_history.json
⚠️  Usando TESTNET da Binance

Pressione Ctrl+C para encerrar.
`)

	if err := trader.Start(); err != nil {
		log.Fatal(err)
	}
}
