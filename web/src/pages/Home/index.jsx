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

import React, { useContext, useEffect, useMemo, useState } from 'react';
import { Typography } from '@douyinfe/semi-ui';
import { API, copy, showError, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { StatusContext } from '../../context/Status';
import { useActualTheme } from '../../context/Theme';
import { marked } from 'marked';
import { useTranslation } from 'react-i18next';
import { IconCopy, IconFile, IconPlay } from '@douyinfe/semi-icons';
import { Link } from 'react-router-dom';
import NoticeModal from '../../components/layout/NoticeModal';
import {
  AzureAI,
  Claude,
  Cohere,
  DeepSeek,
  Gemini,
  Grok,
  Hunyuan,
  Minimax,
  Midjourney,
  Moonshot,
  OpenAI,
  Qwen,
  Qingyan,
  Spark,
  Suno,
  Volcengine,
  Wenxin,
  XAI,
  Xinference,
  Zhipu,
} from '@lobehub/icons';

const { Text, Title, Paragraph } = Typography;

const PROVIDER_SHOWCASE = [
  { name: 'Moonshot', Icon: Moonshot },
  { name: 'OpenAI', Icon: OpenAI },
  { name: 'xAI', Icon: XAI },
  { name: 'Zhipu', Icon: Zhipu.Color },
  { name: 'Volcengine', Icon: Volcengine.Color },
  { name: 'Cohere', Icon: Cohere.Color },
  { name: 'Spark', Icon: Spark.Color },
  { name: 'Gemini', Icon: Gemini.Color },
  { name: 'Suno', Icon: Suno },
  { name: 'Minimax', Icon: Minimax.Color },
  { name: 'Wenxin', Icon: Wenxin.Color },
  { name: 'Qingyan', Icon: Qingyan.Color },
  { name: 'Qwen', Icon: Qwen.Color },
  { name: 'DeepSeek', Icon: DeepSeek.Color },
  { name: 'Hunyuan', Icon: Hunyuan.Color },
  { name: 'Midjourney', Icon: Midjourney },
  { name: 'Grok', Icon: Grok },
  { name: 'Azure AI', Icon: AzureAI.Color },
  { name: 'Xinference', Icon: Xinference.Color },
  { name: 'Claude', Icon: Claude.Color },
];

const Home = () => {
  const { t, i18n } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const isMobile = useIsMobile();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState('');
  const [noticeVisible, setNoticeVisible] = useState(false);
  const serverAddress =
    statusState?.status?.server_address || `${window.location.origin}`;
  const isSelfUseMode = statusState?.status?.self_use_mode_enabled || false;
  const isEnglish = i18n.language.startsWith('en');

  const endpointPreview = useMemo(
    () => `${serverAddress.replace(/\/$/, '')}/v1/messages`,
    [serverAddress],
  );

  const displayHomePageContent = async () => {
    setHomePageContent(localStorage.getItem('home_page_content') || '');
    const res = await API.get('/api/home_page_content');
    const { success, message, data } = res.data;
    if (success) {
      let content = data;
      if (!data.startsWith('https://')) {
        content = marked.parse(data);
      }
      setHomePageContent(content);
      localStorage.setItem('home_page_content', content);

      if (data.startsWith('https://')) {
        const iframe = document.querySelector('iframe');
        if (iframe) {
          iframe.onload = () => {
            iframe.contentWindow.postMessage({ themeMode: actualTheme }, '*');
            iframe.contentWindow.postMessage({ lang: i18n.language }, '*');
          };
        }
      }
    } else {
      showError(message);
      setHomePageContent(t('加载首页内容失败...'));
    }
    setHomePageContentLoaded(true);
  };

  useEffect(() => {
    const checkNoticeAndShow = async () => {
      const lastCloseDate = localStorage.getItem('notice_close_date');
      const today = new Date().toDateString();
      if (lastCloseDate !== today) {
        try {
          const res = await API.get('/api/notice');
          const { success, data } = res.data;
          if (success && data && data.trim() !== '') {
            setNoticeVisible(true);
          }
        } catch (error) {
          console.error('获取公告失败:', error);
        }
      }
    };

    checkNoticeAndShow();
  }, []);

  useEffect(() => {
    displayHomePageContent().then();
  }, []);

  const handleCopy = async (value) => {
    const ok = await copy(value);
    if (ok) {
      showSuccess(t('已复制到剪切板'));
    }
  };

  const renderProviderVisual = (provider) => {
    const ProviderIcon = provider.Icon;
    if (!ProviderIcon) {
      return (
        <span className='inline-flex h-9 w-9 items-center justify-center rounded-full border border-slate-200 bg-white text-sm font-semibold text-slate-700'>
          {(provider?.name || '?').slice(0, 1).toUpperCase()}
        </span>
      );
    }
    return <ProviderIcon size={34} />;
  };

  if (homePageContentLoaded && homePageContent !== '') {
    return (
      <div className='overflow-x-hidden w-full'>
        {homePageContent.startsWith('https://') ? (
          <iframe
            src={homePageContent}
            className='w-full h-screen border-none'
            title='home-page-content'
          />
        ) : (
          <div
            className='mt-[60px]'
            dangerouslySetInnerHTML={{ __html: homePageContent }}
          />
        )}
      </div>
    );
  }

  return (
    <div className='relative w-full overflow-x-hidden pb-14 pt-20 sm:pt-24'>
      <NoticeModal
        visible={noticeVisible}
        onClose={() => setNoticeVisible(false)}
        isMobile={isMobile}
      />

      <div className='pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(circle_at_top,rgba(59,130,246,0.14),transparent_40%),linear-gradient(180deg,#f8fbff_0%,#f2f7ff_46%,#f8fbff_100%)]' />

      <section className='mx-auto max-w-5xl px-4 sm:px-6'>
        <div className='text-center'>
          <Title
            heading={1}
            className='!mb-2 !text-[56px] !leading-[0.98] !tracking-[-0.04em] !text-slate-900 md:!text-[80px]'
          >
            {isEnglish ? 'Unified' : '统一的'}
            <br />
            {isEnglish ? 'LLM Gateway' : '大模型接口网关'}
          </Title>

          <Paragraph className='!mx-auto !mb-0 !mt-6 !max-w-3xl !text-xl !leading-8 !text-slate-600'>
            {isEnglish
              ? 'Better price, better stability, only switch your base URL.'
              : '更好的价格，更好的稳定性，只需要将模型基址替换为：'}
          </Paragraph>

          <div className='mx-auto mt-7 flex w-full max-w-2xl items-center rounded-full border border-slate-200 bg-white/88 px-5 py-3 shadow-sm backdrop-blur'>
            <code className='truncate text-base font-semibold text-slate-700'>
              {serverAddress.replace(/\/$/, '')}
            </code>
            <span className='mx-3 shrink-0 text-base font-semibold text-blue-600'>
              /v1/messages
            </span>
            <button
              type='button'
              className='ml-auto inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-500 transition hover:border-blue-300 hover:text-blue-700'
              title={isEnglish ? 'Copy endpoint' : '复制端点'}
              onClick={() => handleCopy(endpointPreview)}
            >
              <IconCopy />
            </button>
          </div>

          <div className='mt-7 flex flex-wrap items-center justify-center gap-3'>
            <Link
              to={isSelfUseMode ? '/login' : '/register'}
              className='home-cta-primary inline-flex items-center gap-2 rounded-full bg-blue-600 px-7 py-3 text-sm font-semibold !text-white shadow-[0_14px_32px_rgba(37,99,235,0.28)] transition hover:bg-blue-700 hover:!text-white'
            >
              <IconPlay />
              {isEnglish ? 'Get API Key' : '获取密钥'}
            </Link>

            <Link
              to='/docs'
              className='inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-7 py-3 text-sm font-semibold text-slate-800 transition hover:border-blue-300 hover:text-blue-700'
            >
              <IconFile />
              {isEnglish ? 'Docs' : '文档'}
            </Link>

            <Link
              to='/console'
              className='inline-flex items-center rounded-full border border-slate-200 bg-white px-7 py-3 text-sm font-semibold text-slate-800 transition hover:border-blue-300 hover:text-blue-700'
            >
              {isEnglish ? 'Console' : '控制台'}
            </Link>
          </div>
        </div>
      </section>

      <section className='mx-auto mt-16 max-w-6xl px-4 sm:px-6'>
        <Text className='!block !text-center !text-[36px] !font-medium !tracking-[-0.02em] !text-slate-500'>
          {isEnglish ? 'Supports mainstream model providers' : '支持众多的大模型供应商'}
        </Text>

        <div className='mt-8 flex flex-wrap items-center justify-center gap-x-9 gap-y-7'>
          {PROVIDER_SHOWCASE.map((provider) => (
            <div
              key={provider.name}
              className='text-slate-800 transition hover:scale-105 hover:text-blue-700'
              title={provider.name}
            >
              {renderProviderVisual(provider)}
            </div>
          ))}
        </div>

        <Text className='!mt-7 !block !text-center !text-3xl !font-bold !text-slate-700'>
          30+
        </Text>
      </section>
    </div>
  );
};

export default Home;
