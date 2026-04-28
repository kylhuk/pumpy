package dashboard

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
	"golang.org/x/term"
)

// Package-level styles (shared so we don't reallocate per call).
var (
	headerStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			Padding(0, 2)

	headerTitleStyle = lipgloss.NewStyle().Bold(true)

	tileStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Width(26)

	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	valueStyle = lipgloss.NewStyle().Bold(true)
	naStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))

	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00d700"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffaf00"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5f5f"))

	sectionTitleStyle = lipgloss.NewStyle().Bold(true)
)

var (
	isTTYOnce sync.Once
	isTTYVal  bool
)

// IsTTY reports whether stdout is attached to a terminal.
func IsTTY() bool {
	isTTYOnce.Do(func() { isTTYVal = term.IsTerminal(int(os.Stdout.Fd())) })
	return isTTYVal
}

// StatusDot returns a coloured "●" based on data age.
// Green ≤2m, yellow ≤10m, red otherwise.
func StatusDot(age time.Duration) string {
	switch {
	case age <= 2*time.Minute:
		return greenStyle.Render("●")
	case age <= 10*time.Minute:
		return yellowStyle.Render("●")
	default:
		return redStyle.Render("●")
	}
}

func timeStatusDot(r Result[time.Time], now time.Time) string {
	if !r.OK() {
		return StatusDot(999 * time.Hour)
	}
	return StatusDot(now.Sub(r.Val))
}

func neo4jStatusDot(r Result[bool]) string {
	if r.OK() && r.Val {
		return StatusDot(0)
	}
	return StatusDot(999 * time.Hour)
}

func tile(label, value string) string {
	return tileStyle.Render(labelStyle.Render(label) + "\n" + valueStyle.Render(value))
}

func tileNA(label string) string {
	return tileStyle.Render(labelStyle.Render(label) + "\n" + naStyle.Render("n/a"))
}

func tileI(label string, r Result[int64]) string {
	if !r.OK() {
		return tileNA(label)
	}
	return tile(label, fmt.Sprintf("%d", r.Val))
}

func tileF(label string, r Result[float64], format string) string {
	if !r.OK() {
		return tileNA(label)
	}
	return tile(label, fmt.Sprintf(format, r.Val))
}

func migratedTile(d DashboardData) string {
	if !d.MigratedTotal.OK() {
		return tileNA("Graduated Tokens")
	}
	last24h := int64(0)
	if d.MigratedLast24h.OK() {
		last24h = d.MigratedLast24h.Val
	}
	val := fmt.Sprintf("%d total  (+%d today)", d.MigratedTotal.Val, last24h)
	return tile("Graduated Tokens", val)
}

func buySellTile(d DashboardData) string {
	if !d.BuysLast24h.OK() && !d.SellsLast24h.OK() {
		return tileNA("Buy / Sell 24h")
	}
	buys := int64(0)
	sells := int64(0)
	if d.BuysLast24h.OK() {
		buys = d.BuysLast24h.Val
	}
	if d.SellsLast24h.OK() {
		sells = d.SellsLast24h.Val
	}
	return tile("Buy / Sell 24h", fmt.Sprintf("%d / %d", buys, sells))
}

func hottestTile(d DashboardData) string {
	if !d.HottestTokenSymbol.OK() || d.HottestTokenSymbol.Val == "" {
		return tileNA("Hottest Token 1h")
	}
	val := d.HottestTokenSymbol.Val
	if d.HottestTokenTrades.OK() {
		val = fmt.Sprintf("%s  (%d trades)", val, d.HottestTokenTrades.Val)
	}
	return tile("Hottest Token 1h", val)
}

func crawlerTile(d DashboardData) string {
	if !d.CrawlerTotal.OK() {
		return tileNA("Crawler Queue")
	}
	total := d.CrawlerTotal.Val
	complete := int64(0)
	errs := int64(0)
	if d.CrawlerComplete.OK() {
		complete = d.CrawlerComplete.Val
	}
	if d.CrawlerErrors.OK() {
		errs = d.CrawlerErrors.Val
	}
	val := fmt.Sprintf("%d/%d done  (%d err)", complete, total, errs)
	return tile("Crawler Queue", val)
}

// BuildTable builds a go-pretty table and returns its rendered string.
// When tty is false, plain ASCII style is used so the output is pipe-friendly.
func BuildTable(headers []string, rows [][]string, tty bool) string {
	return buildTable(headers, rows, tty)
}

// NewTile renders a labelled value tile using the package tile style.
func NewTile(label, value string) string { return tile(label, value) }

// NewTileStyled renders a tile where the value is pre-styled by the caller.
func NewTileStyled(label, value string, valueStyle lipgloss.Style) string {
	return tileStyle.Render(labelStyle.Render(label) + "\n" + valueStyle.Render(value))
}

