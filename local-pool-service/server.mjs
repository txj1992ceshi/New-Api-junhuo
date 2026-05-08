import http from 'node:http';
import http2 from 'node:http2';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { URL } from 'node:url';
import {
  CodeWhispererStreaming,
  GenerateAssistantResponseCommand,
} from '@aws/codewhisperer-streaming-client';

const provider = String(process.env.PROVIDER || '').trim().toLowerCase();
if (!provider || !['cursor', 'kiro', 'windsurf', 'codex'].includes(provider)) {
  console.error('PROVIDER must be one of: cursor, kiro, windsurf, codex');
  process.exit(1);
}

const port = Number(
  process.env.PORT ||
    (provider === 'cursor'
      ? 3401
      : provider === 'kiro'
        ? 3501
        : provider === 'codex'
          ? 3601
          : 3003),
);
const apiKey = String(process.env.API_KEY || `demo-${provider}-key`).trim();
const dashboardPassword = String(process.env.DASHBOARD_PASSWORD || `demo-${provider}-dashboard`).trim();
const dataDir = path.resolve(
  process.env.DATA_DIR || path.join(process.cwd(), 'runtime', 'local-pools'),
);
const dataFile = path.join(dataDir, `${provider}.accounts.json`);
const snapshotPath = String(process.env.SNAPSHOT_PATH || '').trim();
const providerModels = (process.env.DEFAULT_MODELS || '').trim();
const cursorProviderMode = String(process.env.CURSOR_PROVIDER_MODE || 'direct')
  .trim()
  .toLowerCase();
const cursorDirectProtocol = String(process.env.CURSOR_DIRECT_PROTOCOL || 'rest')
  .trim()
  .toLowerCase();
const cursorDirectBaseUrl = String(
  process.env.CURSOR_DIRECT_BASE_URL || process.env.CURSOR_UPSTREAM_BASE_URL || '',
)
  .trim()
  .replace(/\/+$/, '');
const cursorDirectModelsPath = String(process.env.CURSOR_DIRECT_MODELS_PATH || '/v1/models')
  .trim();
const cursorDirectResponsesPath = String(
  process.env.CURSOR_DIRECT_RESPONSES_PATH || '/v1/responses',
).trim();
const cursorDirectChatCompletionsPath = String(
  process.env.CURSOR_DIRECT_CHAT_COMPLETIONS_PATH || '/v1/chat/completions',
).trim();
const cursorConnectModelsPath = String(process.env.CURSOR_CONNECT_MODELS_PATH || '').trim();
const cursorConnectResponsesPath = String(process.env.CURSOR_CONNECT_RESPONSES_PATH || '').trim();
const cursorConnectChatCompletionsPath = String(
  process.env.CURSOR_CONNECT_CHAT_COMPLETIONS_PATH || '',
).trim();
const cursorConnectProtocolVersion = String(process.env.CURSOR_CONNECT_PROTOCOL_VERSION || '1')
  .trim();
const cursorConnectAccept = String(process.env.CURSOR_CONNECT_ACCEPT || 'application/json').trim();
const cursorConnectContentType = String(
  process.env.CURSOR_CONNECT_CONTENT_TYPE || 'application/json',
).trim();
const cursorConnectTimeoutMs = Number(process.env.CURSOR_CONNECT_TIMEOUT_MS || 60_000);
const cursorConnectPayloadMode = String(process.env.CURSOR_CONNECT_PAYLOAD_MODE || 'passthrough')
  .trim()
  .toLowerCase();
const cursorConnectModelPaths = String(
  process.env.CURSOR_CONNECT_MODEL_PATHS ||
    'models.*.modelId,models.*.displayModelId,models.*.aliases.*,data.*.id,models.*.id,models.*.name,availableModels.*',
)
  .trim();
const cursorConnectTextPaths = String(
  process.env.CURSOR_CONNECT_TEXT_PATHS ||
    'output_text,interactionUpdate.textDelta.text,text,content,result,message.content,output.0.content.0.text,candidates.0.content',
)
  .trim();
const cursorConnectExtraHeadersRaw = String(process.env.CURSOR_CONNECT_EXTRA_HEADERS_JSON || '').trim();
const cursorDirectAuthHeader = String(process.env.CURSOR_DIRECT_AUTH_HEADER || 'Authorization')
  .trim();
const cursorDirectAuthScheme = String(process.env.CURSOR_DIRECT_AUTH_SCHEME || 'Bearer').trim();
const cursorAuthStrategy = String(process.env.CURSOR_AUTH_STRATEGY || 'local_state_direct')
  .trim()
  .toLowerCase();
const codexAuthStrategy = String(process.env.CODEX_AUTH_STRATEGY || 'provider_bridge')
  .trim()
  .toLowerCase();
const windsurfAuthStrategy = String(process.env.WINDSURF_AUTH_STRATEGY || 'local_state_direct')
  .trim()
  .toLowerCase();
const inferenceMode = String(process.env.INFERENCE_MODE || 'responses')
  .trim()
  .toLowerCase();

const defaultModelMap = {
  cursor: ['default', 'gpt-4.1-mini', 'gpt-4o-mini', 'gpt-5-mini'],
  windsurf: [
    'gpt-4o-mini',
    'gpt-4.1-mini',
    'gpt-5-mini',
    'gemini-2.5-flash',
    'glm-4.7',
    'swe-1.5-fast',
  ],
  kiro: [
    'auto',
    'claude-sonnet-4.5',
    'claude-sonnet-4',
    'claude-haiku-4.5',
    'deepseek-3.2',
    'minimax-m2.5',
    'minimax-m2.1',
    'glm-5',
    'qwen3-coder-next',
  ],
  codex: ['gpt-5.5', 'gpt-5.4', 'gpt-5', 'gpt-5-mini', 'o3-mini', 'codex-mini', 'gpt-5.3-codex'],
};
const fixedKiroProfileMap = {
  BuilderId: 'arn:aws:codewhisperer:us-east-1:638616132270:profile/AAAACCCCXXXX',
  Github: 'arn:aws:codewhisperer:us-east-1:699475941385:profile/EHGA3GRVQMUK',
  Google: 'arn:aws:codewhisperer:us-east-1:699475941385:profile/EHGA3GRVQMUK',
};

function nowIso() {
  return new Date().toISOString();
}

function ensureDir() {
  fs.mkdirSync(dataDir, { recursive: true });
}

function json(res, statusCode, payload) {
  const body = JSON.stringify(payload);
  res.writeHead(statusCode, {
    'Content-Type': 'application/json',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type, Authorization, x-api-key',
  });
  res.end(body);
}

function html(res, statusCode, content) {
  res.writeHead(statusCode, { 'Content-Type': 'text/html; charset=utf-8' });
  res.end(content);
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    req.on('data', (chunk) => chunks.push(chunk));
    req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
    req.on('error', reject);
  });
}

function parseBearerToken(req) {
  const auth = String(req.headers.authorization || '').trim();
  if (/^Bearer\s+/i.test(auth)) {
    return auth.replace(/^Bearer\s+/i, '').trim();
  }
  const apiKeyHeader = String(req.headers['x-api-key'] || '').trim();
  return apiKeyHeader;
}

function ensureApiAuth(req, res) {
  const token = parseBearerToken(req);
  if (!token || token !== apiKey) {
    json(res, 401, {
      error: {
        message: 'Invalid API key',
        type: 'auth_error',
      },
    });
    return false;
  }
  return true;
}

function parseCookies(req) {
  const raw = String(req.headers.cookie || '');
  const out = {};
  for (const item of raw.split(';')) {
    const [key, ...rest] = item.split('=');
    const name = key?.trim();
    if (!name) continue;
    out[name] = rest.join('=').trim();
  }
  return out;
}

function ensureDashboardAuth(req, res) {
  if (!dashboardPassword) return true;
  const cookies = parseCookies(req);
  const ok = cookies[`${provider}_dashboard`] === dashboardPassword;
  if (!ok) {
    html(res, 401, renderLoginPage(''));
    return false;
  }
  return true;
}

function makeId(prefix) {
  return `${prefix}-${crypto.randomBytes(4).toString('hex')}`;
}

function getSourcePaths() {
  const home = os.homedir();
  return {
    cursorDbPath:
      process.env.CURSOR_DB_PATH ||
      path.join(home, 'Library', 'Application Support', 'Cursor', 'User', 'globalStorage', 'state.vscdb'),
    windsurfDbPath:
      process.env.WINDSURF_DB_PATH ||
      path.join(
        home,
        'Library',
        'Application Support',
        'Windsurf',
        'User',
        'globalStorage',
        'state.vscdb',
      ),
    kiroAuthPath:
      process.env.KIRO_AUTH_PATH ||
      path.join(home, '.aws', 'sso', 'cache', 'kiro-auth-token.json'),
    kiroProfilePath:
      process.env.KIRO_PROFILE_PATH ||
      path.join(
        home,
        'Library',
        'Application Support',
        'Kiro',
        'User',
        'globalStorage',
        'kiro.kiroagent',
        'profile.json',
      ),
    codexAuthPath:
      process.env.CODEX_AUTH_PATH ||
      path.join(home, '.codex', 'auth.json'),
    codexConfigPath:
      process.env.CODEX_CONFIG_PATH ||
      path.join(home, '.codex', 'config.toml'),
  };
}

function safeReadJson(filePath) {
  const raw = fs.readFileSync(filePath, 'utf8');
  return JSON.parse(raw);
}

function safeJsonParse(raw, fallback = null) {
  try {
    return JSON.parse(raw);
  } catch {
    return fallback;
  }
}

function parseRequestJson(raw) {
  if (!raw || !String(raw).trim()) return {};
  return safeJsonParse(raw, {});
}

function parseJsonEnv(raw, fallback = {}) {
  if (!raw) return fallback;
  const parsed = safeJsonParse(raw, null);
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return fallback;
  }
  return parsed;
}

const cursorConnectExtraHeaders = parseJsonEnv(cursorConnectExtraHeadersRaw, {});

function cursorConnectUsesAiserver() {
  return cursorDirectBaseUrl.includes('cursor.sh');
}

function resolveCursorConnectPath(explicitPath, fallbackPath) {
  const trimmed = String(explicitPath || '').trim();
  if (trimmed) return trimmed;
  return cursorConnectUsesAiserver() ? fallbackPath : '';
}

function resolveCursorConnectPayloadMode() {
  if (cursorConnectPayloadMode && cursorConnectPayloadMode !== 'passthrough') {
    return cursorConnectPayloadMode;
  }
  return cursorConnectUsesAiserver() ? 'cursor_unified_chat' : 'passthrough';
}

