import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import ReactDOM from 'react-dom/client';
import {
  Button,
  Collapse,
  Descriptions,
  Empty,
  Modal,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError } from '../../../../helpers';
import { getChannelAdminStatusKind } from '../channelCodexProxy';
import { openExternalPoolAuthModal } from './ExternalPoolAuthModal';

const { Text } = Typography;

const providerMetaMap = {
  cursor: {
    label: 'Cursor',
    statusPath: 'cursor/pool_status',
    accountsPath: 'cursor/accounts',
    authPath: 'cursor/auth_view',
    title: 'Cursor 帐号/池状态',
    emptyTitle: '暂无账号',
    emptyDescription: '当前 Cursor 池里还没有可展示的账号。',
    statusError: '读取 Cursor 池状态失败',
    accountsError: '读取 Cursor 账号列表失败',
    usageHint: '先确认 Cursor 外部池服务在线，再访问 Dashboard 或执行补池操作。',
  },
  kiro: {
    label: 'Kiro',
    statusPath: 'kiro/pool_status',
    accountsPath: 'kiro/accounts',
    authPath: 'kiro/auth_view',
    title: 'Kiro 帐号/池状态',
    emptyTitle: '暂无账号',
    emptyDescription: '当前 Kiro 池里还没有可展示的账号。',
    statusError: '读取 Kiro 池状态失败',
    accountsError: '读取 Kiro 账号列表失败',
    usageHint: '先确认 Kiro 外部池服务在线，再访问 Dashboard 或执行补池操作。',
  },
  windsurf: {
    label: 'Windsurf',
    statusPath: 'windsurf/pool_status',
    accountsPath: 'windsurf/accounts',
    authPath: 'windsurf/auth_view',
    title: 'Windsurf 帐号/池状态',
    emptyTitle: '暂无账号',
    emptyDescription: '当前 Windsurf 池里还没有可展示的账号。',
    statusError: '读取 Windsurf 池状态失败',
    accountsError: '读取 Windsurf 账号列表失败',
    usageHint: '先建立到池服务的安全通道，再访问 Dashboard 或执行补池操作。',
  },
};

const getProviderMeta = (record) => {
  const kind = getChannelAdminStatusKind(record) || 'windsurf';
  return {
    kind,
    ...(providerMetaMap[kind] || providerMetaMap.windsurf),
  };
};

const getDisplayText = (value) => {
  if (value == null) return '';
  return String(value).trim();
};

const formatTimeText = (value) => {
  if (!value) return '-';
  try {
    return new Date(value).toLocaleString();
  } catch (error) {
    return String(value);
  }
};

const getModelList = (statusData, accountsData) => {
  if (Array.isArray(statusData?.models)) return statusData.models;
  if (Array.isArray(accountsData?.models)) return accountsData.models;
  if (Array.isArray(statusData?.status?.models)) return statusData.status.models;
  return [];
};

const isValidHttpUrl = (value) => /^https?:\/\//i.test(getDisplayText(value));

