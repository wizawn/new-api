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
import { Button, Tag, Typography } from '@douyinfe/semi-ui';
import { Key } from 'lucide-react';
import CompactModeToggle from '../../common/ui/CompactModeToggle';

const { Text } = Typography;

const STATUS_LABEL_MAP = {
  all: '全部状态',
  enabled: '已启用',
  disabled: '已禁用',
  expired: '已过期',
  exhausted: '已耗尽',
};

const TokensDescription = ({
  compactMode,
  setCompactMode,
  tokens,
  tokenCount,
  selectedKeys,
  statusFilter,
  onStatusFilterChange,
  getFormValues,
  setSelectedKeys,
  t,
}) => {
  const currentFilters =
    typeof getFormValues === 'function' ? getFormValues() : {};
  const keyword = String(currentFilters?.searchKeyword || '').trim();
  const tokenKeyword = String(currentFilters?.searchToken || '').trim();
  const filterSummary = [
    statusFilter && statusFilter !== 'all'
      ? `status=${statusFilter}`
      : '',
    keyword ? `keyword=${keyword}` : '',
    tokenKeyword ? `token=${tokenKeyword}` : '',
  ]
    .filter(Boolean)
    .join(' | ');

  const pageTokens = Array.isArray(tokens) ? tokens : [];
  const enabledCount = pageTokens.filter((item) => Number(item?.status) === 1).length;
  const disabledCount = pageTokens.filter((item) => Number(item?.status) === 2).length;
  const expiredCount = pageTokens.filter((item) => Number(item?.status) === 3).length;
  const exhaustedCount = pageTokens.filter((item) => Number(item?.status) === 4).length;
  const unlimitedCount = pageTokens.filter((item) => item?.unlimited_quota === true).length;
  const lowQuotaCount = pageTokens.filter((item) => {
    if (item?.unlimited_quota === true) {
      return false;
    }
    const used = Number(item?.used_quota) || 0;
    const remain = Number(item?.remain_quota) || 0;
    const total = used + remain;
    if (remain <= 0 || total <= 0) {
      return false;
    }
    return remain / total <= 0.1;
  }).length;

  const cards = [
    { key: 'all', label: t('总匹配'), value: tokenCount },
    { key: 'enabled', label: t('当前页启用'), value: enabledCount },
    { key: 'disabled', label: t('当前页禁用'), value: disabledCount },
    { key: 'expired', label: t('当前页过期'), value: expiredCount },
    { key: 'exhausted', label: t('当前页耗尽'), value: exhaustedCount },
    { key: 'low', label: t('低余额风险'), value: lowQuotaCount },
  ];

  return (
    <div className='flex flex-col gap-3 w-full'>
      <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
        <div className='flex items-center text-blue-500'>
          <Key size={16} className='mr-2' />
          <Text>{t('令牌管理')}</Text>
        </div>

        <CompactModeToggle
          compactMode={compactMode}
          setCompactMode={setCompactMode}
          t={t}
        />
      </div>

      <div className='grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-2'>
        {cards.map((item) => {
          const active = statusFilter === item.key;
          return (
            <button
              key={item.key}
              type='button'
              className={`text-left rounded-xl border px-4 py-3 transition ${
                active
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-950/30'
                  : 'border-[var(--semi-color-border)] bg-[var(--semi-color-bg-1)]'
              }`}
              onClick={() => {
                if (item.key === 'low') {
                  return;
                }
                onStatusFilterChange(item.key);
              }}
            >
              <Text type='secondary'>{item.label}</Text>
              <div style={{ marginTop: 6, fontSize: 24, fontWeight: 700 }}>
                {item.value}
              </div>
              <div style={{ marginTop: 4 }}>
                <Text type='tertiary'>
                  {item.key === 'low'
                    ? t('仅统计当前页')
                    : active
                      ? t('当前筛选')
                      : t('点击筛选')}
                </Text>
              </div>
            </button>
          );
        })}
      </div>

      <div className='flex flex-wrap items-center gap-2 rounded-xl border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] px-3 py-2'>
        <Tag color={statusFilter === 'all' ? 'grey' : 'blue'}>
          {t(STATUS_LABEL_MAP[statusFilter] || STATUS_LABEL_MAP.all)}
        </Tag>
        <Tag color='green'>
          {t('无限额度')}: {unlimitedCount}
        </Tag>
        <Tag color={selectedKeys?.length > 0 ? 'orange' : 'grey'}>
          {t('已选中')}: {selectedKeys?.length || 0}
        </Tag>
        <Text type='secondary'>
          {filterSummary || t('当前未设置搜索关键词，将按状态查看整个令牌池')}
        </Text>
        <Button
          size='small'
          theme='light'
          disabled={(selectedKeys?.length || 0) === 0}
          onClick={() => setSelectedKeys([])}
        >
          {t('清空已选')}
        </Button>
      </div>
    </div>
  );
};

export default TokensDescription;
