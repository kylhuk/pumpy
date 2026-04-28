package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/term"
	"pumpy/internal/bot"
	"pumpy/internal/fanout"
	"pumpy/internal/solrpc"
)

func main() {
	configPath := flag.String("config", "", "path to bot.json (default: $PUMPY_BOT_CONFIG, then ./bot.json)")
	dryRun := flag.Bool("dry-run", false, "log trades without executing them")
	flag.Parse()

	path := *configPath
	if path == "" {
		if v := os.Getenv("PUMPY_BOT_CONFIG"); v != "" {
			path = v
		} else {
			path = "bot.json"
		}
	}

	cfg, err := bot.Load(path)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if *dryRun {
		log.Println("dry-run mode: no real trades will be submitted")
	}

	rpc := solrpc.NewClient(cfg.SolanaRPC)
	drain := bot.NewDrainController()
	ledger := bot.NewLedger(cfg.HistorySize)

	// Capture starting SOL balance.
	startLamports, err := rpc.GetBalance(context.Background(), cfg.Lightning.WalletPubkey)
	if err != nil {
		log.Printf("warn: could not fetch starting balance: %v", err)
	} else {
		ledger.SetStartBalance(startLamports)
		log.Printf("starting balance: %.4f SOL", float64(startLamports)/1e9)
	}

	engine := bot.NewEngine(cfg, *dryRun, ledger, drain)

	// Fanout: Start() reconnects forever internally; events flow from Events() channel.
	wsConnected := new(bool)
	fc := fanout.NewClient(cfg.FanoutAddr)
	go fc.Start()
	go pumpEvents(fc, engine, wsConnected)

	// Safety evaluation ticker.
	go func() {
		tick := time.NewTicker(time.Duration(cfg.EvaluatorIntervalMs) * time.Millisecond)
		defer tick.Stop()
		for range tick.C {
			engine.SafetyTick(context.Background())
		}
	}()

	// Balance refresh.
	go func() {
		tick := time.NewTicker(time.Duration(cfg.BalanceRefreshMs) * time.Millisecond)
		defer tick.Stop()
		for range tick.C {
			lamports, rpcErr := rpc.GetBalance(context.Background(), cfg.Lightning.WalletPubkey)
			if rpcErr == nil {
				ledger.SetLiveBalance(lamports)
			}
		}
	}()

	// Dashboard redraw loop.
	go func() {
		tick := time.NewTicker(time.Duration(cfg.DashboardRefreshMs) * time.Millisecond)
		defer tick.Stop()
		for range tick.C {
			width, _, sizeErr := term.GetSize(int(os.Stdout.Fd()))
			if sizeErr != nil || width <= 0 {
				width = 120
			}
			snap := bot.Snapshot{
				Now:           time.Now().UTC(),
				WsConnected:   *wsConnected,
				RpcOK:         ledger.LiveBalanceLamports() > 0,
				Draining:      drain.IsDraining(),
				SolSpent:      ledger.SolSpent(),
				SolSold:       ledger.SolSold(),
				LiveBalance:   ledger.LiveBalanceSol(),
				BalancePct:    ledger.BalancePct(),
				PnL:           ledger.PnL(),
				OpenCount:     ledger.OpenPositions(),
				ClosedCount:   ledger.ClosedPositions(),
				WinRate:       ledger.WinRate(),
				OpenPositions: engine.OpenPositions(),
				History:       ledger.History(),
			}
			fmt.Print("\033[H\033[2J")
			fmt.Print(bot.Render(snap, width))
		}
	}()

	// Block until first Ctrl+C, then drain all positions.
	drain.Listen()
	log.Printf("drain: force-selling %d open positions", engine.OpenCount())
	engine.SafetyTick(context.Background())
	bot.WaitForExit(engine, ledger)
}

// pumpEvents reads from the fanout channel and forwards events to the engine.
// It tracks connection state via wsConnected.
func pumpEvents(fc *fanout.Client, engine *bot.Engine, wsConnected *bool) {
	ch := fc.Events()
	for ev := range ch {
		*wsConnected = true
		engine.HandleEvent(ev)
	}
}
