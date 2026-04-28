# Sell Algorithm: Price-Adaptive Reactive Seller

**Wallet:** `FWNmzY26FnsmpaWPQQJXQ24PQAyKtDByJz9HMa24s1z5`  
**Source:** Analysis of 600 created tokens, 3,077 sell transactions

---

## Key Finding

The bot is **not a time-based dumper**. It is a **price-adaptive reactive seller** that monitors market cap relative to its creation price and adjusts sell size accordingly. Total dump duration is a consequence of how long snipers sustain the price — not a fixed timer.

---

## The Three Sell Tiers

| Tier | Code | Size | Trigger condition | Median gap to next sell |
|------|------|------|-------------------|------------------------|
| **Small** | S | 3–5% of remaining balance | Price ≥ 100% of creation mcap | **0.96 s** |
| **Quarter** | Q | 25% of remaining balance | Price 40–80% of creation mcap | **1.23 s** |
| **Dump** | ALL | 100% remaining | Price ≤ ~33% of creation mcap | — (terminal) |

Observed market cap at each tier (creation mcap ≈ 82 SOL):

| Tier | Avg mcap | Avg % of creation mcap | Median % of creation mcap |
|------|----------|------------------------|--------------------------|
| S | 110 SOL | 133% | 123% |
| Q | 56 SOL | 68% | 60% |
| ALL | 28 SOL | 34% | 33% |

---

## The Sequence Rule

**Always: `S* → Q* → ALL`**

Zero or more small sells, then zero or more quarter sells, then one final all-dump. No exceptions observed. The sequence never reverses (no `Q→S` except in 15/3077 edge cases where a sniper buy bounced price back above the S threshold mid-sequence).

### Most common patterns

| Sequence | Tokens | % |
|----------|--------|---|
| `ALL` | 224 | 37.5% | ← price never pumped, immediate dump |
| `Q → ALL` | 49 | 8.2% |
| `Q → Q → ALL` | 21 | 3.5% |
| `S → ALL` | 20 | 3.3% |
| `S → S → ALL` | 18 | 3.0% |
| `S → Q → Q → ALL` | 11 | 1.8% |
| `Q → Q → Q → ALL` | 11 | 1.8% |
| `S×5 → ALL` | 9 | 1.5% |
| `S×N → Q → ALL` | ... | (many variants) |
| `S×48 → Q → ALL` | 6 | 1.0% | ← 48 small sells = long pump |

Longest observed: 50 sells total (`S×48 → Q → ALL`) — a token where snipers held the price above 82 SOL for 48 consecutive sell cycles.

---

## The Decision Logic

Reconstructed from observed behavior:

```
loop every ~1 second:
    price_ratio = current_mcap / creation_mcap (~82 SOL)

    if price_ratio >= 1.0:
        sell 3–5% of remaining balance    # S tier
    elif price_ratio >= 0.40:
        sell 25% of remaining balance     # Q tier
    else:
        sell 100% remaining               # ALL — exit
        break
```

The sell percentage values are hardcoded, not continuous. Observed distribution across 3,010 categorized sells:

| Value | Count | % of all sells |
|-------|-------|---------------|
| ~3% | 899 | 29.9% |
| ~5% | 816 | 27.1% |
| 100% | 596 | 19.8% |
| 25% | 425 | 14.1% |
| ~10% | 113 | 3.8% |
| ~50% | 18 | 0.6% |
| ~33% | 11 | 0.4% |

The dominant values are 3%, 5%, 25%, and 100%. The 10%, 50%, and 33% values likely represent tokens where the token balance math rounded to a different tier boundary.

---

## Tier Transitions

| Transition | Count | Median gap | Avg gap |
|------------|-------|-----------|---------|
| S → S | 1,700 | 0.96 s | 1.47 s |
| Q → ALL | 247 | 1.84 s | 2.77 s |
| Q → Q | 231 | 1.23 s | 1.83 s |
| S → Q | 162 | 2.78 s | 3.65 s |
| S → ALL | 122 | 3.63 s | 4.88 s |
| Q → S | 15 | 3.26 s | 3.67 s |

The S→Q transition (price falling below creation) takes a median 2.78 seconds to trigger — likely one polling cycle that detects the threshold crossing. The S→ALL shortcut (3.63 s) occurs when price collapses past both thresholds in one move.

---

## What Drives Sequence Length

The number of S sells is entirely determined by how long snipers hold the price above creation mcap (~82 SOL):

- **No sniper activity** → price never crosses 82 SOL → `ALL` immediately (37.5% of tokens)
- **Brief sniper pump** → 1–5 S sells before price falls → `S×1-5 → Q* → ALL`
- **Strong sniper activity** → price sustained above 82 SOL for many cycles → `S×10-48 → Q* → ALL`

The number of Q sells is determined by how slowly the price falls through the 40–80% zone. Each Q sell dumps 25% of balance, so 4 Q sells exhaust the remaining position through that zone.

---

## Price Thresholds (Absolute SOL values)

Given creation mcap ≈ 82.4 SOL:

| Threshold | Ratio | Absolute mcap |
|-----------|-------|---------------|
| S → Q crossover | 1.00× | ~82 SOL |
| Q → ALL crossover | ~0.33× | ~27 SOL |

These are fixed ratios relative to the creation mcap, making the thresholds consistent across all 600 tokens despite minor variation in creation price.

---

## Implications for Detection and Counter-Strategy

1. **Detection fingerprint**: Any wallet that exclusively sells in 3%, 5%, 25%, or 100% increments of remaining balance, in that order, is running this algorithm.

2. **Price signal**: When you see the first Q sell (25%), it means price has just crossed below creation mcap. The final dump (ALL) follows within ~2–5 seconds after the last Q sell.

3. **Timing the ALL dump**: The ALL sell is predictable — it fires when mcap reaches ~27 SOL (~33% of creation). Monitoring for the Q→ALL transition gap of 1.84 s (median) gives a narrow window.

4. **Counter-sniper implication**: Sniper bots that buy into the S phase are buying above creation price and competing directly against the bot's trickle-selling. The longer they sustain price above 82 SOL, the more S sells the bot executes against them. They are the exit liquidity.
