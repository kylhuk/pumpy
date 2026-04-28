package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"pumpy/internal/dashboard"
)

// Snapshot is a point-in-time capture of engine + ledger state for rendering.
type Snapshot struct {
	Now           time.Time
	WsConnected   bool
	RpcOK         bool
	Draining      bool
	SolSpent      float64
	SolSold       float64
	LiveBalance   float64
	BalancePct    float64
	PnL           float64
	OpenCount     int64
	ClosedCount   int64
	WinRate       float64
	OpenPositions []*Position
	History       []TxRecord
}

var (
	drainStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5f5f")).Bold(true)
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00d700")).Bold(true)
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
)

// Render returns the full dashboard string for clearing and reprinting.
func Render(snap Snapshot, width int) string {
	if width <= 0 {
		width = 120
	}
	tty := dashboard.IsTTY()

	// Header
	wsStatus := wsStatusStr(snap.WsConnected)
	rpcStatus := rpcStatusStr(snap.RpcOK)
	drainStr := dimStyle.Render("DRAIN OFF")
	if snap.Draining {
		drainStr = drainStyle.Render("DRAIN ON")
	}
	headerInner := fmt.Sprintf(
		"%s  │  %s  │  WS %s  RPC %s  %s",
		lipgloss.NewStyle().Bold(true).Render("PUMPY-BOT"),
		snap.Now.Format("2006-01-02 15:04:05 UTC"),
		wsStatus,
		rpcStatus,
		drainStr,
	)
	header := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		Padding(0, 2).
		Width(width - 4).
		Render(headerInner)

	// Tiles row 1: financial summary
	pnlStr := fmt.Sprintf("%+.4f SOL", snap.PnL)
	pnlStyle := lipgloss.NewStyle()
	switch {
	case snap.PnL > 0:
		pnlStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00d700")).Bold(true)
	case snap.PnL < 0:
		pnlStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5f5f")).Bold(true)
	}

	tiles1 := []string{
		dashboard.NewTile("Spent", fmt.Sprintf("%.4f SOL", snap.SolSpent)),
		dashboard.NewTile("Sold", fmt.Sprintf("%.4f SOL", snap.SolSold)),
		dashboard.NewTile("Balance", fmt.Sprintf("%.4f SOL", snap.LiveBalance)),
		dashboard.NewTile("Balance %", fmt.Sprintf("%.1f%%", snap.BalancePct)),
	}
	tiles2 := []string{
		dashboard.NewTile("Open", fmt.Sprintf("%d", snap.OpenCount)),
		dashboard.NewTile("Closed", fmt.Sprintf("%d", snap.ClosedCount)),
		dashboard.NewTileStyled("PnL", pnlStr, pnlStyle),
		dashboard.NewTile("Win Rate", fmt.Sprintf("%.1f%%", snap.WinRate)),
	}

	tileGrid := joinRows(tiles1, tiles2)

	// Open positions table
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n\n")
	sb.WriteString(tileGrid)
	sb.WriteString("\n\n")

	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("▼ Open Positions"))
	sb.WriteString("\n")
	if len(snap.OpenPositions) == 0 {
		sb.WriteString(dimStyle.Render("  (none)"))
		sb.WriteString("\n")
	} else {
		rows := make([][]string, 0, len(snap.OpenPositions))
		for _, p := range snap.OpenPositions {
			rows = append(rows, []string{
				shortMint(p.Mint),
				p.TierName(),
				fmt.Sprintf("%.2f×", p.PriceRatio()),
				fmt.Sprintf("%.4f SOL", p.BoughtSol()),
				fmt.Sprintf("%.0fs", p.Age().Seconds()),
				fmt.Sprintf("%d", p.SellsExecuted()),
			})
		}
		sb.WriteString(dashboard.BuildTable(
			[]string{"Mint", "Tier", "Mcap×", "Held SOL", "Age", "Sells"},
			rows, tty,
		))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("▼ Recent Transactions  (last %d)", len(snap.History)),
	))
	sb.WriteString("\n")
	if len(snap.History) == 0 {
		sb.WriteString(dimStyle.Render("  (none)"))
		sb.WriteString("\n")
	} else {
		rows := make([][]string, 0, len(snap.History))
		for _, r := range snap.History {
			rows = append(rows, []string{
				r.At.Format("15:04:05"),
				shortMint(r.Mint),
				r.Side,
				fmt.Sprintf("%.4f SOL", r.SolAmount),
				fmt.Sprintf("%.1f SOL", r.McapSol),
				shortSig(r.Signature),
			})
		}
		sb.WriteString(dashboard.BuildTable(
			[]string{"Time", "Mint", "Side", "SOL", "Mcap", "Sig"},
			rows, tty,
		))
	}

	return sb.String()
}

func wsStatusStr(ok bool) string {
	if ok {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#00d700")).Render("●")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5f5f")).Render("●")
}

func rpcStatusStr(ok bool) string {
	if ok {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#00d700")).Render("●")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#ffaf00")).Render("●")
}

func joinRows(rows ...[]string) string {
	rendered := make([]string, 0, len(rows))
	for _, tiles := range rows {
		rendered = append(rendered, lipgloss.JoinHorizontal(lipgloss.Top, tiles...))
	}
	return strings.Join(rendered, "\n")
}

func shortMint(mint string) string {
	if len(mint) <= 12 {
		return mint
	}
	return mint[:4] + "…" + mint[len(mint)-4:]
}

func shortSig(sig string) string {
	if len(sig) <= 12 {
		return sig
	}
	return sig[:8] + "…"
}
