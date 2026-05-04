#!/usr/bin/env bash

set -euo pipefail

REMOTE_HOST="${REMOTE_HOST:?set REMOTE_HOST, e.g. 198.13.35.85}"
REMOTE_USER="${REMOTE_USER:-root}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/tencent_hk_mac}"
LOCAL_PORT="${LOCAL_PORT:-3003}"
REMOTE_PORT="${REMOTE_PORT:-3003}"

exec ssh \
  -o IdentitiesOnly=yes \
  -o StrictHostKeyChecking=accept-new \
  -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 \
  -o ServerAliveCountMax=3 \
  -i "$SSH_KEY" \
  -N \
  -L "${LOCAL_PORT}:127.0.0.1:${REMOTE_PORT}" \
  "${REMOTE_USER}@${REMOTE_HOST}"