const inferPoolDiagnosis = (statusData, accountsData, t) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const diagnosis = String(statusData?.diagnosis || accountsData?.diagnosis || '')
    .trim()
    .toLowerCase();
  const availability = String(statusData?.availability || accountsData?.availability || '')
    .trim()
    .toLowerCase();
  if (diagnosis === 'auth_only') {
    return {
      color: 'yellow',
      title: tt('优先排查'),
      text: tt('认证通过，但当前推理接口不可用。优先检查渠道 inference mode 与池服务开放路径是否一致。'),
    };
  }
  if (diagnosis === 'unprobed' || availability === 'unprobed') {
    return {
      color: 'grey',
      title: tt('当前判断'),
      text: tt('推理探测未开启，当前仅能确认认证状态，尚无法判断推理可用性。'),
    };
  }
  const state = String(statusData?.pool_state || accountsData?.pool_state || '')
    .trim()
    .toLowerCase();
  const upstreamError = getDisplayText(
    statusData?.upstream_error || accountsData?.upstream_error,
  ).toLowerCase();

  if (state === 'ready') {
    return {
      color: 'green',
      title: tt('当前判断'),
      text: tt('池状态正常，可以继续做拉模型和 /v1/responses 验证。'),
    };
  }

  if (state === 'empty_pool') {
    return {
      color: 'orange',
      title: tt('优先排查'),
      text: tt('上游已连通，但池里还没有可用账号。先回上游补池，不必优先怀疑 new-api 配置。'),
    };
  }

  if (state === 'degraded') {
    return {
      color: 'yellow',
      title: tt('优先排查'),
      text: tt('池已连通但存在异常账号。先看错误账号数、限流状态和模型支持列表，再决定是否继续灰度。'),
    };
  }

  if (state === 'upstream_error') {
    if (
      upstreamError.includes('401') ||
      upstreamError.includes('403') ||
      upstreamError.includes('unauthorized') ||
      upstreamError.includes('forbidden')
    ) {
      return {
        color: 'red',
        title: tt('优先排查'),
        text: tt('更像是认证问题。先检查 key、认证 Header、认证 Scheme，以及上游是否允许当前凭据访问状态接口。'),
      };
    }
    if (upstreamError.includes('404') || upstreamError.includes('not found')) {
      return {
        color: 'red',
        title: tt('优先排查'),
        text: tt('更像是路径配置错误。先检查状态接口路径、账号列表路径和 Dashboard 路径是否与上游实现一致。'),
      };
    }
    if (
      upstreamError.includes('connection refused') ||
      upstreamError.includes('timeout') ||
      upstreamError.includes('deadline exceeded') ||
      upstreamError.includes('no such host') ||
      upstreamError.includes('eof')
    ) {
      return {
        color: 'red',
        title: tt('优先排查'),
        text: tt('更像是连通性问题。先检查 base_url、池服务是否在线，以及 new-api 到池服务之间的网络。'),
      };
    }
    return {
      color: 'red',
      title: tt('优先排查'),
      text: tt('上游状态接口未连通。先检查 base_url、key、路径配置和网络，再回头看具体错误文案。'),
    };
  }

  return {
    color: 'grey',
    title: tt('当前判断'),
    text: tt('状态信息还不够完整，建议先刷新一次，再结合原始 JSON 和上游错误继续排查。'),
  };
};

const StatusTag = ({ status }) => {
  const normalized = String(status || '').trim().toLowerCase();
  if (normalized === 'active' || normalized === 'healthy' || normalized === 'ok') {
    return <Tag color='green'>{status || 'active'}</Tag>;
  }
  if (normalized === 'error' || normalized === 'failed') {
    return <Tag color='red'>{status || 'error'}</Tag>;
  }
  if (normalized === 'disabled') {
    return <Tag color='grey'>{status || 'disabled'}</Tag>;
  }
  return <Tag color='light-blue'>{status || '-'}</Tag>;
};

const PoolStateTag = ({ state, t }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  switch (String(state || '').trim().toLowerCase()) {
    case 'ready':
      return <Tag color='green'>{tt('可承接请求')}</Tag>;
    case 'empty_pool':
      return <Tag color='orange'>{tt('上游已连通但空池')}</Tag>;
    case 'degraded':
      return <Tag color='yellow'>{tt('上游已连通但降级')}</Tag>;
    case 'upstream_error':
      return <Tag color='red'>{tt('上游未连通')}</Tag>;
    default:
      return <Tag color='grey'>{tt('状态未知')}</Tag>;
  }
};

const buildRawText = (statusPayload, accountsPayload) =>
  JSON.stringify(
    {
      pool_status: statusPayload,
      accounts: accountsPayload,
    },
    null,
    2,
  );

