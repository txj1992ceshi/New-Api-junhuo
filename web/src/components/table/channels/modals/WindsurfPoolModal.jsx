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

const { Text } = Typography;

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

const StatusTag = ({ status }) => {
  const normalized = String(status || '').trim().toLowerCase();
  if (normalized === 'active') {
    return <Tag color='green'>{status || 'active'}</Tag>;
  }
  if (normalized === 'error') {
    return <Tag color='red'>{status || 'error'}</Tag>;
  }
  if (normalized === 'disabled') {
    return <Tag color='grey'>{status || 'disabled'}</Tag>;
  }
  return <Tag color='light-blue'>{status || '-'}</Tag>;
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

const WindsurfPoolLoader = ({ t, record, onCopy }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const [loading, setLoading] = useState(true);
  const [statusPayload, setStatusPayload] = useState(null);
  const [accountsPayload, setAccountsPayload] = useState(null);
  const mountedRef = useRef(true);
  const recordId = record?.id;

  const fetchData = useCallback(async () => {
    if (!recordId) {
      return;
    }
    if (mountedRef.current) setLoading(true);
    try {
      const [statusRes, accountsRes] = await Promise.all([
        API.get(`/api/channel/${recordId}/windsurf/pool_status`, {
          skipErrorHandler: true,
        }),
        API.get(`/api/channel/${recordId}/windsurf/accounts`, {
          skipErrorHandler: true,
        }),
      ]);

      if (!mountedRef.current) return;
      setStatusPayload(statusRes?.data ?? null);
      setAccountsPayload(accountsRes?.data ?? null);

      if (statusRes?.data?.success === false) {
        showError(getDisplayText(statusRes?.data?.message) || tt('读取 Windsurf 池状态失败'));
      } else if (accountsRes?.data?.success === false) {
        showError(getDisplayText(accountsRes?.data?.message) || tt('读取 Windsurf 账号列表失败'));
      }
    } catch (error) {
      if (!mountedRef.current) return;
      showError(
        getDisplayText(error?.response?.data?.message) ||
          error?.message ||
          tt('读取 Windsurf 池状态失败'),
      );
      setStatusPayload({
        success: false,
        message: String(error),
      });
      setAccountsPayload({
        success: false,
        message: String(error),
      });
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  }, [recordId, tt]);

  useEffect(() => {
    mountedRef.current = true;
    fetchData().catch(() => {});
    return () => {
      mountedRef.current = false;
    };
  }, [fetchData]);

  const statusData = statusPayload?.data ?? {};
  const accountsData = accountsPayload?.data ?? {};
  const accounts = Array.isArray(accountsData?.accounts)
    ? accountsData.accounts
    : [];
  const rawText = useMemo(
    () => buildRawText(statusPayload, accountsPayload),
    [statusPayload, accountsPayload],
  );

  const columns = [
    {
      title: tt('账号'),
      dataIndex: 'email',
      render: (value, row) => getDisplayText(value) || getDisplayText(row?.id) || '-',
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
      title: tt('Blocked Models'),
      dataIndex: 'blocked_models',
      render: (value) => (Array.isArray(value) ? value.length : 0),
    },
  ];

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex items-center justify-between'>
        <Space>
          <Tag color={statusData?.connection_ok === false ? 'red' : 'light-blue'}>
            {tt('Windsurf 池')}
          </Tag>
          <Text type='secondary'>{record?.name || '-'}</Text>
        </Space>
        <Space>
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
          <Descriptions column={2} dataSource={[
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
              value: getDisplayText(statusData?.dashboard_url) || 'http://127.0.0.1:3303/dashboard',
            },
          ]} />

          <div className='rounded-lg border border-solid border-semi-color-border bg-semi-color-fill-0 p-3 text-sm'>
            <div className='font-medium mb-1'>{tt('使用提示')}</div>
            <div>{tt('先在本机开 SSH 隧道，再访问 127.0.0.1:3303/dashboard 或运行本地补池脚本。')}</div>
            {getDisplayText(statusData?.upstream_error || accountsData?.upstream_error) && (
              <div className='mt-2 text-red-500'>
                {tt('上游错误')}: {getDisplayText(statusData?.upstream_error || accountsData?.upstream_error)}
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
              title={tt('暂无账号')}
              description={tt('当前 Windsurf 池里还没有可展示的账号。')}
            />
          )}

          <Collapse>
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

const WindsurfPoolModalDialog = ({ t, record, onCopy, visible, onClose }) => (
  <Modal
    title={t('Windsurf 帐号/池状态')}
    visible={visible}
    onCancel={onClose}
    footer={null}
    width={960}
    centered
  >
    <WindsurfPoolLoader t={t} record={record} onCopy={onCopy} />
  </Modal>
);

export const openWindsurfPoolModal = ({ t, record, onCopy }) => {
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
      <WindsurfPoolModalDialog
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
    <WindsurfPoolModalDialog
      t={t}
      record={record}
      onCopy={onCopy}
      visible
      onClose={handleClose}
    />,
  );
};