const effectiveCursorConnectPayloadMode = resolveCursorConnectPayloadMode();
const effectiveCursorConnectModelsPath = resolveCursorConnectPath(
  cursorConnectModelsPath,
  effectiveCursorConnectPayloadMode === 'agent_run'
    ? '/agent.v1.AgentService/GetUsableModels'
    : '/aiserver.v1.AiService/AvailableModels',
);
const effectiveCursorConnectResponsesPath = resolveCursorConnectPath(
  cursorConnectResponsesPath,
  effectiveCursorConnectPayloadMode === 'agent_run'
    ? '/agent.v1.AgentService/Run'
    : '/aiserver.v1.ChatService/StreamUnifiedChat',
);
const effectiveCursorConnectChatCompletionsPath = resolveCursorConnectPath(
  cursorConnectChatCompletionsPath,
  effectiveCursorConnectResponsesPath,
);

function loadPersistedAccounts() {
  ensureDir();
  if (!fs.existsSync(dataFile)) {
    return [];
  }
  try {
    const payload = safeReadJson(dataFile);
    return Array.isArray(payload?.accounts) ? payload.accounts : [];
  } catch (error) {
    console.warn(`[${provider}] failed to parse persisted accounts:`, error.message);
    return [];
  }
}

function savePersistedAccounts(accounts) {
  ensureDir();
  fs.writeFileSync(
    dataFile,
    JSON.stringify(
      {
        provider,
        updated_at: nowIso(),
        accounts,
      },
      null,
      2,
    ),
  );
}

function defaultModels() {
  if (providerModels) {
    return providerModels
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean);
  }
  return defaultModelMap[provider] || [];
}

function readCursorLocalAccount() {
  const { cursorDbPath } = getSourcePaths();
  if (!fs.existsSync(cursorDbPath)) return null;
  let output = '';
  try {
    output = execFileSync('sqlite3', [
      cursorDbPath,
      "select coalesce((select value from ItemTable where key='cursorAuth/cachedEmail' limit 1), ''),coalesce((select value from ItemTable where key='cursorAuth/cachedSignUpType' limit 1), ''),coalesce((select value from ItemTable where key='cursorAuth/stripeMembershipType' limit 1), ''),coalesce((select value from ItemTable where key='cursorAuth/accessToken' limit 1), ''),coalesce((select value from ItemTable where key='cursorAuth/refreshToken' limit 1), '');",
    ], { encoding: 'utf8' }).trim();
  } catch (error) {
    console.warn(`[cursor] failed to query state.vscdb:`, error.message);
    return null;
  }
  const [email, signUpType, membershipType, accessToken, refreshToken] = output.split('|');
  if (!email || !accessToken) return null;
  return {
    id: 'cursor-local',
    email,
    source: 'local_cursor_state_vscdb',
    source_path: cursorDbPath,
    method: 'local',
    provider: 'cursor',
    sign_up_type: signUpType || null,
    membership_type: membershipType || null,
    access_token: accessToken,
    refresh_token: refreshToken || null,
    status: 'active',
    added_at: nowIso(),
    available_models: defaultModels(),
    tier: membershipType || 'unknown',
  };
}

function readWindsurfStateValue(dbPath, key) {
  try {
    return execFileSync(
      'sqlite3',
      [dbPath, `select coalesce((select value from ItemTable where key='${key}' limit 1), '');`],
      { encoding: 'utf8' },
    ).trim();
  } catch (error) {
    console.warn(`[windsurf] failed to query state.vscdb key=${key}:`, error.message);
    return '';
  }
}

function readWindsurfLocalAccount() {
  const { windsurfDbPath } = getSourcePaths();
  if (!fs.existsSync(windsurfDbPath)) return null;
  const authStatusRaw = readWindsurfStateValue(windsurfDbPath, 'windsurfAuthStatus');
  const configRaw = readWindsurfStateValue(windsurfDbPath, 'codeium.windsurf');
  const planRaw = readWindsurfStateValue(windsurfDbPath, 'windsurf.settings.cachedPlanInfo');
  const authStatus = safeJsonParse(authStatusRaw, null);
  const config = safeJsonParse(configRaw, null);
  const plan = safeJsonParse(planRaw, null);
  const accessToken = String(authStatus?.apiKey || '').trim();
  const email = String(config?.lastLoginEmail || '').trim();
  const apiServerUrl = String(config?.apiServerUrl || '').trim();
  const planName = String(plan?.planName || '').trim();
  const preferredModels = Array.isArray(config?.['windsurf.state.lastSelectedCascadeModelUids'])
    ? config['windsurf.state.lastSelectedCascadeModelUids']
        .map((item) => String(item || '').trim())
        .filter(Boolean)
    : [];
  if (!email || !accessToken) return null;
  return {
    id: 'windsurf-local',
    email,
    source: 'local_windsurf_state_vscdb',
    source_path: windsurfDbPath,
    method: 'local',
    provider: 'windsurf',
    access_token: accessToken,
    api_server_url: apiServerUrl || null,
    status: 'active',
    added_at: nowIso(),
    available_models: preferredModels.length > 0 ? preferredModels : defaultModels(),
    tier: planName || 'unknown',
    raw: {
      auth_status: authStatus,
      config,
      plan,
    },
  };
}

function readKiroLocalAccount() {
  const { kiroAuthPath, kiroProfilePath } = getSourcePaths();
  if (!fs.existsSync(kiroAuthPath)) return null;
  try {
    const payload = safeReadJson(kiroAuthPath);
    const profile = fs.existsSync(kiroProfilePath) ? safeReadJson(kiroProfilePath) : null;
    const email = String(payload.email || '').trim();
    const accessToken = String(payload.accessToken || payload.access_token || '').trim();
    const refreshToken = String(payload.refreshToken || payload.refresh_token || '').trim();
    if (!email || !accessToken) return null;
    return {
      id: 'kiro-local',
      email,
      source: 'local_kiro_auth_token',
      source_path: kiroAuthPath,
      method: 'local',
      provider: 'kiro',
      auth_method: payload.authMethod || payload.auth_method || null,
      login_provider: payload.provider || payload.loginProvider || null,
      profile_arn:
        payload.profileArn ||
        payload.profile_arn ||
        profile?.arn ||
        fixedKiroProfileMap[payload.provider] ||
        null,
      access_token: accessToken,
      refresh_token: refreshToken || null,
      expires_at: payload.expiresAt || payload.expires_at || null,
      region: payload.region || payload.idc_region || null,
      status: 'active',
      added_at: nowIso(),
      available_models: defaultModels(),
      tier: payload.provider || 'unknown',
    };
  } catch (error) {
    console.warn(`[kiro] failed to parse local auth file:`, error.message);
    return null;
  }
}

function parseSimpleTomlValue(raw, key) {
  const match = String(raw || '').match(new RegExp(`^\\s*${key}\\s*=\\s*"([^"]*)"`, 'm'));
  return String(match?.[1] || '').trim();
}

function readTomlSection(raw, sectionName) {
  const lines = String(raw || '').split(/\r?\n/);
  const header = `[${sectionName}]`;
  let start = -1;
  for (let i = 0; i < lines.length; i += 1) {
    if (lines[i].trim() === header) {
      start = i + 1;
      break;
    }
  }
  if (start < 0) return '';
  const body = [];
  for (let i = start; i < lines.length; i += 1) {
    const line = lines[i];
    if (/^\s*\[.+\]\s*$/.test(line)) {
      break;
    }
    body.push(line);
  }
  return body.join('\n');
}

function parseCodexProviderBaseUrl(configRaw) {
  const providerName = parseSimpleTomlValue(configRaw, 'model_provider');
  if (!providerName) return '';
  const sectionBody = readTomlSection(configRaw, `model_providers.${providerName}`);
  return parseSimpleTomlValue(sectionBody, 'base_url');
}

function readCodexLocalAccount() {
  const { codexAuthPath, codexConfigPath } = getSourcePaths();
  if (!fs.existsSync(codexAuthPath) || !fs.existsSync(codexConfigPath)) return null;
  try {
    const authPayload = safeReadJson(codexAuthPath);
    const configRaw = fs.readFileSync(codexConfigPath, 'utf8');
    const email = String(authPayload.email || authPayload.account_email || 'codex-local').trim();
    const apiKey = String(authPayload.OPENAI_API_KEY || authPayload.openai_api_key || '').trim();
    const baseUrl = parseCodexProviderBaseUrl(configRaw);
    const modelProvider = parseSimpleTomlValue(configRaw, 'model_provider');
    if (!apiKey || !baseUrl) return null;
    return {
      id: 'codex-local-provider',
      email,
      source: 'local_codex_provider_config',
      source_path: codexConfigPath,
      method: 'provider_bridge',
      provider: 'codex',
      provider_base_url: baseUrl.replace(/\/+$/, ''),
      provider_name: modelProvider || 'custom',
      access_token: apiKey,
      status: 'active',
      added_at: nowIso(),
      available_models: defaultModels(),
      tier: modelProvider || 'custom',
      metadata: {
        auth_path: codexAuthPath,
      },
    };
  } catch (error) {
    console.warn('[codex] failed to parse provider config:', error.message);
    return null;
  }
}

function readSnapshotAccount() {
  if (!snapshotPath || !fs.existsSync(snapshotPath)) return null;
  try {
    const payload = safeReadJson(snapshotPath);
    const email = String(payload.email || '').trim();
    const accessToken = String(payload.access_token || payload.accessToken || '').trim();
    if (!email || !accessToken) return null;
    return {
      id: `${provider}-snapshot`,
      email,
      source: 'snapshot_file',
      source_path: snapshotPath,
      method: 'snapshot',
      provider,
      access_token: accessToken,
      refresh_token: payload.refresh_token || payload.refreshToken || null,
      status: 'active',
      added_at: nowIso(),
      available_models: Array.isArray(payload.available_models)
        ? payload.available_models
        : defaultModels(),
      tier: payload.membership_type || payload.plan_name || payload.login_provider || 'unknown',
      raw: payload,
    };
  } catch (error) {
    console.warn(`[${provider}] failed to parse snapshot file:`, error.message);
    return null;
  }
}

function providerLocalAccount() {
  if (provider === 'cursor') return readCursorLocalAccount();
  if (provider === 'codex') return readCodexLocalAccount();
  if (provider === 'windsurf') return readWindsurfLocalAccount();
  return readKiroLocalAccount();
}

