import React, { useMemo } from 'react';
import { Button, Card, Space, Tag, Typography } from '@douyinfe/semi-ui';
import {
  Activity,
  Blocks,
  BookText,
  CircleAlert,
  CreditCard,
  KeyRound,
  RefreshCcw,
  ServerCog,
  Settings2,
  ShieldCheck,
  Sparkles,
  SquareTerminal,
  Users,
  Wallet,
} from 'lucide-react';

const { Text } = Typography;

const DashboardWorkbench = ({
  userState,
  statusState,
  isAdminUser,
  navigate,
  CARD_PROPS,
  FLEX_CENTER_GAP2,
  dataExportDefaultTime,
  t,
}) => {
  const status = statusState?.status || {};
  const user = userState?.user || {};
  const quota = Number(user?.quota || 0);
  const requestCount = Number(user?.request_count || 0);

  const paymentMethods = [
    status?.enable_online_topup,
    status?.enable_stripe_topup,
    status?.enable_creem_topup,
    status?.enable_waffo_topup,
  ].filter(Boolean).length;

  const quickActions = useMemo(() => {
    if (isAdminUser) {
      return [
        {
          key: 'channel',
          title: t('渠道管理'),
          description: t('处理上游可用性与路由问题'),
          icon: <ServerCog size={16} />,
          to: '/console/channel',
        },
        {
          key: 'models',
          title: t('模型管理'),
          description: t('检查展示模型与供应商同步'),
          icon: <Blocks size={16} />,
          to: '/console/models',
        },
        {
          key: 'tokens',
          title: t('令牌管理'),
          description: t('处理配额、状态和密钥发放'),
          icon: <KeyRound size={16} />,
          to: '/console/token',
        },
        {
          key: 'logs',
          title: t('使用日志'),
          description: t('追踪报错、消费与请求详情'),
          icon: <Activity size={16} />,
          to: '/console/log',
        },
        {
          key: 'users',
          title: t('用户管理'),
          description: t('查看角色、状态和订阅归属'),
          icon: <Users size={16} />,
          to: '/console/user',
        },
        {
          key: 'subscriptions',
          title: t('订阅管理'),
          description: t('调整售卖套餐与启停状态'),
          icon: <CreditCard size={16} />,
          to: '/console/subscription',
        },
      ];
    }

    return [
      {
        key: 'tokens',
        title: t('令牌管理'),
        description: t('创建和维护可调用的访问令牌'),
        icon: <KeyRound size={16} />,
        to: '/console/token',
      },
      {
        key: 'logs',
        title: t('使用日志'),
        description: t('查看最近调用、报错与消费'),
        icon: <Activity size={16} />,
        to: '/console/log',
      },
      {
        key: 'wallet',
        title: t('钱包管理'),
        description: t('充值、兑换和查看套餐权益'),
        icon: <Wallet size={16} />,
        to: '/console/topup',
      },
      {
        key: 'playground',
        title: t('操练场'),
        description: t('快速验证模型、参数和响应'),
        icon: <SquareTerminal size={16} />,
        to: '/console/playground',
      },
      {
        key: 'personal',
        title: t('个人设置'),
        description: t('管理账户安全和通知偏好'),
        icon: <Settings2 size={16} />,
        to: '/console/personal',
      },
      {
        key: 'pricing',
        title: t('价格页'),
        description: t('查看模型价格和可用能力'),
        icon: <BookText size={16} />,
        to: '/pricing',
      },
    ];
  }, [isAdminUser, t]);

  const systemTags = [
    {
      key: 'role',
      label: t('角色'),
      value: isAdminUser ? t('管理员') : t('普通用户'),
      color: isAdminUser ? 'orange' : 'blue',
    },
    {
      key: 'time',
      label: t('统计视角'),
      value:
        dataExportDefaultTime === 'week'
          ? t('周')
          : dataExportDefaultTime === 'day'
            ? t('天')
            : t('小时'),
      color: 'cyan',
    },
    {
      key: 'payment',
      label: t('支付通道'),
      value:
        paymentMethods > 0
          ? t('已启用 {{count}} 个', { count: paymentMethods })
          : t('未开启'),
      color: paymentMethods > 0 ? 'green' : 'red',
    },
    {
      key: 'api',
      label: t('API信息'),
      value: status?.api_info_enabled ?? true ? t('开启') : t('关闭'),
      color: status?.api_info_enabled ?? true ? 'blue' : 'grey',
    },
    {
      key: 'uptime',
      label: t('可用性监控'),
      value: status?.uptime_kuma_enabled ?? true ? t('开启') : t('关闭'),
      color: status?.uptime_kuma_enabled ?? true ? 'green' : 'grey',
    },
    {
      key: 'announce',
      label: t('公告'),
      value: status?.announcements_enabled ?? true ? t('开启') : t('关闭'),
      color: status?.announcements_enabled ?? true ? 'violet' : 'grey',
    },
  ];

  const recommendations = useMemo(() => {
    const next = [];

    if (quota <= 0) {
      next.push({
        key: 'quota-empty',
        title: t('余额已耗尽'),
        description: t('继续调用前需要先充值、兑换或切换到订阅额度'),
        buttonText: t('去钱包管理'),
        icon: <CircleAlert size={16} />,
        tone: 'danger',
        to: '/console/topup',
      });
    }

    if (requestCount === 0) {
      next.push({
        key: 'cold-start',
        title: t('当前账号还没有调用记录'),
        description: t('先创建令牌，再到操练场做一次实际请求验证'),
        buttonText: t('去令牌管理'),
        icon: <Sparkles size={16} />,
        tone: 'primary',
        to: '/console/token',
      });
    }

    if (isAdminUser && paymentMethods === 0) {
      next.push({
        key: 'payment-disabled',
        title: t('当前未开启任何支付通道'),
        description: t('用户侧钱包只能兑换，无法在线购买或续费'),
        buttonText: t('去系统设置'),
        icon: <CreditCard size={16} />,
        tone: 'warning',
        to: '/console/setting',
      });
    }

    if (isAdminUser && !(status?.uptime_kuma_enabled ?? true)) {
      next.push({
        key: 'uptime-hidden',
        title: t('首页未展示可用性监控'),
        description: t('如果依赖状态页做运维观察，建议重新开启监控面板'),
        buttonText: t('去系统设置'),
        icon: <RefreshCcw size={16} />,
        tone: 'warning',
        to: '/console/setting',
      });
    }

    if (next.length === 0) {
      next.push({
        key: 'healthy',
        title: t('当前首页运行信号正常'),
        description: isAdminUser
          ? t('可以从左侧快捷入口继续处理渠道、模型和订阅配置')
          : t('可以直接从令牌管理或操练场开始日常使用'),
        buttonText: isAdminUser ? t('去渠道管理') : t('去操练场'),
        icon: <ShieldCheck size={16} />,
        tone: 'success',
        to: isAdminUser ? '/console/channel' : '/console/playground',
      });
    }

    return next.slice(0, 3);
  }, [quota, requestCount, isAdminUser, paymentMethods, status, t]);

  return (
    <div className='mb-4'>
      <div className='grid grid-cols-1 xl:grid-cols-5 gap-4'>
        <Card
          {...CARD_PROPS}
          className='!rounded-2xl xl:col-span-3 border-0 bg-[linear-gradient(135deg,#f8fbff_0%,#eef6ff_45%,#ffffff_100%)]'
          title={
            <div className={FLEX_CENTER_GAP2}>
              <Sparkles size={16} />
              {t('首页工作台')}
            </div>
          }
        >
          <div className='grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3'>
            {quickActions.map((item) => (
              <button
                key={item.key}
                type='button'
                onClick={() => navigate(item.to)}
                className='group rounded-2xl border border-[var(--semi-color-border)] bg-white/80 px-4 py-4 text-left transition hover:-translate-y-0.5 hover:border-blue-300 hover:shadow-sm'
              >
                <div className='flex items-center justify-between gap-3'>
                  <div className='flex items-center gap-2 text-[var(--semi-color-text-0)]'>
                    <span className='flex h-9 w-9 items-center justify-center rounded-full bg-slate-100 text-slate-700'>
                      {item.icon}
                    </span>
                    <span className='font-semibold'>{item.title}</span>
                  </div>
                  <span className='text-xs text-[var(--semi-color-text-2)] transition group-hover:text-blue-600'>
                    {t('进入')}
                  </span>
                </div>
                <div className='mt-3'>
                  <Text type='secondary'>{item.description}</Text>
                </div>
              </button>
            ))}
          </div>
        </Card>

        <Card
          {...CARD_PROPS}
          className='!rounded-2xl xl:col-span-2 border-0 bg-[linear-gradient(180deg,#ffffff_0%,#f6f8fb_100%)]'
          title={
            <div className={FLEX_CENTER_GAP2}>
              <ShieldCheck size={16} />
              {t('运行状态')}
            </div>
          }
        >
          <div className='flex flex-col gap-3'>
            <Space wrap>
              {systemTags.map((item) => (
                <Tag key={item.key} color={item.color} size='large'>
                  {item.label}: {item.value}
                </Tag>
              ))}
            </Space>

            <div className='grid grid-cols-1 gap-3'>
              {recommendations.map((item) => {
                const toneClass =
                  item.tone === 'danger'
                    ? 'border-red-200 bg-red-50'
                    : item.tone === 'warning'
                      ? 'border-amber-200 bg-amber-50'
                      : item.tone === 'success'
                        ? 'border-emerald-200 bg-emerald-50'
                        : 'border-blue-200 bg-blue-50';

                return (
                  <div
                    key={item.key}
                    className={`rounded-2xl border px-4 py-3 ${toneClass}`}
                  >
                    <div className='flex items-start justify-between gap-3'>
                      <div className='flex gap-3'>
                        <div className='mt-0.5 text-[var(--semi-color-text-0)]'>
                          {item.icon}
                        </div>
                        <div>
                          <div className='font-semibold text-[var(--semi-color-text-0)]'>
                            {item.title}
                          </div>
                          <div className='mt-1'>
                            <Text type='secondary'>{item.description}</Text>
                          </div>
                        </div>
                      </div>
                      <Button
                        size='small'
                        theme='solid'
                        type='primary'
                        onClick={() => navigate(item.to)}
                      >
                        {item.buttonText}
                      </Button>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
};

export default DashboardWorkbench;
