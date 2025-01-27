package traderbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2"
)

type Trade struct {
    Timestamp   int64   `json:"timestamp"`
    Action      string  `json:"action"`
    Price       float64 `json:"price"`
    Quantity    float64 `json:"quantity"`
    ProfitLoss  float64 `json:"profit_loss,omitempty"`
    BTCBalance  float64 `json:"btc_balance"`    // Saldo de BTC após a operação
    USDTBalance float64 `json:"usdt_balance"`   // Saldo de USDT após a operação
}

type BTCTrader struct {
    client     *binance.Client
    prices     []float64
    positions  map[string]float64  // Preços de entrada das posições
    rsiPeriod  int
    maShort    int
    maLong     int
    funds      float64            // Fundos disponíveis para trading
    inPosition bool
    lastTradeTime int64          // Timestamp da última operação
    takerFee    float64         // Taxa de taker da Binance (0.1% = 0.001)
    tradeHistory []Trade        // Histórico de trades
    historyFile  string         // Nome do arquivo para salvar histórico
    historyMutex sync.Mutex     // Mutex para proteger o acesso ao histórico
    logger      *Logger         // Logger personalizado
}

type InitialPosition struct {
    InPosition bool    `json:"in_position"`
    EntryPrice float64 `json:"entry_price,omitempty"`
    Quantity   float64 `json:"quantity,omitempty"`
}

func (t *BTCTrader) loadCurrentPosition() error {
    // Buscar informações da conta
    account, err := t.client.NewGetAccountService().Do(context.Background())
    if err != nil {
        return fmt.Errorf("erro ao buscar informações da conta: %v", err)
    }

    // Verificar última operação no histórico
    t.historyMutex.Lock()
    var lastAction string
    if len(t.tradeHistory) > 0 {
        lastAction = t.tradeHistory[len(t.tradeHistory)-1].Action
    }
    t.historyMutex.Unlock()

    // Procurar por BTC nos balanços
    for _, balance := range account.Balances {
        if balance.Asset == "BTC" {
            free, err := strconv.ParseFloat(balance.Free, 64)
            if err != nil {
                return fmt.Errorf("erro ao converter saldo BTC: %v", err)
            }

            // Se tiver BTC E a última operação não foi uma venda, estamos em posição
            if free > 0 && lastAction != "sell" {
                // Buscar trades recentes para encontrar o preço médio
                trades, err := t.client.NewListTradesService().
                    Symbol("BTCUSDT").
                    Limit(1000). // Limite máximo para ter certeza de pegar o trade mais recente
                    Do(context.Background())
                if err != nil {
                    return fmt.Errorf("erro ao buscar trades: %v", err)
                }

                // Encontrar o último trade de compra
                var lastBuyPrice float64
                for i := len(trades) - 1; i >= 0; i-- {
                    trade := trades[i]
                    if trade.IsBuyer {
                        lastBuyPrice, err = strconv.ParseFloat(trade.Price, 64)
                        if err != nil {
                            return fmt.Errorf("erro ao converter preço do trade: %v", err)
                        }
                        break
                    }
                }

                if lastBuyPrice > 0 {
                    t.inPosition = true
                    t.positions["BTC"] = lastBuyPrice
                    log.Printf("Posição existente detectada - Quantidade: %.8f BTC, Preço de entrada: $%.2f", 
                        free, lastBuyPrice)
                }
            } else {
                t.inPosition = false
                delete(t.positions, "BTC")
                log.Printf("Saldo BTC: %.8f, Última ação: %s - Considerado fora de posição", 
                    free, lastAction)
            }
            break
        }
    }

    if !t.inPosition {
        log.Println("Nenhuma posição existente detectada")
    }

    return nil
}

func NewBTCTrader(apiKey, apiSecret string, initialFunds float64, testnet bool, historyFile string) *BTCTrader {
    binance.UseTestnet = testnet
    trader := &BTCTrader{
        client:      binance.NewClient(apiKey, apiSecret),
        prices:      make([]float64, 0),
        positions:   make(map[string]float64),
        rsiPeriod:   14,
        maShort:     9,
        maLong:      21,
        funds:       initialFunds,
        inPosition:  false,
        takerFee:    0.001, // 0.1% por operação
        historyFile: historyFile,
        tradeHistory: make([]Trade, 0),
    }

    // Carregar histórico existente se o arquivo existir
    if _, err := os.Stat(historyFile); err == nil {
        if data, err := os.ReadFile(historyFile); err == nil {
            json.Unmarshal(data, &trader.tradeHistory)
            log.Printf("Histórico de trades carregado: %d operações encontradas", len(trader.tradeHistory))
        }
    }

    // Verificar posição atual na Binance
    if err := trader.loadCurrentPosition(); err != nil {
        log.Printf("Aviso: Não foi possível carregar a posição atual: %v", err)
    }

    return trader
}

