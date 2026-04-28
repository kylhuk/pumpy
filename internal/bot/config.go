package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// Config holds all bot parameters. All fields have env overrides (BOT_ prefix).
type Config struct {
	TargetWallet string `json:"target_wallet"`
	BuySol       float64 `json:"buy_sol"`
	SlippagePct  int     `json:"slippage_pct"`
	PriorityFee  float64 `json:"priority_fee_sol"`
	Pool         string  `json:"pool"`

	TierSRatio float64 `json:"tier_s_ratio"`
	TierSPct   float64 `json:"tier_s_pct"`
	TierQRatio float64 `json:"tier_q_ratio"`
	TierQPct   float64 `json:"tier_q_pct"`
	TierAllPct float64 `json:"tier_all_pct"`

	EvaluatorIntervalMs int `json:"evaluator_interval_ms"`

	FanoutAddr string `json:"fanout_addr"`

	Lightning struct {
		APIKey       string `json:"api_key"`
		WalletPubkey string `json:"wallet_pubkey"`
	} `json:"lightning"`

	SolanaRPC        string `json:"solana_rpc"`
	BalanceRefreshMs int    `json:"balance_refresh_ms"`

	DashboardRefreshMs int `json:"dashboard_refresh_ms"`
	HistorySize        int `json:"history_size"`
}

// Defaults returns a Config populated with the documented defaults.
func Defaults() Config {
	return Config{
		TargetWallet:        "FWNmzY26FnsmpaWPQQJXQ24PQAyKtDByJz9HMa24s1z5",
		BuySol:              0.01,
		SlippagePct:         50,
		PriorityFee:         0.00005,
		Pool:                "pump",
		TierSRatio:          1.1,
		TierSPct:            5,
		TierQRatio:          0.5,
		TierQPct:            25,
		TierAllPct:          100,
		EvaluatorIntervalMs: 200,
		FanoutAddr:          "127.0.0.1:9988",
		SolanaRPC:           "https://api.mainnet-beta.solana.com",
		BalanceRefreshMs:    1000,
		DashboardRefreshMs:  250,
		HistorySize:         100,
	}
}

// Load reads the config file at path and applies env overrides on top.
func Load(path string) (Config, error) {
	cfg := Defaults()

	f, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("decode config: %w", err)
	}

	applyEnv(&cfg)

	if err := validate(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func applyEnv(c *Config) {
	if v := os.Getenv("BOT_TARGET_WALLET"); v != "" {
		c.TargetWallet = v
	}
	if v := os.Getenv("BOT_BUY_SOL"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.BuySol = f
		}
	}
	if v := os.Getenv("BOT_SLIPPAGE_PCT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.SlippagePct = n
		}
	}
	if v := os.Getenv("BOT_LIGHTNING_API_KEY"); v != "" {
		c.Lightning.APIKey = v
	}
	if v := os.Getenv("BOT_LIGHTNING_WALLET"); v != "" {
		c.Lightning.WalletPubkey = v
	}
	if v := os.Getenv("BOT_FANOUT_ADDR"); v != "" {
		c.FanoutAddr = v
	}
	if v := os.Getenv("BOT_SOLANA_RPC"); v != "" {
		c.SolanaRPC = v
	}
}

func validate(c Config) error {
	if c.TargetWallet == "" {
		return fmt.Errorf("target_wallet is required")
	}
	if c.BuySol <= 0 {
		return fmt.Errorf("buy_sol must be > 0")
	}
	if c.Lightning.APIKey == "" {
		return fmt.Errorf("lightning.api_key is required (or set BOT_LIGHTNING_API_KEY)")
	}
	if c.Lightning.WalletPubkey == "" {
		return fmt.Errorf("lightning.wallet_pubkey is required (or set BOT_LIGHTNING_WALLET)")
	}
	return nil
}