function normalizeImportedAccount(raw) {
  const email = String(raw.email || '').trim();
  const accessToken = String(
    raw.access_token || raw.accessToken || raw.token || raw.api_key || '',
  ).trim();
  if (!email || !accessToken) {
    throw new Error('missing email or access token');
  }
  const localCodexAccount = provider === 'codex' ? readCodexLocalAccount() : null;
  return {
    id: raw.id || makeId(provider),
    email,
    source: raw.source || 'import_api',
    source_path: null,
    method: raw.method || 'manual',
    provider,
    provider_base_url:
      raw.provider_base_url ||
      raw.base_url ||
      localCodexAccount?.provider_base_url ||
      null,
    access_token: accessToken,
    refresh_token: raw.refresh_token || raw.refreshToken || null,
    expires_at: raw.expires_at || raw.expiresAt || null,
    last_refresh: raw.last_refresh || raw.lastRefresh || nowIso(),
    last_success_at: raw.last_success_at || null,
    last_failure_at: raw.last_failure_at || null,
    failure_count: Number(raw.failure_count || 0),
    status_reason: raw.status_reason || '',
    status: 'active',
    added_at: nowIso(),
    available_models: Array.isArray(raw.available_models)
      ? raw.available_models
      : defaultModels(),
    tier: raw.tier || raw.membership_type || raw.plan_name || 'unknown',
    raw,
  };
}

function resolveProviderAuthStrategy(payload = {}) {
  const requested = String(payload?.auth_strategy || '').trim().toLowerCase();
  const fallback =
    provider === 'codex'
      ? codexAuthStrategy || 'provider_bridge'
      : provider === 'cursor'
      ? cursorAuthStrategy || 'local_state_direct'
      : provider === 'windsurf'
        ? windsurfAuthStrategy || 'local_state_direct'
        : 'local_state_direct';
  const resolved = requested || fallback;
  if (provider === 'codex') {
    switch (resolved) {
      case 'provider_bridge':
      case 'manual_token_import':
        return resolved;
      default:
        return 'provider_bridge';
    }
  }
  if (provider === 'cursor') {
    switch (resolved) {
      case 'manual_token_import':
      case 'oauth_callback':
      case 'local_state_direct':
        return resolved;
      default:
        return 'local_state_direct';
    }
  }
  switch (resolved) {
    case 'manual_token_import':
    case 'local_state_direct':
      return resolved;
    default:
      return 'local_state_direct';
  }
}

function parseManualImportInput(payload = {}) {
  const rawInput = payload?.input;
  if (rawInput && typeof rawInput === 'object' && !Array.isArray(rawInput)) {
    return rawInput;
  }
  const trimmed = String(rawInput || '').trim();
  if (!trimmed) {
    return {};
  }
  const parsed = safeJsonParse(trimmed, null);
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('manual_token_import expects a JSON object payload');
  }
  return parsed;
}

function parseAuthUrlLikeInput(rawInput) {
  const trimmed = String(rawInput || '').trim();
  if (!trimmed) {
    return {};
  }
  try {
    const url = new URL(trimmed);
    const params = new URLSearchParams(url.search);
    const hash = String(url.hash || '').replace(/^#/, '').trim();
    if (hash) {
      const hashParams = new URLSearchParams(hash);
      for (const [key, value] of hashParams.entries()) {
        if (!params.has(key)) {
          params.set(key, value);
        }
      }
    }
    const data = {};
    for (const key of [
      'email',
      'access_token',
      'refresh_token',
      'token',
      'api_key',
      'tier',
      'plan_name',
      'membership_type',
      'expires_at',
      'expiresAt',
      'code',
    ]) {
      const value = String(params.get(key) || '').trim();
      if (value) {
        data[key] = value;
      }
    }
    const availableModels = params.getAll('available_models').filter(Boolean);
    if (availableModels.length > 0) {
      data.available_models = availableModels;
    } else {
      const csvModels = String(params.get('models') || params.get('available_models_csv') || '').trim();
      if (csvModels) {
        data.available_models = csvModels
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean);
      }
    }
    return data;
  } catch (error) {
    return {};
  }
}

function parseOAuthCallbackInput(payload = {}) {
  const rawInput = payload?.input;
  if (rawInput && typeof rawInput === 'object' && !Array.isArray(rawInput)) {
    return rawInput;
  }
  const trimmed = String(rawInput || '').trim();
  if (!trimmed) {
    return {};
  }
  const parsed = safeJsonParse(trimmed, null);
  if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
    return parsed;
  }
  return parseAuthUrlLikeInput(trimmed);
}

function withLifecycleDefaults(account) {
  if (!account || typeof account !== 'object') return account;
  const next = { ...account };
  next.failure_count = Number(next.failure_count || 0);
  next.last_refresh = next.last_refresh || next.lastRefresh || null;
  next.last_success_at = next.last_success_at || null;
  next.last_failure_at = next.last_failure_at || null;
  next.status_reason = next.status_reason || '';
  if (next.expiresAt && !next.expires_at) next.expires_at = next.expiresAt;
  return next;
}

function mergeLifecycleFromPersisted(live, persisted) {
  const next = withLifecycleDefaults(live);
  if (!persisted || typeof persisted !== 'object') return next;
  next.failure_count = Number(persisted.failure_count || next.failure_count || 0);
  next.last_success_at = persisted.last_success_at || next.last_success_at || null;
  next.last_failure_at = persisted.last_failure_at || next.last_failure_at || null;
  next.status_reason = persisted.status_reason || next.status_reason || '';
  next.last_refresh = persisted.last_refresh || next.last_refresh || null;
  next.status = persisted.status || next.status || 'active';
  return next;
}

function upsertAccountLifecycle(accountId, updates) {
  if (!accountId) return;
  const persisted = loadPersistedAccounts();
  const idx = persisted.findIndex((item) => item?.id === accountId);
  if (idx === -1) return;
  persisted[idx] = {
    ...persisted[idx],
    ...updates,
  };
  savePersistedAccounts(persisted);
}

function markAccountRequestOutcome(account, error) {
  if (!account?.id) return;
  const now = nowIso();
  if (!error) {
    upsertAccountLifecycle(account.id, {
      status: 'active',
      status_reason: '',
      last_success_at: now,
      failure_count: 0,
    });
    return;
  }
  const persisted = loadPersistedAccounts();
  const current = persisted.find((item) => item?.id === account.id);
  const prevCount = Number(current?.failure_count || 0);
  upsertAccountLifecycle(account.id, {
    status: 'degraded',
    status_reason: String(error.message || error),
    last_failure_at: now,
    failure_count: prevCount + 1,
  });
}

function mergeAccounts() {
  const persisted = loadPersistedAccounts();
  const persistedMap = new Map(persisted.map((item) => [item.id, withLifecycleDefaults(item)]));
  const liveAccounts = [readSnapshotAccount(), providerLocalAccount()].filter(Boolean);
  const map = new Map();
  for (const account of persisted) {
    map.set(account.id, withLifecycleDefaults(account));
  }
  for (const account of liveAccounts) {
    const merged = mergeLifecycleFromPersisted(account, persistedMap.get(account.id));
    map.set(account.id, merged);
  }
  return [...map.values()].map((item) => withLifecycleDefaults(item));
}

function statusView(accounts) {
  const active = accounts.filter((item) => item.status === 'active').length;
  const models = [...new Set(accounts.flatMap((item) => item.available_models || []))];
  return {
    authenticated: active > 0,
    total: accounts.length,
    active,
    error: accounts.filter((item) => item.status !== 'active').length,
    models,
  };
}

async function verifyPoolAccounts(accounts) {
  const status = statusView(accounts);
  if (!status.authenticated || status.active <= 0) {
    return {
      ok: false,
      status: 'degraded',
      status_reason: status.total === 0 ? 'no_accounts' : 'no_active_accounts',
      models_count: 0,
      models: [],
    };
  }
  try {
    let models = [];
    if (provider === 'kiro') {
      const active = findActiveAccount(accounts);
      models = active ? await fetchKiroModels(active) : [];
    } else if (provider === 'codex') {
      models = await fetchCodexModels(accounts);
    } else if (provider === 'windsurf') {
      const active = findActiveAccount(accounts);
      models = Array.isArray(active?.available_models) ? active.available_models : defaultModels();
    } else if (cursorProviderMode === 'direct') {
      models = await fetchCursorDirectModels(accounts);
    } else {
      models = readCursorAgentModels();
    }
    return {
      ok: Array.isArray(models) && models.length > 0,
      status: Array.isArray(models) && models.length > 0 ? 'ready' : 'degraded',
      status_reason: Array.isArray(models) && models.length > 0 ? '' : 'no_models_available',
      models_count: Array.isArray(models) ? models.length : 0,
      models: Array.isArray(models) ? models : [],
      inference_mode: inferenceMode,
    };
  } catch (error) {
    return {
      ok: false,
      status: 'degraded',
      status_reason: String(error.message || error),
      models_count: 0,
      models: [],
      inference_mode: inferenceMode,
    };
  }
}

function buildAuthStartPayload(strategy) {
  const providerLabel =
    provider === 'cursor'
      ? 'Cursor'
      : provider === 'windsurf'
        ? 'Windsurf'
        : provider === 'codex'
          ? 'Codex'
          : 'Kiro';
  const isLocalStateDirect = strategy === 'local_state_direct';
  if (strategy === 'provider_bridge') {
    return {
      success: true,
      message: 'provider bridge is ready',
      recoverable: true,
      data: {
        auth_strategy: strategy,
        next_action: 'complete_auth',
        authorize_hint: `读取本机 ${providerLabel} 当前 provider 配置并导入当前渠道池。`,
        required_fields: [],
      },
    };
  }
  if (strategy === 'oauth_callback') {
    return {
      success: true,
      message: 'oauth callback adapter is ready',
      recoverable: true,
      data: {
        auth_strategy: strategy,
        next_action: 'open_authorize_then_submit_callback',
        authorize_hint:
          '完成 Cursor 授权后，将浏览器最终跳转的完整 URL 或上游返回的 JSON 结果粘贴回来，适配器会尝试解析并导入账号池。',
        required_fields: ['callback_url_or_json'],
      },
    };
  }
  if (strategy === 'manual_token_import') {
    return {
      success: true,
      message: 'manual token import is ready',
      recoverable: true,
      data: {
        auth_strategy: strategy,
        next_action: 'submit_manual_token_import',
        authorize_hint: '粘贴 JSON 凭据并导入账号池。',
        required_fields: ['email', 'access_token'],
      },
    };
  }
  return {
    success: true,
    message: `local ${provider} state scan is ready`,
    recoverable: true,
    data: {
      auth_strategy: strategy,
      next_action: 'complete_auth',
      authorize_hint: isLocalStateDirect
        ? `读取本机 ${providerLabel} 登录态并导入当前渠道池。`
        : `准备导入 ${providerLabel} 手工凭据。`,
      required_fields: [],
    },
  };
}

function localAuthSourceName() {
  if (provider === 'codex') return 'local_codex_provider_config';
  if (provider === 'cursor') return 'local_cursor_state_vscdb';
  if (provider === 'windsurf') return 'local_windsurf_state_vscdb';
  return 'local_kiro_auth_token';
}

