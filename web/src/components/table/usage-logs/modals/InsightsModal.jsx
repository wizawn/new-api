import React from 'react';
import { Empty, Modal, Space, Tag, Typography } from '@douyinfe/semi-ui';

const { Text, Paragraph } = Typography;

const InsightSection = ({ title, items, emptyText, showLongText = false }) => {
  return (
    <div className='rounded-xl border border-[var(--semi-color-border)] p-4'>
      <div className='flex items-center justify-between mb-3'>
        <Text strong>{title}</Text>
        <Tag color='grey'>{items.length}</Tag>
      </div>
      {items.length === 0 ? (
        <Empty description={emptyText} image={null} style={{ padding: 12 }} />
      ) : (
        <div className='flex flex-col gap-2'>
          {items.map((item, index) => (
            <div
              key={`${title}-${item.key || item.label || index}`}
              className='rounded-lg bg-[var(--semi-color-fill-0)] px-3 py-2'
            >
              <div className='flex items-start justify-between gap-3'>
                <div className='min-w-0 flex-1'>
                  <Text type='tertiary'>#{index + 1}</Text>
                  {showLongText ? (
                    <Paragraph
                      style={{ margin: '4px 0 0 0' }}
                      ellipsis={{ rows: 2, showTooltip: true }}
                    >
                      {item.label || '-'}
                    </Paragraph>
                  ) : (
                    <Text
                      style={{
                        display: 'block',
                        marginTop: 4,
                        wordBreak: 'break-word',
                      }}
                    >
                      {item.label || '-'}
                    </Text>
                  )}
                </div>
                <Tag color='blue'>{item.count}</Tag>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

const InsightsModal = ({
  showInsightsModal,
  setShowInsightsModal,
  loadingInsights,
  insightsData,
  isAdminUser,
  t,
}) => {
  return (
    <Modal
      title={t('失败热点聚合')}
      visible={showInsightsModal}
      onCancel={() => setShowInsightsModal(false)}
      footer={null}
      width={900}
    >
      <Space vertical align='start' style={{ width: '100%' }} spacing='medium'>
        <Text type='secondary'>
          {t('基于当前日志筛选条件聚合，帮助快速定位最常出现的模型、渠道和日志内容热点。')}
        </Text>

        <div className='grid grid-cols-1 lg:grid-cols-2 gap-3 w-full'>
          <InsightSection
            title={t('模型热点')}
            items={Array.isArray(insightsData?.models) ? insightsData.models : []}
            emptyText={t('暂无模型聚合结果')}
          />
          {isAdminUser ? (
            <InsightSection
              title={t('渠道热点')}
              items={Array.isArray(insightsData?.channels) ? insightsData.channels : []}
              emptyText={t('暂无渠道聚合结果')}
            />
          ) : null}
        </div>

        <InsightSection
          title={t('日志内容热点')}
          items={Array.isArray(insightsData?.contents) ? insightsData.contents : []}
          emptyText={t('暂无日志内容聚合结果')}
          showLongText
        />

        {loadingInsights ? (
          <Text type='secondary'>{t('正在加载聚合数据...')}</Text>
        ) : null}
      </Space>
    </Modal>
  );
};

export default InsightsModal;
