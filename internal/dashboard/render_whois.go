package dashboard

import (
	"fmt"
	"strings"
	"time"

	"pumpy/internal/graph"
	"pumpy/internal/store"
)

// RenderWhois returns the whois view for a single wallet as a string for printing.
func RenderWhois(d WhoisData, now time.Time, width int) string {
	if width <= 0 {
		width = 80
	}
	tty := IsTTY()

	// 1. Header bar
	neo4jIndicator := naStyle.Render("●")
	if d.Neo4jOK {
		neo4jIndicator = greenStyle.Render("●")
	}
	headerInner := fmt.Sprintf(
		"%s  │  %s  │  Neo4j %s\n%s",
		headerTitleStyle.Render("PUMPY WHOIS"),
		now.Format("2006-01-02 15:04:05 UTC"),
		neo4jIndicator,
		labelStyle.Render(d.Wallet),
	)
	header := headerStyle.Width(width - 4).Render(headerInner)

	// 2. Tile grid (PnL / Trades / Distinct tokens × 3 windows)
	windows := [3]struct {
		label string
		ws    WindowedStats
	}{
		{"24h", d.Window24h},
		{"7d", d.Window7d},
		{"14d", d.Window14d},
	}

	tiles := make([]string, 0, 9)
	for _, w := range windows {
		tiles = append(tiles, tileF("PnL "+w.label, w.ws.PnLSOL, "%+.4f SOL"))
	}
	for _, w := range windows {
		tiles = append(tiles, tileI("Trades "+w.label, w.ws.TradeCount))
	}
	for _, w := range windows {
		tiles = append(tiles, tileI("Tokens "+w.label, w.ws.DistinctTokens))
	}

	perRow := 1
	switch {
	case width >= 120:
		perRow = 3
	case width >= 80:
		perRow = 3
	}
	tileGrid := joinTilesGrid(tiles, perRow)

	// 3. Top tokens section (one table per window)
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n\n")
	sb.WriteString(tileGrid)
	sb.WriteString("\n\n")

	tokenHeaders := []string{"#", "Mint", "Symbol", "SOL Volume", "Trades"}
	sb.WriteString(sectionTitleStyle.Render("Top Pump.fun Tokens by SOL Volume"))
	sb.WriteString("\n")
	for _, w := range windows {
		sb.WriteString(labelStyle.Render("  "+w.label) + "\n")
		sb.WriteString(buildTable(tokenHeaders, tokenRows(w.ws.TopTokens), tty))
		sb.WriteString("\n")
	}

	// 4. Top counterparties section (one table per window)
	cpHeaders := []string{"#", "Wallet", "SOL Sent", "Transfers"}
	sb.WriteString("\n")
	sb.WriteString(sectionTitleStyle.Render("Top Outbound Counterparties by SOL Volume"))
	sb.WriteString("\n")
	if !d.Neo4jOK {
		sb.WriteString(naStyle.Render("  Neo4j unavailable — transfer graph not shown"))
		sb.WriteString("\n")
	} else {
		for _, w := range windows {
			sb.WriteString(labelStyle.Render("  "+w.label) + "\n")
			sb.WriteString(buildTable(cpHeaders, counterpartyRows(w.ws.TopCounterparties), tty))
			sb.WriteString("\n")
		}
	}

	// 5. All counterparties (all-time)
	sb.WriteString("\n")
	sb.WriteString(sectionTitleStyle.Render("All Outbound Counterparties (all-time)"))
	sb.WriteString("\n")
	if !d.Neo4jOK {
		sb.WriteString(naStyle.Render("  Neo4j unavailable — transfer graph not shown"))
		sb.WriteString("\n")
	} else {
		sb.WriteString(buildTable(cpHeaders, counterpartyRows(d.AllCounterparties), tty))
		sb.WriteString("\n")
	}

	return sb.String()
}

func tokenRows(r Result[[]store.WalletTopToken]) [][]string {
	if !r.OK() || len(r.Val) == 0 {
		return [][]string{{"-", "(no data)", "", "", ""}}
	}
	rows := make([][]string, 0, len(r.Val))
	for i, t := range r.Val {
		name := t.Symbol
		if name == "" {
			name = t.Name
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", i+1),
			t.Mint,
			name,
			fmt.Sprintf("%.4f", t.SOLVolume),
			fmt.Sprintf("%d", t.TradeCount),
		})
	}
	return rows
}

func counterpartyRows(r Result[[]graph.Counterparty]) [][]string {
	if !r.OK() || len(r.Val) == 0 {
		return [][]string{{"-", "(no data)", "", ""}}
	}
	rows := make([][]string, 0, len(r.Val))
	for i, c := range r.Val {
		rows = append(rows, []string{
			fmt.Sprintf("%d", i+1),
			c.Address,
			fmt.Sprintf("%.4f", c.SOLVolume),
			fmt.Sprintf("%d", c.TransferCount),
		})
	}
	return rows
}