function localAuthProviderLabel() {
  if (provider === 'codex') return 'Codex';
  if (provider === 'cursor') return 'Cursor';
  if (provider === 'windsurf') return 'Windsurf';
  return 'Kiro';
}

async function completeProviderAuth(payload = {}) {
  const authStrategy = resolveProviderAuthStrategy(payload);
  if (authStrategy === 'oauth_callback') {
    let imported;
    try {
      const parsed = parseOAuthCallbackInput(payload);
      imported = normalizeImportedAccount({
        ...parsed,
        method: parsed?.method || 'oauth_callback',
        source: 'cursor_oauth_callback_adapter',
      });
    } catch (error) {
      return {
        success: false,
        message:
          String(error.message || error) ||
          'oauth_callback adapter failed to parse callback payload',
        recoverable: true,
        data: {
          auth_strategy: authStrategy,
          account_source: 'cursor_oauth_callback_adapter',
          imported: false,
          active_count: statusView(mergeAccounts()).active,
          verification: {
            ok: false,
            status: 'degraded',
            status_reason: String(error.message || error),
          },
          status_reason: String(error.message || error),
        },
      };
    }
    const next = loadPersistedAccounts().filter((item) => item.id !== imported.id);
    next.push(imported);
    savePersistedAccounts(next);
    const accounts = mergeAccounts();
    const verification = await verifyPoolAccounts(accounts);
    const activeCount = statusView(accounts).active;
    return {
      success: verification.ok,
      message: verification.ok ? 'OAuth 回调结果已导入并通过最小验池' : 'OAuth 回调结果已导入，但最小验池失败',
      recoverable: !verification.ok,
      data: {
        auth_strategy: authStrategy,
        account_source: 'cursor_oauth_callback_adapter',
        account_email: imported.email,
        imported: true,
        active_count: activeCount,
        verification,
        status_reason: verification.status_reason || '',
      },
    };
  }
  if (authStrategy === 'manual_token_import') {
    let imported;
    try {
      imported = normalizeImportedAccount(parseManualImportInput(payload));
    } catch (error) {
      return {
        success: false,
        message: String(error.message || error),
        recoverable: true,
        data: {
          auth_strategy: authStrategy,
          account_source: 'manual_token_import',
          imported: false,
          active_count: statusView(mergeAccounts()).active,
          verification: {
            ok: false,
            status: 'degraded',
            status_reason: String(error.message || error),
          },
        },
      };
    }
    const next = loadPersistedAccounts().filter((item) => item.id !== imported.id);
    next.push(imported);
    savePersistedAccounts(next);
    const accounts = mergeAccounts();
    const verification = await verifyPoolAccounts(accounts);
    const activeCount = statusView(accounts).active;
    return {
      success: verification.ok,
      message: verification.ok ? '账号已导入并通过最小验池' : '账号已导入，但最小验池失败',
      recoverable: !verification.ok,
      data: {
        auth_strategy: authStrategy,
        account_source: imported.source,
        account_email: imported.email,
        imported: true,
        active_count: activeCount,
        verification,
        status_reason: verification.status_reason || '',
      },
    };
  }

  const localAccount = providerLocalAccount();
  if (!localAccount) {
    const providerLabel = localAuthProviderLabel();
    const sourceName = localAuthSourceName();
    const statusReason =
      provider === 'codex'
        ? 'codex_provider_config_not_found'
        : provider === 'cursor'
        ? 'cursor_local_state_not_found'
        : provider === 'windsurf'
          ? 'windsurf_local_state_not_found'
          : 'kiro_local_state_not_found';
    return {
      success: false,
      message:
        provider === 'codex'
          ? `未发现本机 ${providerLabel} provider 配置，请先确认 ~/.codex/config.toml 与 ~/.codex/auth.json 可用`
          : `未发现本机 ${providerLabel} 登录态，请先在客户端完成登录`,
      recoverable: true,
      data: {
        auth_strategy: authStrategy,
        account_source: sourceName,
        account_email: '',
        imported: false,
        active_count: statusView(mergeAccounts()).active,
        verification: {
          ok: false,
          status: 'degraded',
          status_reason: statusReason,
        },
        status_reason: statusReason,
      },
    };
  }
  const accounts = mergeAccounts();
  const verification = await verifyPoolAccounts(accounts);
  const activeCount = statusView(accounts).active;
  const providerLabel = localAuthProviderLabel();
  return {
    success: verification.ok,
    message: verification.ok
      ? provider === 'codex'
        ? `已读取本机 ${providerLabel} provider 配置并通过最小验池`
        : `已读取本机 ${providerLabel} 登录态并通过最小验池`
      : provider === 'codex'
        ? `已读取本机 ${providerLabel} provider 配置，但最小验池失败`
        : `已读取本机 ${providerLabel} 登录态，但最小验池失败`,
    recoverable: !verification.ok,
    data: {
      auth_strategy: authStrategy,
      account_source: localAccount.source,
      account_email: localAccount.email,
      imported: false,
      active_count: activeCount,
      verification,
      status_reason: verification.status_reason || '',
    },
  };
}

function buildOpenAIResponse(model, text, extra = {}) {
  return {
    id: `resp_${crypto.randomUUID().replace(/-/g, '')}`,
    object: 'response',
    created_at: Math.floor(Date.now() / 1000),
    status: 'completed',
    model,
    output_text: text,
    output: [
      {
        id: `msg_${crypto.randomUUID().replace(/-/g, '')}`,
        type: 'message',
        role: 'assistant',
        content: [
          {
            type: 'output_text',
            text,
            annotations: [],
          },
        ],
      },
    ],
    ...extra,
  };
}

function buildOpenAIChatCompletion(model, text, extra = {}) {
  return {
    id: `chatcmpl_${crypto.randomUUID().replace(/-/g, '')}`,
    object: 'chat.completion',
    created: Math.floor(Date.now() / 1000),
    model,
    choices: [
      {
        index: 0,
        message: {
          role: 'assistant',
          content: text,
        },
        finish_reason: 'stop',
      },
    ],
    usage: null,
    ...extra,
  };
}

function extractTextFromInput(input) {
  if (typeof input === 'string') {
    return input.trim();
  }
  if (Array.isArray(input)) {
    return input
      .map((item) => {
        if (typeof item === 'string') return item;
        if (!item || typeof item !== 'object') return '';
        if (typeof item.text === 'string') return item.text;
        if (Array.isArray(item.content)) return extractTextFromInput(item.content);
        return '';
      })
      .filter(Boolean)
      .join('\n')
      .trim();
  }
  if (input && typeof input === 'object') {
    if (typeof input.text === 'string') return input.text.trim();
    if (typeof input.input_text === 'string') return input.input_text.trim();
    if (typeof input.output_text === 'string') return input.output_text.trim();
    if (Array.isArray(input.content)) return extractTextFromInput(input.content);
  }
  return '';
}

function extractTextFromMessages(messages) {
  if (!Array.isArray(messages)) return '';
  return messages
    .map((msg) => {
      if (!msg || typeof msg !== 'object') return '';
      const content = msg.content;
      if (typeof content === 'string') return content;
      if (Array.isArray(content)) return extractTextFromInput(content);
      return '';
    })
    .filter(Boolean)
    .join('\n')
    .trim();
}

function normalizePrompt(payload) {
  const direct = extractTextFromInput(payload?.input);
  if (direct) return direct;
  const instructions = extractTextFromInput(payload?.instructions);
  if (instructions) return instructions;
  return '';
}

function normalizeChatPrompt(payload) {
  const fromMessages = extractTextFromMessages(payload?.messages);
  if (fromMessages) return fromMessages;
  // Allow client to pass `input` in chat completion for convenience.
  return normalizePrompt(payload);
}