const ExternalPoolLoader = ({ t, record, onCopy }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const [loading, setLoading] = useState(true);
  const [statusPayload, setStatusPayload] = useState(null);
  const [accountsPayload, setAccountsPayload] = useState(null);
  const [authPayload, setAuthPayload] = useState(null);
  const mountedRef = useRef(true);
  const recordId = record?.id;
  const providerMeta = useMemo(() => getProviderMeta(record), [record]);

  const fetchData = useCallback(async () => {
    if (!recordId) {
      return;
    }
    if (mountedRef.current) setLoading(true);
    try {
      const [statusRes, accountsRes, authRes] = await Promise.all([
        API.get(`/api/channel/${recordId}/${providerMeta.statusPath}`, {
          skipErrorHandler: true,
        }),
        API.get(`/api/channel/${recordId}/${providerMeta.accountsPath}`, {
          skipErrorHandler: true,
        }),
        API.get(`/api/channel/${recordId}/${providerMeta.authPath}`, {
          skipErrorHandler: true,
        }),
      ]);

      if (!mountedRef.current) return;
      setStatusPayload(statusRes?.data ?? null);
      setAccountsPayload(accountsRes?.data ?? null);
      setAuthPayload(authRes?.data ?? null);

      if (statusRes?.data?.success === false) {
        showError(getDisplayText(statusRes?.data?.message) || tt(providerMeta.statusError));
      } else if (accountsRes?.data?.success === false) {
        showError(getDisplayText(accountsRes?.data?.message) || tt(providerMeta.accountsError));
      } else if (authRes?.data?.success === false) {
        showError(getDisplayText(authRes?.data?.message) || tt('读取授权入口失败'));
      }
    } catch (error) {
      if (!mountedRef.current) return;
      showError(
        getDisplayText(error?.response?.data?.message) ||
          error?.message ||
          tt(providerMeta.statusError),
      );
      setStatusPayload({
        success: false,
        message: String(error),
      });
      setAccountsPayload({
        success: false,
        message: String(error),
      });
      setAuthPayload({
        success: false,
        message: String(error),
      });
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  }, [
    providerMeta.authPath,
    providerMeta.accountsError,
    providerMeta.accountsPath,
    providerMeta.statusError,
    providerMeta.statusPath,
    recordId,
    tt,
  ]);

  useEffect(() => {
    mountedRef.current = true;
    fetchData().catch(() => {});
    return () => {
      mountedRef.current = false;
    };
  }, [fetchData]);

  const statusData = statusPayload?.data ?? {};
  const accountsData = accountsPayload?.data ?? {};
  const authData = authPayload?.data ?? {};
  const accounts = Array.isArray(accountsData?.accounts) ? accountsData.accounts : [];
  const statusModels = getModelList(statusData, accountsData)
    .map((model) => getDisplayText(model))
    .filter(Boolean);
  const statusModelsPreview = statusModels.slice(0, 10);
  const omittedModelCount = statusModels.length - statusModelsPreview.length;
  const diagnosis = inferPoolDiagnosis(statusData, accountsData, tt);
  const authCapable = statusData?.auth_capable ?? statusData?.status?.authenticated ?? statusData?.authenticated ?? false;
  const inferenceProbed = statusData?.inference_probed === true;
  const inferenceCapable = statusData?.inference_capable === true;
  const inferenceError = getDisplayText(statusData?.inference_error);
  const dashboardUrl = getDisplayText(statusData?.dashboard_url);
  const canOpenDashboard = isValidHttpUrl(dashboardUrl);
  const authorizeUrl = getDisplayText(
    authData?.authorize_url || authData?.dashboard_url,
  );
  const canOpenAuthorize = isValidHttpUrl(authorizeUrl);
  const authorizeHint = getDisplayText(authData?.authorize_hint);
  const rawText = useMemo(
    () => buildRawText(statusPayload, accountsPayload),
    [statusPayload, accountsPayload],
  );

  const columns = [
    {
      title: tt('账号'),
      dataIndex: 'email',
      render: (value, row) =>
        getDisplayText(value) ||
        getDisplayText(row?.display_name) ||
        getDisplayText(row?.id) ||
        '-',
    },
    {
      title: tt('状态'),
      dataIndex: 'status',
      render: (value) => <StatusTag status={value} />,
    },
    {
      title: tt('方式'),
      dataIndex: 'method',
      render: (value) => getDisplayText(value) || '-',
    },
    {
      title: tt('层级'),
      dataIndex: 'tier',
      render: (value) => getDisplayText(value) || '-',
    },
    {
      title: tt('限流'),
      dataIndex: 'rate_limited',
      render: (value) =>
        value ? <Tag color='orange'>{tt('限流中')}</Tag> : <Tag color='grey'>{tt('正常')}</Tag>,
    },
    {
      title: tt('加入时间'),
      dataIndex: 'added_at',
      render: (value) => formatTimeText(value),
    },
    {
      title: tt('可用模型'),
      dataIndex: 'available_models',
      render: (value) => (Array.isArray(value) ? value.length : 0),
    },
  ];

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex items-center justify-between'>
        <Space>
          <Tag color={statusData?.connection_ok === false ? 'red' : 'light-blue'}>
            {tt(`${providerMeta.label} 池`)}
          </Tag>
          <Text type='secondary'>{record?.name || '-'}</Text>
        </Space>
        <Space>
          <Button
            size='small'
            theme='outline'
            disabled={!canOpenAuthorize && !record?.id}
            onClick={() => {
              if (record?.id) {
                openExternalPoolAuthModal({
                  record,
                  onCompleted: () => fetchData(),
                });
                return;
              }
              if (!canOpenAuthorize) return;
              window.open(authorizeUrl, '_blank', 'noopener');
            }}
          >
            {tt('授权登录')}
          </Button>
          <Button
            size='small'
            theme='outline'
            disabled={!canOpenDashboard}
            onClick={() => {
              if (!canOpenDashboard) return;
              window.open(dashboardUrl, '_blank', 'noopener');
            }}
          >
            {tt('打开 Dashboard')}
          </Button>
          <Button size='small' theme='outline' onClick={() => onCopy?.(rawText)}>
            {tt('复制原始 JSON')}
          </Button>
          <Button size='small' type='primary' theme='outline' onClick={() => fetchData()}>
            {tt('刷新')}
          </Button>
        </Space>
      </div>

      {loading ? (
        <div className='flex justify-center py-10'>
          <Spin size='large' />
        </div>
      ) : (
        <>
          <Descriptions
            column={2}
            dataSource={[
              {
                key: 'pool_state',
                label: tt('池状态'),
                value: <PoolStateTag t={tt} state={statusData?.pool_state || accountsData?.pool_state} />,
              },
              {
                key: 'auth_capable',
                label: tt('认证能力'),
                value: authCapable ? <Tag color='green'>{tt('已认证')}</Tag> : <Tag color='grey'>{tt('未认证')}</Tag>,
              },
              {
                key: 'inference_capable',
                label: tt('推理能力'),
                value:
                  inferenceProbed
                    ? inferenceCapable
                      ? <Tag color='green'>{tt('可推理')}</Tag>
                      : <Tag color='orange'>{tt('不可推理')}</Tag>
                    : <Tag color='grey'>{tt('未探测')}</Tag>,
              },
              {
                key: 'active',
                label: tt('活跃账号'),
                value: accountsData?.active ?? statusData?.status?.active ?? 0,
              },
              {
                key: 'total',
                label: tt('总账号'),
                value: accountsData?.total ?? statusData?.status?.total ?? 0,
              },
              {
                key: 'error',
                label: tt('异常账号'),
                value: accountsData?.error ?? statusData?.status?.error ?? 0,
              },
              {
                key: 'fetched',
                label: tt('最近刷新'),
                value: formatTimeText(accountsData?.last_fetched_at || statusData?.last_fetched_at),
              },
              {
                key: 'base_url',
                label: tt('池地址'),
                value: getDisplayText(statusData?.base_url || accountsData?.base_url) || '-',
              },
              {
                key: 'dashboard',
                label: tt('Dashboard'),
                value: getDisplayText(statusData?.dashboard_url) || '-',
              },
              {
                key: 'authorize',
                label: tt('授权入口'),
                value: authorizeUrl || '-',
              },
            ]}
          />

          <div className='rounded-lg border border-solid border-semi-color-border bg-semi-color-fill-0 p-3 text-sm'>
            <div className='font-medium mb-1'>{tt('使用提示')}</div>
            <div>{tt(providerMeta.usageHint)}</div>
            {authorizeHint && (
              <div className='mt-2 text-semi-color-text-1'>
                {tt('授权提示')}: {authorizeHint}
              </div>
            )}
            {getDisplayText(statusData?.upstream_error || accountsData?.upstream_error) && (
              <div className='mt-2 text-red-500'>
                {tt('上游错误')}: {getDisplayText(statusData?.upstream_error || accountsData?.upstream_error)}
              </div>
            )}
            {inferenceError && (
              <div className='mt-2 text-red-500'>
                {tt('推理错误')}: {inferenceError}
              </div>
            )}
            {diagnosis?.text && (
              <div
                className={`mt-3 rounded border border-solid px-3 py-2 ${
                  diagnosis.color === 'green'
                    ? 'border-green-200 bg-green-50 text-green-700'
                    : diagnosis.color === 'orange'
                      ? 'border-orange-200 bg-orange-50 text-orange-700'
                      : diagnosis.color === 'yellow'
                        ? 'border-yellow-200 bg-yellow-50 text-yellow-700'
                        : diagnosis.color === 'red'
                          ? 'border-red-200 bg-red-50 text-red-700'
                          : 'border-semi-color-border bg-semi-color-bg-1 text-semi-color-text-1'
                }`}
              >
                <div className='font-medium mb-1'>{diagnosis.title}</div>
                <div>{diagnosis.text}</div>
              </div>
            )}
            {statusModels.length > 0 && (
              <div className='mt-3'>
                <div className='mb-2 font-medium'>{tt('状态接口模型')}</div>
                <div className='flex flex-wrap gap-2'>
                  {statusModelsPreview.map((model) => (
                    <Tag key={model} color='cyan'>
                      {model}
                    </Tag>
                  ))}
                  {omittedModelCount > 0 && (
                    <Tag color='grey'>{tt('还有 {{count}} 个', { count: omittedModelCount })}</Tag>
                  )}
                </div>
              </div>
            )}
          </div>

          {accounts.length > 0 ? (
            <Table
              size='small'
              pagination={false}
              columns={columns}
              dataSource={accounts.map((item, index) => ({ key: item.id || index, ...item }))}
            />
          ) : (
            <Empty
              title={tt(providerMeta.emptyTitle)}
              description={tt(providerMeta.emptyDescription)}
            />
          )}

          <Collapse>
            <Collapse.Panel header={tt('联调顺序')} itemKey='validation-sequence'>
              <div className='flex flex-col gap-2 text-sm'>
                <div>1. {tt('先看状态列和池状态标签，确认是池可用 / 空池 / 降级 / 断连哪一种')}</div>
                <div>2. {tt('若状态正常，先执行拉取上游模型，再看状态接口模型是否符合预期')}</div>
                <div>3. {tt('再验证 /v1/models 和 /v1/responses，不要一上来直接提权')}</div>
                <div>4. {tt('最后回看请求日志里的 admin_info.external_pool_*')}</div>
              </div>
            </Collapse.Panel>
            <Collapse.Panel header={tt('原始 JSON')} itemKey='raw-json'>
              <pre className='max-h-[40vh] overflow-y-auto rounded-lg bg-semi-color-fill-0 p-3 text-xs text-semi-color-text-0'>
                {rawText}
              </pre>
            </Collapse.Panel>
          </Collapse>
        </>
      )}
    </div>
  );
};

const ExternalPoolModalDialog = ({ t, record, onCopy, visible, onClose }) => (
  <Modal
    title={t(getProviderMeta(record).title)}
    visible={visible}
    onCancel={onClose}
    footer={null}
    width={960}
    centered
  >
    <ExternalPoolLoader t={t} record={record} onCopy={onCopy} />
  </Modal>
);

export const openExternalPoolModal = ({ t, record, onCopy }) => {
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
      <ExternalPoolModalDialog
        t={t}
        record={record}
        onCopy={onCopy}
        visible={false}
        onClose={cleanup}
      />,
    );
    setTimeout(cleanup, 150);
  };

  root.render(
    <ExternalPoolModalDialog
      t={t}
      record={record}
      onCopy={onCopy}
      visible
      onClose={handleClose}
    />,
  );
};

export const openWindsurfPoolModal = (props) => openExternalPoolModal(props);

export default function WindsurfPoolModal(props) {
  return <ExternalPoolModalDialog {...props} />;
}
