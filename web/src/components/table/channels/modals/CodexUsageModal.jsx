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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import ReactDOM from 'react-dom/client';
import {
  Button,
  Progress,
  Typography,
  Spin,
  Tag,
  Descriptions,
  Collapse,
  Space,
} from '@douyinfe/semi-ui';
import { API, showError } from '../../../../helpers';

const { Text } = Typography;

const clampPercent = (value) => {
  const v = Number(value);
  if (!Number.isFinite(v)) return 0;
  return Math.max(0, Math.min(100, v));
};

const pickStrokeColor = (percent) => {
  const p = clampPercent(percent);
  if (p >= 95) return '#ef4444';
  if (p >= 80) return '#f59e0b';
  return '#3b82f6';
};

const normalizePlanType = (value) => {
  if (value == null) return '';
  return String(value).trim().toLowerCase();
};

const getWindowDurationSeconds = (windowData) => {
  const value = Number(windowData?.limit_window_seconds);
  if (!Number.isFinite(value) || value <= 0) return null;
  return value;
};

const classifyWindowByDuration = (windowData) => {
  const seconds = getWindowDurationSeconds(windowData);
  if (seconds == null) return null;
  return seconds >= 24 * 60 * 60 ? 'weekly' : 'fiveHour';
};

const resolveRateLimitWindows = (data) => {
  const rateLimit = data?.rate_limit ?? {};
  const primary = rateLimit?.primary_window ?? null;
  const secondary = rateLimit?.secondary_window ?? null;
  const windows = [primary, secondary].filter(Boolean);
  const planType = normalizePlanType(data?.plan_type ?? rateLimit?.plan_type);

  let fiveHourWindow = null;
  let weeklyWindow = null;

  for (const windowData of windows) {
    const bucket = classifyWindowByDuration(windowData);
    if (bucket === 'fiveHour' && !fiveHourWindow) {
      fiveHourWindow = windowData;
      continue;
    }
    if (bucket === 'weekly' && !weeklyWindow) {
      weeklyWindow = windowData;
    }
  }

  if (planType === 'free') {
    if (!weeklyWindow) {
      weeklyWindow = primary ?? secondary ?? null;
    }
    return { fiveHourWindow: null, weeklyWindow };
  }

  if (!fiveHourWindow && !weeklyWindow) {
    return {
      fiveHourWindow: primary ?? null,
      weeklyWindow: secondary ?? null,
    };
  }

  if (!fiveHourWindow) {
    fiveHourWindow =
      windows.find((windowData) => windowData !== weeklyWindow) ?? null;
  }
  if (!weeklyWindow) {
    weeklyWindow =
      windows.find((windowData) => windowData !== fiveHourWindow) ?? null;
  }

  return { fiveHourWindow, weeklyWindow };
};

const formatDurationSeconds = (seconds, t) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const s = Number(seconds);
  if (!Number.isFinite(s) || s <= 0) return '-';
  const total = Math.floor(s);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  if (hours > 0) return `${hours}${tt('小时')} ${minutes}${tt('分钟')}`;
  if (minutes > 0) return `${minutes}${tt('分钟')} ${secs}${tt('秒')}`;
  return `${secs}${tt('秒')}`;
};

const formatUnixSeconds = (unixSeconds) => {
  const v = Number(unixSeconds);
  if (!Number.isFinite(v) || v <= 0) return '-';
  try {
    return new Date(v * 1000).toLocaleString();
  } catch (error) {
    return String(unixSeconds);
  }
};

const getDisplayText = (value) => {
  if (value == null) return '';
  return String(value).trim();
};

const formatNumber = (value) => {
  const num = Number(value);
  if (!Number.isFinite(num)) return '-';
  return String(num);
};

const formatPercentText = (value) => {
  const num = Number(value);
  if (!Number.isFinite(num)) return '-';
  return `${(num * 100).toFixed(0)}%`;
};

const formatTimeText = (value) => {
  if (!value) return '-';
  try {
    return new Date(value).toLocaleString();
  } catch (error) {
    return String(value);
  }
};

