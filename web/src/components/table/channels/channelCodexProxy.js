/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

export const parseChannelOtherInfo = (record) => {
  if (!record?.other_info) {
    return {};
  }
  if (typeof record.other_info === 'object') {
    return record.other_info || {};
  }
  try {
    const parsed = JSON.parse(record.other_info);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch (error) {
    return {};
  }
};

export const isRemoteCodexPoolProxy = (record) => {
  if (!record) {
    return false;
  }
  const otherInfo = parseChannelOtherInfo(record);
  if (otherInfo.remote_codex_pool_proxy === true) {
    return true;
  }
  const name = String(record.name || '')
    .trim()
    .toLowerCase();
  const baseUrl = String(record.base_url || '')
    .trim()
    .toLowerCase();
  return (
    name === 'codex-e2e-temp' &&
    (baseUrl.includes('127.0.0.1:18080') || baseUrl.includes('localhost:18080'))
  );
};

export const isWindsurfPoolProxy = (record) => {
  if (!record) {
    return false;
  }
  const otherInfo = parseChannelOtherInfo(record);
  return otherInfo.windsurf_pool_proxy === true;
};

export const isCursorPoolProxy = (record) => {
  if (!record) {
    return false;
  }
  const otherInfo = parseChannelOtherInfo(record);
  return otherInfo.cursor_pool_proxy === true;
};

export const isKiroPoolProxy = (record) => {
  if (!record) {
    return false;
  }
  const otherInfo = parseChannelOtherInfo(record);
  return otherInfo.kiro_pool_proxy === true;
};

export const isCodexPoolProxy = (record) => {
  if (!record) {
    return false;
  }
  const otherInfo = parseChannelOtherInfo(record);
  return otherInfo.codex_pool_proxy === true;
};

export const isCodexStatusCapableChannel = (record) =>
  Number(record?.type) === 57 || isRemoteCodexPoolProxy(record);

export const getChannelAdminStatusKind = (record) => {
  if (isCodexPoolProxy(record)) {
    return 'codex_pool';
  }
  if (isCodexStatusCapableChannel(record)) {
    return 'codex';
  }
  if (isCursorPoolProxy(record)) {
    return 'cursor';
  }
  if (isKiroPoolProxy(record)) {
    return 'kiro';
  }
  if (isWindsurfPoolProxy(record)) {
    return 'windsurf';
  }
  return null;
};

export const isAdminStatusCapableChannel = (record) =>
  getChannelAdminStatusKind(record) !== null;

export const getChannelAdminStatusActionText = (record, t) => {
  const adminStatusKind = getChannelAdminStatusKind(record);
  if (adminStatusKind === 'codex_pool') {
    return {
      tooltip: t('查看 Codex 帐号信息与池状态'),
      label: t('池状态'),
    };
  }
  if (adminStatusKind === 'codex') {
    return {
      tooltip: t('查看 Codex 帐号信息与用量'),
      label: t('帐号信息'),
    };
  }
  if (adminStatusKind === 'cursor') {
    return {
      tooltip: t('查看 Cursor 帐号信息与池状态'),
      label: t('池状态'),
    };
  }
  if (adminStatusKind === 'kiro') {
    return {
      tooltip: t('查看 Kiro 帐号信息与池状态'),
      label: t('池状态'),
    };
  }
  if (adminStatusKind === 'windsurf') {
    return {
      tooltip: t('查看 Windsurf 帐号信息与池状态'),
      label: t('池状态'),
    };
  }
  return null;
};

export const getExternalPoolAuthConfig = (record) => {
  const kind = getChannelAdminStatusKind(record);
  const poolKind = kind === 'codex_pool' ? 'codex' : kind;
  if (!poolKind || kind === 'codex') {
    return {
      kind: poolKind,
      authorizeUrl: '',
      dashboardUrl: '',
      authStartPath: '',
      authCompletePath: '',
      configured: false,
    };
  }
  const otherInfo = parseChannelOtherInfo(record);
  const authorizeUrl =
    String(otherInfo?.[`${poolKind}_pool_authorize_url`] || '').trim();
  const dashboardPath =
    String(otherInfo?.[`${poolKind}_pool_dashboard_path`] || '').trim();
  const baseUrl = String(record?.base_url || '').trim().replace(/\/+$/, '');
  const dashboardUrl = dashboardPath
    ? /^https?:\/\//.test(dashboardPath)
      ? dashboardPath
      : `${baseUrl}${dashboardPath.startsWith('/') ? dashboardPath : `/${dashboardPath}`}`
    : '';
  const authStartPath =
    String(otherInfo?.[`${poolKind}_pool_auth_start_path`] || '/auth/start').trim();
  const authCompletePath =
    String(otherInfo?.[`${poolKind}_pool_auth_complete_path`] || '/auth/complete').trim();
  const configured = Boolean(
    authorizeUrl ||
      dashboardUrl ||
      (authStartPath && authCompletePath && baseUrl),
  );
  return {
    kind,
    authorizeUrl,
    dashboardUrl,
    authStartPath,
    authCompletePath,
    configured,
  };
};

export const getExternalPoolSummary = (record) => {
  const kind = getChannelAdminStatusKind(record);
  if (kind === 'codex_pool') return record?.codex_pool_summary || null;
  if (kind === 'cursor') return record?.cursor_pool_summary || null;
  if (kind === 'kiro') return record?.kiro_pool_summary || null;
  if (kind === 'windsurf') return record?.windsurf_pool_summary || null;
  return null;
};

export const getExternalPoolAvailabilityRank = (record) => {
  const summary = getExternalPoolSummary(record);
  if (!summary) return 99;

  const availability = String(summary.availability || '').trim().toLowerCase();
  const diagnosis = String(summary.diagnosis || '').trim().toLowerCase();

  switch (availability) {
    case 'unavailable':
      switch (diagnosis) {
        case 'sidecar_expired':
          return 0;
        case 'sidecar_unactivated':
          return 1;
        case 'bridge_unreachable':
          return 2;
        case 'auth_rejected':
          return 3;
        case 'upstream_path_not_found':
          return 4;
        case 'upstream_unreachable':
          return 5;
        case 'upstream_server_error':
          return 6;
        case 'empty_pool':
        case 'no_active_accounts':
          return 7;
        default:
          return 8;
      }
    case 'degraded':
      if (diagnosis === 'rate_limited') return 9;
      if (diagnosis === 'provider_not_ready') return 10;
      return 11;
    case 'available':
      return 12;
    default:
      return 13;
  }
};

export const hasExternalPoolIssue = (record) =>
  getExternalPoolAvailabilityRank(record) < 8;

export const matchExternalPoolQuickFilter = (record, filter) => {
  const normalized = String(filter || '').trim().toLowerCase();
  if (!normalized || normalized === 'all') {
    return true;
  }

  const summary = getExternalPoolSummary(record);
  const availability = String(summary?.availability || '').trim().toLowerCase();
  const diagnosis = String(summary?.diagnosis || '').trim().toLowerCase();

  switch (normalized) {
    case 'available':
      return availability === 'available';
    case 'unavailable':
      return availability === 'unavailable';
    case 'degraded':
      return availability === 'degraded';
    case 'auth_rejected':
      return diagnosis === 'auth_rejected';
    case 'empty_pool':
      return diagnosis === 'empty_pool' || diagnosis === 'no_active_accounts';
    case 'upstream_unreachable':
      return diagnosis === 'upstream_unreachable' || diagnosis === 'bridge_unreachable';
    case 'upstream_path_not_found':
      return diagnosis === 'upstream_path_not_found';
    case 'rate_limited':
      return diagnosis === 'rate_limited';
    default:
      return true;
  }
};
