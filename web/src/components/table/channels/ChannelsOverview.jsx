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

import React, { useMemo } from 'react';
import { Button, Tag, Typography } from '@douyinfe/semi-ui';
import { Activity } from 'lucide-react';
import { CHANNEL_OPTIONS } from '../../../constants';

const { Text } = Typography;

const ChannelsOverview = ({
  channels,
  channelCount,
  selectedChannels,
  enableTagMode,
  statusFilter,
  setStatusFilter,
  activeTypeKey,
  setActivePage,
  pageSize,
  idSort,
  loadChannels,
  getFormValues,
  formApi,
  t,
}) => {
  const flatChannels = useMemo(() => {
    const source = Array.isArray(channels) ? channels : [];
    return source.flatMap((item) => {
      if (Array.isArray(item?.children) && item.children.length > 0) {
        return item.children;
      }
      return item ? [item] : [];
    });
  }, [channels]);

  const enabledCount = flatChannels.filter((item) => Number(item?.status) === 1).length;
  const disabledCount = flatChannels.filter((item) => Number(item?.status) === 2).length;
  const autoDisabledCount = flatChannels.filter((item) => Number(item?.status) === 3).length;
  const multiKeyCount = flatChannels.filter(
    (item) => item?.channel_info?.is_multi_key === true,
  ).length;
  const untestedCount = flatChannels.filter(
    (item) => Number(item?.response_time) === 0,
  ).length;
  const slowCount = flatChannels.filter(
    (item) => Number(item?.response_time) > 3000,
  ).length;

  const formValues =
    typeof getFormValues === 'function' ? getFormValues() : {};
  const searchKeyword = String(formValues?.searchKeyword || '').trim();
  const searchGroup = String(formValues?.searchGroup || '').trim();
  const searchModel = String(formValues?.searchModel || '').trim();

  const currentTypeLabel =
    activeTypeKey === 'all'
      ? t('全部类型')
      : CHANNEL_OPTIONS.find(
          (item) => String(item?.value) === String(activeTypeKey),
        )?.label || activeTypeKey;

  const filterSummary = [
    statusFilter !== 'all' ? `status=${statusFilter}` : '',
    activeTypeKey !== 'all' ? `type=${currentTypeLabel}` : '',
    searchKeyword ? `keyword=${searchKeyword}` : '',
    searchGroup ? `group=${searchGroup}` : '',
    searchModel ? `model=${searchModel}` : '',
  ]
    .filter(Boolean)
    .join(' | ');

  const overviewCards = [
    { key: 'all', label: t('总匹配'), value: channelCount },
    { key: 'enabled', label: t('当前页启用'), value: enabledCount },
    { key: 'disabled', label: t('当前页禁用'), value: disabledCount },
    { key: 'slow', label: t('慢响应'), value: slowCount },
    { key: 'untested', label: t('未测试'), value: untestedCount },
    { key: 'multi_key', label: t('多 Key'), value: multiKeyCount },
  ];

  const applyStatusFilter = async (nextStatus) => {
    if (!nextStatus) {
      return;
    }
    localStorage.setItem('channel-status-filter', nextStatus);
    setStatusFilter(nextStatus);
    setActivePage(1);
    await loadChannels(1, pageSize, idSort, enableTagMode, activeTypeKey, nextStatus);
  };

  return (
    <div className='flex flex-col gap-3 w-full'>
      <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
        <div className='flex items-center text-cyan-500'>
          <Activity size={16} className='mr-2' />
          <Text>{t('渠道管理总览')}</Text>
        </div>
        <Tag color={enableTagMode ? 'orange' : 'blue'}>
          {enableTagMode ? t('当前为标签聚合模式') : t('当前为单渠道视图')}
        </Tag>
      </div>

      <div className='grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-2'>
        {overviewCards.map((item) => {
          const active =
            (item.key === 'all' && statusFilter === 'all') ||
            (item.key === 'enabled' && statusFilter === 'enabled') ||
            (item.key === 'disabled' && statusFilter === 'disabled');
          const clickable = ['all', 'enabled', 'disabled'].includes(item.key);
          return (
            <button
              key={item.key}
              type='button'
              disabled={!clickable}
              className={`text-left rounded-xl border px-4 py-3 transition ${
                active
                  ? 'border-cyan-500 bg-cyan-50 dark:bg-cyan-950/30'
                  : 'border-[var(--semi-color-border)] bg-[var(--semi-color-bg-1)]'
              } ${clickable ? 'cursor-pointer' : 'cursor-default'}`}
              onClick={() => {
                if (item.key === 'all') {
                  applyStatusFilter('all');
                } else if (item.key === 'enabled') {
                  applyStatusFilter('enabled');
                } else if (item.key === 'disabled') {
                  applyStatusFilter('disabled');
                }
              }}
            >
              <Text type='secondary'>{item.label}</Text>
              <div style={{ marginTop: 6, fontSize: 24, fontWeight: 700 }}>
                {item.value}
              </div>
              <div style={{ marginTop: 4 }}>
                <Text type='tertiary'>
                  {clickable
                    ? active
                      ? t('当前筛选')
                      : t('点击筛选')
                    : t('仅统计当前页')}
                </Text>
              </div>
            </button>
          );
        })}
      </div>

      <div className='flex flex-wrap items-center gap-2 rounded-xl border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] px-3 py-2'>
        <Tag color={statusFilter === 'all' ? 'grey' : 'blue'}>
          {statusFilter === 'all'
            ? t('全部状态')
            : statusFilter === 'enabled'
              ? t('已启用')
              : t('已禁用 / 自动禁用')}
        </Tag>
        <Tag color='orange'>
          {t('自动禁用')}: {autoDisabledCount}
        </Tag>
        <Tag color={selectedChannels?.length > 0 ? 'red' : 'grey'}>
          {t('已选中')}: {selectedChannels?.length || 0}
        </Tag>
        <Text type='secondary'>
          {filterSummary || t('当前未设置搜索条件，展示整个渠道池的当前分页结果')}
        </Text>
        <Button
          size='small'
          theme='light'
          disabled={!searchKeyword && !searchGroup && !searchModel}
          onClick={async () => {
            formApi?.reset?.();
            setActivePage(1);
            await loadChannels(1, pageSize, idSort, enableTagMode, activeTypeKey, statusFilter);
          }}
        >
          {t('清空搜索')}
        </Button>
      </div>
    </div>
  );
};

export default ChannelsOverview;
