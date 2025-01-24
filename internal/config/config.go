package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ApiKey     string
	ApiSecret  string
	Testnet    bool
}

func LoadFromEnv() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("erro ao carregar .env: %v", err)
	}

	apiKey := os.Getenv("BINANCE_API_KEY")
	apiSecret := os.Getenv("BINANCE_API_SECRET")
	testnet := os.Getenv("BINANCE_TESTNET") == "true"
	
	return &Config{
		ApiKey:     apiKey,
		ApiSecret:  apiSecret,
		Testnet:    testnet,
	}, nil
}
