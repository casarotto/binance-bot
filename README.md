# 🤖 Binance Trading Bot

Automated Bitcoin trading bot for Binance, developed in Go.

## 🌟 Features

- Interactive and user-friendly TUI (Terminal User Interface)
- Technical indicators:
  - RSI (Relative Strength Index)
  - Moving Averages (9 and 21 periods)
- Automated trading strategy
- Binance testnet support
- Trade history tracking
- Detailed logging
- Real-time charts
- Docker containerization

## 🚀 Getting Started

### Prerequisites

- Go 1.23.5 or higher
- Docker (optional)
- Binance account (real or testnet)

### Setup

1. Clone the repository
2. Create a `.env` file in the project root:

```env
BINANCE_API_KEY=your_api_key_here
BINANCE_API_SECRET=your_api_secret_here
INITIAL_FUNDS=100.0
USE_TESTNET=true
```

### Running

#### Locally

```bash
go run cmd/main.go
```

#### With Docker

```bash
docker-compose up -d
```

## 📊 Trading Strategy

The bot uses a combination of technical indicators:

- **Buy Signals:**
  - RSI < 30
  - MA9 > MA21

- **Sell Signals:**
  - RSI > 70
  - MA9 < MA21 (with RSI > 50)
  - Profit > 0.3%

## 🛠️ Technologies

- [Go](https://golang.org/)
- [Binance API](https://github.com/adshao/go-binance)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) (TUI)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) (Styling)
- [TermUI](https://github.com/gizak/termui) (Charts)
- [Docker](https://www.docker.com/)

## 📝 License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details. 