const STATE_META = {
  healthy: { color: 'green', label: 'Healthy' },
  new: { color: 'blue', label: 'New' },
  cooldown: { color: 'orange', label: 'Cooldown' },
  suspect: { color: 'amber', label: 'Suspect' },
  dead: { color: 'red', label: 'Dead' },
  refreshing: { color: 'cyan', label: 'Refreshing' },
};

const TRIGGER_REASON_META = {
  low_watermark: { color: 'orange', label: '低水位' },
  healthy_ratio_low: { color: 'orange', label: '健康占比过低' },
  cooldown_ratio_high: { color: 'red', label: 'Cooldown 占比过高' },
  dead_growth_fast: { color: 'red', label: 'Dead 增长过快' },
  no_available_tokens: { color: 'red', label: '连续无可用 Token' },
};

const BLOCK_REASON_META = {
  circuit_open: { color: 'red', label: '熔断中' },
  cooldown: { color: 'orange', label: '触发冷却中' },
  rate_limited: { color: 'red', label: '触发次数过多' },
  already_running: { color: 'blue', label: '补号任务运行中' },
};

const RESULT_CODE_META = {
  register_timeout: {
    color: 'orange',
    label: '补号超时',
    description: '在超时窗口内未检测到新 token 文件变化',
  },
  export_detected_after_timeout: {
    color: 'green',
    label: '超时后检测到新号',
    description: '控制层虽然超时结束，但随后检测到了新的 CursorPro token 导出文件',
  },
  register_trigger_failed: {
    color: 'red',
    label: '触发补号失败',
    description: '本地触发 CursorPro 补号动作时发生错误',
  },
  stale_task_recovered: {
    color: 'grey',
    label: '已回收陈旧任务',
    description: '恢复了上一次遗留的运行中任务状态',
  },
};

const getReasonMeta = (code, type) => {
  const normalized = getDisplayText(code);
  if (!normalized) return null;
  const source = type === 'block' ? BLOCK_REASON_META : TRIGGER_REASON_META;
  const meta = source[normalized];
  if (meta) {
    return {
      code: normalized,
      label: meta.label,
      color: meta.color,
    };
  }
  return {
    code: normalized,
    label: normalized,
    color: 'grey',
  };
};

const getResultCodeMeta = (code) => {
  const normalized = getDisplayText(code);
  if (!normalized) return null;
  const meta = RESULT_CODE_META[normalized];
  if (meta) {
    return {
      code: normalized,
      label: meta.label,
      color: meta.color,
      description: meta.description,
    };
  }
  return {
    code: normalized,
    label: normalized,
    color: 'grey',
    description: '',
  };
};

const formatAccountTypeLabel = (value, t) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const normalized = normalizePlanType(value);
  switch (normalized) {
    case 'free':
      return 'Free';
    case 'plus':
      return 'Plus';
    case 'pro':
      return 'Pro';
    case 'team':
      return 'Team';
    case 'enterprise':
      return 'Enterprise';
    default:
      return getDisplayText(value) || tt('未识别');
  }
};

const getAccountTypeTagColor = (value) => {
  const normalized = normalizePlanType(value);
  switch (normalized) {
    case 'enterprise':
      return 'green';
    case 'team':
      return 'cyan';
    case 'pro':
      return 'blue';
    case 'plus':
      return 'violet';
    case 'free':
      return 'amber';
    default:
      return 'grey';
  }
};

const resolveUsageStatusTag = (t, rateLimit) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  if (!rateLimit || Object.keys(rateLimit).length === 0) {
    return <Tag color='grey'>{tt('待确认')}</Tag>;
  }
  if (rateLimit?.allowed && !rateLimit?.limit_reached) {
    return <Tag color='green'>{tt('可用')}</Tag>;
  }
  return <Tag color='red'>{tt('受限')}</Tag>;
};

