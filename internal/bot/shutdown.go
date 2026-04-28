package bot

import (
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

// DrainController handles SIGINT: first signal enters drain mode, subsequent signals
// are logged and ignored. The bot exits only when all positions are closed.
type DrainController struct {
	draining atomic.Bool
}

func NewDrainController() *DrainController {
	return &DrainController{}
}

// IsDraining reports whether drain mode is active.
func (d *DrainController) IsDraining() bool { return d.draining.Load() }

// Listen blocks until the first SIGINT/SIGTERM, sets drain mode, then returns.
func (d *DrainController) Listen() {
	ch := make(chan os.Signal, 8)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	sig := <-ch
	d.draining.Store(true)
	log.Printf("drain: received %v — selling all open positions; Ctrl+C again to check status", sig)
}

// WaitForExit blocks until engine has zero open positions, logging status on each
// subsequent Ctrl+C. Calls os.Exit(0) when drain is complete.
func WaitForExit(engine *Engine, ledger *Ledger) {
	ch := make(chan os.Signal, 8)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for range ch {
			open := engine.OpenCount()
			if open == 0 {
				log.Println("drain: all positions closed — exiting")
				os.Exit(0)
			}
			log.Printf("drain: %d position(s) still open — waiting for sells to confirm", open)
		}
	}()

	for {
		if engine.OpenCount() == 0 {
			log.Printf("drain: complete — PnL=%.4f SOL  win_rate=%.1f%%",
				ledger.PnL(), ledger.WinRate())
			os.Exit(0)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
