package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	ApiKey       string
	ApiSecret    string
	Testnet      bool
	InitialFunds float64
}

func LoadFromEnv(envPath string) (*Config, error) {
	err := godotenv.Load(envPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao carregar .env: %v", err)
	}

	apiKey := os.Getenv("BINANCE_API_KEY")
	apiSecret := os.Getenv("BINANCE_API_SECRET")
	testnet := os.Getenv("USE_TESTNET") == "true"
	
	initialFunds := 1000.0 // valor padrão
	if fundsStr := os.Getenv("INITIAL_FUNDS"); fundsStr != "" {
		initialFunds, err = strconv.ParseFloat(fundsStr, 64)
		if err != nil {
			return nil, fmt.Errorf("erro ao converter INITIAL_FUNDS para float: %v", err)
		}
	}
	
	return &Config{
		ApiKey:       apiKey,
		ApiSecret:    apiSecret,
		Testnet:      testnet,
		InitialFunds: initialFunds,
	}, nil
}