const AccountInfoValue = ({ t, value, onCopy, monospace = false }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const text = getDisplayText(value);
  const hasValue = text !== '';

  return (
    <div className='flex min-w-0 items-start justify-between gap-2'>
      <div
        className={`min-w-0 flex-1 break-all text-xs leading-5 text-semi-color-text-1 ${
          monospace ? 'font-mono' : ''
        }`}
      >
        {hasValue ? text : '-'}
      </div>
      <Button
        size='small'
        type='tertiary'
        theme='borderless'
        className='shrink-0 px-1 text-xs'
        disabled={!hasValue}
        onClick={() => onCopy?.(text)}
      >
        {tt('复制')}
      </Button>
    </div>
  );
};

const RateLimitWindowCard = ({ t, title, windowData }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const hasWindowData =
    !!windowData &&
    typeof windowData === 'object' &&
    Object.keys(windowData).length > 0;
  const percent = clampPercent(windowData?.used_percent ?? 0);
  const resetAt = windowData?.reset_at;
  const resetAfterSeconds = windowData?.reset_after_seconds;
  const limitWindowSeconds = windowData?.limit_window_seconds;

  return (
    <div className='rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3'>
      <div className='flex items-center justify-between gap-2'>
        <div className='font-medium'>{title}</div>
        <Text type='tertiary' size='small'>
          {tt('重置时间：')}
          {formatUnixSeconds(resetAt)}
        </Text>
      </div>

      {hasWindowData ? (
        <div className='mt-2'>
          <Progress
            percent={percent}
            stroke={pickStrokeColor(percent)}
            showInfo={true}
          />
        </div>
      ) : (
        <div className='mt-3 text-sm text-semi-color-text-2'>-</div>
      )}

      <div className='mt-1 flex flex-wrap items-center gap-2 text-xs text-semi-color-text-2'>
        <div>
          {tt('已使用：')}
          {hasWindowData ? `${percent}%` : '-'}
        </div>
        <div>
          {tt('距离重置：')}
          {hasWindowData ? formatDurationSeconds(resetAfterSeconds, tt) : '-'}
        </div>
        <div>
          {tt('窗口：')}
          {hasWindowData ? formatDurationSeconds(limitWindowSeconds, tt) : '-'}
        </div>
      </div>
    </div>
  );
};

const PoolStateGrid = ({ t, stateCounters }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const entries = Object.entries(STATE_META);

  return (
    <div className='grid grid-cols-2 gap-2 md:grid-cols-3'>
      {entries.map(([key, meta]) => (
        <div
          key={key}
          className='rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3'
        >
          <div className='flex items-center justify-between gap-2'>
            <Text size='small' type='tertiary'>
              {tt(meta.label)}
            </Text>
            <Tag color={meta.color} type='light' shape='circle'>
              {formatNumber(stateCounters?.[key] ?? 0)}
            </Tag>
          </div>
        </div>
      ))}
    </div>
  );
};

const StatusMetricCard = ({ title, value, hint }) => (
  <div className='rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3'>
    <div className='text-xs text-semi-color-text-2'>{title}</div>
    <div className='mt-2 text-lg font-semibold text-semi-color-text-0'>
      {value}
    </div>
    {hint ? (
      <div className='mt-1 text-xs text-semi-color-text-2'>{hint}</div>
    ) : null}
  </div>
);

const ReasonTag = ({ reason, type }) => {
  const meta = getReasonMeta(reason, type);
  if (!meta) return null;
  return (
    <Tag color={meta.color} type='light'>
      {meta.label}
    </Tag>
  );
};

const ReasonText = ({ t, label, reason, type }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const meta = getReasonMeta(reason, type);
  if (!meta) return null;
  return (
    <div className='mt-1 text-xs text-semi-color-text-2'>
      {tt(label)}: {meta.label}
      {meta.code !== meta.label ? ` (${meta.code})` : ''}
    </div>
  );
};

