import React from 'react';
import { Button, Tag, Typography } from '@douyinfe/semi-ui';
import { CalendarClock } from 'lucide-react';
import CompactModeToggle from '../../common/ui/CompactModeToggle';

const { Text } = Typography;

const STATUS_LABEL_MAP = {
  all: '全部状态',
  enabled: '已启用',
  disabled: '已禁用',
};

const SubscriptionsOverview = ({
  compactMode,
  setCompactMode,
  allPlans,
  filteredPlans,
  plans,
  statusFilter,
  onStatusFilterChange,
  onResetStatusFilter,
  enableEpay,
  t,
}) => {
  const sourcePlans = Array.isArray(allPlans) ? allPlans : [];
  const currentPlans = Array.isArray(plans) ? plans : [];
  const filtered = Array.isArray(filteredPlans) ? filteredPlans : [];

  const enabledCount = sourcePlans.filter((item) => item?.plan?.enabled === true).length;
  const disabledCount = sourcePlans.filter((item) => item?.plan?.enabled !== true).length;
  const stripeCount = sourcePlans.filter(
    (item) => String(item?.plan?.stripe_price_id || '').trim() !== '',
  ).length;
  const creemCount = sourcePlans.filter(
    (item) => String(item?.plan?.creem_product_id || '').trim() !== '',
  ).length;
  const upgradeCount = sourcePlans.filter(
    (item) => String(item?.plan?.upgrade_group || '').trim() !== '',
  ).length;

  const cards = [
    { key: 'all', label: t('套餐总数'), value: sourcePlans.length, clickable: true },
    { key: 'enabled', label: t('已启用'), value: enabledCount, clickable: true },
    { key: 'disabled', label: t('已禁用'), value: disabledCount, clickable: true },
    { key: 'stripe', label: 'Stripe', value: stripeCount, clickable: false },
    { key: 'creem', label: 'Creem', value: creemCount, clickable: false },
    {
      key: 'upgrade',
      label: t('升级分组'),
      value: upgradeCount,
      clickable: false,
    },
  ];

  return (
    <div className='flex flex-col gap-3 w-full'>
      <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
        <div className='flex items-center text-blue-500'>
          <CalendarClock size={16} className='mr-2' />
          <Text>{t('订阅管理')}</Text>
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
                    : t('全量统计')}
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
        <Tag color='cyan'>
          {t('当前匹配')}: {filtered.length}
        </Tag>
        <Tag color='green'>
          {t('当前页')}: {currentPlans.length}
        </Tag>
        <Tag color={enableEpay ? 'orange' : 'grey'}>
          {enableEpay ? t('易支付已开启') : t('易支付未开启')}
        </Tag>
        <Text type='secondary'>
          {statusFilter === 'all'
            ? t('当前展示全部套餐，可直接切换到启用或禁用视角')
            : t('当前按套餐状态过滤，可快速回到完整套餐列表')}
        </Text>
        <Button
          size='small'
          theme='light'
          disabled={statusFilter === 'all'}
          onClick={() => onResetStatusFilter?.()}
        >
          {t('清空筛选')}
        </Button>
      </div>
    </div>
  );
};

export default SubscriptionsOverview;
