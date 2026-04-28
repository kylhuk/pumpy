package bot

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"pumpy/internal/fanout"
	"pumpy/internal/lightning"
)

// Engine is the trading core. It receives fanout events and manages positions.
type Engine struct {
	cfg     Config
	dryRun  bool
	lc      *lightning.Client
	ledger  *Ledger
	drain   *DrainController

	positions sync.Map // map[mint string]*Position
	mu        sync.RWMutex
	openSnap  []*Position // protected by mu, refreshed on each position change
}

func NewEngine(cfg Config, dryRun bool, ledger *Ledger, drain *DrainController) *Engine {
	return &Engine{
		cfg:    cfg,
		dryRun: dryRun,
		lc:     lightning.NewClient(10*time.Second, 3),
		ledger: ledger,
		drain:  drain,
	}
}

// HandleEvent processes a single fanout event on the hot path.
// Never blocks — spawns goroutines for HTTP calls.
func (e *Engine) HandleEvent(ev fanout.Event) {
	switch ev.Type {
	case fanout.EventCreate:
		if ev.Create.TraderPublicKey == e.cfg.TargetWallet && !e.drain.IsDraining() {
			go e.openPosition(ev.Create.Mint, ev.Create.MarketCapSol)
		}
	case fanout.EventTrade:
		val, ok := e.positions.Load(ev.Trade.Mint)
		if !ok {
			return
		}
		pos := val.(*Position)
		pos.SetLastMcap(ev.Trade.MarketCapSol)
		go e.evaluateSell(context.Background(), pos, false)
	}
}

// SafetyTick is called by the engine's periodic ticker to re-evaluate all open positions.
func (e *Engine) SafetyTick(ctx context.Context) {
	e.positions.Range(func(_, val any) bool {
		pos := val.(*Position)
		if !pos.IsClosed() {
			go e.evaluateSell(ctx, pos, e.drain.IsDraining())
		}
		return true
	})
}

// OpenPositions returns a snapshot for the dashboard.
func (e *Engine) OpenPositions() []*Position {
	var out []*Position
	e.positions.Range(func(_, val any) bool {
		pos := val.(*Position)
		if !pos.IsClosed() {
			out = append(out, pos)
		}
		return true
	})
	return out
}

// OpenCount returns the number of positions not yet closed.
func (e *Engine) OpenCount() int {
	n := 0
	e.positions.Range(func(_, val any) bool {
		if !val.(*Position).IsClosed() {
			n++
		}
		return true
	})
	return n
}

func (e *Engine) openPosition(mint string, creationMcap float64) {
	pos := newPosition(mint, creationMcap, e.cfg.BuySol)

	if e.dryRun {
		log.Printf("[DRY-RUN] WOULD BUY %.4f SOL of %s (mcap=%.1f SOL)", e.cfg.BuySol, mint, creationMcap)
		e.positions.Store(mint, pos)
		e.ledger.RecordBuy(0, TxRecord{
			At:        time.Now(),
			Mint:      mint,
			Side:      "buy",
			SolAmount: e.cfg.BuySol,
			McapSol:   creationMcap,
			Signature: "dry-run",
		})
		return
	}

	req := lightning.TradeRequest{
		Action:       "buy",
		Mint:         mint,
		Amount:       e.cfg.BuySol,
		Denomination: "true",
		Slippage:     e.cfg.SlippagePct,
		PriorityFee:  e.cfg.PriorityFee,
		Pool:         e.cfg.Pool,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := e.lc.Trade(ctx, e.cfg.Lightning.APIKey, req)
	if err != nil {
		log.Printf("buy %s: %v", mint, err)
		return
	}

	e.positions.Store(mint, pos)
	e.ledger.RecordBuy(e.cfg.BuySol, TxRecord{
		At:        time.Now(),
		Mint:      mint,
		Side:      "buy",
		SolAmount: e.cfg.BuySol,
		McapSol:   creationMcap,
		Signature: result.Signature,
	})
	log.Printf("buy %s sig=%s", mint[:8], result.Signature[:8])
}

func (e *Engine) evaluateSell(ctx context.Context, pos *Position, forceAll bool) {
	if pos.IsClosed() {
		return
	}
	if !pos.TryLockInFlight() {
		return
	}
	defer pos.UnlockInFlight()

	tier := EvaluateTier(pos.PriceRatio(), e.cfg)
	if forceAll {
		tier = TierAll
	}

	// Never go backwards in tier — avoid re-selling at S after Q.
	if tier < pos.currentTier.Load() {
		tier = pos.currentTier.Load()
	}
	pos.currentTier.Store(tier)

	sellPct := SellPct(tier, e.cfg)
	remaining := pos.RemainingTokens()
	var sellAmount float64

	if remaining > 0 {
		sellAmount = float64(remaining) * sellPct
	} else {
		// Fallback: sell a fixed fraction of original buy in SOL terms.
		sellAmount = e.cfg.BuySol * sellPct
	}

	if sellAmount <= 0 {
		return
	}

	denomination := "false" // sell token units
	if remaining == 0 {
		// We don't have token balance yet (buy may not have populated it) — sell by SOL estimate.
		denomination = "true"
		sellAmount = e.cfg.BuySol * sellPct
	}

	if e.dryRun {
		tierName := map[int32]string{TierS: "S", TierQ: "Q", TierAll: "ALL"}[tier]
		log.Printf("[DRY-RUN] WOULD SELL %s %.4f (%s tier) of %s (ratio=%.2f)",
			tierName, sellAmount, denomination, pos.Mint, pos.PriceRatio())
		pos.sellsExecuted.Add(1)
		if tier == TierAll {
			e.closeDryRun(pos)
		}
		return
	}

	req := lightning.TradeRequest{
		Action:       "sell",
		Mint:         pos.Mint,
		Amount:       sellAmount,
		Denomination: denomination,
		Slippage:     e.cfg.SlippagePct,
		PriorityFee:  e.cfg.PriorityFee,
		Pool:         e.cfg.Pool,
	}
	result, err := e.lc.Trade(ctx, e.cfg.Lightning.APIKey, req)
	if err != nil {
		log.Printf("sell %s: %v", pos.Mint, err)
		return
	}

	pos.sellsExecuted.Add(1)
	solReceived := sellAmount
	if denomination == "true" {
		solReceived = sellAmount
	}
	e.ledger.RecordSell(solReceived, TxRecord{
		At:        time.Now(),
		Mint:      pos.Mint,
		Side:      "sell",
		SolAmount: solReceived,
		McapSol:   pos.LastMcap(),
		Signature: result.Signature,
	})

	if tier == TierAll {
		pos.MarkClosed()
		win := solReceived > pos.BoughtSol()
		e.ledger.RecordPositionClosed(win)
		log.Printf("closed %s sig=%s win=%v", pos.Mint[:8], result.Signature[:8], win)
	} else {
		log.Printf("sell[%s] %s sig=%s", fmt.Sprintf("%d", tier), pos.Mint[:8], result.Signature[:8])
	}
}

func (e *Engine) closeDryRun(pos *Position) {
	pos.MarkClosed()
	e.ledger.RecordPositionClosed(false)
}