const PoolHealthSection = ({ t, poolHealthPayload }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const data = poolHealthPayload?.data ?? null;
  const health = data?.health ?? {};

  if (!poolHealthPayload) return null;

  return (
    <div className='flex flex-col gap-3'>
      <div className='flex flex-wrap items-start justify-between gap-2'>
        <div>
          <div className='text-sm font-semibold text-semi-color-text-0'>
            {tt('Pool Health')}
          </div>
          <Text type='tertiary' size='small'>
            {tt('观察当前 token 池可用度、退避状态和补号建议')}
          </Text>
        </div>
        <Space wrap spacing='tight'>
          <Tag
            color={data?.auto_import_enabled ? 'green' : 'grey'}
            type='light'
          >
            {tt('自动导入')}:{' '}
            {data?.auto_import_enabled ? tt('开启') : tt('关闭')}
          </Tag>
          <Tag
            color={data?.trigger_recommended ? 'orange' : 'green'}
            type='light'
          >
            {tt('补号建议')}:{' '}
            {data?.trigger_recommended ? tt('建议触发') : tt('暂不需要')}
          </Tag>
          <ReasonTag reason={data?.trigger_reason} type='trigger' />
        </Space>
      </div>

      <div className='grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4'>
        <StatusMetricCard
          title={tt('可用 Key')}
          value={formatNumber(health?.available_count)}
          hint={`${tt('总数')}: ${formatNumber(health?.total)}`}
        />
        <StatusMetricCard
          title={tt('健康占比')}
          value={formatPercentText(health?.healthy_ratio)}
          hint={`${tt('最低水位')}: ${formatNumber(data?.min_healthy_watermark)}`}
        />
        <StatusMetricCard
          title={tt('Cooldown 占比')}
          value={formatPercentText(health?.cooldown_ratio)}
          hint={`${tt('5分钟无可用')}: ${formatNumber(data?.recent_no_available_5m)}`}
        />
        <StatusMetricCard
          title={tt('30分钟新增 Dead')}
          value={formatNumber(health?.recent_dead_30m)}
          hint={tt('用于观察池子衰减速度')}
        />
      </div>

      <PoolStateGrid t={tt} stateCounters={data?.key_state_counters} />

      <div className='rounded-lg bg-semi-color-fill-0 px-3 py-2 text-xs text-semi-color-text-2 break-all'>
        {tt('CursorPro 导出目录')}: {data?.cursorpro_export_dir || '-'}
        <ReasonText
          t={tt}
          label='建议原因'
          reason={data?.trigger_reason}
          type='trigger'
        />
      </div>
    </div>
  );
};

