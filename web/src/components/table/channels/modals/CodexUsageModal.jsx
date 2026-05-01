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

const formatDurationMs = (value) => {
  const num = Number(value);
  if (!Number.isFinite(num) || num < 0) return '-';
  if (num < 1000) return `${Math.round(num)} ms`;
  return `${(num / 1000).toFixed(num >= 10000 ? 0 : 2)} s`;
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
  request_rate_limit_hot_path: { color: 'red', label: '请求热路径遇到限额，已快速补号' },
  request_exhausted_after_failover: { color: 'orange', label: '本次请求切号后仍失败，已补号' },
  request_no_available_tokens: { color: 'red', label: '本次请求已无可用 Token，已补号' },
};

const BLOCK_REASON_META = {
  circuit_open: { color: 'red', label: '熔断中' },
  cooldown: { color: 'orange', label: '触发冷却中' },
  rate_limited: { color: 'red', label: '触发次数过多' },
  already_running: { color: 'blue', label: '补号任务运行中' },
};

const COOLDOWN_MODE_META = {
  time_lock: { color: 'grey', label: '固定时间锁' },
  result_aware: { color: 'blue', label: '结果感知' },
  broken_by_pool_critical: { color: 'orange', label: '池风险破冷却' },
};

const COOLDOWN_BREAK_REASON_META = {
  cooldown_break_available_count_zero: {
    color: 'red',
    label: '池子已归零，允许提前补号',
  },
  cooldown_break_no_available_spike: {
    color: 'orange',
    label: '连续无可用过多，允许提前补号',
  },
  cooldown_break_rate_limit_spike: {
    color: 'orange',
    label: '连续限额过多，允许提前补号',
  },
};

const SYNC_DIAGNOSIS_META = {
  source_updated_not_exported: {
    color: 'orange',
    label: '源 token 已刷新，但尚未导出',
  },
  export_updated_not_imported: {
    color: 'amber',
    label: '导出文件已更新，但尚未入池',
  },
  imported_pending_probe: {
    color: 'blue',
    label: '已入池，等待验活',
  },
  probe_succeeded: {
    color: 'green',
    label: '新号验活成功',
  },
  probe_failed_rate_limit: {
    color: 'orange',
    label: '新号验活即限额',
  },
  register_gui_blocked: {
    color: 'red',
    label: '自动补号被系统权限拦住',
  },
  trigger_failed_but_sync_ok: {
    color: 'green',
    label: 'GUI 触发失败，但同步链已恢复',
  },
  trigger_failed_no_new_source: {
    color: 'red',
    label: 'GUI 触发失败，且没有检测到新源号',
  },
  source_quiet_pool_low: {
    color: 'orange',
    label: '源目录静默且池低水位',
  },
};

const TRIGGER_RESULT_META = {
  trigger_succeeded: { color: 'green', label: 'GUI 补号触发成功' },
  trigger_failed: { color: 'red', label: 'GUI 补号触发失败' },
  trigger_skipped: { color: 'grey', label: '本轮未触发 GUI 补号' },
};

const RECOVERY_RESULT_META = {
  source_sync_succeeded: { color: 'green', label: '源目录同步成功' },
  imported_pending_probe: { color: 'blue', label: '已入池，等待验活' },
  probe_succeeded: { color: 'green', label: '已检测到新号并验活成功' },
  recovery_failed: { color: 'red', label: '本轮恢复失败' },
  imported_to_channel: { color: 'blue', label: '新号已入池' },
  updated_existing_tokens: { color: 'green', label: '现有 token 已更新' },
};

const RISK_LEVEL_META = {
  ok: { color: 'green', label: '正常' },
  degraded: { color: 'orange', label: '降级' },
  critical: { color: 'red', label: '危险' },
};

const TIMELINE_STAGE_META = {
  triggered: { color: 'grey', label: '已触发' },
  source_detected: { color: 'blue', label: '已检测源刷新' },
  exported: { color: 'cyan', label: '已导出' },
  imported: { color: 'violet', label: '已入池' },
  probed: { color: 'green', label: '已验活' },
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
  source_token_detected: {
    color: 'blue',
    label: '检测到源 token',
    description: '控制服务已观察到 CursorPro 源目录中的 token 变化。',
  },
  export_written: {
    color: 'green',
    label: '已写出导出文件',
    description: '源 token 已同步写入 CursorPro exports 目录。',
  },
  noop: {
    color: 'grey',
    label: '本轮无变化',
    description: '本次同步检查未发现新的源 token 变化。',
  },
};

