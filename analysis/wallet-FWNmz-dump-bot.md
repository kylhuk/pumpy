# Wallet Analysis: Mass Token Creation Dump Bot

**Wallet:** `FWNmzY26FnsmpaWPQQJXQ24PQAyKtDByJz9HMa24s1z5`  
**Observed:** 2026-04-27 14:54 UTC → 2026-04-28 03:45 UTC (12.9 hours)  
**Associated wallet:** `CoULsKJpCccJY22ZXNMuoVCYCVHu3zo6LNGQ9uJR8xy8`

---

## Summary

Fully automated **mass token creation + instant dev-dump bot**. Creates a new pump.fun token every ~19 seconds using a precisely calibrated SOL amount that lands at exactly ~82 SOL market cap every time. Dumps the entire position within 17 seconds by selling into sniper bots that rush to buy new token launches. Interspersed tiny re-buys during the sell are wash trades designed to fake organic chart activity.

**Total net PnL: +750 SOL in 12.9 hours** (~$125,000 USD at $167/SOL)

---

## The Core Pattern

### Step 1 — Calibrated creation (identical across all 600 tokens)

| Metric | Value |
|--------|-------|
| Avg SOL spent at creation | 21.46 SOL |
| Std deviation | 1.81 SOL |
| Median market cap at creation | 82.5 SOL |
| IQR of creation market cap | 81.4 – 84.2 SOL |

The consistency is not a coincidence — 600 tokens, all landing at the same market cap. This is a precisely tuned bonding curve calculation baked into the bot code. See *Why 82 SOL* below for the exact math.

### Step 2 — Sniper pump (passive)

After creation, other sniper bots automatically buy the new token, pushing price up. **81% of tokens peak above creation price.** Average peak market cap: **102.5 SOL** (+24% above creation). Median peak return: **+26.6%**. The dump wallet does nothing during this phase.

**When does the peak happen?**

| Percentile | Seconds to peak |
|-----------|----------------|
| P10 | 0.0 s |
| P25 | 1.1 s |
| Median | **3.3 s** |
| P75 | 7.3 s |
| P90 | 13.0 s |

### Step 3 — Rapid dump (~17 seconds total)

| Metric | Value |
|--------|-------|
| Avg time to first sell | 9 s |
| Median time to first sell | 7 s |
| Avg total dump duration | 17 s |
| 69% of tokens fully dumped within | 20 s |
| Avg chronological first-sell market cap | 64.3 SOL |
| Avg last-sell market cap | 29.3 SOL |
| Avg sells per token | 5.1 |

### Step 4 — Interspersed wash trades

During the sell sequence, the bot makes small buy transactions:

| | Sells | Buys |
|--|-------|------|
| Count per token (avg) | 5.1 | 3.2 |
| Median SOL size | **2.1 SOL** | **0.19 SOL** |
| Avg market cap at execution | 85.6 SOL | 76.7 SOL |
| Size ratio | 11× larger than buys | — |

Buys are distributed across all market cap levels (not only dips). Purpose: generate two-way chart activity and evade basic bot filters that screen for pure-dump wallets.

### Step 5 — Repeat every 19 seconds

| Percentile | Gap between creates |
|-----------|---------------------|
| P10 | 8 s |
| P25 | 13 s |
| Median | **19 s** |
| P75 | 27 s |
| P90 | 36 s |

Peak throughput: **3.5 tokens per minute** (211 tokens in the 21:00 UTC hour alone).

---

## Two-Speed Outcome Per Token

| | Fast dump (<20s) | Slow dump (≥20s) |
|--|-----------------|-----------------|
| Count | 414 tokens (69%) | 186 tokens (31%) |
| Win rate | 47.6% | **58.6%** |
| Avg PnL | **–0.76 SOL** | **+4.59 SOL** |

**Fast = failed.** No snipers came in; price crashed immediately. Bot exits at a small loss and moves on.  
**Slow = success.** Snipers bought in and provided exit liquidity. The bot sold into sustained demand.

The bot doesn't try to predict whether snipers will appear. It creates tokens mechanically and the positive expected value comes from asymmetry: wins are large when snipers arrive, losses are small when they don't.

---

## Overall Financial Results

| Metric | Value |
|--------|-------|
| Tokens created | 600 |
| Total SOL spent (creates + rebuys) | 14,329 SOL |
| Total SOL received (sells) | 15,079 SOL |
| **Net PnL** | **+750 SOL** |
| Avg per created token | +0.90 SOL |
| Overall win rate (per token) | 51% |
| Best single token | +61 SOL |
| Worst single token | –25 SOL |

---

## Multi-Wallet Coordination

The wallet also dumps tokens from an associated wallet `CoULsKJpCccJY22ZXNMuoVCYCVHu3zo6LNGQ9uJR8xy8`:

- **73 tokens** where `FWNmz` has only sells (no buys) — tokens received off-chain from `CoULs`
- All 5 of the buy+sell non-self-created tokens are also from `CoULs`
- `FWNmz` starts selling `CoULs` tokens within a **median of 9 seconds** of their creation
- `CoULs` itself created 116 tokens with an identical strategy
- At least 4 other wallets each dumped exactly 78 `CoULs`-created tokens, forming a coordinated dumper ring