const ReplacementStatusSection = ({ t, replacementPayload }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const data = replacementPayload?.data ?? null;
  const resultCodeMeta = getResultCodeMeta(data?.last_result_code);

  if (!replacementPayload) return null;

  return (
    <div className='flex flex-col gap-3'>
      <div className='flex flex-wrap items-start justify-between gap-2'>
        <div>
          <div className='text-sm font-semibold text-semi-color-text-0'>
            {tt('Replacement Status')}
          </div>
          <Text type='tertiary' size='small'>
            {tt('观察 CursorPro 补号任务、保护阈值和熔断状态')}
          </Text>
        </div>
        <Space wrap spacing='tight'>
          <Tag color={data?.trigger_allowed ? 'green' : 'red'} type='light'>
            {tt('允许触发')}: {data?.trigger_allowed ? tt('是') : tt('否')}
          </Tag>
          <ReasonTag reason={data?.block_reason} type='block' />
          <Tag
            color={data?.trigger_recommended ? 'orange' : 'grey'}
            type='light'
          >
            {tt('建议补号')}: {data?.trigger_recommended ? tt('是') : tt('否')}
          </Tag>
        </Space>
      </div>

      <div className='grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4'>
        <StatusMetricCard
          title={tt('30分钟触发次数')}
          value={formatNumber(data?.recent_triggers_30m)}
          hint={`${tt('上限')}: ${formatNumber(data?.max_triggers_per_30m)}`}
        />
        <StatusMetricCard
          title={tt('连续无产出')}
          value={formatNumber(data?.consecutive_no_yield)}
          hint={`${tt('熔断阈值')}: ${formatNumber(data?.open_circuit_after_no_yield)}`}
        />
        <StatusMetricCard
          title={tt('最近触发')}
          value={formatTimeText(data?.last_trigger_at)}
          hint={`${tt('最小间隔')}: ${formatNumber(data?.min_trigger_interval_sec)}s`}
        />
        <StatusMetricCard
          title={tt('熔断到期')}
          value={formatTimeText(data?.circuit_open_until)}
          hint={
            getReasonMeta(data?.trigger_reason, 'trigger')?.label
              ? `${tt('推荐原因')}: ${getReasonMeta(data?.trigger_reason, 'trigger')?.label}`
              : undefined
          }
        />
      </div>

      <div className='rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3'>
        <Descriptions>
          <Descriptions.Item itemKey={tt('控制服务')}>
            <AccountInfoValue
              t={tt}
              value={data?.control_base_url}
              monospace={true}
            />
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('任务 ID')}>
            <AccountInfoValue
              t={tt}
              value={data?.last_task_id}
              monospace={true}
            />
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('最后结果')}>
            {getDisplayText(data?.last_result_status) || '-'}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('最后结果原因')}>
            {resultCodeMeta?.label || getDisplayText(data?.last_result_code) || '-'}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('任务完成时间')}>
            {formatTimeText(data?.last_task_finished_at)}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('注册器状态')}>
            {getDisplayText(data?.register_status?.status) || '-'}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('注册器产出')}>
            {data?.register_status
              ? `${formatNumber(data.register_status.created_count)} / ${formatNumber(
                  data.register_status.updated_count,
                )}`
              : '-'}
          </Descriptions.Item>
        </Descriptions>
        {data?.register_status_error ? (
          <div className='mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700'>
            {tt('读取 CursorPro 状态失败')}: {data.register_status_error}
          </div>
        ) : null}
        {resultCodeMeta?.description || data?.last_result_message ? (
          <div className='mt-3 rounded-lg border border-semi-color-border bg-semi-color-fill-0 px-3 py-2 text-xs text-semi-color-text-1'>
            <span className='font-medium text-semi-color-text-0'>
              {tt('结果说明')}:
            </span>{' '}
            {resultCodeMeta?.description || data?.last_result_message}
          </div>
        ) : null}
        <ReasonText
          t={tt}
          label='阻塞原因'
          reason={data?.block_reason}
          type='block'
        />
      </div>
    </div>
  );
};

