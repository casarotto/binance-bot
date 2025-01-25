package tui

import (
	"fmt"
	"image"
	"os"
	"time"

	traderbot "github.com/casarotto/binance-bot/internal/trader-bot"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"golang.org/x/term"
)

var (
	highlightColor = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	
	// Estilos para diferentes seções
	titleStyle = lipgloss.NewStyle().
		Foreground(highlightColor).
		Bold(true).
		Padding(1, 2).
		MarginBottom(1)

	sectionStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(highlightColor).
		Padding(1, 2)

	// Adicionar estilo para cabeçalhos de seção
	sectionHeaderStyle = lipgloss.NewStyle().
		Foreground(highlightColor).
		Bold(true).
		MarginBottom(1)

	// Adicionar estilo para loading
	loadingStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Italic(true)

	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 1).
		MarginTop(1)

	// Estilos específicos para cada tipo de informação
	priceStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	rsiStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("213"))

	maStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	warningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("203"))

	positiveStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	negativeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("161"))
)

// Mensagem para atualizar os dados
type tickMsg time.Time

// Modelo principal do TUI
type Model struct {
	trader      *traderbot.BTCTrader
	lastPrice   float64
	rsi         float64
	maShort     float64
	maLong      float64
	inPosition  bool
	entryPrice  float64
	btcBalance  float64
	usdtBalance float64
	table       table.Model
	err         error
	currentTab  int    // Nova variável para controlar a aba atual
	showConfig  bool   // Controla se está mostrando a tela de configuração
}

func New(trader *traderbot.BTCTrader) *Model {
	columns := []table.Column{
		{Title: "Timestamp", Width: 20},
		{Title: "Ação", Width: 10},
		{Title: "Preço", Width: 15},
		{Title: "Quantidade", Width: 15},
		{Title: "Lucro/Perda", Width: 15},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(highlightColor).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(highlightColor).
		Bold(true)
	t.SetStyles(s)

	return &Model{
		trader:     trader,
		table:      t,
		currentTab: 0,
		showConfig: false,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		tea.EnterAltScreen,
	)
}

func tickCmd() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "right", "l":
			if !m.showConfig {
				m.currentTab = (m.currentTab + 1) % 2
			}
		case "shift+tab", "left", "h":
			if !m.showConfig {
				m.currentTab = (m.currentTab - 1 + 2) % 2
			}
		case "c":
			if !m.showConfig {
				m.showConfig = true
			}
		case "y":
			if m.showConfig {
				m.trader.SetInitialPosition(true, m.lastPrice)
				m.showConfig = false
			}
		case "n":
			if m.showConfig {
				m.trader.SetInitialPosition(false, 0)
				m.showConfig = false
			}
		case "esc":
			if m.showConfig {
				m.showConfig = false
			}
		}

	case tickMsg:
		// Atualizar dados a cada tick
		m.updateData()
		return m, tickCmd()
	}

	return m, nil
}

func (m *Model) updateData() {
	// Atualizar preço e indicadores
	if len(m.trader.GetPrices()) > 0 {
		m.lastPrice = m.trader.GetPrices()[len(m.trader.GetPrices())-1]
		m.rsi = m.trader.CalculateRSI()
		m.maShort = m.trader.CalculateMA(m.trader.GetMAShortPeriod())
		m.maLong = m.trader.CalculateMA(m.trader.GetMALongPeriod())
	}

	// Atualizar posição e saldos
	m.inPosition = m.trader.IsInPosition()
	m.entryPrice = m.trader.GetEntryPrice()
	m.btcBalance, m.usdtBalance, _ = m.trader.GetBalances()

	// Atualizar histórico de trades
	trades := m.trader.GetTradeHistory()
	rows := make([]table.Row, len(trades))
	for i, trade := range trades {
		rows[i] = table.Row{
			time.Unix(trade.Timestamp, 0).Format("2006-01-02 15:04:05"),
			trade.Action,
			fmt.Sprintf("$%.2f", trade.Price),
			fmt.Sprintf("%.8f", trade.Quantity),
			fmt.Sprintf("$%.2f", trade.ProfitLoss),
		}
	}
	m.table.SetRows(rows)
}

// Funções auxiliares para formatação
func (m Model) formatRSI() string {
	if m.rsi == 0 {
		return "Carregando..."
	}
	
	value := fmt.Sprintf("%.2f", m.rsi)
	if m.rsi > 70 {
		return warningStyle.Render(value + " ↑")
	} else if m.rsi < 30 {
		return positiveStyle.Render(value + " ↓")
	}
	return value
}

func (m Model) formatMA(period int) string {
	var value float64
	if period == 9 {
		value = m.maShort
	} else {
		value = m.maLong
	}
	
	if value == 0 {
		return "Carregando..."
	}
	
	return fmt.Sprintf("%.2f", value)
}

