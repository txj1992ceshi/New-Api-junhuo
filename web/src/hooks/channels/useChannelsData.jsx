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

import { useState, useEffect, useRef, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  API,
  showError,
  showInfo,
  showSuccess,
  loadChannelModels,
  copy,
  toBoolean,
} from '../../helpers';
import {
  CHANNEL_OPTIONS,
  ITEMS_PER_PAGE,
  MODEL_TABLE_PAGE_SIZE,
} from '../../constants';
import { useIsMobile } from '../common/useIsMobile';
import { useTableCompactMode } from '../common/useTableCompactMode';
import { useChannelUpstreamUpdates } from './useChannelUpstreamUpdates';
import { parseUpstreamUpdateMeta } from './upstreamUpdateUtils';
import { Modal, Button } from '@douyinfe/semi-ui';
import { openCodexUsageModal } from '../../components/table/channels/modals/CodexUsageModal';
import { openExternalPoolModal } from '../../components/table/channels/modals/WindsurfPoolModal';
import {
  getChannelAdminStatusKind,
  getExternalPoolAvailabilityRank,
  matchExternalPoolQuickFilter,
} from '../../components/table/channels/channelCodexProxy';

export const useChannelsData = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();

  // Basic states
  const [channels, setChannels] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [idSort, setIdSort] = useState(false);
  const [searching, setSearching] = useState(false);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [channelCount, setChannelCount] = useState(0);
  const [groupOptions, setGroupOptions] = useState([]);

  // UI states
  const [showEdit, setShowEdit] = useState(false);
  const [enableBatchDelete, setEnableBatchDelete] = useState(false);
  const [editingChannel, setEditingChannel] = useState({ id: undefined });
  const [showEditTag, setShowEditTag] = useState(false);
  const [editingTag, setEditingTag] = useState('');
  const [selectedChannels, setSelectedChannels] = useState([]);
  const [enableTagMode, setEnableTagMode] = useState(false);
  const [showBatchSetTag, setShowBatchSetTag] = useState(false);
  const [batchSetTagValue, setBatchSetTagValue] = useState('');
  const [compactMode, setCompactMode] = useTableCompactMode('channels');

  // Column visibility states
  const [visibleColumns, setVisibleColumns] = useState({});
  const [showColumnSelector, setShowColumnSelector] = useState(false);

  // Status filter
  const [statusFilter, setStatusFilter] = useState(
    localStorage.getItem('channel-status-filter') || 'all',
  );
  const [externalPoolIssueFirst, setExternalPoolIssueFirst] = useState(
    localStorage.getItem('channel-external-pool-issue-first') === 'true',
  );
  const [externalPoolQuickFilter, setExternalPoolQuickFilter] = useState(
    localStorage.getItem('channel-external-pool-quick-filter') || 'all',
  );

  // Type tabs states
  const [activeTypeKey, setActiveTypeKey] = useState('all');
  const [typeCounts, setTypeCounts] = useState({});

  // Model test states
  const [showModelTestModal, setShowModelTestModal] = useState(false);
  const [currentTestChannel, setCurrentTestChannel] = useState(null);
  const [modelSearchKeyword, setModelSearchKeyword] = useState('');
  const [modelTestResults, setModelTestResults] = useState({});
  const [testingModels, setTestingModels] = useState(new Set());
  const [selectedModelKeys, setSelectedModelKeys] = useState([]);
  const [isBatchTesting, setIsBatchTesting] = useState(false);
  const [modelTablePage, setModelTablePage] = useState(1);
  const [selectedEndpointType, setSelectedEndpointType] = useState('');
  const [isStreamTest, setIsStreamTest] = useState(false);
  const [globalPassThroughEnabled, setGlobalPassThroughEnabled] =
    useState(false);

  const fetchGlobalPassThroughEnabled = async () => {
    try {
      const res = await API.get('/api/option/');
      const { success, data } = res?.data || {};
      if (!success || !Array.isArray(data)) {
        return;
      }
      const option = data.find(
        (item) => item?.key === 'global.pass_through_request_enabled',
      );
      if (option) {
        setGlobalPassThroughEnabled(toBoolean(option.value));
      }
    } catch (error) {
      setGlobalPassThroughEnabled(false);
    }
  };

  // 使用 ref 来避免闭包问题，类似旧版实现
  const shouldStopBatchTestingRef = useRef(false);

  // Multi-key management states
  const [showMultiKeyManageModal, setShowMultiKeyManageModal] = useState(false);
  const [currentMultiKeyChannel, setCurrentMultiKeyChannel] = useState(null);

  // Refs
  const requestCounter = useRef(0);
  const allSelectingRef = useRef(false);
  const [formApi, setFormApi] = useState(null);

  const formInitValues = {
    searchKeyword: '',
    searchGroup: '',
    searchModel: '',
  };

  // Column keys
  const COLUMN_KEYS = {
    ID: 'id',
    NAME: 'name',
    GROUP: 'group',
    TYPE: 'type',
    STATUS: 'status',
    RESPONSE_TIME: 'response_time',
    BALANCE: 'balance',
    PRIORITY: 'priority',
    WEIGHT: 'weight',
    OPERATE: 'operate',
  };

  // Initialize from localStorage
  useEffect(() => {
    const localIdSort = localStorage.getItem('id-sort') === 'true';
    const localPageSize =
      parseInt(localStorage.getItem('page-size')) || ITEMS_PER_PAGE;
    const localEnableTagMode =
      localStorage.getItem('enable-tag-mode') === 'true';
    const localEnableBatchDelete =
      localStorage.getItem('enable-batch-delete') === 'true';

    setIdSort(localIdSort);
    setPageSize(localPageSize);
    setEnableTagMode(localEnableTagMode);
    setEnableBatchDelete(localEnableBatchDelete);

    loadChannels(1, localPageSize, localIdSort, localEnableTagMode)
      .then()
      .catch((reason) => {
        showError(reason);
      });
    fetchGroups().then();
    loadChannelModels().then();
    fetchGlobalPassThroughEnabled().then();
  }, []);

  // Column visibility management
  const getDefaultColumnVisibility = () => {
    return {
      [COLUMN_KEYS.ID]: true,
      [COLUMN_KEYS.NAME]: true,
      [COLUMN_KEYS.GROUP]: true,
      [COLUMN_KEYS.TYPE]: true,
      [COLUMN_KEYS.STATUS]: true,
      [COLUMN_KEYS.RESPONSE_TIME]: true,
      [COLUMN_KEYS.BALANCE]: true,
      [COLUMN_KEYS.PRIORITY]: true,
      [COLUMN_KEYS.WEIGHT]: true,
      [COLUMN_KEYS.OPERATE]: true,
    };
  };

  const initDefaultColumns = () => {
    const defaults = getDefaultColumnVisibility();
    setVisibleColumns(defaults);
  };

  // Load saved column preferences
  useEffect(() => {
    const savedColumns = localStorage.getItem('channels-table-columns');
    if (savedColumns) {
      try {
        const parsed = JSON.parse(savedColumns);
        const defaults = getDefaultColumnVisibility();
        const merged = { ...defaults, ...parsed };
        setVisibleColumns(merged);
      } catch (e) {
        console.error('Failed to parse saved column preferences', e);
        initDefaultColumns();
      }
    } else {
      initDefaultColumns();
    }
  }, []);

  // Save column preferences
  useEffect(() => {
    if (Object.keys(visibleColumns).length > 0) {
      localStorage.setItem(
        'channels-table-columns',
        JSON.stringify(visibleColumns),
      );
    }
  }, [visibleColumns]);

  const handleColumnVisibilityChange = (columnKey, checked) => {
    const updatedColumns = { ...visibleColumns, [columnKey]: checked };
    setVisibleColumns(updatedColumns);
  };

  const handleSelectAll = (checked) => {
    const allKeys = Object.keys(COLUMN_KEYS).map((key) => COLUMN_KEYS[key]);
    const updatedColumns = {};
    allKeys.forEach((key) => {
      updatedColumns[key] = checked;
    });
    setVisibleColumns(updatedColumns);
  };

  // Data formatting
  const sortChannelsByExternalPoolHealth = (
    items,
    issueFirst = externalPoolIssueFirst,
  ) => {
    if (!issueFirst || !Array.isArray(items) || items.length <= 1) {
      return items;
    }
    return [...items].sort((a, b) => {
      const aRank = getExternalPoolAvailabilityRank(a);
      const bRank = getExternalPoolAvailabilityRank(b);
      if (aRank !== bRank) {
        return aRank - bRank;
      }
      return Number(a?.id || 0) - Number(b?.id || 0);
    });
  };

  const setChannelFormat = (
    channels,
    enableTagMode,
    issueFirst = externalPoolIssueFirst,
  ) => {
    const sortedChannels = sortChannelsByExternalPoolHealth(channels, issueFirst);
    let channelDates = [];
    let channelTags = {};

    for (let i = 0; i < sortedChannels.length; i++) {
      sortedChannels[i].upstreamUpdateMeta = parseUpstreamUpdateMeta(
        sortedChannels[i].settings,
      );
      sortedChannels[i].key = '' + sortedChannels[i].id;
      if (!enableTagMode) {
        channelDates.push(sortedChannels[i]);
      } else {
        let tag = sortedChannels[i].tag ? sortedChannels[i].tag : '';
        let tagIndex = channelTags[tag];
        let tagChannelDates = undefined;

        if (tagIndex === undefined) {
          channelTags[tag] = 1;
          tagChannelDates = {
            key: tag,
            id: tag,
            tag: tag,
            name: '标签：' + tag,
            group: '',
            used_quota: 0,
            response_time: 0,
            priority: -1,
            weight: -1,
          };
          tagChannelDates.children = [];
          channelDates.push(tagChannelDates);
        } else {
          tagChannelDates = channelDates.find((item) => item.key === tag);
        }

        if (tagChannelDates.priority === -1) {
          tagChannelDates.priority = sortedChannels[i].priority;
        } else {
          if (tagChannelDates.priority !== sortedChannels[i].priority) {
            tagChannelDates.priority = '';
          }
        }

        if (tagChannelDates.weight === -1) {
          tagChannelDates.weight = sortedChannels[i].weight;
        } else {
          if (tagChannelDates.weight !== sortedChannels[i].weight) {
            tagChannelDates.weight = '';
          }
        }

        if (tagChannelDates.group === '') {
          tagChannelDates.group = sortedChannels[i].group;
        } else {
          let channelGroupsStr = sortedChannels[i].group;
          channelGroupsStr.split(',').forEach((item, index) => {
            if (tagChannelDates.group.indexOf(item) === -1) {
              tagChannelDates.group += ',' + item;
            }
          });
        }

        tagChannelDates.children.push(sortedChannels[i]);
        if (sortedChannels[i].status === 1) {
          tagChannelDates.status = 1;
        }
        tagChannelDates.used_quota += sortedChannels[i].used_quota;
        tagChannelDates.response_time += sortedChannels[i].response_time;
        tagChannelDates.response_time = tagChannelDates.response_time / 2;
      }
    }
    setChannels(channelDates);
  };

  // Get form values helper
  const getFormValues = () => {
    const formValues = formApi ? formApi.getValues() : {};
    return {
      searchKeyword: formValues.searchKeyword || '',
      searchGroup: formValues.searchGroup || '',
      searchModel: formValues.searchModel || '',
    };
  };

  // Load channels
  const loadChannels = async (
    page,
    pageSize,
    idSort,
    enableTagMode,
    typeKey = activeTypeKey,
    statusF,
    issueFirst = externalPoolIssueFirst,
  ) => {
    if (statusF === undefined) statusF = statusFilter;

    const { searchKeyword, searchGroup, searchModel } = getFormValues();
    if (searchKeyword !== '' || searchGroup !== '' || searchModel !== '') {
      setLoading(true);
      await searchChannels(
        enableTagMode,
        typeKey,
        statusF,
        page,
        pageSize,
        idSort,
        issueFirst,
      );
      setLoading(false);
      return;
    }

    const reqId = ++requestCounter.current;
    setLoading(true);
    const typeParam = typeKey !== 'all' ? `&type=${typeKey}` : '';
    const statusParam = statusF !== 'all' ? `&status=${statusF}` : '';
    const res = await API.get(
      `/api/channel/?p=${page}&page_size=${pageSize}&id_sort=${idSort}&tag_mode=${enableTagMode}${typeParam}${statusParam}`,
    );

    if (res === undefined || reqId !== requestCounter.current) {
      return;
    }

    const { success, message, data } = res.data;
    if (success) {
      const { items, total, type_counts } = data;
      if (type_counts) {
        const sumAll = Object.values(type_counts).reduce(
          (acc, v) => acc + v,
          0,
        );
        setTypeCounts({ ...type_counts, all: sumAll });
      }
      setChannelFormat(items, enableTagMode, issueFirst);
      setChannelCount(total);
    } else {
      showError(message);
    }
    setLoading(false);
  };

  // Search channels
  const searchChannels = async (
    enableTagMode,
    typeKey = activeTypeKey,
    statusF = statusFilter,
    page = 1,
    pageSz = pageSize,
    sortFlag = idSort,
    issueFirst = externalPoolIssueFirst,
  ) => {
    const { searchKeyword, searchGroup, searchModel } = getFormValues();
    setSearching(true);
    try {
      if (searchKeyword === '' && searchGroup === '' && searchModel === '') {
        await loadChannels(
          page,
          pageSz,
          sortFlag,
          enableTagMode,
          typeKey,
          statusF,
          issueFirst,
        );
        return;
      }

      const typeParam = typeKey !== 'all' ? `&type=${typeKey}` : '';
      const statusParam = statusF !== 'all' ? `&status=${statusF}` : '';
      const res = await API.get(
        `/api/channel/search?keyword=${searchKeyword}&group=${searchGroup}&model=${searchModel}&id_sort=${sortFlag}&tag_mode=${enableTagMode}&p=${page}&page_size=${pageSz}${typeParam}${statusParam}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        const { items = [], total = 0, type_counts = {} } = data;
        const sumAll = Object.values(type_counts).reduce(
          (acc, v) => acc + v,
          0,
        );
        setTypeCounts({ ...type_counts, all: sumAll });
        setChannelFormat(items, enableTagMode, issueFirst);
        setChannelCount(total);
        setActivePage(page);
      } else {
        showError(message);
      }
    } finally {
      setSearching(false);
    }
  };

  // Refresh
  const refresh = async (page = activePage) => {
    const { searchKeyword, searchGroup, searchModel } = getFormValues();
    if (searchKeyword === '' && searchGroup === '' && searchModel === '') {
      await loadChannels(page, pageSize, idSort, enableTagMode);
    } else {
      await searchChannels(
        enableTagMode,
        activeTypeKey,
        statusFilter,
        page,
        pageSize,
        idSort,
      );
    }
  };

  const upstreamUpdates = useChannelUpstreamUpdates({ t, refresh });

  // Channel management
  const manageChannel = async (id, action, record, value) => {
    let data = { id };
    let res;
    switch (action) {
      case 'delete':
        res = await API.delete(`/api/channel/${id}/`);
        break;
      case 'enable':
        data.status = 1;
        res = await API.put('/api/channel/', data);
        break;
      case 'disable':
        data.status = 2;
        res = await API.put('/api/channel/', data);
        break;
      case 'priority':
        if (value === '') return;
        data.priority = parseInt(value);
        res = await API.put('/api/channel/', data);
        break;
      case 'weight':
        if (value === '') return;
        data.weight = parseInt(value);
        if (data.weight < 0) data.weight = 0;
        res = await API.put('/api/channel/', data);
        break;
      case 'enable_all':
        data.channel_info = record.channel_info;
        data.channel_info.multi_key_status_list = {};
        res = await API.put('/api/channel/', data);
        break;
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      let channel = res.data.data;
      let newChannels = [...channels];
      if (action !== 'delete') {
        record.status = channel.status;
      }
      setChannels(newChannels);
    } else {
      showError(message);
    }
  };

  // Tag management
  const manageTag = async (tag, action) => {
    let res;
    switch (action) {
      case 'enable':
        res = await API.post('/api/channel/tag/enabled', { tag: tag });
        break;
      case 'disable':
        res = await API.post('/api/channel/tag/disabled', { tag: tag });
        break;
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      let newChannels = [...channels];
      for (let i = 0; i < newChannels.length; i++) {
        if (newChannels[i].tag === tag) {
          let status = action === 'enable' ? 1 : 2;
          newChannels[i]?.children?.forEach((channel) => {
            channel.status = status;
          });
          newChannels[i].status = status;
        }
      }
      setChannels(newChannels);
    } else {
      showError(message);
    }
  };

  // Page handlers
  const handlePageChange = (page) => {
    const { searchKeyword, searchGroup, searchModel } = getFormValues();
    setActivePage(page);
    if (searchKeyword === '' && searchGroup === '' && searchModel === '') {
      loadChannels(page, pageSize, idSort, enableTagMode).then(() => {});
    } else {
      searchChannels(
        enableTagMode,
        activeTypeKey,
        statusFilter,
        page,
        pageSize,
        idSort,
      );
    }
  };

  const handlePageSizeChange = async (size) => {
    localStorage.setItem('page-size', size + '');
    setPageSize(size);
    setActivePage(1);
    const { searchKeyword, searchGroup, searchModel } = getFormValues();
    if (searchKeyword === '' && searchGroup === '' && searchModel === '') {
      loadChannels(1, size, idSort, enableTagMode)
        .then()
        .catch((reason) => {
          showError(reason);
        });
    } else {
      searchChannels(
        enableTagMode,
        activeTypeKey,
        statusFilter,
        1,
        size,
        idSort,
      );
    }
  };

  // Fetch groups
  const fetchGroups = async () => {
    try {
      let res = await API.get(`/api/group/`);
      if (res === undefined) return;
      setGroupOptions(
        res.data.data.map((group) => ({
          label: group,
          value: group,
        })),
      );
    } catch (error) {
      showError(error.message);
    }
  };

  // Copy channel
  const copySelectedChannel = async (record) => {
    try {
      const res = await API.post(`/api/channel/copy/${record.id}`);
      if (res?.data?.success) {
        showSuccess(t('渠道复制成功'));
        await refresh();
      } else {
        showError(res?.data?.message || t('渠道复制失败'));
      }
    } catch (error) {
      showError(
        t('渠道复制失败: ') +
          (error?.response?.data?.message || error?.message || error),
      );
    }
  };

  // Update channel property
  const updateChannelProperty = (channelId, updateFn) => {
    const newChannels = [...channels];
    let updated = false;

    newChannels.forEach((channel) => {
      if (channel.children !== undefined) {
        channel.children.forEach((child) => {
          if (child.id === channelId) {
            updateFn(child);
            updated = true;
          }
        });
      } else if (channel.id === channelId) {
        updateFn(channel);
        updated = true;
      }
    });

    if (updated) {
      setChannels(newChannels);
    }
  };

  // Tag edit
  const submitTagEdit = async (type, data) => {
    switch (type) {
      case 'priority':
        if (data.priority === undefined || data.priority === '') {
          showInfo('优先级必须是整数！');
          return;
        }
        data.priority = parseInt(data.priority);
        break;
      case 'weight':
        if (
          data.weight === undefined ||
          data.weight < 0 ||
          data.weight === ''
        ) {
          showInfo('权重必须是非负整数！');
          return;
        }
        data.weight = parseInt(data.weight);
        break;
    }

    try {
      const res = await API.put('/api/channel/tag', data);
      if (res?.data?.success) {
        showSuccess('更新成功！');
        await refresh();
      }
    } catch (error) {
      showError(error);
    }
  };

  // Close edit
  const closeEdit = () => {
    setShowEdit(false);
  };

  // Row style
  const handleRow = (record, index) => {
    if (record.status !== 1) {
      return {
        style: {
          background: 'var(--semi-color-disabled-border)',
        },
      };
    } else {
      return {};
    }
  };

  // Batch operations
  const batchSetChannelTag = async () => {
    if (selectedChannels.length === 0) {
      showError(t('请先选择要设置标签的渠道！'));
      return;
    }
    if (batchSetTagValue === '') {
      showError(t('标签不能为空！'));
      return;
    }
    let ids = selectedChannels.map((channel) => channel.id);
    const res = await API.post('/api/channel/batch/tag', {
      ids: ids,
      tag: batchSetTagValue === '' ? null : batchSetTagValue,
    });
    if (res.data.success) {
      showSuccess(
        t('已为 ${count} 个渠道设置标签！').replace('${count}', res.data.data),
      );
      await refresh();
      setShowBatchSetTag(false);
    } else {
      showError(res.data.message);
    }
  };

  const batchDeleteChannels = async () => {
    if (selectedChannels.length === 0) {
      showError(t('请先选择要删除的通道！'));
      return;
    }
    setLoading(true);
    let ids = [];
    selectedChannels.forEach((channel) => {
      ids.push(channel.id);
    });
    const res = await API.post(`/api/channel/batch`, { ids: ids });
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(t('已删除 ${data} 个通道！').replace('${data}', data));
      await refresh();
      setTimeout(() => {
        if (channels.length === 0 && activePage > 1) {
          refresh(activePage - 1);
        }
      }, 100);
    } else {
      showError(message);
    }
    setLoading(false);
  };

  // Channel operations
  const testAllChannels = async () => {
    const res = await API.get(`/api/channel/test`);
    const { success, message } = res.data;
    if (success) {
      showInfo(t('已成功开始测试所有已启用通道，请刷新页面查看结果。'));
    } else {
      showError(message);
    }
  };

  const deleteAllDisabledChannels = async () => {
    const res = await API.delete(`/api/channel/disabled`);
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(
        t('已删除所有禁用渠道，共计 ${data} 个').replace('${data}', data),
      );
      await refresh();
    } else {
      showError(message);
    }
  };

  const updateAllChannelsBalance = async () => {
    const res = await API.get(`/api/channel/update_balance`);
    const { success, message } = res.data;
    if (success) {
      showInfo(t('已更新完毕所有已启用通道余额！'));
    } else {
      showError(message);
    }
  };

  const updateChannelBalance = async (record) => {
    const adminStatusKind = getChannelAdminStatusKind(record);
    if (adminStatusKind === 'codex') {
      openCodexUsageModal({
        t,
        record,
        onCopy: async (text) => {
          const ok = await copy(text);
          if (ok) showSuccess(t('已复制'));
          else showError(t('复制失败'));
        },
      });
      return;
    }
    if (adminStatusKind === 'windsurf' || adminStatusKind === 'cursor' || adminStatusKind === 'kiro') {
      openExternalPoolModal({
        t,
        record,
        onCopy: async (text) => {
          const ok = await copy(text);
          if (ok) showSuccess(t('已复制'));
          else showError(t('复制失败'));
        },
      });
      return;
    }

    const res = await API.get(`/api/channel/update_balance/${record.id}/`);
    const { success, message, balance } = res.data;
    if (success) {
      updateChannelProperty(record.id, (channel) => {
        channel.balance = balance;
        channel.balance_updated_time = Date.now() / 1000;
      });
      showInfo(
        t('通道 ${name} 余额更新成功！').replace('${name}', record.name),
      );
    } else {
      showError(message);
    }
  };

  const fixChannelsAbilities = async () => {
    const res = await API.post(`/api/channel/fix`);
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(
        t('已修复 ${success} 个通道，失败 ${fails} 个通道。')
          .replace('${success}', data.success)
          .replace('${fails}', data.fails),
      );
      await refresh();
    } else {
      showError(message);
    }
  };

  const checkOllamaVersion = async (record) => {
    try {
      const res = await API.get(`/api/channel/ollama/version/${record.id}`);
      const { success, message, data } = res.data;

      if (success) {
        const version = data?.version || '-';
        const infoMessage = t('当前 Ollama 版本为 ${version}').replace(
          '${version}',
          version,
        );

        const handleCopyVersion = async () => {
          if (!version || version === '-') {
            showInfo(t('暂无可复制的版本信息'));
            return;
          }

          const copied = await copy(version);
          if (copied) {
            showSuccess(t('已复制版本号'));
          } else {
            showError(t('复制失败，请手动复制'));
          }
        };

        Modal.info({
          title: t('Ollama 版本信息'),
          content: infoMessage,
          centered: true,
          footer: (
            <div className='flex justify-end gap-2'>
              <Button type='tertiary' onClick={handleCopyVersion}>
                {t('复制版本号')}
              </Button>
              <Button
                type='primary'
                theme='solid'
                onClick={() => Modal.destroyAll()}
              >
                {t('关闭')}
              </Button>
            </div>
          ),
          hasCancel: false,
          hasOk: false,
          closable: true,
          maskClosable: true,
        });
      } else {
        showError(message || t('获取 Ollama 版本失败'));
      }
    } catch (error) {
      const errMsg =
        error?.response?.data?.message ||
        error?.message ||
        t('获取 Ollama 版本失败');
      showError(errMsg);
    }
  };

  // Test channel - 单个模型测试，参考旧版实现
  const testChannel = async (
    record,
    model,
    endpointType = '',
    stream = false,
  ) => {
    const testKey = `${record.id}-${model}`;

    // 检查是否应该停止批量测试
    if (shouldStopBatchTestingRef.current && isBatchTesting) {
      return Promise.resolve();
    }

    // 添加到正在测试的模型集合
    setTestingModels((prev) => new Set([...prev, model]));

    try {
      let url = `/api/channel/test/${record.id}?model=${model}`;
      if (endpointType) {
        url += `&endpoint_type=${endpointType}`;
      }
      if (stream) {
        url += `&stream=true`;
      }
      const res = await API.get(url);

      // 检查是否在请求期间被停止
      if (shouldStopBatchTestingRef.current && isBatchTesting) {
        return Promise.resolve();
      }

      const { success, message, time, error_code } = res.data;

      // 更新测试结果
      setModelTestResults((prev) => ({
        ...prev,
        [testKey]: {
          success,
          message,
          time: time || 0,
          timestamp: Date.now(),
          errorCode: error_code || null,
        },
      }));

      if (success) {
        // 更新渠道响应时间
        updateChannelProperty(record.id, (channel) => {
          channel.response_time = time * 1000;
          channel.test_time = Date.now() / 1000;
        });

        if (!model || model === '') {
          showInfo(
            t('通道 ${name} 测试成功，耗时 ${time.toFixed(2)} 秒。')
              .replace('${name}', record.name)
              .replace('${time.toFixed(2)}', time.toFixed(2)),
          );
        } else {
          showInfo(
            t(
              '通道 ${name} 测试成功，模型 ${model} 耗时 ${time.toFixed(2)} 秒。',
            )
              .replace('${name}', record.name)
              .replace('${model}', model)
              .replace('${time.toFixed(2)}', time.toFixed(2)),
          );
        }
      } else {
        showError(message);
      }
    } catch (error) {
      // 处理网络错误
      const testKey = `${record.id}-${model}`;
      setModelTestResults((prev) => ({
        ...prev,
        [testKey]: {
          success: false,
          message: error.message || t('网络错误'),
          time: 0,
          timestamp: Date.now(),
          errorCode: null,
        },
      }));
      showError(error.message || t('测试失败'));
    } finally {
      // 从正在测试的模型集合中移除
      setTestingModels((prev) => {
        const newSet = new Set(prev);
        newSet.delete(model);
        return newSet;
      });
    }
  };

  // 批量测试单个渠道的所有模型，参考旧版实现
  const batchTestModels = async () => {
    if (!currentTestChannel || !currentTestChannel.models) {
      showError(t('渠道模型信息不完整'));
      return;
    }

    const models = currentTestChannel.models
      .split(',')
      .filter((model) =>
        model.toLowerCase().includes(modelSearchKeyword.toLowerCase()),
      );

    if (models.length === 0) {
      showError(t('没有找到匹配的模型'));
      return;
    }

    setIsBatchTesting(true);
    shouldStopBatchTestingRef.current = false; // 重置停止标志

    // 清空该渠道之前的测试结果
    setModelTestResults((prev) => {
      const newResults = { ...prev };
      models.forEach((model) => {
        const testKey = `${currentTestChannel.id}-${model}`;
        delete newResults[testKey];
      });
      return newResults;
    });

    try {
      showInfo(
        t('开始批量测试 ${count} 个模型，已清空上次结果...').replace(
          '${count}',
          models.length,
        ),
      );

      // 提高并发数量以加快测试速度，参考旧版的并发限制
      const concurrencyLimit = 5;
      const results = [];

      for (let i = 0; i < models.length; i += concurrencyLimit) {
        // 检查是否应该停止
        if (shouldStopBatchTestingRef.current) {
          showInfo(t('批量测试已停止'));
          break;
        }

        const batch = models.slice(i, i + concurrencyLimit);
        showInfo(
          t('正在测试第 ${current} - ${end} 个模型 (共 ${total} 个)')
            .replace('${current}', i + 1)
            .replace('${end}', Math.min(i + concurrencyLimit, models.length))
            .replace('${total}', models.length),
        );

        const batchPromises = batch.map((model) =>
          testChannel(
            currentTestChannel,
            model,
            selectedEndpointType,
            isStreamTest,
          ),
        );
        const batchResults = await Promise.allSettled(batchPromises);
        results.push(...batchResults);

        // 再次检查是否应该停止
        if (shouldStopBatchTestingRef.current) {
          showInfo(t('批量测试已停止'));
          break;
        }

        // 短暂延迟避免过于频繁的请求
        if (i + concurrencyLimit < models.length) {
          await new Promise((resolve) => setTimeout(resolve, 100));
        }
      }

      if (!shouldStopBatchTestingRef.current) {
        // 等待一小段时间确保所有结果都已更新
        await new Promise((resolve) => setTimeout(resolve, 300));

        // 使用当前状态重新计算结果统计
        setModelTestResults((currentResults) => {
          let successCount = 0;
          let failCount = 0;

          models.forEach((model) => {
            const testKey = `${currentTestChannel.id}-${model}`;
            const result = currentResults[testKey];
            if (result && result.success) {
              successCount++;
            } else {
              failCount++;
            }
          });

          // 显示完成消息
          setTimeout(() => {
            showSuccess(
              t('批量测试完成！成功: ${success}, 失败: ${fail}, 总计: ${total}')
                .replace('${success}', successCount)
                .replace('${fail}', failCount)
                .replace('${total}', models.length),
            );
          }, 100);

          return currentResults; // 不修改状态，只是为了获取最新值
        });
      }
    } catch (error) {
      showError(t('批量测试过程中发生错误: ') + error.message);
    } finally {
      setIsBatchTesting(false);
    }
  };

  // 停止批量测试
  const stopBatchTesting = () => {
    shouldStopBatchTestingRef.current = true;
    setIsBatchTesting(false);
    setTestingModels(new Set());
    showInfo(t('已停止批量测试'));
  };

  // 清空测试结果
  const clearTestResults = () => {
    setModelTestResults({});
    showInfo(t('已清空测试结果'));
  };

  // Handle close modal
  const handleCloseModal = () => {
    // 如果正在批量测试，先停止测试
    if (isBatchTesting) {
      shouldStopBatchTestingRef.current = true;
      showInfo(t('关闭弹窗，已停止批量测试'));
    }

    setShowModelTestModal(false);
    setModelSearchKeyword('');
    setIsBatchTesting(false);
    setTestingModels(new Set());
    setSelectedModelKeys([]);
    setModelTablePage(1);
    setSelectedEndpointType('');
    setIsStreamTest(false);
    // 可选择性保留测试结果，这里不清空以便用户查看
  };

  // Type counts
  const channelTypeCounts = useMemo(() => {
    if (Object.keys(typeCounts).length > 0) return typeCounts;
    const counts = { all: channels.length };
    channels.forEach((channel) => {
      const collect = (ch) => {
        const type = ch.type;
        counts[type] = (counts[type] || 0) + 1;
      };
      if (channel.children !== undefined) {
        channel.children.forEach(collect);
      } else {
        collect(channel);
      }
    });
    return counts;
  }, [typeCounts, channels]);

  const availableTypeKeys = useMemo(() => {
    const keys = ['all'];
    Object.entries(channelTypeCounts).forEach(([k, v]) => {
      if (k !== 'all' && v > 0) keys.push(String(k));
    });
    return keys;
  }, [channelTypeCounts]);

  const filteredChannels = useMemo(() => {
    if (
      !externalPoolQuickFilter ||
      externalPoolQuickFilter === 'all' ||
      !Array.isArray(channels)
    ) {
      return channels;
    }

    return channels
      .map((channel) => {
        if (channel.children !== undefined) {
          const filteredChildren = channel.children.filter((child) =>
            matchExternalPoolQuickFilter(child, externalPoolQuickFilter),
          );
          if (filteredChildren.length === 0) {
            return null;
          }
          return {
            ...channel,
            children: filteredChildren,
          };
        }
        return matchExternalPoolQuickFilter(channel, externalPoolQuickFilter)
          ? channel
          : null;
      })
      .filter(Boolean);
  }, [channels, externalPoolQuickFilter]);

  const filteredChannelCount = useMemo(() => {
    let count = 0;
    filteredChannels.forEach((channel) => {
      if (channel?.children !== undefined) {
        count += channel.children.length;
      } else {
        count += 1;
      }
    });
    return count;
  }, [filteredChannels]);

  const externalPoolPageStats = useMemo(() => {
    const stats = {
      total: 0,
      available: 0,
      degraded: 0,
      unavailable: 0,
      authRejected: 0,
      emptyPool: 0,
      upstreamUnreachable: 0,
      upstreamPathNotFound: 0,
      rateLimited: 0,
    };

    const collect = (record) => {
      const kind = getChannelAdminStatusKind(record);
      if (!kind || kind === 'codex') {
        return;
      }
      stats.total += 1;
      if (matchExternalPoolQuickFilter(record, 'available')) {
        stats.available += 1;
      }
      if (matchExternalPoolQuickFilter(record, 'degraded')) {
        stats.degraded += 1;
      }
      if (matchExternalPoolQuickFilter(record, 'unavailable')) {
        stats.unavailable += 1;
      }
      if (matchExternalPoolQuickFilter(record, 'auth_rejected')) {
        stats.authRejected += 1;
      }
      if (matchExternalPoolQuickFilter(record, 'empty_pool')) {
        stats.emptyPool += 1;
      }
      if (matchExternalPoolQuickFilter(record, 'upstream_unreachable')) {
        stats.upstreamUnreachable += 1;
      }
      if (matchExternalPoolQuickFilter(record, 'upstream_path_not_found')) {
        stats.upstreamPathNotFound += 1;
      }
      if (matchExternalPoolQuickFilter(record, 'rate_limited')) {
        stats.rateLimited += 1;
      }
    };

    filteredChannels.forEach((channel) => {
      if (channel?.children !== undefined) {
        channel.children.forEach(collect);
      } else {
        collect(channel);
      }
    });

    return stats;
  }, [filteredChannels]);

  const externalPoolNextAction = useMemo(() => {
    if (externalPoolPageStats.total <= 0) {
      return {
        kind: 'idle',
        title: t('当前页暂无外部池渠道'),
        description: t('等你把 Cursor / Windsurf / Kiro 代理渠道配进来后，这里会开始给出联调建议。'),
      };
    }

    const candidates = [
      {
        key: 'authRejected',
        filter: 'auth_rejected',
        count: externalPoolPageStats.authRejected,
        title: t('先查认证失败'),
        description: t('当前页认证失败最多。先点“认证失败”，再检查 key、认证 Header、认证 Scheme 和上游是否已换密钥。'),
      },
      {
        key: 'upstreamUnreachable',
        filter: 'upstream_unreachable',
        count: externalPoolPageStats.upstreamUnreachable,
        title: t('先查连接失败'),
        description: t('当前页连接失败最多。先点“连接失败”，再检查 base_url、端口监听、池服务进程和网络连通性。'),
      },
      {
        key: 'upstreamPathNotFound',
        filter: 'upstream_path_not_found',
        count: externalPoolPageStats.upstreamPathNotFound,
        title: t('先查路径错误'),
        description: t('当前页路径错误最多。优先核对 status/accounts/auth 相关路径配置，不要先怀疑模型或账号。'),
      },
      {
        key: 'emptyPool',
        filter: 'empty_pool',
        count: externalPoolPageStats.emptyPool,
        title: t('先补空池'),
        description: t('当前页空池最多。先确认上游是否真的写入了账号，再继续做 /v1/models 或 /v1/responses 验证。'),
      },
      {
        key: 'rateLimited',
        filter: 'rate_limited',
        count: externalPoolPageStats.rateLimited,
        title: t('先处理限流'),
        description: t('当前页限流最多。优先降 priority、缩 models，避免在限流状态下继续放量。'),
      },
      {
        key: 'unavailable',
        filter: 'unavailable',
        count: externalPoolPageStats.unavailable,
        title: t('先排不可用渠道'),
        description: t('当前页不可用渠道较多。建议打开“外部池异常前置”，优先处理最靠前的几条。'),
      },
      {
        key: 'degraded',
        filter: 'degraded',
        count: externalPoolPageStats.degraded,
        title: t('先查降级渠道'),
        description: t('当前页降级渠道较多。先看池状态弹窗里的错误账号数、限流状态和可用模型。'),
      },
    ];

    const top = candidates
      .filter((item) => item.count > 0)
      .sort((a, b) => b.count - a.count)[0];

    if (top) {
      return top;
    }

    if (externalPoolPageStats.available > 0) {
      return {
        kind: 'available',
        filter: 'available',
        title: t('可以继续做真实验证'),
        description: t('当前页外部池以“可用”为主。建议先从一条渠道开始，按顺序验证拉模型、/v1/models、/v1/responses。'),
      };
    }

    return {
      kind: 'unknown',
      filter: 'all',
      title: t('先看池状态弹窗'),
      description: t('当前页已有外部池渠道，但分布还不够明确。建议先打开池状态弹窗，再结合原始 JSON 和请求日志继续判断。'),
    };
  }, [externalPoolPageStats, t]);

  return {
    // Basic states
    channels,
    filteredChannels,
    loading,
    searching,
    activePage,
    pageSize,
    channelCount,
    groupOptions,
    idSort,
    enableTagMode,
    enableBatchDelete,
    statusFilter,
    externalPoolIssueFirst,
    externalPoolQuickFilter,
    filteredChannelCount,
    externalPoolPageStats,
    externalPoolNextAction,
    compactMode,
    globalPassThroughEnabled,

    // UI states
    showEdit,
    setShowEdit,
    editingChannel,
    setEditingChannel,
    showEditTag,
    setShowEditTag,
    editingTag,
    setEditingTag,
    selectedChannels,
    setSelectedChannels,
    setExternalPoolIssueFirst,
    setExternalPoolQuickFilter,
    showBatchSetTag,
    setShowBatchSetTag,
    batchSetTagValue,
    setBatchSetTagValue,

    // Column states
    visibleColumns,
    showColumnSelector,
    setShowColumnSelector,
    COLUMN_KEYS,

    // Type tab states
    activeTypeKey,
    setActiveTypeKey,
    typeCounts,
    channelTypeCounts,
    availableTypeKeys,

    // Model test states
    showModelTestModal,
    setShowModelTestModal,
    currentTestChannel,
    setCurrentTestChannel,
    modelSearchKeyword,
    setModelSearchKeyword,
    modelTestResults,
    testingModels,
    selectedModelKeys,
    setSelectedModelKeys,
    isBatchTesting,
    modelTablePage,
    setModelTablePage,
    selectedEndpointType,
    setSelectedEndpointType,
    isStreamTest,
    setIsStreamTest,
    allSelectingRef,

    // Multi-key management states
    showMultiKeyManageModal,
    setShowMultiKeyManageModal,
    currentMultiKeyChannel,
    setCurrentMultiKeyChannel,
    ...upstreamUpdates,

    // Form
    formApi,
    setFormApi,
    formInitValues,

    // Helpers
    t,
    isMobile,

    // Functions
    loadChannels,
    searchChannels,
    refresh,
    manageChannel,
    manageTag,
    handlePageChange,
    handlePageSizeChange,
    copySelectedChannel,
    updateChannelProperty,
    submitTagEdit,
    closeEdit,
    handleRow,
    batchSetChannelTag,
    batchDeleteChannels,
    testAllChannels,
    deleteAllDisabledChannels,
    updateAllChannelsBalance,
    updateChannelBalance,
    fixChannelsAbilities,
    checkOllamaVersion,
    testChannel,
    batchTestModels,
    handleCloseModal,
    getFormValues,

    // Column functions
    handleColumnVisibilityChange,
    handleSelectAll,
    initDefaultColumns,
    getDefaultColumnVisibility,

    // Setters
    setIdSort,
    setEnableTagMode,
    setEnableBatchDelete,
    setStatusFilter,
    setCompactMode,
    setActivePage,
  };
};
