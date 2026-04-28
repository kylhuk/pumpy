#!/usr/bin/env bash
# Validates every Cypher query in dashboards/wallet-graph.json.
# Runs non-parameterized queries once; parameterized queries twice
# (empty params = first-load state, real params = populated state).
# Exits 1 if any query errors or if a value-type query returns 0 rows on empty params.
set -euo pipefail

DASH="$(cd "$(dirname "$0")/.." && pwd)/dashboards/wallet-graph.json"
NEO4J_PASS="${NEO4J_PASSWORD:-pumpypumpy}"

cypher() {
  # cypher <extra args...> — reads query from stdin
  docker compose exec -T neo4j cypher-shell -u neo4j -p "$NEO4J_PASS" "$@"
}

count_rows() {
  # Count data rows in cypher-shell output (first line is the header)
  echo "$1" | tail -n +2 | grep -c '[^[:space:]]' || true
}

PASS=0
FAIL=0
fail() { echo "  [FAIL] $*"; FAIL=$((FAIL+1)); }
pass() { echo "  [OK]   $*"; PASS=$((PASS+1)); }

echo "=== Fetching sample parameter values ==="
SAMPLE_WALLET=$(echo "MATCH (w:Wallet {source:'pump_seed'}) RETURN w.address AS a LIMIT 1" \
  | cypher 2>/dev/null | tail -n +2 | tr -d '"' | grep -v '^$' | head -1)
SAMPLE_HUB=$(echo "MATCH (h:Wallet {source:'discovered'})-[:TRANSFER]-(s:Wallet {source:'pump_seed'}) WITH h, count(DISTINCT s) AS n WHERE n >= 2 RETURN h.address AS a LIMIT 1" \
  | cypher 2>/dev/null | tail -n +2 | tr -d '"' | grep -v '^$' | head -1)

SAMPLE_WALLET="${SAMPLE_WALLET:-}"
SAMPLE_HUB="${SAMPLE_HUB:-}"
echo "  wallet: ${SAMPLE_WALLET:-(none found)}"
echo "  hub:    ${SAMPLE_HUB:-(none found)}"
echo ""

check() {
  local id="$1" type="$2" query="$3"
  [[ "$type" == "text" || "$type" == "select" || -z "$query" ]] && return 0

  local has_wallet=false has_hub=false
  echo "$query" | grep -q '\$neodash_wallet' && has_wallet=true
  echo "$query" | grep -q '\$neodash_hub'    && has_hub=true

  if ! $has_wallet && ! $has_hub; then
    echo "Report $id ($type) — static"
    local out
    out=$(echo "$query" | cypher 2>&1) || { fail "id=$id query error: $(echo "$out" | head -2)"; return 0; }
    local n; n=$(count_rows "$out")
    pass "id=$id → $n row(s)"
    return 0
  fi

  echo "Report $id ($type) — parameterized"

  # Build param flags
  local empty_flags=() real_flags=()
  $has_wallet && empty_flags+=(--param 'neodash_wallet => ""')
  $has_hub    && empty_flags+=(--param 'neodash_hub => ""')
  $has_wallet && real_flags+=(--param "neodash_wallet => \"$SAMPLE_WALLET\"")
  $has_hub    && real_flags+=(--param "neodash_hub => \"$SAMPLE_HUB\"")

  # Run with empty params
  local out_empty
  out_empty=$(echo "$query" | cypher "${empty_flags[@]}" 2>&1) || {
    fail "id=$id (empty params) query error: $(echo "$out_empty" | head -2)"
    return 0
  }
  local n_empty; n_empty=$(count_rows "$out_empty")

  # value-type with OPTIONAL MATCH + coalesce must return 1 row on empty params
  if [[ "$type" == "value" && "$n_empty" -lt 1 ]]; then
    fail "id=$id (value) returned 0 rows on empty params — TypeError will fire"
  else
    pass "id=$id empty → $n_empty row(s)"
    PASS=$((PASS+1))  # already incremented inside pass, but count the empty check separately
  fi

  # Run with real params (skip if no sample data)
  if [[ -n "$SAMPLE_WALLET$SAMPLE_HUB" ]]; then
    local out_real
    out_real=$(echo "$query" | cypher "${real_flags[@]}" 2>&1) || {
      fail "id=$id (real params) query error: $(echo "$out_real" | head -2)"
      return 0
    }
    local n_real; n_real=$(count_rows "$out_real")
    pass "id=$id real → $n_real row(s)"
  fi
}

echo "=== Running queries ==="
while IFS=$'\t' read -r id type query; do
  check "$id" "$type" "$query"
done < <(jq -r '.pages[].reports[] | [(.id|tostring), .type, (.query // "")] | @tsv' "$DASH")

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[[ $FAIL -eq 0 ]]