// createPriceChart cria um gráfico ASCII simples dos últimos preços
func createPriceChart(prices []float64, width, height int) string {
	if len(prices) < 2 {
		return ""
	}

	// Encontrar min e max para normalização
	min := prices[0]
	max := prices[0]
	for _, p := range prices {
		if p < min {
			min = p
		}
		if p > max {
			max = p
		}
	}

	// Caracteres para o gráfico
	chars := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

	// Criar o gráfico
	var chart string
	startIdx := 0
	if len(prices) > width {
		startIdx = len(prices) - width
	}
	
	for i := startIdx; i < len(prices); i++ {
		// Normalizar o valor entre 0 e 7 (número de caracteres disponíveis)
		normalized := 0
		if max > min {
			normalized = int(((prices[i] - min) / (max - min)) * 7)
		}
		if normalized < 0 {
			normalized = 0
		} else if normalized > 7 {
			normalized = 7
		}
		chart += chars[normalized]
	}

	// Adicionar min e max
	result := fmt.Sprintf("Max: $%.2f\n%s\nMin: $%.2f", max, chart, min)
	return result
}

// createBrailleChart cria um gráfico usando termui em modo braille
func createBrailleChart(prices []float64, width, height int) string {
	if len(prices) < 2 {
		return ""
	}

	// Inicializar termui
	if err := ui.Init(); err != nil {
		return fmt.Sprintf("Erro ao inicializar termui: %v", err)
	}
	defer ui.Close()

	// Criar plot
	p := widgets.NewPlot()
	p.Title = "Preço BTC/USDT"
	p.Data = make([][]float64, 1)
	p.Data[0] = prices
	p.SetRect(0, 0, width, height)
	p.AxesColor = ui.ColorWhite
	p.LineColors[0] = ui.ColorGreen
	p.Marker = widgets.MarkerBraille
	p.PlotType = widgets.ScatterPlot

	// Renderizar em um buffer
	buf := ui.NewBuffer(image.Rect(0, 0, width, height))
	p.Draw(buf)

	// Converter buffer para string
	var result string
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := buf.GetCell(image.Pt(x, y))
			result += string(cell.Rune)
		}
		result += "\n"
	}

	return result
}

