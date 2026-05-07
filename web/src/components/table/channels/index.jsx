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

import React from 'react';
import { Banner, Button, Space, Tag } from '@douyinfe/semi-ui';
import { IconAlertTriangle } from '@douyinfe/semi-icons';
import CardPro from '../../common/ui/CardPro';
import ChannelsTable from './ChannelsTable';
import ChannelsActions from './ChannelsActions';
import ChannelsFilters from './ChannelsFilters';
import ChannelsTabs from './ChannelsTabs';
import { useChannelsData } from '../../../hooks/channels/useChannelsData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import BatchTagModal from './modals/BatchTagModal';
import ModelTestModal from './modals/ModelTestModal';
import ColumnSelectorModal from './modals/ColumnSelectorModal';
import EditChannelModal from './modals/EditChannelModal';
import EditTagModal from './modals/EditTagModal';
import MultiKeyManageModal from './modals/MultiKeyManageModal';
import ChannelUpstreamUpdateModal from './modals/ChannelUpstreamUpdateModal';
import { createCardProPagination } from '../../../helpers/utils';

const ChannelsPage = () => {
  const channelsData = useChannelsData();
  const isMobile = useIsMobile();
  const quickFilter = channelsData.externalPoolQuickFilter;

  const quickFilterTags = [
    { key: 'available', label: channelsData.t('可用'), count: channelsData.externalPoolPageStats.available, color: 'green' },
    { key: 'degraded', label: channelsData.t('降级'), count: channelsData.externalPoolPageStats.degraded, color: 'yellow' },
    { key: 'unavailable', label: channelsData.t('不可用'), count: channelsData.externalPoolPageStats.unavailable, color: 'red' },
    { key: 'auth_rejected', label: channelsData.t('认证失败'), count: channelsData.externalPoolPageStats.authRejected, color: 'red' },
    { key: 'empty_pool', label: channelsData.t('空池'), count: channelsData.externalPoolPageStats.emptyPool, color: 'orange' },
    { key: 'upstream_unreachable', label: channelsData.t('连接失败'), count: channelsData.externalPoolPageStats.upstreamUnreachable, color: 'red' },
    { key: 'upstream_path_not_found', label: channelsData.t('路径错误'), count: channelsData.externalPoolPageStats.upstreamPathNotFound, color: 'red' },
    { key: 'rate_limited', label: channelsData.t('限流'), count: channelsData.externalPoolPageStats.rateLimited, color: 'yellow' },
  ];

  const handleQuickFilterTagClick = (nextFilter) => {
    const target = quickFilter === nextFilter ? 'all' : nextFilter;
    localStorage.setItem('channel-external-pool-quick-filter', target);
    channelsData.setExternalPoolQuickFilter(target);
  };

  const applySuggestedFilter = () => {
    const target = channelsData.externalPoolNextAction.filter || 'all';
    localStorage.setItem('channel-external-pool-quick-filter', target);
    channelsData.setExternalPoolQuickFilter(target);
  };

  return (
    <>
      {/* Modals */}
      <ColumnSelectorModal {...channelsData} />
      <EditTagModal
        visible={channelsData.showEditTag}
        tag={channelsData.editingTag}
        handleClose={() => channelsData.setShowEditTag(false)}
        refresh={channelsData.refresh}
      />
      {channelsData.showEdit && (
        <EditChannelModal
          refresh={channelsData.refresh}
          visible={channelsData.showEdit}
          handleClose={channelsData.closeEdit}
          editingChannel={channelsData.editingChannel}
        />
      )}
      <BatchTagModal {...channelsData} />
      <ModelTestModal {...channelsData} />
      <MultiKeyManageModal
        visible={channelsData.showMultiKeyManageModal}
        onCancel={() => channelsData.setShowMultiKeyManageModal(false)}
        channel={channelsData.currentMultiKeyChannel}
        onRefresh={channelsData.refresh}
      />
      <ChannelUpstreamUpdateModal
        visible={channelsData.showUpstreamUpdateModal}
        addModels={channelsData.upstreamUpdateAddModels}
        removeModels={channelsData.upstreamUpdateRemoveModels}
        preferredTab={channelsData.upstreamUpdatePreferredTab}
        confirmLoading={channelsData.upstreamApplyLoading}
        onConfirm={channelsData.applyUpstreamUpdates}
        onCancel={channelsData.closeUpstreamUpdateModal}
      />

      {/* Main Content */}
      {channelsData.globalPassThroughEnabled ? (
        <Banner
          type='warning'
          closeIcon={null}
          icon={
            <IconAlertTriangle
              size='large'
              style={{ color: 'var(--semi-color-warning)' }}
            />
          }
          description={channelsData.t(
            '已开启全局请求透传：参数覆写、模型重定向、渠道适配等 NewAPI 内置功能将失效，非最佳实践；如因此产生问题，请勿提交 issue 反馈。',
          )}
          style={{ marginBottom: 12 }}
        />
      ) : null}
      {channelsData.externalPoolQuickFilter !== 'all' ? (
        <Banner
          type='info'
          closeIcon={null}
          description={channelsData.t(
            '当前启用了“当前页池快筛”：仅筛选本页已加载渠道，不改变后端分页总数。当前页命中 {{count}} 条。',
            { count: channelsData.filteredChannelCount },
          )}
          style={{ marginBottom: 12 }}
        />
      ) : null}
      {channelsData.externalPoolPageStats.total > 0 ? (
        <div
          className='mb-3 flex flex-wrap items-center gap-2'
          style={{ minHeight: 32 }}
        >
          <Tag
            color='grey'
            shape='circle'
            className='cursor-pointer select-none'
            type={quickFilter === 'all' ? 'solid' : 'light'}
            onClick={() => handleQuickFilterTagClick('all')}
          >
            {channelsData.t('当前页外部池')} {channelsData.externalPoolPageStats.total}
          </Tag>
          <Space wrap spacing={6}>
            {quickFilterTags.map((item) => (
              <Tag
                key={item.key}
                color={item.color}
                type={quickFilter === item.key ? 'solid' : 'light'}
                shape='circle'
                className='cursor-pointer select-none'
                onClick={() => handleQuickFilterTagClick(item.key)}
              >
                {item.label} {item.count}
              </Tag>
            ))}
          </Space>
        </div>
      ) : null}
      {channelsData.externalPoolPageStats.total > 0 ? (
        <div style={{ marginBottom: 12 }}>
          <Banner
            type='info'
            closeIcon={null}
            description={`${channelsData.externalPoolNextAction.title}：${channelsData.externalPoolNextAction.description}`}
          />
          <div className='mt-2 flex flex-wrap gap-2'>
            <Button
              size='small'
              type='primary'
              theme='solid'
              onClick={applySuggestedFilter}
            >
              {channelsData.t('按建议筛选')}
            </Button>
            <Button
              size='small'
              theme='outline'
              onClick={() => handleQuickFilterTagClick('all')}
            >
              {channelsData.t('清空快筛')}
            </Button>
          </div>
        </div>
      ) : null}
      <CardPro
        type='type3'
        tabsArea={<ChannelsTabs {...channelsData} />}
        actionsArea={<ChannelsActions {...channelsData} />}
        searchArea={<ChannelsFilters {...channelsData} />}
        paginationArea={createCardProPagination({
          currentPage: channelsData.activePage,
          pageSize: channelsData.pageSize,
          total: channelsData.channelCount,
          onPageChange: channelsData.handlePageChange,
          onPageSizeChange: channelsData.handlePageSizeChange,
          isMobile: isMobile,
          t: channelsData.t,
        })}
        t={channelsData.t}
      >
        <ChannelsTable {...channelsData} />
      </CardPro>
    </>
  );
};

export default ChannelsPage;