func buildTable(headers []string, rows [][]string, tty bool) string {
	t := table.NewWriter()

	if tty {
		t.SetStyle(table.StyleRounded)
	} else {
		t.SetStyle(table.StyleDefault)
	}

	headerRow := make(table.Row, len(headers))
	for i, h := range headers {
		headerRow[i] = h
	}
	t.AppendHeader(headerRow)

	for _, r := range rows {
		row := make(table.Row, len(r))
		for i, v := range r {
			row[i] = v
		}
		t.AppendRow(row)
	}

	return t.Render()
}

// joinTilesGrid arranges tiles in rows of n per row, joining them horizontally
// and stacking each row vertically.
func joinTilesGrid(tiles []string, perRow int) string {
	if perRow < 1 {
		perRow = 1
	}
	var rows []string
	for i := 0; i < len(tiles); i += perRow {
		end := i + perRow
		if end > len(tiles) {
			end = len(tiles)
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top, tiles[i:end]...)
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

func pnlRows(r Result[[]PnLRow]) [][]string {
	if !r.OK() || len(r.Val) == 0 {
		return [][]string{{"-", "no data yet", "-"}}
	}
	rows := make([][]string, 0, len(r.Val))
	for _, p := range r.Val {
		rows = append(rows, []string{
			fmt.Sprintf("%d", p.Rank),
			p.Wallet,
			fmt.Sprintf("%.4f", p.PnLSOL),
		})
	}
	return rows
}

func externalRows(r Result[[]ExternalRow]) [][]string {
	if !r.OK() || len(r.Val) == 0 {
		return [][]string{{"-", "no data yet", "-"}}
	}
	rows := make([][]string, 0, len(r.Val))
	for _, e := range r.Val {
		rows = append(rows, []string{
			fmt.Sprintf("%d", e.Rank),
			e.Wallet,
			fmt.Sprintf("%d", e.Trades),
		})
	}
	return rows
}

// Render returns the entire dashboard as a string for printing to stdout.
func Render(d DashboardData, now time.Time, width int) string {
	if width <= 0 {
		width = 80
	}
	tty := IsTTY()

	// 1. Header bar
	headerInner := fmt.Sprintf(
		"%s  │  %s  │  WS %s  Dune %s  Neo4j %s",
		headerTitleStyle.Render("PUMPY STATS"),
		now.Format("2006-01-02 15:04:05 UTC"),
		timeStatusDot(d.LastTradeAt, now),
		timeStatusDot(d.LastDuneCallAt, now),
		neo4jStatusDot(d.Neo4jOK),
	)
	header := headerStyle.Width(width - 4).Render(headerInner)

	// 2. Tiles grid
	tiles := []string{
		tileI("Total Wallets", d.TotalWallets),
		tileI("Active 24h", d.ActiveWallets24h),
		tileF("Volume 24h", d.Volume24hSOL, "%.2f SOL"),
		tileI("New Tokens 24h", d.NewTokens24h),
		migratedTile(d),
		tileI("Trades/min", d.TradesPerMinute),
		buySellTile(d),
		hottestTile(d),
		crawlerTile(d),
		tileI("Discovered Wallets", d.DiscoveredWallets),
		tileI("Graph Edges", d.GraphEdges),
	}

	perRow := 1
	switch {
	case width >= 120:
		perRow = 4
	case width >= 80:
		perRow = 2
	}
	tileGrid := joinTilesGrid(tiles, perRow)

	// 3. PnL Tables
	pnlHeaders := []string{"#", "Wallet", "PnL (SOL)"}
	topTable := buildTable(pnlHeaders, pnlRows(d.Top5Traders), tty)
	flopTable := buildTable(pnlHeaders, pnlRows(d.Flop5Traders), tty)
	topTitle := sectionTitleStyle.Render("▲ Top 5 Traders — 24h Realized PnL")
	flopTitle := sectionTitleStyle.Render("▼ Flop 5 Traders — 24h Realized PnL")

	// 4. External Wallets section
	externalTitle := sectionTitleStyle.Render("🔗 Top Pump Wallets by External SOL Transfers")
	var externalSection string
	if !d.Neo4jOK.OK() || !d.Neo4jOK.Val {
		externalSection = naStyle.Render("  Neo4j unavailable — graph stats not shown")
	} else {
		externalSection = buildTable(
			[]string{"#", "Wallet", "External Transfers"},
			externalRows(d.TopExternalWallets),
			tty,
		)
	}

	// Assembly
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n\n")
	sb.WriteString(tileGrid)
	sb.WriteString("\n\n")
	sb.WriteString(topTitle + "\n")
	sb.WriteString(topTable)
	sb.WriteString("\n\n")
	sb.WriteString(flopTitle + "\n")
	sb.WriteString(flopTable)
	sb.WriteString("\n\n")
	sb.WriteString(externalTitle + "\n")
	sb.WriteString(externalSection)
	sb.WriteString("\n")
	return sb.String()
}