func (m Model) View() string {
	// Obter tamanho do terminal
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))

	// Se estiver mostrando a tela de configuração
	if m.showConfig {
		configStyle := sectionStyle.Copy().Width(width/2).Align(lipgloss.Center)
		
		content := configStyle.Render(
			sectionHeaderStyle.Render("⚙️ Configuração Inicial") + "\n\n" +
			fmt.Sprintf("Preço Atual: %s\n\n", priceStyle.Render(fmt.Sprintf("$%.2f", m.lastPrice))) +
			"Você está em posição?\n\n" +
			positiveStyle.Render("[y] Sim, usar preço atual como entrada") + "\n" +
			warningStyle.Render("[n] Não, começar fora do mercado") + "\n" +
			infoStyle.Render("[esc] Cancelar"),
		)

		return lipgloss.Place(
			width,
			height,
			lipgloss.Center,
			lipgloss.Center,
			content,
		)
	}

	// Calcular larguras
	mainPanelWidth := width - 4 // -4 para margens

	// Ajustar estilos com larguras
	sectionStyle = sectionStyle.Width(mainPanelWidth - 4)
	titleStyle = titleStyle.Width(width - 4)

	// Cabeçalho
	header := titleStyle.Render("🤖 Binance Trading Bot")

	// Estilo das abas
	tabStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(highlightColor).
		Padding(0, 1)
	
	activeTabStyle := tabStyle.Copy().
		BorderForeground(highlightColor).
		Background(highlightColor).
		Foreground(lipgloss.Color("0"))

	// Renderizar abas
	tab1Style := tabStyle
	tab2Style := tabStyle
	if m.currentTab == 0 {
		tab1Style = activeTabStyle
	} else {
		tab2Style = activeTabStyle
	}

	tabs := lipgloss.JoinHorizontal(
		lipgloss.Top,
		tab1Style.Render("Principal"),
		tab2Style.Render("Histórico"),
	)

	var content string
	if m.currentTab == 0 {
		// Aba Principal - Informações do Preço e Indicadores
		var indicatorsContent string
		if len(m.trader.GetPrices()) <= m.trader.GetMALongPeriod() {
			indicatorsContent = fmt.Sprintf(
				"Preço BTC: %s\n%s\n%s\n%s",
				priceStyle.Render(fmt.Sprintf("$%.2f", m.lastPrice)),
				loadingStyle.Render("RSI: Carregando..."),
				loadingStyle.Render("MA(9): Carregando..."),
				loadingStyle.Render("MA(21): Carregando..."),
			)
		} else {
			indicatorsContent = fmt.Sprintf(
				"Preço BTC: %s\nRSI: %s\nMA(9): %s\nMA(21): %s",
				priceStyle.Render(fmt.Sprintf("$%.2f", m.lastPrice)),
				m.formatRSI(),
				m.formatMA(9),
				m.formatMA(21),
			)
		}

		priceInfo := sectionStyle.Copy().Width(mainPanelWidth/2 - 2).Render(
			sectionHeaderStyle.Render("📊 Indicadores") + "\n" +
			indicatorsContent,
		)

		// Status da Posição
		var positionStatus string
		if m.inPosition {
			positionStatus = positiveStyle.Render(fmt.Sprintf("Em Posição (Entrada: $%.2f)", m.entryPrice))
		} else {
			positionStatus = warningStyle.Render("Fora do Mercado")
		}

		positionInfo := sectionStyle.Copy().Width(mainPanelWidth/2 - 2).Render(
			sectionHeaderStyle.Render("💰 Carteira") + "\n" +
			fmt.Sprintf(
				"Status: %s\nBTC: %s\nUSDT: %s",
				positionStatus,
				priceStyle.Render(fmt.Sprintf("%.8f", m.btcBalance)),
				priceStyle.Render(fmt.Sprintf("%.2f", m.usdtBalance)),
			),
		)

		// Junta os painéis de preço e status lado a lado
		topPanels := lipgloss.JoinHorizontal(
			lipgloss.Top,
			priceInfo,
			positionInfo,
		)

		// Condições de Trading
		var conditions string
		if !m.inPosition {
			// Condições de compra
			rsiCheck := "❌"
			if m.rsi < 30 {
				rsiCheck = positiveStyle.Render("✓")
			} else {
				rsiCheck = negativeStyle.Render("✗")
			}

			maCheck := "❌"
			if m.maShort > m.maLong {
				maCheck = positiveStyle.Render("✓")
			} else {
				maCheck = negativeStyle.Render("✗")
			}

			conditions = fmt.Sprintf(
				"Condições de Compra:\n"+
					"%s RSI < 30 (atual: %.2f)\n"+
					"%s MA9 > MA21 (%.2f > %.2f)",
				rsiCheck, m.rsi,
				maCheck, m.maShort, m.maLong,
			)
		} else {
			// Condições de venda
			rsiHighCheck := "❌"
			if m.rsi > 70 {
				rsiHighCheck = positiveStyle.Render("✓")
			} else {
				rsiHighCheck = negativeStyle.Render("✗")
			}

			maCrossCheck := "❌"
			if m.maShort < m.maLong && m.rsi > 50 {
				maCrossCheck = positiveStyle.Render("✓")
			} else {
				maCrossCheck = negativeStyle.Render("✗")
			}

			var currentProfit float64
			profitCheck := negativeStyle.Render("✗")
			if m.lastPrice > 0 && m.entryPrice > 0 {
				currentProfit = (m.lastPrice - m.entryPrice) / m.entryPrice * 100
				if currentProfit >= 0.3 {
					profitCheck = positiveStyle.Render("✓")
				}
			}

			conditions = fmt.Sprintf(
				"Condições de Venda:\n"+
					"%s RSI > 70\n"+
					"%s MA9 < MA21 e RSI > 50\n"+
					"%s Lucro > 0.3%% (atual: %.2f%%)",
				rsiHighCheck,
				maCrossCheck,
				profitCheck,
				currentProfit,
			)
		}

		tradingConditions := sectionStyle.Copy().Render(
			sectionHeaderStyle.Render("🎯 Condições de Trading") + "\n" +
			conditions,
		)

		// Logs Importantes
		var logEntries string
		if logger, ok := m.trader.GetLogger().(*traderbot.Logger); ok {
			logs := logger.GetRecentLogs()
			if len(logs) > 0 {
				lastLogs := logs
				if len(logs) > 5 {
					lastLogs = logs[len(logs)-5:]
				}
				for _, log := range lastLogs {
					logEntries += fmt.Sprintf("%s %s\n",
						infoStyle.Render(log.Timestamp.Format("15:04:05")),
						log.Message,
					)
				}
			} else {
				logEntries = infoStyle.Render("Nenhum log importante ainda...")
			}
		}

		logsPanel := sectionStyle.Copy().Render(
			sectionHeaderStyle.Render("📝 Logs Importantes") + "\n\n" +
			logEntries,
		)

		content = lipgloss.JoinVertical(
			lipgloss.Left,
			topPanels,
			tradingConditions,
			logsPanel,
		)
	} else {
		// Aba de Histórico
		m.table.SetHeight(height - 10) // Ajustar altura da tabela
		content = sectionStyle.Copy().Render(
			"Histórico de Trades\n\n" +
				m.table.View(),
		)
	}

	// Rodapé
	footer := infoStyle.Render("Pressione 'q' para sair | ←/→ ou h/l para mudar de aba | 'c' para configurar posição inicial")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tabs,
		content,
		footer,
	)
} 