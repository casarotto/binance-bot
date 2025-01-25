package traderbot

// GetPrices retorna o histórico de preços
func (t *BTCTrader) GetPrices() []float64 {
	return t.prices
}

// GetMAShortPeriod retorna o período da média móvel curta
func (t *BTCTrader) GetMAShortPeriod() int {
	return t.maShort
}

// GetMALongPeriod retorna o período da média móvel longa
func (t *BTCTrader) GetMALongPeriod() int {
	return t.maLong
}

// CalculateRSI calcula e retorna o RSI atual
func (t *BTCTrader) CalculateRSI() float64 {
	return t.calculateRSI()
}

// CalculateMA calcula e retorna a média móvel para o período especificado
func (t *BTCTrader) CalculateMA(period int) float64 {
	return t.calculateMA(period)
}

// IsInPosition retorna se está em posição
func (t *BTCTrader) IsInPosition() bool {
	return t.inPosition
}

// GetEntryPrice retorna o preço de entrada da posição atual
func (t *BTCTrader) GetEntryPrice() float64 {
	if price, ok := t.positions["BTC"]; ok {
		return price
	}
	return 0
}

// GetTradeHistory retorna o histórico de trades
func (t *BTCTrader) GetTradeHistory() []Trade {
	t.historyMutex.Lock()
	defer t.historyMutex.Unlock()
	return t.tradeHistory
}

// GetBalances retorna os saldos atuais de BTC e USDT
func (t *BTCTrader) GetBalances() (btc float64, usdt float64, err error) {
	return t.getBalances()
}

// SetLogger configura o logger do trader
func (t *BTCTrader) SetLogger(logger *Logger) {
	t.logger = logger
}

// GetLogger retorna o logger do trader
func (t *BTCTrader) GetLogger() interface{} {
	return t.logger
} 