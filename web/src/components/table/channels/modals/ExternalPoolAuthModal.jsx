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

import React, { useEffect, useMemo, useState } from 'react';
import ReactDOM from 'react-dom/client';
import { useTranslation } from 'react-i18next';
import { Banner, Button, Input, Modal, Space, Typography } from '@douyinfe/semi-ui';
import { API, copy, showError, showSuccess } from '../../../../helpers';
import { getChannelAdminStatusKind, parseChannelOtherInfo } from '../channelCodexProxy';

const { Text } = Typography;

const providerMetaMap = {
  cursor: {
    label: 'Cursor',
    startPath: 'cursor/auth/start',
    completePath: 'cursor/auth/complete',
  },
  kiro: {
    label: 'Kiro',
    startPath: 'kiro/auth/start',
    completePath: 'kiro/auth/complete',
  },
  windsurf: {
    label: 'Windsurf',
    startPath: 'windsurf/auth/start',
    completePath: 'windsurf/auth/complete',
  },
};

const getProviderMeta = (record) => {
  const kind = getChannelAdminStatusKind(record) || 'windsurf';
  return {
    kind,
    ...(providerMetaMap[kind] || providerMetaMap.windsurf),
  };
};

const getAuthorizeUrlFromPayload = (payload) =>
  payload?.authorize_url || payload?.data?.authorize_url || '';

const getMessageFromPayload = (payload) =>
  payload?.message || payload?.data?.message || '';

const getAuthDataFromPayload = (payload) => payload?.data || {};

const getExternalPoolBaseUrl = (record, kind) => {
  const otherInfo = parseChannelOtherInfo(record);
  const fromOtherInfo = String(otherInfo?.[`${kind}_pool_base_url`] || '').trim();
  if (fromOtherInfo) return fromOtherInfo.replace(/\/+$/, '');
  return String(record?.base_url || '').trim().replace(/\/+$/, '');
};

const getConfiguredAuthorizeUrl = (record, kind) => {
  const otherInfo = parseChannelOtherInfo(record);
  return String(
    otherInfo?.[`${kind}_pool_authorize_url`] ||
      otherInfo?.[`${kind}_pool_dashboard_url`] ||
      otherInfo?.authorize_url ||
      otherInfo?.dashboard_url ||
      '',
  ).trim();
};

const getFriendlyPoolErrorMessage = (error, fallback, poolBaseUrl, providerLabel) => {
  const rawMessage = String(error?.message || fallback || '').trim();
  const lower = rawMessage.toLowerCase();
  if (
    lower.includes('connect refused') ||
    lower.includes('econnrefused') ||
    lower.includes('failed to fetch')
  ) {
    return `${providerLabel} pool service 未启动或不可达：${poolBaseUrl || '未配置 base_url'}`;
  }
  if (lower.includes('unauthorized') || lower.includes('403') || lower.includes('401')) {
    return `${providerLabel} pool service 鉴权失败，请检查渠道密钥或上游授权状态`;
  }
  if (lower.includes('404')) {
    return `${providerLabel} pool service 接口路径不存在，请检查 /auth/start 或 /auth/complete 配置`;
  }
  return rawMessage || fallback;
};

