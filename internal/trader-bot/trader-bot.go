package traderbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

    // Procurar por BTC nos balanços
    for _, balance := range account.Balances {
        if balance.Asset == "BTC" {
            free, err := strconv.ParseFloat(balance.Free, 64)
            if err != nil {
                return fmt.Errorf("erro ao converter saldo BTC: %v", err)
            }

            // Se tiver BTC, precisamos buscar o preço médio de compra
            if free > 0 {
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

func (t *BTCTrader) calculateRSI() float64 {
    // Precisamos de pelo menos rsiPeriod + 1 preços para calcular o RSI
    if len(t.prices) <= t.rsiPeriod+1 {
        log.Printf("RSI: Dados insuficientes. Precisamos de %d preços, temos %d", t.rsiPeriod+1, len(t.prices))
        return 50.0 // Valor neutro até termos dados suficientes
    }

    var gains, losses float64
    for i := 1; i < t.rsiPeriod+1; i++ {
        if len(t.prices)-i-1 < 0 {
            log.Printf("RSI: Índice inválido detectado no cálculo")
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
        log.Printf("RSI: Nenhuma perda detectada, RSI = 100")
        return 100.0
    }

    rs := gains / losses
    rsi := 100.0 - (100.0 / (1.0 + rs))
    log.Printf("RSI calculado: %.2f (Gains: %.2f, Losses: %.2f)", rsi, gains, losses)
    return rsi
}

func (t *BTCTrader) calculateMA(period int) float64 {
    if len(t.prices) < period {
        log.Printf("MA%d: Dados insuficientes. Precisamos de %d preços, temos %d", period, period, len(t.prices))
        return 0
    }

    sum := 0.0
    for i := 0; i < period; i++ {
        sum += t.prices[len(t.prices)-1-i]
    }
    ma := sum / float64(period)
    log.Printf("MA%d calculada: %.2f", period, ma)
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

func (t *BTCTrader) shouldTrade(price float64) (string, bool) {
    log.Printf("\n=== Nova análise de trading ===")
    log.Printf("Preço atual: $%.2f", price)
    
    // Adicionar novo preço ao histórico
    t.prices = append(t.prices, price)
    if len(t.prices) > 100 { // Manter histórico limitado
        t.prices = t.prices[1:]
    }
    log.Printf("Total de preços no histórico: %d", len(t.prices))

    // Verificar se temos dados suficientes para todos os indicadores
    minDataRequired := t.maLong // MA longa é o indicador que precisa de mais dados
    if len(t.prices) < minDataRequired {
        log.Printf("⏳ Aguardando dados suficientes para todos os indicadores (necessário: %d, atual: %d)", 
            minDataRequired, len(t.prices))
        return "", false
    }

    // Calcular indicadores
    rsi := t.calculateRSI()
    maShort := t.calculateMA(t.maShort)
    maLong := t.calculateMA(t.maLong)

    // Se algum indicador retornou valor neutro/inválido, não operar
    if rsi == 50.0 || maShort == 0 || maLong == 0 {
        log.Printf("⚠️ Indicadores ainda não estão prontos para operar")
        return "", false
    }

    log.Printf("Status atual: %s", map[bool]string{true: "Em posição", false: "Fora do mercado"}[t.inPosition])

    // Regras de Trading
    if !t.inPosition {
        log.Printf("Analisando sinais de COMPRA...")
        log.Printf("Condições: RSI < 30 (atual: %.2f) E MA%d > MA%d (%.2f > %.2f)", 
            rsi, t.maShort, t.maLong, maShort, maLong)
        
        if rsi < 30 && maShort > maLong {
            log.Printf("✅ Sinal de COMPRA gerado!")
            return "buy", true
        }
    } else {
        log.Printf("Analisando sinais de VENDA...")
        entryPrice := t.positions["BTC"]
        minProfitPrice := t.calculateMinProfitablePrice(entryPrice)
        
        log.Printf("Preço de entrada: $%.2f", entryPrice)
        log.Printf("Preço mínimo para lucro (incluindo taxas): $%.2f", minProfitPrice)
        log.Printf("Lucro potencial atual: %.2f%%", (price-entryPrice)/entryPrice*100)

        // Primeiro verifica se o preço atual permite lucro
        if price < minProfitPrice {
            log.Printf("⏳ Aguardando preço lucrativo (atual: $%.2f, necessário: $%.2f)", price, minProfitPrice)
            return "", false
        }

        log.Printf("Condições: RSI > 70 (atual: %.2f) OU (MA%d < MA%d (%.2f < %.2f) E RSI > 50)", 
            rsi, t.maShort, t.maLong, maShort, maLong)
        
        // Vende se tiver sinal técnico E o preço permitir lucro
        if rsi > 70 || (maShort < maLong && rsi > 50) {
            log.Printf("✅ Sinal de VENDA gerado!")
            return "sell", true
        }
    }

    log.Printf("❌ Nenhum sinal de trading gerado")
    return "", false
}

func (t *BTCTrader) checkStopLoss(currentPrice float64) bool {
    if !t.inPosition {
        return false
    }

    entryPrice := t.positions["BTC"]
    stopLossPercent := 0.02 // 2% stop loss
    stopLossPrice := entryPrice * (1 - stopLossPercent)

    log.Printf("=== Verificação de Stop Loss ===")
    log.Printf("Preço de entrada: $%.2f", entryPrice)
    log.Printf("Preço atual: $%.2f", currentPrice)
    log.Printf("Preço do Stop Loss: $%.2f", stopLossPrice)

    if currentPrice < stopLossPrice {
        log.Printf("⚠️ Stop Loss atingido! Perda: %.2f%%", (currentPrice-entryPrice)/entryPrice*100)
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
            return err
        }
        
        t.inPosition = true
        t.positions["BTC"] = price

        // Buscar saldos atualizados
        btcBalance, usdtBalance, err := t.getBalances()
        if err != nil {
            log.Printf("Aviso: Não foi possível obter saldos atualizados: %v", err)
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
        
        log.Printf("Compra executada: Preço: %.2f, Quantidade: %.5f", price, quantity)
        log.Printf("Saldos após compra - BTC: %.8f, USDT: %.2f", btcBalance, usdtBalance)
        log.Printf("Ordem: %+v", order)
        
    } else if action == "sell" {
        order, err := t.client.NewCreateOrderService().
            Symbol("BTCUSDT").
            Side(binance.SideTypeSell).
            Type(binance.OrderTypeMarket).
            Quantity(fmt.Sprintf("%.5f", quantity)).
            Do(context.Background())
            
        if err != nil {
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
            log.Printf("Aviso: Não foi possível obter saldos atualizados: %v", err)
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
        
        log.Printf("Venda executada: Preço: %.2f, Quantidade: %.5f, Lucro/Prejuízo: %.2f%%", 
            price, quantity, profitLoss)
        log.Printf("Saldos após venda - BTC: %.8f, USDT: %.2f", btcBalance, usdtBalance)
        log.Printf("Ordem: %+v", order)
    }
    
    return nil
}

func (t *BTCTrader) calculateTradeQuantity(price float64) float64 {
    riskPerTrade := 0.02 // 2% do capital disponível por trade
    tradeAmount := t.funds * riskPerTrade
    return tradeAmount / price
}

func (t *BTCTrader) Start() error {
    wsHandler := func(event *binance.WsKlineEvent) {
        price, _ := strconv.ParseFloat(event.Kline.Close, 64)
        
        // Verificar stop loss
        if t.checkStopLoss(price) {
            log.Println("Stop Loss atingido!")
            t.executeTrade("sell", price)
            return
        }
        
        // Verificar sinais de trading
        action, shouldTrade := t.shouldTrade(price)
        if shouldTrade {
            err := t.executeTrade(action, price)
            if err != nil {
                log.Printf("Erro ao executar %s: %v", action, err)
            }
        }
    }

    errHandler := func(err error) {
        log.Printf("Erro no WebSocket: %v", err)
    }

    // Iniciar WebSocket para BTCUSDT com intervalo de 1 minuto
    _, _, err := binance.WsKlineServe("BTCUSDT", "1m", wsHandler, errHandler)
    if err != nil {
        return err
    }

    // Manter o bot rodando
    select {}
}