const CodexUsageView = ({
  t,
  record,
  payload,
  poolHealthPayload,
  replacementPayload,
  onCopy,
  onRefresh,
}) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const [showRawJson, setShowRawJson] = useState(false);
  const data = payload?.data ?? null;
  const rateLimit = data?.rate_limit ?? {};
  const { fiveHourWindow, weeklyWindow } = resolveRateLimitWindows(data);
  const upstreamStatus = payload?.upstream_status;
  const accountType = data?.plan_type ?? rateLimit?.plan_type;
  const accountTypeLabel = formatAccountTypeLabel(accountType, tt);
  const accountTypeTagColor = getAccountTypeTagColor(accountType);
  const statusTag = resolveUsageStatusTag(tt, rateLimit);
  const userId = data?.user_id;
  const email = data?.email;
  const accountId = data?.account_id;
  const errorMessage =
    payload?.success === false
      ? getDisplayText(payload?.message) || tt('获取用量失败')
      : '';

  const rawText =
    typeof data === 'string' ? data : JSON.stringify(data ?? payload, null, 2);

  return (
    <div className='flex flex-col gap-4'>
      {errorMessage && (
        <div className='rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700'>
          {errorMessage}
        </div>
      )}

      <div className='rounded-xl border border-semi-color-border bg-semi-color-bg-0 p-3'>
        <div className='flex flex-wrap items-start justify-between gap-2'>
          <div className='min-w-0'>
            <div className='text-xs font-medium text-semi-color-text-2'>
              {tt('Codex 帐号')}
            </div>
            <div className='mt-2 flex flex-wrap items-center gap-2'>
              <Tag
                color={accountTypeTagColor}
                type='light'
                shape='circle'
                size='large'
                className='font-semibold'
              >
                {accountTypeLabel}
              </Tag>
              {statusTag}
              <Tag color='grey' type='light' shape='circle'>
                {tt('上游状态码：')}
                {upstreamStatus ?? '-'}
              </Tag>
            </div>
          </div>
          <Button
            size='small'
            type='tertiary'
            theme='outline'
            onClick={onRefresh}
          >
            {tt('刷新')}
          </Button>
        </div>

        <div className='mt-2 rounded-lg bg-semi-color-fill-0 px-3 py-2'>
          <Descriptions>
            <Descriptions.Item itemKey='User ID'>
              <AccountInfoValue
                t={tt}
                value={userId}
                onCopy={onCopy}
                monospace={true}
              />
            </Descriptions.Item>
            <Descriptions.Item itemKey={tt('邮箱')}>
              <AccountInfoValue t={tt} value={email} onCopy={onCopy} />
            </Descriptions.Item>
            <Descriptions.Item itemKey='Account ID'>
              <AccountInfoValue
                t={tt}
                value={accountId}
                onCopy={onCopy}
                monospace={true}
              />
            </Descriptions.Item>
          </Descriptions>
        </div>

        <div className='mt-2 text-xs text-semi-color-text-2'>
          {tt('渠道：')}
          {record?.name || '-'} ({tt('编号：')}
          {record?.id || '-'})
        </div>
      </div>

      <div>
        <div className='mb-2'>
          <div className='text-sm font-semibold text-semi-color-text-0'>
            {tt('额度窗口')}
          </div>
          <Text type='tertiary' size='small'>
            {tt('用于观察当前帐号在 Codex 上游的限额使用情况')}
          </Text>
        </div>
      </div>

      <div className='grid grid-cols-1 gap-3 md:grid-cols-2'>
        <RateLimitWindowCard
          t={tt}
          title={tt('5小时窗口')}
          windowData={fiveHourWindow}
        />
        <RateLimitWindowCard
          t={tt}
          title={tt('每周窗口')}
          windowData={weeklyWindow}
        />
      </div>

      <PoolHealthSection t={tt} poolHealthPayload={poolHealthPayload} />
      <ReplacementStatusSection
        t={tt}
        replacementPayload={replacementPayload}
      />

      <Collapse
        activeKey={showRawJson ? ['raw-json'] : []}
        onChange={(activeKey) => {
          const keys = Array.isArray(activeKey) ? activeKey : [activeKey];
          setShowRawJson(keys.includes('raw-json'));
        }}
      >
        <Collapse.Panel header={tt('原始 JSON')} itemKey='raw-json'>
          <div className='mb-2 flex justify-end'>
            <Button
              size='small'
              type='primary'
              theme='outline'
              onClick={() => onCopy?.(rawText)}
              disabled={!rawText}
            >
              {tt('复制')}
            </Button>
          </div>
          <pre className='max-h-[50vh] overflow-y-auto rounded-lg bg-semi-color-fill-0 p-3 text-xs text-semi-color-text-0'>
            {rawText}
          </pre>
        </Collapse.Panel>
      </Collapse>
    </div>
  );
};

