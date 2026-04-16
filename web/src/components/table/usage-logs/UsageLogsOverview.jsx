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
import { Button, Space, Tag, Typography } from '@douyinfe/semi-ui';
import { ActivitySquare } from 'lucide-react';

const { Text } = Typography;

const LOG_TYPE_LABELS = {
  0: '全部',
  1: '充值',
  2: '消费',
  3: '管理',
  4: '系统',
  5: '错误',
  6: '退款',
};

const QUICK_LOG_TYPES = [0, 2, 5, 3, 6];

const UsageLogsOverview = ({
  logs,
  logCount,
  logType,
  formApi,
  getFormValues,
  setLogType,
  refresh,
  isAdminUser,
  t,
}) => {
  const pageLogs = Array.isArray(logs) ? logs : [];
  const typeCounts = useMemo(() => {
    return pageLogs.reduce(
      (acc, item) => {
        const type = Number(item?.type) || 0;
        acc[type] = (acc[type] || 0) + 1;
        return acc;
      },
      { 0: pageLogs.length },
    );
  }, [pageLogs]);

  const uniqueRequestCount = useMemo(() => {
    return new Set(
      pageLogs
        .map((item) => String(item?.request_id || '').trim())
        .filter(Boolean),
    ).size;
  }, [pageLogs]);

  const uniqueModelCount = useMemo(() => {
    return new Set(
      pageLogs
        .map((item) => String(item?.model_name || '').trim())
        .filter(Boolean),
    ).size;
  }, [pageLogs]);

  const uniqueChannelCount = useMemo(() => {
    return new Set(
      pageLogs
        .map((item) => String(item?.channel || '').trim())
        .filter(Boolean),
    ).size;
  }, [pageLogs]);

  const formValues =
    typeof getFormValues === 'function' ? getFormValues() : {};
  const filterSummary = [
    Number(logType) > 0 ? `type=${t(LOG_TYPE_LABELS[Number(logType)] || '全部')}` : '',
    String(formValues?.token_name || '').trim()
      ? `token=${String(formValues.token_name).trim()}`
      : '',
    String(formValues?.model_name || '').trim()
      ? `model=${String(formValues.model_name).trim()}`
      : '',
    String(formValues?.group || '').trim()
      ? `group=${String(formValues.group).trim()}`
      : '',
    String(formValues?.request_id || '').trim()
      ? `request_id=${String(formValues.request_id).trim()}`
      : '',
    isAdminUser && String(formValues?.channel || '').trim()
      ? `channel=${String(formValues.channel).trim()}`
      : '',
    isAdminUser && String(formValues?.username || '').trim()
      ? `user=${String(formValues.username).trim()}`
      : '',
  ]
    .filter(Boolean)
    .join(' | ');

  const dateSummary =
    Array.isArray(formValues?.dateRange) && formValues.dateRange.length === 2
      ? `${String(formValues.dateRange[0] || '').trim()} ~ ${String(formValues.dateRange[1] || '').trim()}`
      : `${String(formValues?.start_timestamp || '').trim()} ~ ${String(formValues?.end_timestamp || '').trim()}`;

  const summaryCards = [
    { key: 'total', label: t('总匹配'), value: logCount },
    { key: 'page', label: t('当前页'), value: pageLogs.length },
    { key: 'consume', label: t('消费日志'), value: typeCounts[2] || 0 },
    { key: 'error', label: t('错误日志'), value: typeCounts[5] || 0 },
    {
      key: 'unique',
      label: isAdminUser ? t('涉及渠道') : t('涉及模型'),
      value: isAdminUser ? uniqueChannelCount : uniqueModelCount,
    },
    { key: 'request', label: t('Request ID'), value: uniqueRequestCount },
  ];

  const applyQuickLogType = async (nextType) => {
    formApi?.setValue?.('logType', String(nextType));
    setLogType(nextType);
    setTimeout(() => {
      refresh();
    }, 0);
  };

  const resetFilters = async () => {
    formApi?.reset?.();
    setLogType(0);
    setTimeout(() => {
      refresh();
    }, 100);
  };

  return (
    <div className='flex flex-col gap-3 w-full'>
      <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
        <div className='flex items-center text-emerald-500'>
          <ActivitySquare size={16} className='mr-2' />
          <Text>{t('用量日志工作台')}</Text>
        </div>
        <Space wrap>
          {QUICK_LOG_TYPES.map((type) => {
            const active = Number(logType) === type;
            return (
              <Button
                key={`log-type-${type}`}
                size='small'
                theme={active ? 'solid' : 'light'}
                type={type === 5 ? 'danger' : type === 2 ? 'primary' : 'tertiary'}
                onClick={() => applyQuickLogType(type)}
              >
                {t(LOG_TYPE_LABELS[type])}
              </Button>
            );
          })}
        </Space>
      </div>

      <div className='grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-2'>
        {summaryCards.map((item) => (
          <div
            key={item.key}
            className='rounded-xl border border-[var(--semi-color-border)] bg-[var(--semi-color-bg-1)] px-4 py-3'
          >
            <Text type='secondary'>{item.label}</Text>
            <div style={{ marginTop: 6, fontSize: 24, fontWeight: 700 }}>
              {item.value}
            </div>
            <div style={{ marginTop: 4 }}>
              <Text type='tertiary'>{t('当前筛选视角')}</Text>
            </div>
          </div>
        ))}
      </div>

      <div className='flex flex-wrap items-center gap-2 rounded-xl border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] px-3 py-2'>
        <Tag color={Number(logType) > 0 ? 'blue' : 'grey'}>
          {t(LOG_TYPE_LABELS[Number(logType)] || '全部')}
        </Tag>
        <Tag color='cyan'>{dateSummary || '-'}</Tag>
        <Text type='secondary'>
          {filterSummary || t('当前未附加关键词过滤，展示该时间范围内的日志结果')}
        </Text>
        <Button size='small' theme='light' onClick={resetFilters}>
          {t('清空筛选')}
        </Button>
      </div>
    </div>
  );
};

export default UsageLogsOverview;