function splitPathList(raw) {
  return String(raw || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

function getValuesByPath(root, pathExpr) {
  const segments = String(pathExpr || '')
    .split('.')
    .map((item) => item.trim())
    .filter(Boolean);
  if (segments.length === 0) return [];
  let current = [root];
  for (const segment of segments) {
    const next = [];
    for (const value of current) {
      if (value == null) continue;
      if (segment === '*') {
        if (Array.isArray(value)) {
          next.push(...value);
        } else if (typeof value === 'object') {
          next.push(...Object.values(value));
        }
        continue;
      }
      if (Array.isArray(value) && /^\d+$/.test(segment)) {
        const idx = Number(segment);
        if (idx < value.length) next.push(value[idx]);
        continue;
      }
      if (typeof value === 'object' && segment in value) {
        next.push(value[segment]);
      }
    }
    current = next;
    if (current.length === 0) break;
  }
  return current;
}

function firstStringByPaths(root, paths) {
  for (const pathExpr of splitPathList(paths)) {
    const values = getValuesByPath(root, pathExpr);
    for (const value of values) {
      if (typeof value === 'string' && value.trim()) return value.trim();
    }
  }
  return '';
}

function stringListByPaths(root, paths) {
  const out = [];
  for (const pathExpr of splitPathList(paths)) {
    const values = getValuesByPath(root, pathExpr);
    for (const value of values) {
      if (typeof value === 'string' && value.trim()) out.push(value.trim());
    }
  }
  return [...new Set(out)];
}

function parseEventStreamPayload(raw) {
  const source = String(raw || '');
  if (!source.includes('\nevent:') && !source.startsWith('event:')) {
    return null;
  }
  const chunks = source
    .split(/\n\s*\n/)
    .map((item) => item.trim())
    .filter(Boolean);
  const deltas = [];
  let response = null;
  let lastMessageText = '';
  for (const chunk of chunks) {
    const lines = chunk.split('\n');
    let eventName = '';
    const dataLines = [];
    for (const line of lines) {
      if (line.startsWith('event:')) {
        eventName = line.slice(6).trim();
      } else if (line.startsWith('data:')) {
        dataLines.push(line.slice(5).trim());
      }
    }
    if (dataLines.length === 0) continue;
    const parsed = safeJsonParse(dataLines.join('\n'), null);
    if (!parsed || typeof parsed !== 'object') continue;
    if (eventName === 'response.output_text.delta' && typeof parsed.delta === 'string') {
      deltas.push(parsed.delta);
    }
    if (eventName === 'response.output_text.done' && typeof parsed.text === 'string') {
      lastMessageText = parsed.text.trim();
    }
    if (eventName === 'response.completed' && parsed.response && typeof parsed.response === 'object') {
      response = parsed.response;
    }
  }
  if (!response && !lastMessageText && deltas.length === 0) {
    return null;
  }
  const text = lastMessageText || deltas.join('').trim();
  if (response && typeof response === 'object') {
    if (!response.output_text && text) {
      response.output_text = text;
    }
    if ((!Array.isArray(response.output) || response.output.length === 0) && text) {
      response.output = buildOpenAIResponse(String(response.model || ''), text).output;
    }
    return response;
  }
  return {
    object: 'response',
    output_text: text,
  };
}

function findActiveAccount(accounts) {
  return accounts.find((item) => item.status === 'active') || null;
}

async function fetchKiroModels(account) {
  const region = account?.region || 'us-east-1';
  const profileArn = account?.profile_arn;
  if (!account?.access_token || !profileArn) {
    return defaultModels();
  }
  const url = `https://q.${region}.amazonaws.com/ListAvailableModels?origin=AI_EDITOR&profileArn=${encodeURIComponent(profileArn)}`;
  const raw = execFileSync(
    'curl',
    ['-sS', '-H', `Authorization: Bearer ${account.access_token}`, url],
    {
      encoding: 'utf8',
      timeout: 20_000,
    },
  ).trim();
  const payload = safeJsonParse(raw, null);
  if (!payload || typeof payload !== 'object') {
    throw new Error('Kiro models upstream returned invalid JSON');
  }
  const models = Array.isArray(payload?.models)
    ? payload.models.map((item) => String(item?.modelId || '').trim()).filter(Boolean)
    : [];
  return models.length > 0 ? [...new Set(models)] : defaultModels();
}

async function invokeKiroResponse(accounts, payload) {
  const account = findActiveAccount(accounts);
  if (!account?.access_token) {
    throw new Error('No active Kiro account found');
  }
  if (!account.profile_arn) {
    throw new Error('Kiro profile ARN is missing');
  }
  const prompt = normalizePrompt(payload);
  if (!prompt) {
    throw new Error('Missing input text');
  }
  const requestedModel = String(payload?.model || '').trim() || 'auto';
  const region = account.region || 'us-east-1';
  const client = new CodeWhispererStreaming({
    region,
    endpoint: `https://q.${region}.amazonaws.com`,
    token: { token: account.access_token },
    customUserAgent: 'KiroLocalPool/0.1',
  });
  client.middlewareStack.add(
    (next) => async (args) => {
      args.request.headers = {
        ...args.request.headers,
        'x-amzn-codewhisperer-optout': 'true',
        'x-amzn-kiro-agent-mode': 'chat',
      };
      return next(args);
    },
    { step: 'build' },
  );
  const command = new GenerateAssistantResponseCommand({
    profileArn: account.profile_arn,
    conversationState: {
      conversationId: crypto.randomUUID(),
      currentMessage: {
        userInputMessage: {
          content: prompt,
          origin: 'IDE',
          modelId: requestedModel,
        },
      },
      history: [],
      chatTriggerType: 'MANUAL',
    },
  });
  try {
    const response = await client.send(command);
    let text = '';
    for await (const event of response.generateAssistantResponseResponse || []) {
      if (event?.assistantResponseEvent?.content) {
        text += event.assistantResponseEvent.content;
      }
    }
    markAccountRequestOutcome(account, null);
    return buildOpenAIResponse(requestedModel, text.trim(), {
      provider: 'kiro',
      usage: null,
    });
  } catch (error) {
    markAccountRequestOutcome(account, error);
    throw error;
  }
}

function readCursorAgentModels() {
  try {
    const raw = execFileSync(path.join(os.homedir(), '.local', 'bin', 'cursor-agent'), ['models'], {
      encoding: 'utf8',
      timeout: 15_000,
    }).trim();
    const models = raw
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line && !/^available models/i.test(line) && !/^no models/i.test(line))
      .map((line) => line.split(/\s+/)[0])
      .filter(Boolean);
    return models.length > 0 ? models : defaultModels();
  } catch {
    return defaultModels();
  }
}

function buildUpstreamUrl(baseUrl, pathName) {
  const normalizedBase = String(baseUrl || '').trim().replace(/\/+$/, '');
  const normalizedPath = pathName.startsWith('/') ? pathName : `/${pathName}`;
  if (!normalizedBase) return normalizedPath;
  if (normalizedBase.endsWith('/v1') && normalizedPath.startsWith('/v1/')) {
    return `${normalizedBase}${normalizedPath.slice(3)}`;
  }
  return `${normalizedBase}${normalizedPath}`;
}

async function requestCodexProvider(account, method, pathName, body) {
  const baseUrl = String(account?.provider_base_url || '').trim();
  const apiKey = String(account?.access_token || '').trim();
  if (!baseUrl) {
    throw new Error('Codex provider base_url is missing');
  }
  if (!apiKey) {
    throw new Error('Codex provider api key is missing');
  }
  const response = await fetch(buildUpstreamUrl(baseUrl, pathName), {
    method,
    headers: {
      Accept: 'application/json',
      Authorization: `Bearer ${apiKey}`,
      ...(body ? { 'Content-Type': 'application/json' } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const raw = await response.text();
  const parsed = safeJsonParse(raw, null) || parseEventStreamPayload(raw);
  if (!response.ok) {
    const message =
      parsed?.error?.message ||
      parsed?.message ||
      `codex provider upstream failed: status=${response.status}`;
    throw new Error(message);
  }
  return parsed || { raw };
}

async function fetchCodexModels(accounts) {
  const account = findActiveAccount(accounts);
  if (!account) return [];
  const payload = await requestCodexProvider(account, 'GET', '/v1/models', null);
  const models = Array.isArray(payload?.data)
    ? payload.data.map((item) => String(item?.id || '').trim()).filter(Boolean)
    : [];
  return models.length > 0 ? [...new Set(models)] : defaultModels();
}

async function invokeCodexResponse(accounts, payload) {
  const account = findActiveAccount(accounts);
  const requestedModel = String(payload?.model || '').trim() || 'gpt-5.5';
  let responsePayload;
  try {
    responsePayload = await requestCodexProvider(account, 'POST', '/v1/responses', payload);
  } catch (error) {
    markAccountRequestOutcome(account, error);
    throw error;
  }
  markAccountRequestOutcome(account, null);
  if (responsePayload && typeof responsePayload === 'object' && responsePayload.object === 'response') {
    return responsePayload;
  }
  const text =
    responsePayload?.output_text ||
    firstStringByPaths(responsePayload, cursorConnectTextPaths) ||
    responsePayload?.text ||
    responsePayload?.content ||
    '';
  return buildOpenAIResponse(requestedModel, String(text || '').trim(), {
    provider: 'codex',
    upstream_passthrough: true,
  });
}

async function requestCursorDirectUpstream(account, method, pathName, body) {
  if (!cursorDirectBaseUrl) {
    throw new Error('CURSOR_DIRECT_BASE_URL is not configured');
  }
  if (!account?.access_token) {
    throw new Error('No active Cursor access token found');
  }
  const authValue = cursorDirectAuthScheme
    ? `${cursorDirectAuthScheme} ${account.access_token}`
    : account.access_token;
  const response = await fetch(`${cursorDirectBaseUrl}${pathName.startsWith('/') ? pathName : `/${pathName}`}`, {
    method,
    headers: {
      Accept: 'application/json',
      [cursorDirectAuthHeader]: authValue,
      ...(body
        ? {
            'Content-Type': 'application/json',
          }
        : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const raw = await response.text();
  const parsed = safeJsonParse(raw, null);
  if (!response.ok) {
    const message =
      parsed?.error?.message ||
      parsed?.message ||
      `cursor direct upstream failed: status=${response.status}`;
    throw new Error(message);
  }
  return parsed || { raw };
}

function ensureCursorConnectPath(pathName, label) {
  if (!pathName) {
    throw new Error(`${label} is not configured`);
  }
  return pathName.startsWith('/') ? pathName : `/${pathName}`;
}

function getCursorClientVersion() {
  try {
    return execFileSync(
      'defaults',
      ['read', '/Applications/Cursor.app/Contents/Info', 'CFBundleShortVersionString'],
      {
        encoding: 'utf8',
        timeout: 5_000,
      },
    ).trim();
  } catch {
    return '';
  }
}

function getCursorGlobalStorageJson() {
  try {
    const file = path.join(
      os.homedir(),
      'Library',
      'Application Support',
      'Cursor',
      'User',
      'globalStorage',
      'storage.json',
    );
    return safeJsonParse(fs.readFileSync(file, 'utf8'), null);
  } catch {
    return null;
  }
}

function getCursorStateValue(key) {
  try {
    const db = openDatabase(cursorDbPath);
    const row = db.prepare('SELECT value FROM itemTable WHERE key = ? LIMIT 1').get(key);
    return row?.value;
  } catch {
    return undefined;
  }
}

function getCursorServerConfigVersion() {
  try {
    const payload = safeJsonParse(getCursorStateValue('cursorai/serverConfig') || '', null);
    return String(payload?.configVersion || '').trim();
  } catch {
    return '';
  }
}

function getCursorClientLayout() {
  const layout = String(getCursorStateValue('cursor/unifiedAppLayout') || '').trim();
  return layout || 'agent';
}

function getCursorPrivacyModeHeader() {
  const raw = getCursorStateValue('cursorai/donotchange/privacyMode');
  if (raw === 'true' || raw === true) return 'true';
  if (raw === 'false' || raw === false) return 'false';
  return 'implicit-false';
}

function getCursorNewOnboardingCompletedHeader() {
  const eligible = String(getCursorStateValue('cursor/layoutControl.eligibleAfterOnboarding') || '').trim();
  const privacy = getCursorPrivacyModeHeader() === 'true';
  return eligible === 'true' && !privacy ? 'true' : 'false';
}

function getCursorRuntimeSessionId() {
  try {
    const dir = path.join(
      os.homedir(),
      'Library',
      'Application Support',
      'Cursor',
      'process-monitor',
    );
    const files = fs
      .readdirSync(dir)
      .filter((name) => name.endsWith('.log'))
      .sort()
      .reverse();
    for (const name of files) {
      const file = path.join(dir, name);
      const firstLine = String(fs.readFileSync(file, 'utf8').split('\n')[0] || '').trim();
      if (!firstLine) continue;
      const payload = safeJsonParse(firstLine, null);
      const sessionId = String(payload?.sessionId || '').trim();
      if (sessionId) return sessionId;
    }
  } catch {}
  return '';
}

function getCursorOsVersion() {
  try {
    const raw = execFileSync('sw_vers', ['-productVersion'], {
      encoding: 'utf8',
      timeout: 5_000,
    }).trim();
    return raw;
  } catch {
    return '';
  }
}

function encodeCursorChecksumPrefix() {
  const x = Math.floor(Date.now() / 1e6);
  const bytes = new Uint8Array([
    (x >> 40) & 255,
    (x >> 32) & 255,
    (x >> 24) & 255,
    (x >> 16) & 255,
    (x >> 8) & 255,
    x & 255,
  ]);
  let state = 165;
  for (let i = 0; i < bytes.length; i += 1) {
    bytes[i] = (bytes[i] ^ state) + (i % 256);
    state = bytes[i];
  }
  return Buffer.from(bytes).toString('base64').replace(/=+$/g, '');
}

function buildCursorConnectMetadataHeaders() {
  const version = getCursorClientVersion();
  const storage = getCursorGlobalStorageJson();
  const machineId = String(storage?.['telemetry.machineId'] || '').trim();
  const macMachineId = String(storage?.['telemetry.macMachineId'] || '').trim();
  const configVersion = getCursorServerConfigVersion();
  const sessionId = getCursorRuntimeSessionId();
  const clientLayout = getCursorClientLayout();
  const osVersion = getCursorOsVersion();
  const requestId = crypto.randomUUID();
  const headers = {
    'x-request-id': requestId,
    'x-amzn-trace-id': `Root=${requestId}`,
    'x-cursor-client-type': 'ide',
    'x-cursor-client-layout': clientLayout,
    'x-cursor-client-os': process.platform,
    'x-cursor-client-arch': process.arch,
    'x-cursor-client-device-type': 'desktop',
    'x-ghost-mode': getCursorPrivacyModeHeader(),
    'x-new-onboarding-completed': getCursorNewOnboardingCompletedHeader(),
  };
  if (version) {
    headers['x-cursor-client-version'] = version;
  }
  if (configVersion) {
    headers['x-cursor-config-version'] = configVersion;
  }
  if (sessionId) {
    headers['x-session-id'] = sessionId;
  }
  if (osVersion) {
    headers['x-cursor-client-os-version'] = osVersion;
  }
  if (machineId) {
    headers['x-cursor-checksum'] = macMachineId
      ? `${encodeCursorChecksumPrefix()}${machineId}/${macMachineId}`
      : `${encodeCursorChecksumPrefix()}${machineId}`;
  }
  try {
    const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (timezone) headers['x-cursor-timezone'] = timezone;
  } catch {}
  return headers;
}

function encodeConnectJsonEnvelope(payload) {
  const body = Buffer.from(JSON.stringify(payload || {}), 'utf8');
  const frame = Buffer.alloc(5 + body.length);
  frame.writeUInt8(0, 0);
  frame.writeUInt32BE(body.length, 1);
  body.copy(frame, 5);
  return frame;
}

function decodeConnectEnvelopes(buffer) {
  const messages = [];
  const trailers = [];
  let offset = 0;
  while (offset + 5 <= buffer.length) {
    const flags = buffer.readUInt8(offset);
    const size = buffer.readUInt32BE(offset + 1);
    const end = offset + 5 + size;
    if (end > buffer.length) break;
    const payloadText = buffer.subarray(offset + 5, end).toString('utf8');
    const payload = safeJsonParse(payloadText, null) || { raw: payloadText };
    const target = (flags & 0x02) === 0x02 ? trailers : messages;
    target.push(payload);
    offset = end;
  }
  return {
    messages,
    trailers,
    rawRemainder: offset < buffer.length ? buffer.subarray(offset).toString('hex') : '',
  };
}

function tryConsumeConnectFrames(buffer) {
  const frames = [];
  let offset = 0;
  while (offset + 5 <= buffer.length) {
    const flags = buffer.readUInt8(offset);
    const size = buffer.readUInt32BE(offset + 1);
    const end = offset + 5 + size;
    if (end > buffer.length) break;
    const payloadText = buffer.subarray(offset + 5, end).toString('utf8');
    frames.push({
      flags,
      payloadText,
      payload: safeJsonParse(payloadText, null) || { raw: payloadText },
    });
    offset = end;
  }
  return {
    frames,
    remainder: offset < buffer.length ? buffer.subarray(offset) : Buffer.alloc(0),
  };
}

function buildCursorConnectRequestContextReply(execServerMessage) {
  const reqArgs = execServerMessage?.requestContextArgs;
  if (!reqArgs || typeof reqArgs !== 'object') return null;
  return {
    execClientMessage: {
      ...(execServerMessage?.id != null ? { id: execServerMessage.id } : {}),
      ...(execServerMessage?.execId ? { execId: execServerMessage.execId } : {}),
      requestContextResult: {
        success: {
          requestContext: {},
        },
      },
    },
  };
}

function formatCursorConnectTrailerError(trailers) {
  for (const payload of trailers || []) {
    const err = payload?.error;
    if (!err || typeof err !== 'object') continue;
    const detail =
      err?.details?.find((item) => item?.debug?.details?.detail)?.debug?.details?.detail ||
      err?.details?.find((item) => item?.debug?.details?.title)?.debug?.details?.title ||
      err?.details?.find((item) => item?.debug?.error)?.debug?.error ||
      '';
    const code = String(err?.code || '').trim();
    const message = String(err?.message || '').trim();
    return [code, message, detail].filter(Boolean).join(': ');
  }
  return '';
}

function collectCursorConnectText(messages) {
  const chunks = [];
  for (const payload of messages || []) {
    const update = payload?.interactionUpdate;
    if (!update || typeof update !== 'object') continue;
    const text =
      String(update?.textDelta?.text || '') ||
      String(update?.thinkingDelta?.text || '') ||
      String(update?.postRequestPrompt?.message || '') ||
      '';
    if (text) chunks.push(text);
  }
  return chunks.join('').trim();
}

function buildCursorConnectPayload(kind, payload) {
  const prompt = kind === 'chat_completions' ? normalizeChatPrompt(payload) : normalizePrompt(payload);
  const model = String(payload?.model || '').trim();
  switch (effectiveCursorConnectPayloadMode) {
    case 'agent_run':
      return {
        runRequest: {
          conversationState: {},
          action: {
            userMessageAction: {
              userMessage: {
                text: prompt,
                messageId: crypto.randomUUID(),
              },
            },
          },
          requestedModel: {
            modelId: model || 'gpt-5-mini',
            maxMode: false,
          },
          conversationId: crypto.randomUUID(),
          clientSupportsInlineImages: false,
        },
      };
    case 'cursor_unified_chat':
      return {
        conversation: [
          {
            text: prompt,
            type: 1,
          },
        ],
        modelDetails: {
          modelName: model || 'default',
        },
        conversationId: crypto.randomUUID(),
        allowModelFallbacks: true,
        shouldCache: false,
        isChat: true,
        unifiedMode: 1,
        useUnifiedChatPrompt: true,
      };
    case 'prompt_model':
      return {
        model,
        prompt,
        stream: false,
      };
    case 'chat_model_messages':
      return {
        model,
        messages: Array.isArray(payload?.messages)
          ? payload.messages
          : [
              {
                role: 'user',
                content: prompt,
              },
            ],
        stream: false,
      };
    case 'passthrough':
    default:
      return {
        ...payload,
        ...(prompt && !payload?.input && kind !== 'chat_completions' ? { input: prompt } : {}),
      };
  }
}

async function requestCursorConnectUpstream(account, pathName, payload) {
  if (!cursorDirectBaseUrl) {
    throw new Error('CURSOR_DIRECT_BASE_URL is not configured');
  }
  if (!account?.access_token) {
    throw new Error('No active Cursor access token found');
  }
  const authValue = cursorDirectAuthScheme
    ? `${cursorDirectAuthScheme} ${account.access_token}`
    : account.access_token;
  const upstreamUrl = new URL(`${cursorDirectBaseUrl}${pathName}`);
  const origin = `${upstreamUrl.protocol}//${upstreamUrl.host}`;
  const requestPath = `${upstreamUrl.pathname}${upstreamUrl.search}`;
  const body = JSON.stringify(payload || {});
  return await new Promise((resolve, reject) => {
    const client = http2.connect(origin);
    let settled = false;
    const timeout = setTimeout(() => {
      finish(new Error(`cursor connect upstream timeout after ${cursorConnectTimeoutMs}ms`));
    }, Math.max(1_000, cursorConnectTimeoutMs));

    function cleanup() {
      clearTimeout(timeout);
      try {
        client.close();
      } catch {}
    }

    function finish(err, value) {
      if (settled) return;
      settled = true;
      cleanup();
      if (err) {
        reject(err);
      } else {
        resolve(value);
      }
    }

    client.on('error', (error) => {
      finish(error);
    });

    const req = client.request({
      ':method': 'POST',
      ':path': requestPath,
      accept: cursorConnectAccept,
      'content-type': cursorConnectContentType,
      'connect-protocol-version': cursorConnectProtocolVersion,
      [String(cursorDirectAuthHeader || 'authorization').toLowerCase()]: authValue,
      ...buildCursorConnectMetadataHeaders(),
      ...Object.fromEntries(
        Object.entries(cursorConnectExtraHeaders).map(([key, value]) => [
          String(key).toLowerCase(),
          String(value),
        ]),
      ),
    });

    let statusCode = 0;
    let raw = '';
    req.setEncoding('utf8');
    req.on('response', (headers) => {
      statusCode = Number(headers[':status'] || 0);
    });
    req.on('data', (chunk) => {
      raw += chunk;
    });
    req.on('error', (error) => {
      finish(error);
    });
    req.on('end', () => {
      const parsed = safeJsonParse(raw, null);
      if (statusCode < 200 || statusCode >= 300) {
        const message =
          parsed?.error?.message ||
          parsed?.message ||
          raw ||
          `cursor connect upstream failed: status=${statusCode || 'unknown'}`;
        finish(new Error(message));
        return;
      }
      finish(null, parsed || { raw });
    });
    req.end(body);
  });
}

async function requestCursorConnectStreamingUpstream(account, pathName, payload) {
  if (!cursorDirectBaseUrl) {
    throw new Error('CURSOR_DIRECT_BASE_URL is not configured');
  }
  if (!account?.access_token) {
    throw new Error('No active Cursor access token found');
  }
  const authValue = cursorDirectAuthScheme
    ? `${cursorDirectAuthScheme} ${account.access_token}`
    : account.access_token;
  const upstreamUrl = new URL(`${cursorDirectBaseUrl}${pathName}`);
  const origin = `${upstreamUrl.protocol}//${upstreamUrl.host}`;
  const requestPath = `${upstreamUrl.pathname}${upstreamUrl.search}`;
  const body = encodeConnectJsonEnvelope(payload);
  return await new Promise((resolve, reject) => {
    const client = http2.connect(origin);
    let settled = false;
    const timeout = setTimeout(() => {
      finish(new Error(`cursor connect upstream timeout after ${cursorConnectTimeoutMs}ms`));
    }, Math.max(1_000, cursorConnectTimeoutMs));

    function cleanup() {
      clearTimeout(timeout);
      try {
        client.close();
      } catch {}
    }

    function finish(err, value) {
      if (settled) return;
      settled = true;
      cleanup();
      if (err) {
        reject(err);
      } else {
        resolve(value);
      }
    }

    client.on('error', (error) => {
      finish(error);
    });

    const req = client.request({
      ':method': 'POST',
      ':path': requestPath,
      accept: 'application/connect+json',
      'content-type': 'application/connect+json',
      'connect-protocol-version': cursorConnectProtocolVersion,
      'x-cursor-streaming': 'true',
      [String(cursorDirectAuthHeader || 'authorization').toLowerCase()]: authValue,
      ...buildCursorConnectMetadataHeaders(),
      ...Object.fromEntries(
        Object.entries(cursorConnectExtraHeaders).map(([key, value]) => [
          String(key).toLowerCase(),
          String(value),
        ]),
      ),
    });

    let statusCode = 0;
    let buffered = Buffer.alloc(0);
    const messages = [];
    const trailers = [];
    let completed = false;
    let requestClosed = false;

    function closeRequestStream() {
      if (requestClosed) return;
      requestClosed = true;
      try {
        req.end();
      } catch {}
    }

    function succeedIfComplete() {
      if (!completed) return false;
      const trailerError = formatCursorConnectTrailerError(trailers);
      if (trailerError) {
        finish(new Error(trailerError));
        return true;
      }
      finish(null, {
        messages,
        trailers,
        output_text: collectCursorConnectText(messages),
        raw_remainder: buffered.length > 0 ? buffered.toString('hex') : undefined,
      });
      return true;
    }

    req.on('response', (headers) => {
      statusCode = Number(headers[':status'] || 0);
    });
    req.on('data', (chunk) => {
      buffered = Buffer.concat([buffered, Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk)]);
      const consumed = tryConsumeConnectFrames(buffered);
      buffered = consumed.remainder;
      for (const frame of consumed.frames) {
        if ((frame.flags & 0x02) === 0x02) {
          trailers.push(frame.payload);
          completed = true;
          continue;
        }
        messages.push(frame.payload);
        const reply = buildCursorConnectRequestContextReply(frame.payload?.execServerMessage);
        if (reply) {
          try {
            if (!requestClosed) {
              req.write(encodeConnectJsonEnvelope(reply));
            }
          } catch (error) {
            finish(error);
            return;
          }
        }
        if (frame.payload?.interactionUpdate?.turnEnded) {
          completed = true;
        }
      }
      if (succeedIfComplete()) {
        closeRequestStream();
      }
    });
    req.on('error', (error) => {
      finish(error);
    });
    req.on('end', () => {
      if (statusCode < 200 || statusCode >= 300) {
        const trailerError = formatCursorConnectTrailerError(trailers);
        finish(
          new Error(
            trailerError || `cursor connect upstream failed: status=${statusCode || 'unknown'}`,
          ),
        );
        return;
      }
      completed = true;
      if (succeedIfComplete()) return;
      const trailerError = formatCursorConnectTrailerError(trailers);
      if (trailerError) {
        finish(new Error(trailerError));
        return;
      }
      finish(null, {
        messages,
        trailers,
        output_text: collectCursorConnectText(messages),
        raw_remainder: buffered.length > 0 ? buffered.toString('hex') : undefined,
      });
    });
    req.write(body);
  });
}

async function fetchCursorDirectModels(accounts) {
  const account = findActiveAccount(accounts);
  if (cursorDirectProtocol === 'connect') {
    const connectPath = ensureCursorConnectPath(
      effectiveCursorConnectModelsPath,
      'CURSOR_CONNECT_MODELS_PATH',
    );
    const payload = await requestCursorConnectUpstream(account, connectPath, {
      includeLongContextModels: false,
      excludeMaxNamedModels: false,
    });
    const models = stringListByPaths(payload, cursorConnectModelPaths);
    return models.length > 0 ? models : defaultModels();
  }
  const payload = await requestCursorDirectUpstream(
    account,
    'GET',
    cursorDirectModelsPath,
    null,
  );
  const models = Array.isArray(payload?.data)
    ? payload.data.map((item) => String(item?.id || '').trim()).filter(Boolean)
    : [];
  return models.length > 0 ? [...new Set(models)] : defaultModels();
}

function invokeCursorCliResponse(payload) {
  const prompt = normalizePrompt(payload);
  if (!prompt) {
    throw new Error('Missing input text');
  }
  const agentPath = path.join(os.homedir(), '.local', 'bin', 'cursor-agent');
  if (!fs.existsSync(agentPath)) {
    throw new Error('cursor-agent is not installed');
  }
  let status = '';
  try {
    status = execFileSync(agentPath, ['status'], {
      encoding: 'utf8',
      timeout: 10_000,
    }).trim();
  } catch (error) {
    status = String(error?.stdout || error?.stderr || '').trim();
  }
  if (/not logged in/i.test(status)) {
    throw new Error('cursor-agent is installed but not logged in');
  }
  const requestedModel = String(payload?.model || '').trim() || 'gpt-5';
  const raw = execFileSync(
    agentPath,
    ['--print', '--output-format', 'json', '--model', requestedModel, '--force', '--trust', prompt],
    {
      encoding: 'utf8',
      timeout: 120_000,
      cwd: process.cwd(),
    },
  ).trim();
  const parsed = safeJsonParse(raw, null);
  const text =
    parsed?.output_text ||
    parsed?.text ||
    parsed?.content ||
    (typeof parsed?.result === 'string' ? parsed.result : '') ||
    raw;
  return buildOpenAIResponse(requestedModel, String(text || '').trim(), {
    provider: 'cursor',
  });
}

async function invokeCursorDirectResponse(accounts, payload) {
  const account = findActiveAccount(accounts);
  const requestedModel = String(payload?.model || '').trim() || 'gpt-5';
  let responsePayload;
  try {
    if (cursorDirectProtocol === 'connect') {
      const connectPath = ensureCursorConnectPath(
        effectiveCursorConnectResponsesPath,
        'CURSOR_CONNECT_RESPONSES_PATH',
      );
      const connectPayload = buildCursorConnectPayload('responses', payload);
      responsePayload =
        effectiveCursorConnectPayloadMode === 'agent_run' ||
        effectiveCursorConnectPayloadMode === 'cursor_unified_chat'
          ? await requestCursorConnectStreamingUpstream(account, connectPath, connectPayload)
          : await requestCursorConnectUpstream(account, connectPath, connectPayload);
    } else {
      responsePayload = await requestCursorDirectUpstream(
        account,
        'POST',
        cursorDirectResponsesPath,
        payload,
      );
    }
  } catch (error) {
    markAccountRequestOutcome(account, error);
    throw error;
  }
  markAccountRequestOutcome(account, null);
  // Prefer upstream native response payload when available.
  if (responsePayload && typeof responsePayload === 'object' && responsePayload.object === 'response') {
    return responsePayload;
  }
  const text =
    firstStringByPaths(responsePayload, cursorConnectTextPaths) ||
    responsePayload?.output_text ||
    responsePayload?.text ||
    responsePayload?.content ||
    (typeof responsePayload?.result === 'string' ? responsePayload.result : '') ||
    '';
  return buildOpenAIResponse(requestedModel, String(text || '').trim(), {
    provider: 'cursor',
    upstream_passthrough: true,
  });
}

async function invokeCursorResponse(accounts, payload) {
  if (cursorProviderMode === 'cli') {
    return invokeCursorCliResponse(payload);
  }
  return invokeCursorDirectResponse(accounts, payload);
}

async function invokeChatCompletion(accounts, payload) {
  const requestedModel = String(payload?.model || '').trim() || 'gpt-5';
  const prompt = normalizeChatPrompt(payload);
  if (!prompt) {
    throw new Error('Missing input text');
  }
  if (provider === 'cursor' && cursorProviderMode !== 'cli' && cursorDirectProtocol === 'connect') {
    const account = findActiveAccount(accounts);
    let responsePayload;
    try {
      const connectPath = ensureCursorConnectPath(
        effectiveCursorConnectChatCompletionsPath || effectiveCursorConnectResponsesPath,
        'CURSOR_CONNECT_CHAT_COMPLETIONS_PATH',
      );
      const connectPayload = buildCursorConnectPayload('chat_completions', payload);
      responsePayload =
        effectiveCursorConnectPayloadMode === 'agent_run' ||
        effectiveCursorConnectPayloadMode === 'cursor_unified_chat'
          ? await requestCursorConnectStreamingUpstream(account, connectPath, connectPayload)
          : await requestCursorConnectUpstream(account, connectPath, connectPayload);
    } catch (error) {
      markAccountRequestOutcome(account, error);
      throw error;
    }
    markAccountRequestOutcome(account, null);
    const text =
      firstStringByPaths(responsePayload, cursorConnectTextPaths) ||
      responsePayload?.output_text ||
      responsePayload?.text ||
      responsePayload?.content ||
      '';
    if (responsePayload?.object === 'chat.completion') {
      return responsePayload;
    }
    return buildOpenAIChatCompletion(requestedModel, String(text || '').trim(), {
      provider,
      upstream_passthrough: true,
    });
  }
  // Use Responses implementation as the single internal execution path.
  if (provider === 'windsurf') {
    throw new Error(
      'Windsurf local_state_direct pool currently supports auth/import only; inference remains on your external Windsurf pool service',
    );
  }
  let resp;
  if (provider === 'kiro') {
    resp = await invokeKiroResponse(accounts, { model: requestedModel, input: prompt });
  } else if (provider === 'codex') {
    const account = findActiveAccount(accounts);
    try {
      resp = await requestCodexProvider(account, 'POST', '/v1/chat/completions', payload);
    } catch (error) {
      markAccountRequestOutcome(account, error);
      throw error;
    }
    markAccountRequestOutcome(account, null);
    const codexChatText = String(resp?.choices?.[0]?.message?.content || '').trim();
    if (!codexChatText) {
      const fallback = await invokeCodexResponse(accounts, {
        model: requestedModel,
        input: prompt,
      });
      return buildOpenAIChatCompletion(
        requestedModel,
        String(fallback?.output_text || '').trim(),
        {
          provider: 'codex',
          upstream_passthrough: true,
          usage: resp?.usage || fallback?.usage || null,
        },
      );
    }
  } else {
    resp = await invokeCursorResponse(accounts, { model: requestedModel, input: prompt });
  }
  const text = String(resp?.output_text || '').trim();
  if (provider === 'codex' && resp?.object === 'chat.completion') {
    return resp;
  }
  return buildOpenAIChatCompletion(
    requestedModel,
    text ||
      String(resp?.choices?.[0]?.message?.content || '').trim(),
    {
      provider,
      upstream_passthrough: resp?.upstream_passthrough || provider === 'codex',
    },
  );
}

function renderLoginPage(errorMessage) {
  return `<!doctype html>
<html lang="zh-CN">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${provider} pool login</title></head>
<body style="font-family:-apple-system,BlinkMacSystemFont,sans-serif;background:#0f1115;color:#f5f7fa;display:flex;align-items:center;justify-content:center;min-height:100vh">
  <form method="POST" action="/dashboard/login" style="width:360px;background:#171a21;border:1px solid #2a3040;border-radius:14px;padding:24px;display:flex;flex-direction:column;gap:12px">
    <h2 style="margin:0">${provider} 控制台登录</h2>
    <div style="color:#9aa4b2;font-size:14px">请输入管理密码以继续</div>
    ${errorMessage ? `<div style="color:#ff7b72;font-size:13px">${errorMessage}</div>` : ''}
    <input type="password" name="password" placeholder="dashboard password" style="padding:10px 12px;border-radius:8px;border:1px solid #394150;background:#0f1115;color:#f5f7fa" />
    <button type="submit" style="padding:10px 12px;border:0;border-radius:8px;background:#5865f2;color:white;cursor:pointer">登录</button>
  </form>
</body></html>`;
}

function renderDashboard(accounts) {
  const status = statusView(accounts);
  const rows = accounts
    .map((account) => {
      const models = (account.available_models || []).join(', ') || '-';
      return `<tr>
        <td>${account.id}</td>
        <td>${account.email}</td>
        <td>${account.method || '-'}</td>
        <td>${account.tier || '-'}</td>
        <td>${account.status}</td>
        <td>${models}</td>
      </tr>`;
    })
    .join('');
  return `<!doctype html>
<html lang="zh-CN">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${provider} local pool</title></head>
<body style="font-family:-apple-system,BlinkMacSystemFont,sans-serif;background:#0f1115;color:#f5f7fa;padding:24px">
  <h1 style="margin-top:0">${provider} local pool</h1>
  <p style="color:#9aa4b2">最小本地池服务。当前实现了状态、账号、模型视图、手工导入，以及 provider-specific 的最小推理代理。</p>
  <div style="display:flex;gap:16px;margin:18px 0">
    <div>authenticated: <b>${String(status.authenticated)}</b></div>
    <div>active: <b>${status.active}</b></div>
    <div>total: <b>${status.total}</b></div>
  </div>
  <table style="width:100%;border-collapse:collapse">
    <thead><tr><th align="left">ID</th><th align="left">Email</th><th align="left">Method</th><th align="left">Tier</th><th align="left">Status</th><th align="left">Models</th></tr></thead>
    <tbody>${rows}</tbody>
  </table>
</body></html>`;
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, `http://127.0.0.1:${port}`);
  const accounts = mergeAccounts();

  if (req.method === 'OPTIONS') {
    res.writeHead(204, {
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
      'Access-Control-Allow-Headers': 'Content-Type, Authorization, x-api-key',
    });
    res.end();
    return;
  }

  if (url.pathname === '/healthz') {
    json(res, 200, { ok: true, provider, port });
    return;
  }

  if (url.pathname === '/dashboard/login' && req.method === 'GET') {
    html(res, 200, renderLoginPage(''));
    return;
  }

  if (url.pathname === '/dashboard/login' && req.method === 'POST') {
    const raw = await readBody(req);
    const params = new URLSearchParams(raw);
    const password = String(params.get('password') || '').trim();
    if (dashboardPassword && password !== dashboardPassword) {
      html(res, 401, renderLoginPage('密码错误'));
      return;
    }
    res.writeHead(302, {
      'Set-Cookie': `${provider}_dashboard=${dashboardPassword}; Path=/; HttpOnly`,
      Location: '/dashboard',
    });
    res.end();
    return;
  }

  if (url.pathname === '/dashboard' && req.method === 'GET') {
    if (!ensureDashboardAuth(req, res)) return;
    html(res, 200, renderDashboard(accounts));
    return;
  }

  if (url.pathname === '/auth/start' && req.method === 'POST') {
    if (!ensureApiAuth(req, res)) return;
    const payload = parseRequestJson(await readBody(req));
    json(res, 200, buildAuthStartPayload(resolveProviderAuthStrategy(payload)));
    return;
  }

  if (url.pathname === '/auth/complete' && req.method === 'POST') {
    if (!ensureApiAuth(req, res)) return;
    const payload = parseRequestJson(await readBody(req));
    json(res, 200, await completeProviderAuth(payload));
    return;
  }

  if (url.pathname === '/auth/status' && req.method === 'GET') {
    if (!ensureApiAuth(req, res)) return;
    json(res, 200, statusView(accounts));
    return;
  }

  if (url.pathname === '/auth/accounts' && req.method === 'GET') {
    if (!ensureApiAuth(req, res)) return;
    json(res, 200, {
      accounts: accounts.map((item) => ({
        id: item.id,
        email: item.email,
        method: item.method,
        status: item.status,
        addedAt: item.added_at,
        tier: item.tier,
        availableModels: item.available_models || [],
        source: item.source,
        sourcePath: item.source_path,
        expiresAt: item.expires_at || null,
        lastRefresh: item.last_refresh || null,
        lastSuccessAt: item.last_success_at || null,
        lastFailureAt: item.last_failure_at || null,
        failureCount: Number(item.failure_count || 0),
        statusReason: item.status_reason || '',
      })),
    });
    return;
  }

  if (url.pathname === '/auth/login' && req.method === 'POST') {
    if (!ensureApiAuth(req, res)) return;
    let payload = {};
    try {
      payload = JSON.parse(await readBody(req));
    } catch {
      json(res, 400, { error: 'Invalid JSON' });
      return;
    }
    try {
      const imported = normalizeImportedAccount(payload);
      const next = loadPersistedAccounts().filter((item) => item.id !== imported.id);
      next.push(imported);
      savePersistedAccounts(next);
      json(res, 200, {
        success: true,
        account: {
          id: imported.id,
          email: imported.email,
          status: imported.status,
          availableModels: imported.available_models || [],
        },
        ...statusView(mergeAccounts()),
      });
    } catch (error) {
      json(res, 400, { error: String(error.message || error) });
    }
    return;
  }

  if (url.pathname === '/v1/models' && req.method === 'GET') {
    if (!ensureApiAuth(req, res)) return;
    try {
      const active = findActiveAccount(accounts);
      const models =
        provider === 'kiro'
          ? (active ? await fetchKiroModels(active) : [])
          : provider === 'codex'
            ? (active ? await fetchCodexModels(accounts) : [])
          : provider === 'windsurf'
            ? (active ? active.available_models || defaultModels() : [])
          : cursorProviderMode === 'direct'
            ? (active ? await fetchCursorDirectModels(accounts) : [])
            : readCursorAgentModels();
      json(res, 200, {
        object: 'list',
        data: [...new Set(models)].map((id) => ({
          id,
          object: 'model',
          created: Math.floor(Date.now() / 1000),
          owned_by: provider,
        })),
      });
    } catch (error) {
      json(res, 200, {
        object: 'list',
        data: [],
        warning: String(error.message || error),
      });
    }
    return;
  }

  if (url.pathname === '/v1/responses' && req.method === 'POST') {
    if (!ensureApiAuth(req, res)) return;
    if (inferenceMode !== 'responses' && inferenceMode !== 'dual') {
      json(res, 404, {
        error: {
          message: 'Responses API is disabled by INFERENCE_MODE',
          type: 'invalid_request_error',
        },
      });
      return;
    }
    let payload = {};
    try {
      payload = JSON.parse(await readBody(req));
    } catch {
      json(res, 400, {
        error: {
          message: 'Invalid JSON',
          type: 'invalid_request_error',
        },
      });
      return;
    }
    try {
      const response =
        provider === 'kiro'
          ? await invokeKiroResponse(accounts, payload)
          : provider === 'codex'
            ? await invokeCodexResponse(accounts, payload)
          : provider === 'windsurf'
            ? (() => {
                throw new Error('Windsurf local_state_direct pool currently supports auth/import only; inference remains on your external Windsurf pool service');
              })()
          : await invokeCursorResponse(accounts, payload);
      json(res, 200, response);
    } catch (error) {
      json(res, 503, {
        error: {
          message: String(error.message || error),
          type: 'upstream_error',
        },
      });
    }
    return;
  }

  if (url.pathname === '/v1/chat/completions' && req.method === 'POST') {
    if (!ensureApiAuth(req, res)) return;
    if (inferenceMode !== 'chat_completions' && inferenceMode !== 'dual') {
      json(res, 404, {
        error: {
          message: 'Chat Completions API is disabled by INFERENCE_MODE',
          type: 'invalid_request_error',
        },
      });
      return;
    }
    let payload = {};
    try {
      payload = JSON.parse(await readBody(req));
    } catch {
      json(res, 400, {
        error: {
          message: 'Invalid JSON',
          type: 'invalid_request_error',
        },
      });
      return;
    }
    try {
      const response = await invokeChatCompletion(accounts, payload);
      json(res, 200, response);
    } catch (error) {
      json(res, 503, {
        error: {
          message: String(error.message || error),
          type: 'upstream_error',
        },
      });
    }
    return;
  }

  json(res, 404, {
    error: {
      message: `Invalid URL (${req.method} ${url.pathname})`,
      type: 'invalid_request_error',
    },
  });
});

server.listen(port, '127.0.0.1', () => {
  ensureDir();
  console.log(
    JSON.stringify({
      provider,
      port,
      dataFile,
      snapshotPath: snapshotPath || null,
      sourcePaths: getSourcePaths(),
      codexAuthStrategy: provider === 'codex' ? codexAuthStrategy : undefined,
      cursorProviderMode: provider === 'cursor' ? cursorProviderMode : undefined,
      windsurfAuthStrategy: provider === 'windsurf' ? windsurfAuthStrategy : undefined,
      cursorDirectProtocol: provider === 'cursor' ? cursorDirectProtocol : undefined,
      cursorDirectBaseUrl: provider === 'cursor' ? cursorDirectBaseUrl || null : undefined,
      inferenceMode,
      cursorDirectChatCompletionsPath: provider === 'cursor' ? cursorDirectChatCompletionsPath : undefined,
      cursorConnectPayloadMode:
        provider === 'cursor' ? effectiveCursorConnectPayloadMode : undefined,
      cursorConnectModelsPath:
        provider === 'cursor' ? effectiveCursorConnectModelsPath || null : undefined,
      cursorConnectResponsesPath:
        provider === 'cursor' ? effectiveCursorConnectResponsesPath || null : undefined,
      cursorConnectChatCompletionsPath:
        provider === 'cursor' ? effectiveCursorConnectChatCompletionsPath || null : undefined,
    }),
  );
});