const CodexUsageLoader = ({ t, record, initialPayload, onCopy }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const [loading, setLoading] = useState(!initialPayload);
  const [payload, setPayload] = useState(initialPayload ?? null);
  const [poolHealthPayload, setPoolHealthPayload] = useState(null);
  const [replacementPayload, setReplacementPayload] = useState(null);
  const hasShownErrorRef = useRef(false);
  const mountedRef = useRef(true);
  const recordId = record?.id;

  const fetchUsage = useCallback(async () => {
    if (!recordId) {
      if (mountedRef.current) setPayload(null);
      return;
    }

    if (mountedRef.current) setLoading(true);
    try {
      const [usageRes, poolHealthRes, replacementRes] =
        await Promise.allSettled([
          API.get(`/api/channel/${recordId}/codex/usage`, {
            skipErrorHandler: true,
          }),
          API.get(`/api/channel/${recordId}/codex/pool_health`, {
            skipErrorHandler: true,
          }),
          API.get(`/api/channel/${recordId}/codex/replacement_status`, {
            skipErrorHandler: true,
          }),
        ]);
      if (!mountedRef.current) return;
      const usageData =
        usageRes.status === 'fulfilled' ? (usageRes.value?.data ?? null) : null;
      const poolData =
        poolHealthRes.status === 'fulfilled'
          ? (poolHealthRes.value?.data ?? null)
          : {
              success: false,
              message: String(poolHealthRes.reason || ''),
            };
      const replacementData =
        replacementRes.status === 'fulfilled'
          ? (replacementRes.value?.data ?? null)
          : {
              success: false,
              message: String(replacementRes.reason || ''),
            };

      setPayload(usageData);
      setPoolHealthPayload(poolData);
      setReplacementPayload(replacementData);

      if (!usageData?.success && !hasShownErrorRef.current) {
        hasShownErrorRef.current = true;
        showError(getDisplayText(usageData?.message) || tt('获取用量失败'));
      }
    } catch (error) {
      if (!mountedRef.current) return;
      if (!hasShownErrorRef.current) {
        hasShownErrorRef.current = true;
        showError(
          getDisplayText(error?.response?.data?.message) ||
            error?.message ||
            tt('获取用量失败'),
        );
      }
      setPayload({ success: false, message: String(error) });
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  }, [recordId, tt]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    if (initialPayload) return;
    fetchUsage().catch(() => {});
  }, [fetchUsage, initialPayload]);

  if (loading) {
    return (
      <div className='flex items-center justify-center py-10'>
        <Spin spinning={true} size='large' tip={tt('加载中...')} />
      </div>
    );
  }

  if (!payload) {
    return (
      <div className='flex flex-col gap-3'>
        <Text type='danger'>{tt('获取用量失败')}</Text>
        <div className='flex justify-end'>
          <Button
            size='small'
            type='primary'
            theme='outline'
            onClick={fetchUsage}
          >
            {tt('刷新')}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <CodexUsageView
      t={tt}
      record={record}
      payload={payload}
      poolHealthPayload={poolHealthPayload}
      replacementPayload={replacementPayload}
      onCopy={onCopy}
      onRefresh={fetchUsage}
    />
  );
};

const CodexUsageModalDialog = ({
  t,
  record,
  payload,
  onCopy,
  visible,
  onClose,
}) => {
  const tt = typeof t === 'function' ? t : (v) => v;

  useEffect(() => {
    document.body.classList.add('codex-usage-modal-open');
    return () => {
      document.body.classList.remove('codex-usage-modal-open');
    };
  }, []);

  return (
    <div className='codex-usage-overlay' onClick={onClose}>
      <div
        className='codex-usage-panel'
        role='dialog'
        aria-modal='true'
        aria-label={tt('Codex 帐号与用量')}
        onClick={(event) => event.stopPropagation()}
      >
        <div className='codex-usage-panel__header'>
          <div className='codex-usage-panel__title'>{tt('Codex 帐号与用量')}</div>
          <button
            type='button'
            className='codex-usage-panel__close'
            onClick={onClose}
            aria-label={tt('关闭')}
          >
            ×
          </button>
        </div>
        <div
          className='codex-usage-modal__scroll'
          onWheelCapture={(event) => {
            event.stopPropagation();
          }}
        >
          <CodexUsageLoader
            t={tt}
            record={record}
            initialPayload={payload}
            onCopy={onCopy}
          />
        </div>
        <div className='codex-usage-panel__footer'>
          <Button type='primary' theme='solid' onClick={onClose}>
            {tt('关闭')}
          </Button>
        </div>
      </div>
    </div>
  );
};

export const openCodexUsageModal = ({ t, record, payload, onCopy }) => {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = ReactDOM.createRoot(container);

  const cleanup = () => {
    try {
      root.unmount();
    } finally {
      if (container.parentNode) {
        container.parentNode.removeChild(container);
      }
    }
  };

  root.render(
    <CodexUsageModalDialog
      t={t}
      record={record}
      payload={payload}
      onCopy={onCopy}
      visible={true}
      onClose={cleanup}
    />,
  );
};
