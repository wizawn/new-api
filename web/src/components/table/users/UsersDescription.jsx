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
import { Button, Space, Tag, Typography } from '@douyinfe/semi-ui';
import { IconUserAdd } from '@douyinfe/semi-icons';
import CompactModeToggle from '../../common/ui/CompactModeToggle';

const { Text } = Typography;

const STATUS_LABEL_MAP = {
  all: '全部状态',
  enabled: '已启用',
  disabled: '已禁用',
  deleted: '已注销',
};

const ROLE_LABEL_MAP = {
  all: '全部角色',
  common: '普通用户',
  admin: '管理员',
  root: '超级管理员',
};

const ROLE_FILTERS = ['all', 'common', 'admin', 'root'];

const UsersDescription = ({
  compactMode,
  setCompactMode,
  users,
  userCount,
  statusFilter,
  roleFilter,
  onStatusFilterChange,
  onRoleFilterChange,
  getFormValues,
  resetAllFilters,
  t,
}) => {
  const pageUsers = Array.isArray(users) ? users : [];
  const isDeletedUser = (item) =>
    item?.DeletedAt !== null && item?.DeletedAt !== undefined;
  const enabledCount = pageUsers.filter(
    (item) => !isDeletedUser(item) && Number(item?.status) === 1,
  ).length;
  const disabledCount = pageUsers.filter(
    (item) => !isDeletedUser(item) && Number(item?.status) === 2,
  ).length;
  const deletedCount = pageUsers.filter((item) => isDeletedUser(item)).length;
  const adminCount = pageUsers.filter((item) => Number(item?.role) === 10).length;
  const rootCount = pageUsers.filter((item) => Number(item?.role) === 100).length;

  const currentFilters =
    typeof getFormValues === 'function' ? getFormValues() : {};
  const keyword = String(currentFilters?.searchKeyword || '').trim();
  const group = String(currentFilters?.searchGroup || '').trim();
  const filterSummary = [
    statusFilter !== 'all' ? `status=${statusFilter}` : '',
    roleFilter !== 'all' ? `role=${roleFilter}` : '',
    keyword ? `keyword=${keyword}` : '',
    group ? `group=${group}` : '',
  ]
    .filter(Boolean)
    .join(' | ');

  const cards = [
    { key: 'all', label: t('总匹配'), value: userCount, clickable: true },
    {
      key: 'enabled',
      label: t('当前页启用'),
      value: enabledCount,
      clickable: true,
    },
    {
      key: 'disabled',
      label: t('当前页禁用'),
      value: disabledCount,
      clickable: true,
    },
    {
      key: 'deleted',
      label: t('当前页已注销'),
      value: deletedCount,
      clickable: true,
    },
    { key: 'admins', label: t('管理员'), value: adminCount, clickable: false },
    {
      key: 'roots',
      label: t('超级管理员'),
      value: rootCount,
      clickable: false,
    },
  ];

  return (
    <div className='flex flex-col gap-3 w-full'>
      <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
        <div className='flex items-center text-blue-500'>
          <IconUserAdd className='mr-2' />
          <Text>{t('用户管理')}</Text>
        </div>
        <CompactModeToggle
          compactMode={compactMode}
          setCompactMode={setCompactMode}
          t={t}
        />
      </div>

      <Space wrap>
        {ROLE_FILTERS.map((item) => (
          <Button
            key={item}
            size='small'
            theme={roleFilter === item ? 'solid' : 'light'}
            type={item === 'all' ? 'tertiary' : 'primary'}
            onClick={() => onRoleFilterChange?.(item)}
          >
            {t(ROLE_LABEL_MAP[item])}
          </Button>
        ))}
      </Space>

      <div className='grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-2'>
        {cards.map((item) => {
          const active = statusFilter === item.key;
          return (
            <button
              key={item.key}
              type='button'
              disabled={!item.clickable}
              className={`text-left rounded-xl border px-4 py-3 transition ${
                active
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-950/30'
                  : 'border-[var(--semi-color-border)] bg-[var(--semi-color-bg-1)]'
              } ${item.clickable ? 'cursor-pointer' : 'cursor-default'}`}
              onClick={() => {
                if (item.clickable) {
                  onStatusFilterChange?.(item.key);
                }
              }}
            >
              <Text type='secondary'>{item.label}</Text>
              <div style={{ marginTop: 6, fontSize: 24, fontWeight: 700 }}>
                {item.value}
              </div>
              <div style={{ marginTop: 4 }}>
                <Text type='tertiary'>
                  {item.clickable
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
          {t(STATUS_LABEL_MAP[statusFilter] || STATUS_LABEL_MAP.all)}
        </Tag>
        <Tag color={roleFilter === 'all' ? 'grey' : 'orange'}>
          {t(ROLE_LABEL_MAP[roleFilter] || ROLE_LABEL_MAP.all)}
        </Tag>
        <Tag color='cyan'>
          {t('当前页')}: {pageUsers.length}
        </Tag>
        <Text type='secondary'>
          {filterSummary || t('当前未设置关键词、分组或角色筛选，展示用户池的当前分页结果')}
        </Text>
        <Button size='small' theme='light' onClick={() => resetAllFilters?.()}>
          {t('清空筛选')}
        </Button>
      </div>
    </div>
  );
};

export default UsersDescription;
