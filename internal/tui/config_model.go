package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"

	traderbot "github.com/casarotto/binance-bot/internal/trader-bot"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type ConfigModel struct {
	trader    *traderbot.BTCTrader
	lastPrice float64
	quitting  bool
}

func NewConfigModel(trader *traderbot.BTCTrader) *ConfigModel {
	model := &ConfigModel{
		trader: trader,
	}
	
	// Buscar último preço de compra do histórico de trades
	trades := trader.GetTradeHistory()
	for i := len(trades) - 1; i >= 0; i-- {
		if trades[i].Action == "buy" {
			model.lastPrice = trades[i].Price
			break
		}
	}
	
	return model
}

func (m *ConfigModel) getCurrentPrice() (float64, error) {
	// Buscar o preço atual do BTC via API da Binance
	prices, err := m.trader.GetClient().NewListPricesService().Symbol("BTCUSDT").Do(context.Background())
	if err != nil {
		return 0, err
	}

	if len(prices) > 0 {
		return strconv.ParseFloat(prices[0].Price, 64)
	}

	return 0, fmt.Errorf("preço não encontrado")
}

func (m ConfigModel) Init() tea.Cmd {
	return nil
}

func (m ConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "y":
			m.trader.SetInitialPosition(true, m.lastPrice)
			return m, tea.Quit
		case "n":
			m.trader.SetInitialPosition(false, 0)
			return m, tea.Quit
		}
	case tickMsg:
		// Atualizar preço atual
		if len(m.trader.GetPrices()) > 0 {
			m.lastPrice = m.trader.GetPrices()[len(m.trader.GetPrices())-1]
		}
		return m, tickCmd()
	}

	return m, nil
}

func (m ConfigModel) View() string {
	// Obter tamanho do terminal
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))

	configStyle := sectionStyle.Copy().Width(width/2).Align(lipgloss.Center)
	
	content := configStyle.Render(
		titleStyle.Render("🤖 Binance Trading Bot - Configuração Inicial") + "\n\n" +
		sectionHeaderStyle.Render("⚙️ Configuração da Posição") + "\n\n" +
		fmt.Sprintf("Preço Atual: %s\n\n", priceStyle.Render(fmt.Sprintf("$%.2f", m.lastPrice))) +
		"Você está em posição?\n\n" +
		positiveStyle.Render("[y] Sim, usar preço atual como entrada") + "\n" +
		warningStyle.Render("[n] Não, começar fora do mercado") + "\n\n" +
		infoStyle.Render("Pressione 'esc' para sair sem configurar"),
	)

	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
} 