| Wallet | Tokens dumped from CoULs | SOL received |
|--------|--------------------------|--------------|
| `FWNmz...` | 78 | 218 SOL |
| `Gtnc2...` | 78 | 116 SOL |
| `94QGj...` | 78 | 104 SOL |
| `Fppib...` | 78 | 35 SOL |
| `BnHZ9...` | 51 | 21 SOL |

---

## Token Names

All tokens are pure throwaways. 19 name variants across 600 creations, zero legitimate projects:

| Name | Symbol | Count |
|------|--------|-------|
| `s` | `s` | 132 |
| `just a trustworthy guy` | `ELON` | 97 |
| *(blank)* | *(blank)* | 63 |
| `JewHouse` | `JewHouse` | 60 |
| *(blank)* | `pls` | 59 |
| `last one` | `last` | 41 |
| `laslast one` | `please` | 32 |
| ... | ... | ... |

The bot does not attempt to create tokens with legitimate names because tokens never live long enough for names to matter.

---

## Why Exactly 82 SOL? The Math

This is fully derivable from pump.fun's bonding curve.

**Pump.fun AMM initial state (per token):**
- Virtual SOL reserve: 30 SOL
- Virtual token reserve: 1,073,000,191 tokens
- Constant product: k = 30 × 1,073,000,191 = 32,190,005,730
- Total supply: 1,000,000,000 tokens

**Market cap formula after buying X SOL:**

```
mcap = (30 + X)² × 1,000,000,000 / k
     = (30 + X)² × 1,000,000,000 / 32,190,005,730
```

**Solving for X = 21.5 SOL:**

```
mcap = 51.5² × 1,000,000,000 / 32,190,005,730
     = 2,652.25 × 1,000,000,000 / 32,190,005,730
     ≈ 82.4 SOL  ✓
```

**Inverted: to target exactly 82 SOL market cap:**

```
X = √(82 × 32,190,005,730 / 1,000,000,000) - 30
  = √2,639.58 - 30
  = 51.38 - 30
  = 21.38 SOL
```

The observed average of 21.46 SOL differs by only 0.08 SOL — the trading fee (~1%) taken by the protocol.

**82 SOL is not magical.** It is exactly what you get when you spend ~21.5 SOL at creation. The bot likely arrived at this number through empirical testing — it is the entry size that maximises sniper response while keeping cost-per-token below average sniper-funded sell proceeds.

---

## Counter-Strategy Analysis

### Question: Buy 1 SOL on every token they create, sell at T+15s. Profitable?

**No. Definitively not.**

| Exit time | Tokens | Avg return | Wins | Losses | Total P&L (600 × 1 SOL) |
|-----------|--------|-----------|------|--------|--------------------------|
| T+3s | 599 | **–2.1%** | 252 | 347 | –12.4 SOL |
| T+5s | 598 | –9.1% | 217 | 381 | –54.4 SOL |
| T+8s | 571 | –18.0% | 167 | 404 | –102.8 SOL |
| **T+15s** | **534** | **–35.0%** | **97** | **437** | **–187 SOL** |
| T+30s | 399 | –53.0% | 27 | 372 | –211.6 SOL |

By T+15, the dump is complete. Buying at creation and selling at T+15 means buying at 82 SOL avg and selling at ~55 SOL avg — a 33% loss on every trade.

### Why the early exits are also negative

The median price peak is at T+3.3s. But the T+3 average return is still **–2.1%** across all tokens because:

1. **19% of tokens (113/600) never pump above creation at all** — price drops immediately
2. **The peak window is sub-second** for many tokens — by the time you've bought and your exit TX confirms, the peak has passed
3. **Snipers are competing** — your buy contributes to the very pump you're trying to ride, pushing the peak lower per-participant

### The only window that is positive in isolation

If you magically knew which tokens would pump AND could exit at T+3:
- Pumping tokens only (peak >10% above create): **+8.2% avg at T+3**
- Win rate: 247/425 (58%)

But you cannot know this without already being in the trade. A conditional strategy (buy all, exit immediately if price is flat at T+2, hold to T+3 if pumping) still loses: **–7.7 SOL total** — slightly better than always holding to T+3 (–11.3 SOL) but still negative.

### What a profitable counter-strategy would actually require

1. **Sub-100ms Solana RPC detection** of this wallet's creation transactions
2. **Jito bundle MEV** — submit your buy in the same block as the creation to enter at T=0 price alongside the bot
3. **Automated exit within 2-3 seconds** with stop-loss at –5%, take-profit at +15%
4. Expected value at those parameters (pumping token avg +17.9%, 50% of tokens pump) ≈ **+4.5% per trade** before fees
5. At 46 tokens/hour and 0.1 SOL per trade, that's ~0.21 SOL/hour — marginal

Alternatively: use this wallet's creation events as a **negative signal** — if you see a new token and the creator is this wallet, it will be worthless in 17 seconds. Do not buy under any circumstances.

---

## Detection Fingerprint

Any detection system should flag wallets matching ALL of these:

| Feature | Threshold |
|---------|-----------|
| Tokens created per hour | > 20 |
| Std dev of creation market cap | < 5 SOL |
| Avg time from create to first sell | < 15 seconds |
| Ratio of (buy SOL) / (sell SOL) | < 0.15 |
| Unique token names / tokens created | < 0.05 |
| Associated wallets selling same-creator tokens within 10s | ≥ 2 |