func (t *BTCTrader) saveTradeHistory() {
    t.historyMutex.Lock()
    defer t.historyMutex.Unlock()

    data, err := json.MarshalIndent(t.tradeHistory, "", "    ")
    if err != nil {
        log.Printf("Erro ao serializar histórico de trades: %v", err)
        return
    }

    if err := os.WriteFile(t.historyFile, data, 0644); err != nil {
        log.Printf("Erro ao salvar histórico de trades: %v", err)
    }
}

func (t *BTCTrader) addTradeToHistory(trade Trade) {
    t.historyMutex.Lock()
    t.tradeHistory = append(t.tradeHistory, trade)
    t.historyMutex.Unlock()

    // Salvar histórico em uma goroutine separada
    go t.saveTradeHistory()
}

func (t *BTCTrader) log(format string, v ...interface{}) {
	if t.logger != nil {
		t.logger.Printf(format, v...)
	}
}

func (t *BTCTrader) logImportant(format string, v ...interface{}) {
	if t.logger != nil {
		t.logger.LogImportant(format, v...)
	}
}

func (t *BTCTrader) calculateRSI() float64 {
	// Precisamos de pelo menos rsiPeriod + 1 preços para calcular o RSI
	if len(t.prices) <= t.rsiPeriod+1 {
		t.log("RSI: Dados insuficientes. Precisamos de %d preços, temos %d", t.rsiPeriod+1, len(t.prices))
		return 50.0 // Valor neutro até termos dados suficientes
	}

	var gains, losses float64
	for i := 1; i < t.rsiPeriod+1; i++ {
		if len(t.prices)-i-1 < 0 {
			t.log("RSI: Índice inválido detectado no cálculo")
			return 50.0 // Proteção adicional contra índices negativos
		}
		change := t.prices[len(t.prices)-i] - t.prices[len(t.prices)-i-1]
		if change >= 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	if losses == 0 {
		t.log("RSI: Nenhuma perda detectada, RSI = 100")
		return 100.0
	}

	rs := gains / losses
	rsi := 100.0 - (100.0 / (1.0 + rs))
	t.log("RSI calculado: %.2f (Gains: %.2f, Losses: %.2f)", rsi, gains, losses)
	return rsi
}

func (t *BTCTrader) calculateMA(period int) float64 {
	if len(t.prices) < period {
		t.log("MA%d: Dados insuficientes. Precisamos de %d preços, temos %d", period, period, len(t.prices))
		return 0
	}

	sum := 0.0
	for i := 0; i < period; i++ {
		sum += t.prices[len(t.prices)-1-i]
	}
	ma := sum / float64(period)
	t.log("MA%d calculada: %.2f", period, ma)
	return ma
}

// Calcula o preço mínimo de venda necessário para lucro considerando as taxas
func (t *BTCTrader) calculateMinProfitablePrice(entryPrice float64) float64 {
    // Para cada operação pagamos a taxa (compra e venda)
    // Preço mínimo = Preço de entrada * (1 + 2 * taxa + margem_minima)
    minProfitMargin := 0.001 // 0.1% de margem mínima de lucro
    totalFees := 2 * t.takerFee // Taxa de compra + taxa de venda
    return entryPrice * (1 + totalFees + minProfitMargin)
}

// hasEnoughData verifica se há dados suficientes para calcular todos os indicadores
func (t *BTCTrader) hasEnoughData() bool {
    return len(t.prices) > t.maLong && len(t.prices) > t.rsiPeriod+1
}

func (t *BTCTrader) shouldTrade(price float64) (string, bool) {
    t.log("\n=== Nova análise de trading ===")
    t.log("Preço atual: $%.2f", price)
    
    // Adicionar novo preço ao histórico
    t.prices = append(t.prices, price)
    if len(t.prices) > 100 { // Manter histórico limitado
        t.prices = t.prices[1:]
    }

    // Verificar se temos dados suficientes para todos os indicadores
    if !t.hasEnoughData() {
        t.log("Aguardando dados suficientes para indicadores (MA21: %d/%d, RSI: %d/%d)",
            len(t.prices), t.maLong,
            len(t.prices), t.rsiPeriod+1)
        return "", false
    }

    // Calcular indicadores
    rsi := t.calculateRSI()
    maShort := t.calculateMA(t.maShort)
    maLong := t.calculateMA(t.maLong)

    // Se algum indicador retornou valor neutro/inválido, não operar
    if rsi == 50.0 || maShort == 0 || maLong == 0 {
        return "", false
    }

    // Regras de Trading
    if !t.inPosition {
        if rsi < 30 && maShort > maLong {
            t.logImportant("✅ Sinal de COMPRA - RSI: %.2f, MA9: %.2f, MA21: %.2f", rsi, maShort, maLong)
            return "buy", true
        }
    } else {
        entryPrice := t.positions["BTC"]
        currentProfit := (price - entryPrice) / entryPrice * 100
        
        if price < t.calculateMinProfitablePrice(entryPrice) {
            return "", false
        }

        if (rsi > 70 || (maShort < maLong && rsi > 50)) && currentProfit >= 0.3 {
            t.logImportant("✅ Sinal de VENDA - RSI: %.2f, MA9: %.2f, MA21: %.2f, Lucro: %.2f%%", 
                rsi, maShort, maLong, currentProfit)
            return "sell", true
        }
    }

    return "", false
}

func (t *BTCTrader) checkStopLoss(currentPrice float64) bool {
    if !t.inPosition {
        return false
    }

    entryPrice := t.positions["BTC"]
    stopLossPercent := 0.02 // 2% stop loss
    stopLossPrice := entryPrice * (1 - stopLossPercent)

    if currentPrice < stopLossPrice {
        loss := (currentPrice-entryPrice)/entryPrice*100
        t.logImportant("⚠️ Stop Loss atingido! Perda: %.2f%%", loss)
        return true
    }

    return false
}

func (t *BTCTrader) getBalances() (btcBalance, usdtBalance float64, err error) {
    account, err := t.client.NewGetAccountService().Do(context.Background())
    if err != nil {
        return 0, 0, fmt.Errorf("erro ao buscar saldos: %v", err)
    }

    for _, balance := range account.Balances {
        switch balance.Asset {
        case "BTC":
            btcBalance, err = strconv.ParseFloat(balance.Free, 64)
            if err != nil {
                return 0, 0, fmt.Errorf("erro ao converter saldo BTC: %v", err)
            }
        case "USDT":
            usdtBalance, err = strconv.ParseFloat(balance.Free, 64)
            if err != nil {
                return 0, 0, fmt.Errorf("erro ao converter saldo USDT: %v", err)
            }
        }
    }

    return btcBalance, usdtBalance, nil
}

func (t *BTCTrader) executeTrade(action string, price float64) error {
    quantity := t.calculateTradeQuantity(price)
    
    if action == "buy" {
        order, err := t.client.NewCreateOrderService().
            Symbol("BTCUSDT").
            Side(binance.SideTypeBuy).
            Type(binance.OrderTypeMarket).
            Quantity(fmt.Sprintf("%.5f", quantity)).
            Do(context.Background())
            
        if err != nil {
            t.logImportant("❌ Erro ao executar compra: %v", err)
            return err
        }
        
        t.inPosition = true
        t.positions["BTC"] = price

        // Buscar saldos atualizados
        btcBalance, usdtBalance, err := t.getBalances()
        if err != nil {
            t.log("Aviso: Não foi possível obter saldos atualizados: %v", err)
        }

        // Registrar trade no histórico
        trade := Trade{
            Timestamp:   time.Now().Unix(),
            Action:      "buy",
            Price:       price,
            Quantity:    quantity,
            BTCBalance:  btcBalance,
            USDTBalance: usdtBalance,
        }
        t.addTradeToHistory(trade)
        
        t.logImportant("💰 Compra executada - Preço: $%.2f, Quantidade: %.5f BTC", price, quantity)
        t.log("Saldos após compra - BTC: %.8f, USDT: %.2f", btcBalance, usdtBalance)
        t.log("Ordem: %+v", order)
        
    } else if action == "sell" {
        order, err := t.client.NewCreateOrderService().
            Symbol("BTCUSDT").
            Side(binance.SideTypeSell).
            Type(binance.OrderTypeMarket).
            Quantity(fmt.Sprintf("%.5f", quantity)).
            Do(context.Background())
            
        if err != nil {
            t.logImportant("❌ Erro ao executar venda: %v", err)
            return err
        }
        
        // Calcular lucro/prejuízo antes de limpar a posição
        entryPrice := t.positions["BTC"]
        profitLoss := (price - entryPrice) / entryPrice * 100

        t.inPosition = false
        delete(t.positions, "BTC")

        // Buscar saldos atualizados
        btcBalance, usdtBalance, err := t.getBalances()
        if err != nil {
            t.log("Aviso: Não foi possível obter saldos atualizados: %v", err)
        }

        // Registrar trade no histórico
        trade := Trade{
            Timestamp:   time.Now().Unix(),
            Action:      "sell",
            Price:       price,
            Quantity:    quantity,
            ProfitLoss:  profitLoss,
            BTCBalance:  btcBalance,
            USDTBalance: usdtBalance,
        }
        t.addTradeToHistory(trade)
        
        t.logImportant("💰 Venda executada - Preço: $%.2f, Quantidade: %.5f BTC, Lucro: %.2f%%", 
            price, quantity, profitLoss)
        t.log("Saldos após venda - BTC: %.8f, USDT: %.2f", btcBalance, usdtBalance)
        t.log("Ordem: %+v", order)
    }
    
    return nil
}

func (t *BTCTrader) calculateTradeQuantity(price float64) float64 {
    // Valor mínimo da ordem na Binance (11 USDT para garantir)
    minOrderValue := 11.0

    // Calcular quantidade baseada no risco
    riskPerTrade := 0.02 // 2% do capital disponível por trade
    tradeAmount := t.funds * riskPerTrade

    // Garantir que o valor da ordem seja pelo menos o mínimo
    if tradeAmount < minOrderValue {
        tradeAmount = minOrderValue
    }

    // Se o saldo disponível for menor que o valor mínimo, usar todo o saldo
    _, usdtBalance, err := t.getBalances()
    if err == nil && !t.inPosition && usdtBalance < tradeAmount {
        tradeAmount = usdtBalance
    }

    // Se mesmo assim o valor for menor que o mínimo, não executar
    if tradeAmount < minOrderValue {
        t.logImportant("⚠️ Saldo insuficiente para atingir o valor mínimo de ordem (%.2f USDT)", minOrderValue)
        return 0
    }

    // Calcular quantidade
    quantity := tradeAmount / price

    // Arredondar para 5 casas decimais (padrão da Binance para BTC)
    quantity = math.Floor(quantity*100000) / 100000

    // Verificar se o valor total da ordem atende ao mínimo
    if quantity * price < minOrderValue {
        t.logImportant("⚠️ Valor total da ordem (%.2f USDT) abaixo do mínimo permitido (%.2f USDT)", quantity * price, minOrderValue)
        return 0
    }

    return quantity
}

func (t *BTCTrader) Start() error {
    wsHandler := func(event *binance.WsKlineEvent) {
        price, _ := strconv.ParseFloat(event.Kline.Close, 64)
        
        // Verificar stop loss
        if t.checkStopLoss(price) {
            t.logImportant("Stop Loss atingido! Executando venda...")
            t.executeTrade("sell", price)
            return
        }
        
        // Verificar sinais de trading
        action, shouldTrade := t.shouldTrade(price)
        if shouldTrade {
            t.logImportant("Executando %s...", action)
            err := t.executeTrade(action, price)
            if err != nil {
                t.logImportant("❌ Erro ao executar %s: %v", action, err)
            }
        }
    }

    errHandler := func(err error) {
        t.logImportant("❌ Erro no WebSocket: %v", err)
    }

    // Iniciar WebSocket para BTCUSDT com intervalo de 1 minuto
    _, _, err := binance.WsKlineServe("BTCUSDT", "1s", wsHandler, errHandler)
    if err != nil {
        return fmt.Errorf("erro ao iniciar WebSocket: %v", err)
    }

    // Manter o bot rodando
    select {}
}

// SetInitialPosition configura a posição inicial do trader
func (t *BTCTrader) SetInitialPosition(inPosition bool, entryPrice float64) {
    t.inPosition = inPosition
    if inPosition {
        t.positions["BTC"] = entryPrice
        t.logImportant("Posição inicial configurada - Em posição com entrada em $%.2f", entryPrice)
    } else {
        delete(t.positions, "BTC")
        t.logImportant("Posição inicial configurada - Fora do mercado")
    }
}

// GetClient retorna o cliente da Binance
func (t *BTCTrader) GetClient() *binance.Client {
    return t.client
}