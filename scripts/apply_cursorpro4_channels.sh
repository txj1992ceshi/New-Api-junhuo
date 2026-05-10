#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export CURSORPRO4_CHANNEL_PREFIX="${CURSORPRO4_CHANNEL_PREFIX:-cursorpro4}"
export CURSORPRO4_ENABLED="${CURSORPRO4_ENABLED:-true}"
export CURSORPRO4_PROBE_INFERENCE="${CURSORPRO4_PROBE_INFERENCE:-true}"

bash "$ROOT_DIR/scripts/external_pool_channels.sh" apply