const PROBE_RESULT_META = {
  probe_pending: {
    color: 'blue',
    label: '新号待验活',
    description: '新导入 token 已入池，正在等待轻量探测。',
  },
  probe_succeeded: {
    color: 'green',
    label: '新号验活成功',
    description: '新导入 token 已完成轻量探测并成功通过。',
  },
  probe_failed_rate_limit: {
    color: 'orange',
    label: '新号验活时已限额',
    description: '新导入 token 在探测时已命中 usage limit 或 429。',
  },
  probe_failed_auth: {
    color: 'red',
    label: '新号验活认证失败',
    description: '新导入 token 在探测时出现认证问题。',
  },
  probe_failed_invalid_key: {
    color: 'red',
    label: '底层 token 结构损坏，已移除',
    description: '新导入 token 结构不合法，已直接移出主池。',
  },
  probe_failed_soft_fail: {
    color: 'amber',
    label: '新号验活网络异常',
    description: '新导入 token 在探测时出现软网络错误。',
  },
  probe_failed_server: {
    color: 'orange',
    label: '新号验活上游异常',
    description: '新导入 token 在探测时出现上游 5xx 或服务异常。',
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

const getProbeResultMeta = (code) => {
  const normalized = getDisplayText(code);
  if (!normalized) return null;
  const meta = PROBE_RESULT_META[normalized];
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

const getSyncDiagnosisMeta = (code) => {
  const normalized = getDisplayText(code);
  if (!normalized) return null;
  const meta = SYNC_DIAGNOSIS_META[normalized];
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

const getSimpleStatusMeta = (code, source) => {
  const normalized = getDisplayText(code);
  if (!normalized) return null;
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

const SyncDiagnosisTag = ({ diagnosis }) => {
  const meta = getSyncDiagnosisMeta(diagnosis);
  if (!meta) return null;
  return (
    <Tag color={meta.color} type='light'>
      {meta.label}
    </Tag>
  );
};

const GenericStatusTag = ({ value, source }) => {
  const meta = getSimpleStatusMeta(value, source);
  if (!meta) return null;
  return (
    <Tag color={meta.color} type='light'>
      {meta.label}
    </Tag>
  );
};

const RecoveryTimelineSection = ({ t, timeline }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  if (!timeline) return null;

  return (
    <div className='rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3'>
      <div className='mb-3 flex items-center justify-between gap-2'>
        <div className='text-sm font-semibold text-semi-color-text-0'>
          {tt('换号时间线')}
        </div>
        <GenericStatusTag value={timeline?.current_stage} source={TIMELINE_STAGE_META} />
      </div>
      <Descriptions>
        <Descriptions.Item itemKey={tt('触发补号时间')}>
          {formatTimeText(timeline?.trigger_at)}
        </Descriptions.Item>
        <Descriptions.Item itemKey={tt('源 token 刷新时间')}>
          {formatTimeText(timeline?.source_detected_at)}
        </Descriptions.Item>
        <Descriptions.Item itemKey={tt('导出完成时间')}>
          {formatTimeText(timeline?.export_written_at)}
        </Descriptions.Item>
        <Descriptions.Item itemKey={tt('入池时间')}>
          {formatTimeText(timeline?.imported_at)}
        </Descriptions.Item>
        <Descriptions.Item itemKey={tt('验活完成时间')}>
          {formatTimeText(timeline?.probed_at)}
        </Descriptions.Item>
      </Descriptions>
      <div className='mt-3 grid grid-cols-1 gap-2 md:grid-cols-2 xl:grid-cols-3'>
        <StatusMetricCard title={tt('触发 -> 源刷新')} value={formatDurationMs(timeline?.trigger_to_source_ms)} />
        <StatusMetricCard title={tt('源刷新 -> 导出')} value={formatDurationMs(timeline?.source_to_export_ms)} />
        <StatusMetricCard title={tt('导出 -> 入池')} value={formatDurationMs(timeline?.export_to_import_ms)} />
        <StatusMetricCard title={tt('入池 -> 验活')} value={formatDurationMs(timeline?.import_to_probe_ms)} />
        <StatusMetricCard title={tt('端到端')} value={formatDurationMs(timeline?.end_to_end_ms)} />
      </div>
    </div>
  );
};

const PoolHealthSection = ({ t, poolHealthPayload }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const data = poolHealthPayload?.data ?? null;
  const health = data?.health ?? {};
  const tokenStatus = data?.token_status ?? null;

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
          <GenericStatusTag value={data?.pool_risk_level} source={RISK_LEVEL_META} />
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
          hint={`${tt('5分钟热触发')}: ${formatNumber(data?.recent_hot_path_triggers_5m)}`}
        />
        <StatusMetricCard
          title={tt('5分钟坏 Key')}
          value={formatNumber(data?.recent_invalid_key_5m)}
          hint={tt('结构损坏 token 已直接移除')}
        />
        <StatusMetricCard
          title={tt('5分钟验活成功')}
          value={formatNumber(data?.recent_probe_success_5m)}
          hint={`${tt('5分钟验活失败')}: ${formatNumber(data?.recent_probe_fail_5m)}`}
        />
        <StatusMetricCard
          title={tt('待验活新号')}
          value={formatNumber(data?.pending_probe_count)}
          hint={
            getProbeResultMeta(data?.last_probe_result)?.label
              ? `${tt('最近验活')}: ${getProbeResultMeta(data?.last_probe_result)?.label}`
              : undefined
          }
        />
        <StatusMetricCard
          title={tt('源目录 token 数')}
          value={formatNumber(tokenStatus?.source_token_count)}
          hint={`${tt('导出目录 token 数')}: ${formatNumber(tokenStatus?.export_token_count)}`}
        />
        <StatusMetricCard
          title={tt('同步延迟')}
          value={
            tokenStatus?.sync_lag_seconds == null
              ? '-'
              : `${Math.round(Number(tokenStatus?.sync_lag_seconds) || 0)}s`
          }
          hint={
            getSyncDiagnosisMeta(data?.sync_diagnosis)?.label
              ? `${tt('链路诊断')}: ${getSyncDiagnosisMeta(data?.sync_diagnosis)?.label}`
              : undefined
          }
        />
        <StatusMetricCard
          title={tt('恢复风险')}
          value={getSimpleStatusMeta(data?.pool_risk_level, RISK_LEVEL_META)?.label || '-'}
          hint={
            data?.last_successful_recovery_reason
              ? `${tt('最近恢复')}: ${getSimpleStatusMeta(data?.last_successful_recovery_reason, RECOVERY_RESULT_META)?.label || data?.last_successful_recovery_reason}`
              : undefined
          }
        />
      </div>

      <PoolStateGrid t={tt} stateCounters={data?.key_state_counters} />

      <div className='rounded-lg bg-semi-color-fill-0 px-3 py-2 text-xs text-semi-color-text-2 break-all'>
        {tt('CursorPro 导出目录')}: {data?.cursorpro_export_dir || '-'}
        <div className='mt-1'>
          <SyncDiagnosisTag diagnosis={data?.sync_diagnosis} />
        </div>
        <ReasonText
          t={tt}
          label='建议原因'
          reason={data?.trigger_reason}
          type='trigger'
        />
      </div>
      {tokenStatus || data?.token_status_error ? (
        <div className='rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3'>
          <Descriptions>
            <Descriptions.Item itemKey={tt('源目录最新文件')}>
              {getDisplayText(tokenStatus?.source_latest_file) || '-'}
            </Descriptions.Item>
            <Descriptions.Item itemKey={tt('源目录更新时间')}>
              {formatTimeText(tokenStatus?.source_latest_mtime)}
            </Descriptions.Item>
            <Descriptions.Item itemKey={tt('导出目录最新文件')}>
              {getDisplayText(tokenStatus?.export_latest_file) || '-'}
            </Descriptions.Item>
            <Descriptions.Item itemKey={tt('导出目录更新时间')}>
              {formatTimeText(tokenStatus?.export_latest_mtime)}
            </Descriptions.Item>
            <Descriptions.Item itemKey={tt('最近同步时间')}>
              {formatTimeText(tokenStatus?.last_sync_at)}
            </Descriptions.Item>
            <Descriptions.Item itemKey={tt('最近同步结果')}>
              {getResultCodeMeta(tokenStatus?.last_sync_result)?.label ||
                getDisplayText(tokenStatus?.last_sync_result) ||
                '-'}
            </Descriptions.Item>
            <Descriptions.Item itemKey={tt('同步原因')}>
              {getDisplayText(tokenStatus?.last_source_to_export_reason) || '-'}
            </Descriptions.Item>
            <Descriptions.Item itemKey={tt('最近入池')}>
              {formatTimeText(data?.last_import_at)}
            </Descriptions.Item>
            <Descriptions.Item itemKey={tt('源静默起点')}>
              {formatTimeText(data?.source_quiet_since)}
            </Descriptions.Item>
          </Descriptions>
          {data?.token_status_error ? (
            <div className='mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700'>
              {tt('读取同步状态失败')}: {data.token_status_error}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
};

const ReplacementStatusSection = ({ t, replacementPayload }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const data = replacementPayload?.data ?? null;
  const resultCodeMeta = getResultCodeMeta(data?.last_result_code);
  const probeResultMeta = getProbeResultMeta(data?.last_probe_result);
  const syncResultMeta = getResultCodeMeta(data?.token_status?.last_sync_result);
  const triggerResultMeta = getSimpleStatusMeta(data?.trigger_result, TRIGGER_RESULT_META);
  const recoveryResultMeta = getSimpleStatusMeta(data?.recovery_result, RECOVERY_RESULT_META);
  const riskLevelMeta = getSimpleStatusMeta(data?.pool_risk_level, RISK_LEVEL_META);
  const cooldownModeMeta = getSimpleStatusMeta(data?.cooldown_mode, COOLDOWN_MODE_META);
  const cooldownBreakReasonMeta = getSimpleStatusMeta(
    data?.cooldown_break_reason,
    COOLDOWN_BREAK_REASON_META,
  );

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
          <SyncDiagnosisTag diagnosis={data?.sync_diagnosis} />
          <GenericStatusTag value={data?.pool_risk_level} source={RISK_LEVEL_META} />
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
          title={tt('5分钟热触发')}
          value={formatNumber(data?.recent_hot_path_triggers_5m)}
          hint={tt('请求侧快速补号次数')}
        />
        <StatusMetricCard
          title={tt('待验活新号')}
          value={formatNumber(data?.pending_probe_count)}
          hint={`${tt('5分钟坏 Key')}: ${formatNumber(data?.recent_invalid_key_5m)}`}
        />
        <StatusMetricCard
          title={tt('冷却模式')}
          value={cooldownModeMeta?.label || '-'}
          hint={
            data?.cooldown_base_seconds
              ? `${tt('基础冷却')}: ${formatNumber(data?.cooldown_base_seconds)}s`
              : undefined
          }
        />
        <StatusMetricCard
          title={tt('最近触发')}
          value={formatTimeText(data?.last_trigger_at)}
          hint={`${tt('最小间隔')}: ${formatNumber(data?.min_trigger_interval_sec)}s`}
        />
        <StatusMetricCard
          title={tt('剩余冷却')}
          value={
            Number(data?.cooldown_seconds_remaining) > 0
              ? `${formatNumber(data?.cooldown_seconds_remaining)}s`
              : '-'
          }
          hint={
            cooldownBreakReasonMeta?.label
              ? `${tt('破冷却')}: ${cooldownBreakReasonMeta.label}`
              : data?.cooldown_break_allowed
                ? tt('当前允许提前补号')
                : undefined
          }
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
        <StatusMetricCard
          title={tt('最近同步')}
          value={formatTimeText(data?.token_status?.last_sync_at)}
          hint={syncResultMeta?.label ? `${tt('同步结果')}: ${syncResultMeta.label}` : undefined}
        />
        <StatusMetricCard
          title={tt('最近入池')}
          value={formatTimeText(data?.last_import_at)}
          hint={getDisplayText(data?.last_import_result) || undefined}
        />
        <StatusMetricCard
          title={tt('风险级别')}
          value={riskLevelMeta?.label || '-'}
          hint={
            recoveryResultMeta?.label
              ? `${tt('实际恢复')}: ${recoveryResultMeta.label}`
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
          <Descriptions.Item itemKey={tt('触发补号')}>
            {triggerResultMeta ? (
              <Tag color={triggerResultMeta.color} type='light'>
                {triggerResultMeta.label}
              </Tag>
            ) : (
              '-'
            )}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('实际恢复')}>
            {recoveryResultMeta ? (
              <Tag color={recoveryResultMeta.color} type='light'>
                {recoveryResultMeta.label}
              </Tag>
            ) : (
              '-'
            )}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('上次触发原因')}>
            <ReasonTag reason={data?.last_trigger_reason} type='trigger' />
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
          <Descriptions.Item itemKey={tt('验活模型')}>
            {getDisplayText(data?.last_probe_model) || '-'}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('验活结果')}>
            {probeResultMeta ? (
              <Tag color={probeResultMeta.color} type='light'>
                {probeResultMeta.label}
              </Tag>
            ) : (
              '-'
            )}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('同步诊断')}>
            <SyncDiagnosisTag diagnosis={data?.sync_diagnosis} />
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('最近同步原因')}>
            {getDisplayText(data?.token_status?.last_source_to_export_reason) || '-'}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('源静默起点')}>
            {formatTimeText(data?.source_quiet_since)}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('最近恢复时间')}>
            {formatTimeText(data?.last_successful_recovery_at)}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('冷却模式')}>
            <GenericStatusTag value={data?.cooldown_mode} source={COOLDOWN_MODE_META} />
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('基础冷却秒数')}>
            {data?.cooldown_base_seconds ? `${formatNumber(data?.cooldown_base_seconds)}s` : '-'}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('冷却到期时间')}>
            {formatTimeText(data?.cooldown_until)}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('剩余冷却秒数')}>
            {Number(data?.cooldown_seconds_remaining) > 0
              ? `${formatNumber(data?.cooldown_seconds_remaining)}s`
              : '-'}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('允许破冷却')}>
            {data?.cooldown_break_allowed ? tt('是') : tt('否')}
          </Descriptions.Item>
          <Descriptions.Item itemKey={tt('破冷却原因')}>
            <GenericStatusTag
              value={data?.cooldown_break_reason}
              source={COOLDOWN_BREAK_REASON_META}
            />
          </Descriptions.Item>
        </Descriptions>
        {data?.register_status_error ? (
          <div className='mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700'>
            {tt('读取 CursorPro 状态失败')}: {data.register_status_error}
          </div>
        ) : null}
        {data?.token_status_error ? (
          <div className='mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700'>
            {tt('读取同步状态失败')}: {data.token_status_error}
          </div>
        ) : null}
        {resultCodeMeta?.description || data?.last_result_message ? (
          <div className='mt-3 rounded-lg border border-semi-color-border bg-semi-color-fill-0 px-3 py-2 text-xs text-semi-color-text-1'>
            <span className='font-medium text-semi-color-text-0'>
              {tt('触发说明')}:
            </span>{' '}
            {resultCodeMeta?.description || data?.last_result_message}
          </div>
        ) : null}
        {recoveryResultMeta?.label || data?.last_successful_recovery_reason ? (
          <div className='mt-3 rounded-lg border border-semi-color-border bg-semi-color-fill-0 px-3 py-2 text-xs text-semi-color-text-1'>
            <span className='font-medium text-semi-color-text-0'>
              {tt('恢复说明')}:
            </span>{' '}
            {recoveryResultMeta?.label ||
              getSimpleStatusMeta(data?.last_successful_recovery_reason, RECOVERY_RESULT_META)?.label ||
              data?.last_successful_recovery_reason}
          </div>
        ) : null}
        {probeResultMeta?.description ? (
          <div className='mt-3 rounded-lg border border-semi-color-border bg-semi-color-fill-0 px-3 py-2 text-xs text-semi-color-text-1'>
            <span className='font-medium text-semi-color-text-0'>
              {tt('验活说明')}:
            </span>{' '}
            {probeResultMeta.description}
          </div>
        ) : null}
        <ReasonText
          t={tt}
          label='阻塞原因'
          reason={data?.block_reason}
          type='block'
        />
      </div>
      <RecoveryTimelineSection t={tt} timeline={data?.recovery_timeline} />
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