const ExternalPoolAuthModalDialog = ({
  visible,
  onClose,
  record,
  onCompleted,
  forcedAuthStrategy = '',
  modeLabel = '',
  primaryActionLabel = '',
  completeActionLabel = '',
}) => {
  const { t } = useTranslation();
  const providerMeta = useMemo(() => getProviderMeta(record), [record]);
  const [loading, setLoading] = useState(false);
  const [authorizeUrl, setAuthorizeUrl] = useState('');
  const [input, setInput] = useState('');
  const [authorizeHint, setAuthorizeHint] = useState('');
  const [authStrategy, setAuthStrategy] = useState('');
  const [requiredFields, setRequiredFields] = useState([]);
  const [nextAction, setNextAction] = useState('');
  const [actionError, setActionError] = useState('');

  const channelId = record?.id;
  const poolBaseUrl = useMemo(() => getExternalPoolBaseUrl(record, providerMeta.kind), [record, providerMeta.kind]);
  const configuredAuthorizeUrl = useMemo(
    () => getConfiguredAuthorizeUrl(record, providerMeta.kind),
    [record, providerMeta.kind],
  );
  const effectiveAuthStrategy = String(forcedAuthStrategy || authStrategy || '').trim().toLowerCase();
  const isLocalStateDirect = effectiveAuthStrategy === 'local_state_direct';
  const isManualImport = effectiveAuthStrategy === 'manual_token_import';
  const isCursorOAuthCallback =
    providerMeta.kind === 'cursor' && effectiveAuthStrategy === 'oauth_callback';

  const fetchAuthView = async () => {
    if (!channelId) return;
    try {
      const authViewUrl = forcedAuthStrategy
        ? `/api/channel/${channelId}/${providerMeta.kind}/auth_view?auth_strategy=${encodeURIComponent(forcedAuthStrategy)}`
        : `/api/channel/${channelId}/${providerMeta.kind}/auth_view`;
      const res = await API.get(authViewUrl, { skipErrorHandler: true });
      const payload = res?.data || {};
      if (payload?.success && payload?.data) {
        setAuthorizeUrl(
          payload.data.authorize_url || payload.data.dashboard_url || configuredAuthorizeUrl,
        );
        setAuthorizeHint(payload.data.authorize_hint || '');
        setAuthStrategy(payload.data.auth_strategy || '');
        setActionError('');
      }
    } catch (error) {
      if (configuredAuthorizeUrl) {
        setAuthorizeUrl((prev) => prev || configuredAuthorizeUrl);
      }
      setActionError(
        getFriendlyPoolErrorMessage(
          error,
          t('加载授权视图失败'),
          poolBaseUrl,
          providerMeta.label,
        ),
      );
    }
  };

  const startAuth = async () => {
    if (!channelId) return;
    if (isCursorOAuthCallback && authorizeUrl) {
      const popup = window.open(authorizeUrl, '_blank', 'noopener,noreferrer');
      if (popup) {
        setActionError('');
        showSuccess(t('已打开授权页面'));
      } else {
        const copied = await copy(authorizeUrl);
        if (copied) {
          showError(t('浏览器拦截了弹窗，已复制授权链接'));
        } else {
          showError(t('浏览器拦截了弹窗，请手动打开授权入口'));
        }
      }
      return;
    }
    setLoading(true);
    try {
      const res = await API.post(
        `/api/channel/${channelId}/${providerMeta.startPath}`,
        forcedAuthStrategy ? { auth_strategy: forcedAuthStrategy } : {},
        { skipErrorHandler: true },
      );
      const payload = res?.data || {};
      if (payload?.success === false) {
        throw new Error(getMessageFromPayload(payload) || t('启动授权失败'));
      }
      const authData = getAuthDataFromPayload(payload);
      const nextAuthorizeUrl = getAuthorizeUrlFromPayload(payload) || authorizeUrl;
      setAuthStrategy(authData?.auth_strategy || authStrategy);
      setAuthorizeHint(authData?.authorize_hint || authorizeHint);
      setRequiredFields(Array.isArray(authData?.required_fields) ? authData.required_fields : []);
      setNextAction(authData?.next_action || '');
      setActionError('');
      if (nextAuthorizeUrl) {
        setAuthorizeUrl(nextAuthorizeUrl);
        window.open(nextAuthorizeUrl, '_blank', 'noopener,noreferrer');
        showSuccess(getMessageFromPayload(payload) || t('已打开授权页面'));
        return;
      }
      showSuccess(getMessageFromPayload(payload) || t('授权准备完成'));
    } catch (error) {
      const message = getFriendlyPoolErrorMessage(
        error,
        t('启动授权失败'),
        poolBaseUrl,
        providerMeta.label,
      );
      setActionError(message);
      showError(message);
    } finally {
      setLoading(false);
    }
  };

  const completeAuth = async () => {
    if (!channelId) return;
    if (!isLocalStateDirect && (!input || !input.trim())) {
      showError(t('请先粘贴回调 URL 或授权结果'));
      return;
    }
    setLoading(true);
    try {
      const res = await API.post(
        `/api/channel/${channelId}/${providerMeta.completePath}`,
        forcedAuthStrategy ? { input, auth_strategy: forcedAuthStrategy } : { input },
        { skipErrorHandler: true },
      );
      const payload = res?.data || {};
      if (payload?.success === false) {
        throw new Error(getMessageFromPayload(payload) || t('授权失败'));
      }
      const verification = payload?.data?.verification;
      const successMessage =
        getMessageFromPayload(payload) ||
        (verification?.ok ? t('授权完成并通过验池') : t('授权结果已提交'));
      setActionError('');
      showSuccess(successMessage);
      onCompleted && onCompleted(payload?.data || payload);
      onClose && onClose();
    } catch (error) {
      const message = getFriendlyPoolErrorMessage(
        error,
        t('授权失败'),
        poolBaseUrl,
        providerMeta.label,
      );
      setActionError(message);
      showError(message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible) return;
    setInput('');
    setAuthorizeUrl(configuredAuthorizeUrl);
    setAuthorizeHint('');
    setAuthStrategy('');
    setRequiredFields([]);
    setNextAction('');
    setActionError('');
    fetchAuthView().catch(() => {});
  }, [visible, channelId, providerMeta.kind, forcedAuthStrategy, poolBaseUrl, configuredAuthorizeUrl, t]);

  return (
    <Modal
      title={t(modeLabel || `${providerMeta.label} 授权登录`)}
      visible={visible}
      onCancel={onClose}
      maskClosable={false}
      closeOnEsc
      width={720}
      footer={
        <Space>
          <Button theme='borderless' onClick={onClose} disabled={loading}>
            {t('取消')}
          </Button>
          <Button theme='solid' type='primary' onClick={completeAuth} loading={loading}>
            {t(
              completeActionLabel ||
                (isLocalStateDirect ? '确认读取登录态' : '提交授权结果'),
            )}
          </Button>
        </Space>
      }
    >
      <Space vertical spacing='tight' style={{ width: '100%' }}>
        {actionError ? <Banner type='danger' description={actionError} /> : null}
        <Banner
          type='info'
          description={
            authorizeHint ||
            (isLocalStateDirect
              ? t(`点击下方按钮后，系统会重新扫描本机 ${providerMeta.label} 登录态并尝试直接导入当前渠道池，然后立即做一次最小验池。`)
              : isCursorOAuthCallback
                ? t(
                    '点击下方按钮后，先完成 Cursor 授权；再把浏览器最终跳转的完整 URL 或上游返回的 JSON 结果粘贴回来，适配器会尝试解析并导入当前渠道池。',
                  )
              : t(
                  '1) 点击「打开授权页面」完成手动授权；2) 浏览器跳转后的完整 URL、授权码或上游返回结果粘贴到下方；3) 点击「提交授权结果」。',
                ))
          }
        />

        <Text type='tertiary' size='small'>
          {t('池地址')}：{poolBaseUrl || t('未配置')}
        </Text>
        {authorizeUrl ? (
          <Text type='tertiary' size='small'>
            {t('授权入口')}：{authorizeUrl}
          </Text>
        ) : null}

        <Space wrap>
          <Button type='primary' onClick={startAuth} loading={loading}>
            {t(
              primaryActionLabel ||
                (isLocalStateDirect ? '开始读取登录态' : '打开授权页面'),
            )}
          </Button>
          <Button
            theme='outline'
            disabled={!authorizeUrl || loading}
            onClick={() => copy(authorizeUrl)}
          >
            {t('复制授权链接')}
          </Button>
        </Space>

        {!isLocalStateDirect && (
          <Input.TextArea
            value={input}
            onChange={(value) => setInput(value)}
            placeholder={
              isManualImport
                ? t('请粘贴 JSON 凭据，例如 {"email":"...","access_token":"..."}')
                : t('请粘贴完整回调 URL、授权码或上游返回结果')
            }
            autosize={{ minRows: 3, maxRows: 8 }}
            showClear
          />
        )}

        {isManualImport && requiredFields.length > 0 && (
          <Text type='secondary' size='small'>
            {t('必填字段')}：{requiredFields.join(', ')}
          </Text>
        )}

        {nextAction && (
          <Text type='tertiary' size='small'>
            next_action: {nextAction}
          </Text>
        )}

        <Text type='tertiary' size='small'>
          {t(
            isLocalStateDirect
              ? `说明：这里的“读取登录态”本质是读取本机 ${providerMeta.label} 登录态并入池，不是标准 OAuth 网页回调。`
              : isCursorOAuthCallback
                ? '说明：这里的“授权登录”走的是 Cursor 专用回调适配层，会把回调 URL 或 JSON 结果解析后再入池。'
                : '说明：new-api 只负责把授权流程转发给外部池服务，账号入池仍由各自池服务完成。',
          )}
        </Text>
      </Space>
    </Modal>
  );
};

export const openExternalPoolAuthModal = ({
  record,
  onCompleted,
  forcedAuthStrategy = '',
  modeLabel = '',
  primaryActionLabel = '',
  completeActionLabel = '',
}) => {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = ReactDOM.createRoot(container);

  const cleanup = () => {
    root.unmount();
    if (container.parentNode) {
      container.parentNode.removeChild(container);
    }
  };

  const handleClose = () => {
    root.render(
      <ExternalPoolAuthModalDialog
        visible={false}
        onClose={cleanup}
        record={record}
        onCompleted={onCompleted}
        forcedAuthStrategy={forcedAuthStrategy}
        modeLabel={modeLabel}
        primaryActionLabel={primaryActionLabel}
        completeActionLabel={completeActionLabel}
      />,
    );
    setTimeout(cleanup, 150);
  };

  root.render(
    <ExternalPoolAuthModalDialog
      visible
      onClose={handleClose}
      record={record}
      onCompleted={onCompleted}
      forcedAuthStrategy={forcedAuthStrategy}
      modeLabel={modeLabel}
      primaryActionLabel={primaryActionLabel}
      completeActionLabel={completeActionLabel}
    />,
  );
};

export default ExternalPoolAuthModalDialog;
