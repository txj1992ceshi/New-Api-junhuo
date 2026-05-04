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

export const isCodexStatusCapableChannel = (record) =>
  Number(record?.type) === 57 || isRemoteCodexPoolProxy(record);

export const getChannelAdminStatusKind = (record) => {
  if (isCodexStatusCapableChannel(record)) {
    return 'codex';
  }
  if (isWindsurfPoolProxy(record)) {
    return 'windsurf';
  }
  return null;
};

export const isAdminStatusCapableChannel = (record) =>
  getChannelAdminStatusKind(record) !== null;
