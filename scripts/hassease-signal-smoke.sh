#!/usr/bin/env bash
# Smoke test for Hassease Signal approval transport.
# Sends a test message, waits for a reply, and reports the result.
#
# Usage: ./scripts/hassease-signal-smoke.sh [config.yaml]
set -euo pipefail

CONFIG="${1:-cmd/hassease/hassease.yaml}"

if [[ ! -f "$CONFIG" ]]; then
  echo "error: config file not found: $CONFIG"
  echo "usage: $0 [path/to/hassease.yaml]"
  exit 1
fi

# Parse signal config from YAML (requires yq).
if ! command -v yq &>/dev/null; then
  echo "error: yq not found — install via: go install github.com/mikefarah/yq/v4@latest"
  exit 1
fi

ACCOUNT=$(yq '.signal.account' "$CONFIG")
RECIPIENT=$(yq '.signal.recipient' "$CONFIG")
BINARY=$(yq '.signal.binary // "signal-cli"' "$CONFIG")

if [[ -z "$ACCOUNT" || "$ACCOUNT" == "null" ]]; then
  echo "error: signal.account not set in $CONFIG"
  exit 1
fi
if [[ -z "$RECIPIENT" || "$RECIPIENT" == "null" ]]; then
  echo "error: signal.recipient not set in $CONFIG"
  exit 1
fi

echo "=== Hassease Signal Smoke Test ==="
echo "Account:   $ACCOUNT"
echo "Recipient: $RECIPIENT"
echo "Binary:    $BINARY"
echo

# Step 1: Check binary.
if ! command -v "$BINARY" &>/dev/null; then
  echo "FAIL: $BINARY not found on PATH"
  echo "See: docs/guides/hassease-signal-setup.md"
  exit 1
fi
echo "[1/4] signal-cli found: $($BINARY --version 2>&1 | head -1)"

# Step 2: Send test message.
TEST_MSG="[hassease smoke test] Reply 'y' to confirm Signal transport is working."
echo "[2/4] Sending test message..."
if ! "$BINARY" -a "$ACCOUNT" send -m "$TEST_MSG" "$RECIPIENT" 2>&1; then
  echo "FAIL: could not send message"
  exit 1
fi
echo "      Message sent."

# Step 3: Wait for reply.
echo "[3/4] Waiting for reply (30s timeout)..."
REPLY=""
FOUND=false

OUTPUT=$("$BINARY" -a "$ACCOUNT" receive --json --timeout 30 2>&1) || true

while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  SENDER=$(echo "$line" | python3 -c "
import sys, json
try:
    env = json.load(sys.stdin)
    dm = env.get('envelope', {}).get('dataMessage')
    if dm and dm.get('message'):
        print(env['envelope']['source'])
except: pass
" 2>/dev/null || true)

  if [[ "$SENDER" == "$RECIPIENT" ]]; then
    REPLY=$(echo "$line" | python3 -c "
import sys, json
env = json.load(sys.stdin)
print(env['envelope']['dataMessage']['message'])
" 2>/dev/null || true)
    FOUND=true
    break
  fi
done <<< "$OUTPUT"

if [[ "$FOUND" != "true" ]]; then
  echo "FAIL: no reply received within 30s"
  echo "      Check your phone — message may be in 'Message Requests'"
  exit 1
fi
echo "      Reply received: '$REPLY'"

# Step 4: Classify reply.
echo "[4/4] Classifying reply..."
LOWER=$(echo "$REPLY" | tr '[:upper:]' '[:lower:]' | xargs)
case "$LOWER" in
  y|yes|approve|go|ok|sure|yep)
    echo "PASS: Signal approval transport is working."
    echo
    echo "You can now run:"
    echo "  echo 'fix the bug' | hassease --signal --config $CONFIG"
    ;;
  n|no|deny|skip)
    echo "PASS: Signal transport works (you denied — that's fine for a smoke test)."
    ;;
  *)
    echo "WARN: Unexpected reply '$REPLY' — transport works but reply was not y/n."
    ;;
esac
