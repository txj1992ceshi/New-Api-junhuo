#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

export PROVIDER=cursor
export PORT="${PORT:-3401}"
export API_KEY="${API_KEY:-demo-cursor-key}"
export DASHBOARD_PASSWORD="${DASHBOARD_PASSWORD:-demo-cursor-dashboard}"
export DATA_DIR="${DATA_DIR:-$ROOT_DIR/runtime/local-pools}"
export SNAPSHOT_PATH="${SNAPSHOT_PATH:-}"
export CURSOR_PROVIDER_MODE="${CURSOR_PROVIDER_MODE:-direct}"
export CURSOR_DIRECT_PROTOCOL="${CURSOR_DIRECT_PROTOCOL:-connect}"
export CURSOR_DIRECT_BASE_URL="${CURSOR_DIRECT_BASE_URL:-https://api2.cursor.sh}"
export CURSOR_DIRECT_MODELS_PATH="${CURSOR_DIRECT_MODELS_PATH:-/v1/models}"
export CURSOR_DIRECT_RESPONSES_PATH="${CURSOR_DIRECT_RESPONSES_PATH:-/v1/responses}"
export CURSOR_DIRECT_CHAT_COMPLETIONS_PATH="${CURSOR_DIRECT_CHAT_COMPLETIONS_PATH:-/v1/chat/completions}"
export CURSOR_DIRECT_AUTH_HEADER="${CURSOR_DIRECT_AUTH_HEADER:-Authorization}"
export CURSOR_DIRECT_AUTH_SCHEME="${CURSOR_DIRECT_AUTH_SCHEME:-Bearer}"
export CURSOR_CONNECT_MODELS_PATH="${CURSOR_CONNECT_MODELS_PATH:-/agent.v1.AgentService/GetUsableModels}"
export CURSOR_CONNECT_RESPONSES_PATH="${CURSOR_CONNECT_RESPONSES_PATH:-/agent.v1.AgentService/Run}"
export CURSOR_CONNECT_CHAT_COMPLETIONS_PATH="${CURSOR_CONNECT_CHAT_COMPLETIONS_PATH:-/agent.v1.AgentService/Run}"
export CURSOR_CONNECT_PROTOCOL_VERSION="${CURSOR_CONNECT_PROTOCOL_VERSION:-1}"
export CURSOR_CONNECT_ACCEPT="${CURSOR_CONNECT_ACCEPT:-application/json}"
export CURSOR_CONNECT_CONTENT_TYPE="${CURSOR_CONNECT_CONTENT_TYPE:-application/json}"
export CURSOR_CONNECT_TIMEOUT_MS="${CURSOR_CONNECT_TIMEOUT_MS:-60000}"
export CURSOR_CONNECT_PAYLOAD_MODE="${CURSOR_CONNECT_PAYLOAD_MODE:-agent_run}"
export CURSOR_CONNECT_MODEL_PATHS="${CURSOR_CONNECT_MODEL_PATHS:-models.*.name,models.*.serverModelName,modelNames.*,composerModelConfig.defaultModel,composerModelConfig.fallbackModels.*,cmdKModelConfig.defaultModel,cmdKModelConfig.fallbackModels.*}"
export CURSOR_CONNECT_TEXT_PATHS="${CURSOR_CONNECT_TEXT_PATHS:-output_text,interactionUpdate.textDelta.text,text,intermediateText,content,result,message.content,output.0.content.0.text,candidates.0.content}"
export CURSOR_CONNECT_EXTRA_HEADERS_JSON="${CURSOR_CONNECT_EXTRA_HEADERS_JSON:-}"
export CURSOR_AUTH_STRATEGY="${CURSOR_AUTH_STRATEGY:-local_state_direct}"
export INFERENCE_MODE="${INFERENCE_MODE:-responses}"

exec node "$ROOT_DIR/local-pool-service/server.mjs"
