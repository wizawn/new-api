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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Banner,
  Button,
  Card,
  Input,
  InputNumber,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import {
  getTeamAutomationSettings,
  saveTeamAutomationSettings,
  showError,
  showSuccess,
  showWarning,
  testTeamAutomationConnection,
} from '../../helpers';
import { useTranslation } from 'react-i18next';
import { ExternalLink, Save, TestTubeDiagonal } from 'lucide-react';

const { Text } = Typography;

const MODE_OPTIONS = [
  { label: 'Local（本机执行）', value: 'local' },
  { label: 'Remote（远端兼容）', value: 'remote' },
  { label: 'Disabled（禁用）', value: 'disabled' },
];

const PLAYWRIGHT_BROWSER_OPTIONS = [
  { label: 'chromium', value: 'chromium' },
  { label: 'firefox', value: 'firefox' },
  { label: 'webkit', value: 'webkit' },
];

const TEST_TYPE_OPTIONS = [
  { label: '引擎连通性', value: 'engine' },
  { label: 'Cloudflare', value: 'cloudflare' },
  { label: 'Gmail / IMAP', value: 'gmail' },
  { label: 'CPA Importer', value: 'cpa' },
  { label: 'NewAPI 本地', value: 'newapi' },
];

const AUTOMATION_DOC_LINKS = [
  {
    label: 'Cloudflare API Token / Zone ID',
    url: 'https://developers.cloudflare.com/fundamentals/api/get-started/create-token/',
  },
  {
    label: 'Cloudflare Email Routing',
    url: 'https://developers.cloudflare.com/email-routing/',
  },
  {
    label: 'QQ 邮箱 IMAP/SMTP 开启说明',
    url: 'https://service.mail.qq.com/detail/0/75',
  },
  {
    label: 'OpenAI ChatGPT 官网入口',
    url: 'https://chatgpt.com/',
  },
  {
    label: 'Playwright 浏览器安装',
    url: 'https://playwright.dev/docs/intro',
  },
];

const DEFAULT_FORM = {
  mode: 'local',
  worker_base_url: '',
  worker_username: '',
  worker_password: '',
  clear_worker_password: false,
  worker_timeout_seconds: 30,
  worker_verify_ssl: true,
  artifact_root: '/data/automation-artifacts',
  playwright_headless: true,
  playwright_browser: 'chromium',
  register_entry_url: 'https://chatgpt.com',
  register_proxy_pool: '',
  register_proxy_gateway: '',
  register_user_agent: '',
  cloudflare_api_token: '',
  cloudflare_api_email: '',
  cloudflare_zone_id: '',
  cloudflare_domain: '',
  tempmail_base_url: '',
  tempmail_admin_auth: '',
  tempmail_domain: '',
  default_mailbox_id: 0,
  cpa_endpoint: '',
  cpa_token: '',
};

const splitProxyPoolLines = (raw) =>
  String(raw || '')
    .split(/\r?\n/)
    .map((item) => String(item || '').trim())
    .filter(Boolean);

const normalizeProxyPoolText = (raw) => {
  const dedup = [];
  const seen = new Set();
  splitProxyPoolLines(raw).forEach((item) => {
    if (seen.has(item)) {
      return;
    }
    seen.add(item);
    dedup.push(item);
  });
  return dedup.join('\n');
};

const resolveProxyDraftMode = (raw) => {
  const value = String(raw || '').trim().toLowerCase();
  if (!value) {
    return 'unknown';
  }
  if (value.startsWith('vmess://')) {
    return 'vmess';
  }
  if (value.startsWith('socks5://') || value.startsWith('socks://')) {
    return 'socks';
  }
  if (value.startsWith('https://')) {
    return 'https';
  }
  if (value.startsWith('http://')) {
    return 'http';
  }
  return 'unknown';
};

const resolveCPAEndpointMode = (raw) => {
  const endpoint = String(raw || '').trim().toLowerCase();
  if (!endpoint) {
    return 'unknown';
  }
  if (endpoint.includes('/v0/management')) {
    return 'cliproxy_management';
  }
  if (endpoint.includes('/api/accounts/import')) {
    return 'legacy_importer';
  }
  return 'generic';
};

const TeamAutomationSetting = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [proxyInspecting, setProxyInspecting] = useState(false);
  const [testType, setTestType] = useState('engine');
  const [settingsState, setSettingsState] = useState({
    configured: false,
    reachable: false,
    warning: '',
    can_configure: false,
    cpa_configured: false,
    proxy_pool_summary: null,
    proxy_pool_source_text: '',
    proxy_pool_source_gateway: '',
  });
  const [form, setForm] = useState(DEFAULT_FORM);
  const [lastCPATestResult, setLastCPATestResult] = useState(null);

  const setField = (key, value) => {
    setForm((prev) => ({
      ...prev,
      [key]: value,
    }));
  };

  const loadSettings = async () => {
    setLoading(true);
    try {
      const payload = await getTeamAutomationSettings();
      setSettingsState({
        configured: payload?.configured === true,
        reachable: payload?.reachable === true,
        warning: payload?.warning || '',
        can_configure: payload?.can_configure === true,
        cpa_configured: payload?.cpa_configured === true,
        proxy_pool_summary: payload?.proxy_pool_summary || null,
        proxy_pool_source_text: payload?.register_proxy_pool || '',
        proxy_pool_source_gateway: payload?.register_proxy_gateway || '',
      });
      setForm({
        mode: payload?.mode || 'local',
        worker_base_url: payload?.worker_base_url || '',
        worker_username: payload?.worker_username || '',
        worker_password: '',
        clear_worker_password: false,
        worker_timeout_seconds: Number(payload?.worker_timeout_seconds) || 30,
        worker_verify_ssl: payload?.worker_verify_ssl !== false,
        artifact_root: payload?.artifact_root || '/data/automation-artifacts',
        playwright_headless: payload?.playwright_headless !== false,
        playwright_browser: payload?.playwright_browser || 'chromium',
        register_entry_url:
          payload?.register_entry_url || 'https://chatgpt.com',
        register_proxy_pool: payload?.register_proxy_pool || '',
        register_proxy_gateway: payload?.register_proxy_gateway || '',
        register_user_agent: payload?.register_user_agent || '',
        cloudflare_api_token: '',
        cloudflare_api_email: payload?.cloudflare_api_email || '',
        cloudflare_zone_id: payload?.cloudflare_zone_id || '',
        cloudflare_domain: payload?.cloudflare_domain || '',
        tempmail_base_url: payload?.tempmail_base_url || '',
        tempmail_admin_auth: '',
        tempmail_domain: payload?.tempmail_domain || '',
        default_mailbox_id: Number(payload?.default_mailbox_id) || 0,
        cpa_endpoint: payload?.cpa_endpoint || '',
        cpa_token: '',
      });
    } catch (error) {
      showError(error?.message || t('加载 Team Automation 设置失败'));
    } finally {
      setLoading(false);
    }
  };

  const validateForm = () => {
    const timeout = Number(form.worker_timeout_seconds) || 0;
    if (timeout < 5 || timeout > 300) {
      throw new Error(t('Worker 超时需在 5-300 秒之间'));
    }
    if (form.mode === 'remote' && !String(form.worker_base_url || '').trim()) {
      throw new Error(t('Remote 模式必须填写 Worker Base URL'));
    }
    if (String(form.worker_base_url || '').trim()) {
      const value = String(form.worker_base_url || '').trim();
      let parsed;
      try {
        parsed = new URL(value);
      } catch {
        throw new Error(t('Worker Base URL 格式不正确'));
      }
      if (!['http:', 'https:'].includes(parsed.protocol)) {
        throw new Error(t('Worker Base URL 仅支持 http/https'));
      }
    }
    if (String(form.register_entry_url || '').trim()) {
      const value = String(form.register_entry_url || '').trim();
      let parsed;
      try {
        parsed = new URL(value);
      } catch {
        throw new Error(t('Register Entry URL 格式不正确'));
      }
      if (!['http:', 'https:'].includes(parsed.protocol)) {
        throw new Error(t('Register Entry URL 仅支持 http/https'));
      }
    }
  };

  const buildPayload = ({ includeTestType = false, testTypeOverride = '' } = {}) => {
    const payload = {
      mode: form.mode,
      worker_base_url: String(form.worker_base_url || '').trim(),
      worker_username: String(form.worker_username || '').trim(),
      worker_timeout_seconds: Number(form.worker_timeout_seconds) || 30,
      worker_verify_ssl: form.worker_verify_ssl === true,
      artifact_root: String(form.artifact_root || '').trim(),
      playwright_headless: form.playwright_headless === true,
      playwright_browser: String(form.playwright_browser || 'chromium').trim(),
      register_entry_url: String(form.register_entry_url || '').trim(),
      register_proxy_pool: String(form.register_proxy_pool || '').trim(),
      register_proxy_gateway: String(form.register_proxy_gateway || '').trim(),
      register_user_agent: String(form.register_user_agent || '').trim(),
      cloudflare_api_email: String(form.cloudflare_api_email || '')
        .trim()
        .toLowerCase(),
      cloudflare_zone_id: String(form.cloudflare_zone_id || '').trim(),
      cloudflare_domain: String(form.cloudflare_domain || '').trim(),
      tempmail_base_url: String(form.tempmail_base_url || '').trim(),
      tempmail_domain: String(form.tempmail_domain || '')
        .trim()
        .toLowerCase(),
      default_mailbox_id: Number(form.default_mailbox_id) || 0,
      cpa_endpoint: String(form.cpa_endpoint || '').trim(),
    };

    const workerPassword = String(form.worker_password || '').trim();
    if (workerPassword || form.clear_worker_password) {
      payload.worker_password = workerPassword;
    }

    const cfToken = String(form.cloudflare_api_token || '').trim();
    if (cfToken) {
      payload.cloudflare_api_token = cfToken;
    }

    const tempmailAdminAuth = String(form.tempmail_admin_auth || '').trim();
    if (tempmailAdminAuth) {
      payload.tempmail_admin_auth = tempmailAdminAuth;
    }

    const cpaToken = String(form.cpa_token || '').trim();
    if (cpaToken) {
      payload.cpa_token = cpaToken;
    }

    const resolvedTestType = String(testTypeOverride || '').trim();
    if (includeTestType || resolvedTestType) {
      payload.test_type = resolvedTestType || testType;
    }

    return payload;
  };

  const updateProxyPoolSummary = (
    summary,
    { sourceText = '', sourceGateway = '' } = {},
  ) => {
    setSettingsState((prev) => ({
      ...prev,
      proxy_pool_summary: summary || null,
      proxy_pool_source_text: String(sourceText || ''),
      proxy_pool_source_gateway: String(sourceGateway || ''),
    }));
  };

  const applyProxyPoolText = (value, emptyMessage, successMessage) => {
    const normalized = normalizeProxyPoolText(value);
    if (!normalized) {
      showWarning(emptyMessage);
      return;
    }
    setField('register_proxy_pool', normalized);
    showSuccess(successMessage);
  };

  const copyText = async (value, successMessage) => {
    const text = String(value || '').trim();
    if (!text) {
      showWarning(t('当前没有可复制的内容'));
      return;
    }
    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
      } else {
        throw new Error('clipboard unavailable');
      }
      showSuccess(successMessage);
    } catch {
      const textarea = document.createElement('textarea');
      textarea.value = text;
      textarea.setAttribute('readonly', 'readonly');
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
      showSuccess(successMessage);
    }
  };

  const onSave = async () => {
    try {
      validateForm();
      setSaving(true);
      await saveTeamAutomationSettings(buildPayload());
      setLastCPATestResult(null);
      showSuccess(t('Team Automation 设置已保存'));
      await loadSettings();
    } catch (error) {
      showError(error?.message || t('保存 Team Automation 设置失败'));
    } finally {
      setSaving(false);
    }
  };

  const performConnectivityTest = async (requestedType = '') => {
    const effectiveType = String(requestedType || testType || 'engine').trim();
    const result = await testTeamAutomationConnection(
      buildPayload({
        includeTestType: !requestedType,
        testTypeOverride: requestedType,
      }),
    );
    updateProxyPoolSummary(result?.proxy_pool_summary || null, {
      sourceText: form.register_proxy_pool,
      sourceGateway: form.register_proxy_gateway,
    });
    if (effectiveType === 'cpa') {
      setLastCPATestResult(result || null);
    }
    return { result, effectiveType };
  };

  const onTest = async () => {
    try {
      validateForm();
      setTesting(true);
      const { result } = await performConnectivityTest();
      showSuccess(t('测试成功'));
      if (result?.warning) {
        showWarning(result.warning);
      }
    } catch (error) {
      showError(error?.message || t('测试失败'));
    } finally {
      setTesting(false);
    }
  };

  const onQuickTestCPA = async () => {
    try {
      validateForm();
      setTesting(true);
      const { result } = await performConnectivityTest('cpa');
      showSuccess(
        result?.mode === 'cliproxy_management'
          ? t('CPA 连通性测试成功，已识别为 CLIProxy Management 模式')
          : t('CPA 连通性测试成功'),
      );
      if (result?.warning) {
        showWarning(result.warning);
      }
    } catch (error) {
      setLastCPATestResult(null);
      showError(error?.message || t('CPA 连通性测试失败'));
    } finally {
      setTesting(false);
    }
  };

  const onInspectProxyPool = async () => {
    try {
      validateForm();
      setProxyInspecting(true);
      const result = await testTeamAutomationConnection(
        buildPayload({ testTypeOverride: 'engine' }),
      );
      updateProxyPoolSummary(result?.proxy_pool_summary || null, {
        sourceText: form.register_proxy_pool,
        sourceGateway: form.register_proxy_gateway,
      });
      showSuccess(t('代理池检测已更新'));
      if (result?.warning) {
        showWarning(result.warning);
      }
    } catch (error) {
      showError(error?.message || t('代理池检测失败'));
    } finally {
      setProxyInspecting(false);
    }
  };

  const statusBanner = useMemo(() => {
    if (form.mode === 'disabled') {
      return {
        type: 'warning',
        text: t('Automation 当前处于禁用状态，管理页将只显示状态信息。'),
      };
    }
    if (settingsState.configured && settingsState.reachable) {
      return {
        type: 'success',
        text: t('当前配置有效，自动化中心可直接运行。'),
      };
    }
    return {
      type: 'warning',
      text:
        settingsState.warning ||
        t('当前配置未完全就绪，请补全必要参数后再运行。'),
    };
  }, [
    form.mode,
    settingsState.configured,
    settingsState.reachable,
    settingsState.warning,
    t,
  ]);

  const proxyPoolBanner = useMemo(() => {
    const summary = settingsState.proxy_pool_summary;
    if (!summary || form.mode !== 'local') {
      return null;
    }
    const totalEntries = Number(summary?.total_entries) || 0;
    const validEntries = Number(summary?.valid_entries) || 0;
    const invalidEntries = Number(summary?.invalid_entries) || 0;
    const vmessEntries = Number(summary?.vmess_entries) || 0;
    const xrayAvailable = summary?.xray_available === true;
    const xrayPath = String(summary?.xray_path || '').trim();
    const gatewayValid = summary?.gateway_valid === true;
    const warning = String(summary?.warning || '').trim();

    if (
      totalEntries === 0 &&
      !String(form.register_proxy_gateway || '').trim()
    ) {
      return {
        type: 'info',
        text: t('当前未配置代理池；本机执行将直接使用本地网络。'),
      };
    }
    if (totalEntries === 0 && gatewayValid) {
      return {
        type: 'success',
        text: t(
          '当前未配置独立代理池；运行任务时将直接通过 proxy_gateway 使用外部稳定入口。',
        ),
      };
    }
    if (warning) {
      return {
        type: 'warning',
        text: `${t('代理池共')} ${totalEntries} ${t('条，已识别')} ${validEntries} ${t('条可用，')} ${invalidEntries} ${t('条无效。')} ${warning}`,
      };
    }
    if (vmessEntries > 0) {
      return {
        type: 'success',
        text: xrayAvailable
          ? `${t('代理池包含')} ${vmessEntries} ${t('条 vmess 节点，当前已检测到 xray：')} ${xrayPath || 'xray'}`
          : gatewayValid
            ? t('代理池中的 vmess 节点将通过 proxy_gateway 回退使用。')
            : t('代理池中的 vmess 节点已配置完成。'),
      };
    }
    return {
      type: 'success',
      text: `${t('代理池共')} ${totalEntries} ${t('条，已识别')} ${validEntries} ${t('条可用代理。')}`,
    };
  }, [
    form.mode,
    form.register_proxy_gateway,
    settingsState.proxy_pool_summary,
    t,
  ]);
  const proxyPoolRows = useMemo(() => {
    if (!settingsState.proxy_pool_summary) {
      return [];
    }
    return Array.isArray(settingsState.proxy_pool_summary?.items)
      ? settingsState.proxy_pool_summary.items
      : [];
  }, [settingsState.proxy_pool_summary]);
  const invalidProxyRows = useMemo(
    () => proxyPoolRows.filter((item) => item?.valid !== true),
    [proxyPoolRows],
  );
  const validProxyRows = useMemo(
    () => proxyPoolRows.filter((item) => item?.valid === true),
    [proxyPoolRows],
  );
  const vmessProxyRows = useMemo(
    () => proxyPoolRows.filter((item) => String(item?.mode || '').trim() === 'vmess'),
    [proxyPoolRows],
  );
  const proxyDraftRows = useMemo(
    () => splitProxyPoolLines(form.register_proxy_pool),
    [form.register_proxy_pool],
  );
  const proxyDraftEntries = useMemo(
    () =>
      proxyDraftRows.map((item) => {
        const mode = resolveProxyDraftMode(item);
        return {
          raw: item,
          mode,
          isNative:
            mode === 'socks' || mode === 'http' || mode === 'https',
        };
      }),
    [proxyDraftRows],
  );
  const proxyDraftMetrics = useMemo(() => {
    const stats = {
      total: proxyDraftEntries.length,
      vmess: 0,
      native: 0,
      unknown: 0,
      duplicated: 0,
    };
    const seen = new Set();
    proxyDraftEntries.forEach((item) => {
      if (seen.has(item.raw)) {
        stats.duplicated += 1;
      } else {
        seen.add(item.raw);
      }
      if (item.mode === 'vmess') {
        stats.vmess += 1;
      } else if (item.isNative) {
        stats.native += 1;
      } else {
        stats.unknown += 1;
      }
    });
    return stats;
  }, [proxyDraftEntries]);
  const proxyDraftVmessText = useMemo(
    () =>
      proxyDraftEntries
        .filter((item) => item.mode === 'vmess')
        .map((item) => item.raw)
        .join('\n'),
    [proxyDraftEntries],
  );
  const proxyDraftNativeText = useMemo(
    () =>
      proxyDraftEntries
        .filter((item) => item.isNative)
        .map((item) => item.raw)
        .join('\n'),
    [proxyDraftEntries],
  );
  const proxySummaryCards = useMemo(() => {
    const summary = settingsState.proxy_pool_summary;
    if (!summary) {
      return [];
    }
    return [
      {
        label: t('总条目'),
        value: Number(summary?.total_entries) || 0,
      },
      {
        label: t('可用'),
        value: Number(summary?.valid_entries) || 0,
      },
      {
        label: t('无效'),
        value: Number(summary?.invalid_entries) || 0,
      },
      {
        label: t('vmess'),
        value: Number(summary?.vmess_entries) || 0,
      },
    ];
  }, [settingsState.proxy_pool_summary, t]);
  const proxyRuntimeSummary = useMemo(() => {
    const summary = settingsState.proxy_pool_summary;
    if (!summary) {
      return [];
    }
    return [
      summary?.xray_available === true
        ? `${t('xray 已就绪')}: ${String(summary?.xray_path || 'xray').trim()}`
        : t('xray 未检测到'),
      summary?.gateway_valid === true
        ? `${t('gateway 可用')}: ${String(summary?.gateway_label || summary?.gateway_mode || '-').trim()}`
        : String(summary?.gateway_error || '').trim()
          ? `${t('gateway 异常')}: ${String(summary?.gateway_error || '').trim()}`
          : t('gateway 未配置'),
      summary?.vmess_ready === true
        ? t('vmess 可直接投入轮询')
        : t('vmess 尚未就绪，请补齐 xray 或可用 gateway'),
    ];
  }, [settingsState.proxy_pool_summary, t]);
  const invalidProxyText = useMemo(
    () =>
      invalidProxyRows
        .map((item) => String(item?.raw || '').trim())
        .filter(Boolean)
        .join('\n'),
    [invalidProxyRows],
  );
  const validProxyText = useMemo(
    () =>
      validProxyRows
        .map((item) => String(item?.raw || '').trim())
        .filter(Boolean)
        .join('\n'),
    [validProxyRows],
  );
  const vmessProxyText = useMemo(
    () =>
      vmessProxyRows
        .map((item) => String(item?.raw || '').trim())
        .filter(Boolean)
        .join('\n'),
    [vmessProxyRows],
  );
  const isProxyPoolSummaryStale = useMemo(() => {
    if (!settingsState.proxy_pool_summary) {
      return false;
    }
    return (
      normalizeProxyPoolText(form.register_proxy_pool) !==
        normalizeProxyPoolText(settingsState.proxy_pool_source_text) ||
      String(form.register_proxy_gateway || '').trim() !==
        String(settingsState.proxy_pool_source_gateway || '').trim()
    );
  }, [
    form.register_proxy_gateway,
    form.register_proxy_pool,
    settingsState.proxy_pool_source_gateway,
    settingsState.proxy_pool_source_text,
    settingsState.proxy_pool_summary,
  ]);
  const showProxyDiagnostics = useMemo(
    () =>
      form.mode === 'local' &&
      (settingsState.proxy_pool_summary ||
        proxyDraftRows.length > 0 ||
        String(form.register_proxy_gateway || '').trim()),
    [
      form.mode,
      form.register_proxy_gateway,
      proxyDraftRows.length,
      settingsState.proxy_pool_summary,
    ],
  );
  const summaryDrivenActionsDisabled = isProxyPoolSummaryStale;
  const cpaEndpointMode = useMemo(
    () => resolveCPAEndpointMode(form.cpa_endpoint),
    [form.cpa_endpoint],
  );
  const cpaStatusBanner = useMemo(() => {
    const endpoint = String(form.cpa_endpoint || '').trim();
    const hasTokenInForm = String(form.cpa_token || '').trim() !== '';
    if (!endpoint) {
      return {
        type: 'info',
        text: t('当前未配置 CPA Endpoint；导入目标若选择 CPA，将无法落地到外部账号池。'),
      };
    }
    if (lastCPATestResult?.ok === true) {
      return {
        type: 'success',
        text:
          lastCPATestResult?.mode === 'cliproxy_management'
            ? t('最近一次 CPA 检测成功，当前端点可作为 CLIProxy Management / auth-files 目标使用。')
            : t('最近一次 CPA 检测成功，当前端点已可访问。'),
      };
    }
    if (settingsState.cpa_configured || hasTokenInForm) {
      return {
        type: 'success',
        text: t('CPA Endpoint 已填写，Token 已保存或本次已输入；建议保存后再执行一次 CPA 连通性测试。'),
      };
    }
    return {
      type: 'warning',
      text: t('当前仅填写了 CPA Endpoint，尚未检测到 Token；若导入目标包含 CPA，请补齐 Token。'),
    };
  }, [
    form.cpa_endpoint,
    form.cpa_token,
    lastCPATestResult,
    settingsState.cpa_configured,
    t,
  ]);

  useEffect(() => {
    loadSettings();
  }, []);

  return (
    <Spin spinning={loading || saving} size='large'>
      <Card style={{ marginTop: '10px' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              gap: '12px',
              flexWrap: 'wrap',
            }}
          >
            <div>
              <Text strong>{t('Automation 引擎设置')}</Text>
              <div style={{ marginTop: '6px' }}>
                <Text type='secondary'>
                  {t('支持 local / remote / disabled，默认 local 本机执行。')}
                </Text>
              </div>
            </div>
            <Space>
              <Select
                optionList={TEST_TYPE_OPTIONS}
                value={testType}
                onChange={setTestType}
                style={{ width: 170 }}
              />
              <Button
                icon={<TestTubeDiagonal size={16} />}
                type='tertiary'
                loading={testing}
                onClick={onTest}
              >
                {t('测试')}
              </Button>
              <Button
                theme='solid'
                type='primary'
                icon={<Save size={16} />}
                onClick={onSave}
                loading={saving}
              >
                {t('保存')}
              </Button>
            </Space>
          </div>

          <Banner
            type={statusBanner.type}
            closeIcon={null}
            description={statusBanner.text}
          />

          {!settingsState.can_configure ? (
            <Banner
              type='warning'
              closeIcon={null}
              description={t('当前账号没有配置权限（仅 root 可写入设置）。')}
            />
          ) : null}

          <Banner
            type='info'
            closeIcon={null}
            description={
              <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                <Text>
                  {t(
                    '当前 TeamAutomation 已接入 Team 资产池：支持 Team owner token 导入、席位统计、待邀请数管理和自动选队预览。',
                  )}
                </Text>
                <Text type='secondary'>
                  {t(
                    'Team 资产管理入口在 Team Automation 页面；设置页继续负责执行引擎、Cloudflare、TempMail 和 CPA 等基础能力。',
                  )}
                </Text>
                <Text type='secondary'>
                  {t(
                    '批量导入兼容 team-manage 的 single / batch 语义，建议在运行任务前先完成 Team 资产池初始化。',
                  )}
                </Text>
              </div>
            }
          />

          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))',
              gap: '16px',
            }}
          >
            <Card title={t('运行模式')} bodyStyle={{ padding: '14px' }}>
              <Space vertical style={{ width: '100%' }}>
                <div>
                  <Text strong>{t('Automation Mode')}</Text>
                  <Select
                    optionList={MODE_OPTIONS}
                    value={form.mode}
                    onChange={(value) => setField('mode', value)}
                    style={{ marginTop: '8px', width: '100%' }}
                    disabled={!settingsState.can_configure}
                  />
                </div>
                <div>
                  <Text strong>{t('Worker Timeout')}</Text>
                  <InputNumber
                    min={5}
                    max={300}
                    step={1}
                    value={form.worker_timeout_seconds}
                    onChange={(value) =>
                      setField('worker_timeout_seconds', Number(value) || 30)
                    }
                    style={{ marginTop: '8px', width: '100%' }}
                    disabled={!settingsState.can_configure}
                  />
                </div>
                <div>
                  <Text strong>{t('Worker Verify SSL')}</Text>
                  <div style={{ marginTop: '8px' }}>
                    <Switch
                      checked={form.worker_verify_ssl}
                      onChange={(value) => setField('worker_verify_ssl', value)}
                      disabled={!settingsState.can_configure}
                    />
                  </div>
                </div>
              </Space>
            </Card>

            <Card title={t('Worker 连接')} bodyStyle={{ padding: '14px' }}>
              <Space vertical style={{ width: '100%' }}>
                <div>
                  <Text strong>{t('Worker Base URL')}</Text>
                  <Input
                    value={form.worker_base_url}
                    onChange={(value) => setField('worker_base_url', value)}
                    placeholder='http://127.0.0.1:38210'
                    style={{ marginTop: '8px' }}
                    disabled={
                      !settingsState.can_configure || form.mode !== 'remote'
                    }
                  />
                </div>
                <div>
                  <Text strong>{t('Worker Username')}</Text>
                  <Input
                    value={form.worker_username}
                    onChange={(value) => setField('worker_username', value)}
                    placeholder='optional'
                    style={{ marginTop: '8px' }}
                    disabled={
                      !settingsState.can_configure || form.mode !== 'remote'
                    }
                  />
                </div>
                <div>
                  <Text strong>{t('Worker Password')}</Text>
                  <Input
                    mode='password'
                    value={form.worker_password}
                    onChange={(value) => setField('worker_password', value)}
                    placeholder={t('留空则不更新')}
                    style={{ marginTop: '8px' }}
                    autoComplete='new-password'
                    disabled={
                      !settingsState.can_configure || form.mode !== 'remote'
                    }
                  />
                </div>
                <div>
                  <Text type='secondary'>{t('清空已保存密码')}</Text>
                  <div style={{ marginTop: '8px' }}>
                    <Switch
                      checked={form.clear_worker_password}
                      onChange={(value) =>
                        setField('clear_worker_password', value)
                      }
                      disabled={
                        !settingsState.can_configure || form.mode !== 'remote'
                      }
                    />
                  </div>
                </div>
              </Space>
            </Card>

            <Card title={t('本机执行')} bodyStyle={{ padding: '14px' }}>
              <Space vertical style={{ width: '100%' }}>
                <div>
                  <Text strong>{t('Artifact Root')}</Text>
                  <Input
                    value={form.artifact_root}
                    onChange={(value) => setField('artifact_root', value)}
                    placeholder='/data/automation-artifacts'
                    style={{ marginTop: '8px' }}
                    disabled={
                      !settingsState.can_configure || form.mode !== 'local'
                    }
                  />
                </div>
                <div>
                  <Text strong>{t('Playwright Browser')}</Text>
                  <Select
                    optionList={PLAYWRIGHT_BROWSER_OPTIONS}
                    value={form.playwright_browser}
                    onChange={(value) => setField('playwright_browser', value)}
                    style={{ marginTop: '8px', width: '100%' }}
                    disabled={
                      !settingsState.can_configure || form.mode !== 'local'
                    }
                  />
                </div>
                <div>
                  <Text strong>{t('Playwright Headless')}</Text>
                  <div style={{ marginTop: '8px' }}>
                    <Switch
                      checked={form.playwright_headless}
                      onChange={(value) =>
                        setField('playwright_headless', value)
                      }
                      disabled={
                        !settingsState.can_configure || form.mode !== 'local'
                      }
                    />
                  </div>
                </div>
                <div>
                  <Text strong>{t('Register Entry URL')}</Text>
                  <Input
                    value={form.register_entry_url}
                    onChange={(value) => setField('register_entry_url', value)}
                    placeholder='https://chatgpt.com'
                    style={{ marginTop: '8px' }}
                    disabled={
                      !settingsState.can_configure || form.mode !== 'local'
                    }
                  />
                  <div style={{ marginTop: '4px' }}>
                    <Text type='tertiary'>
                      {t(
                        '自动注册模块默认从该入口进入，建议保持 chatgpt.com。',
                      )}
                    </Text>
                  </div>
                </div>
                <div>
                  <Text strong>{t('Register Proxy Pool')}</Text>
                  <TextArea
                    autosize={{ minRows: 3, maxRows: 6 }}
                    value={form.register_proxy_pool}
                    onChange={(value) => setField('register_proxy_pool', value)}
                    placeholder={
                      'socks5://127.0.0.1:7890\nsocks5://user:pass@1.2.3.4:1080\nvmess://...'
                    }
                    style={{ marginTop: '8px' }}
                    disabled={
                      !settingsState.can_configure || form.mode !== 'local'
                    }
                  />
                  <div style={{ marginTop: '4px' }}>
                    <Text type='tertiary'>
                      {t(
                        '每行一个代理；支持 socks5/http/https，也支持直接粘贴 vmess://。本机已安装 xray 时会自动转换为可用 socks5 代理。',
                      )}
                    </Text>
                    <div style={{ marginTop: 6 }}>
                      <Text type='secondary'>
                        {t('当前输入 {{count}} 条代理草稿', {
                          count: proxyDraftRows.length,
                        })}
                      </Text>
                    </div>
                  </div>
                  {proxyPoolBanner ? (
                    <Banner
                      type={proxyPoolBanner.type}
                      closeIcon={null}
                      description={proxyPoolBanner.text}
                      style={{ marginTop: '8px' }}
                    />
                  ) : null}
                  {showProxyDiagnostics ? (
                    <div
                      style={{
                        marginTop: 10,
                        padding: '12px',
                        borderRadius: 12,
                        border: '1px solid var(--semi-color-border)',
                        background: 'linear-gradient(135deg, rgba(15,23,42,0.03), rgba(14,165,233,0.08))',
                        display: 'flex',
                        flexDirection: 'column',
                        gap: 10,
                      }}
                    >
                      <div
                        style={{
                          display: 'flex',
                          justifyContent: 'space-between',
                          gap: 12,
                          flexWrap: 'wrap',
                          alignItems: 'center',
                        }}
                      >
                        <div>
                          <Text strong>{t('代理池诊断')}</Text>
                          <div style={{ marginTop: 4 }}>
                            <Text type='secondary'>
                              {!settingsState.proxy_pool_summary
                                ? t('当前还没有检测结果；下面先展示草稿概况，点“重新检测代理池”后会补充可用性和 vmess 运行态诊断。')
                                : isProxyPoolSummaryStale
                                ? t('当前草稿已修改，下面诊断结果不是最新的；请先重新检测，再使用“仅保留可用条目”等基于结果的动作。')
                                : t('以下结果基于已保存配置或最近一次检测；修改文本后可点“重新检测代理池”刷新。')}
                            </Text>
                          </div>
                        </div>
                        <Space wrap>
                          <Button
                            size='small'
                            theme='outline'
                            loading={proxyInspecting}
                            onClick={onInspectProxyPool}
                          >
                            {t('重新检测代理池')}
                          </Button>
                          <Button
                            size='small'
                            theme='light'
                            disabled={proxyDraftRows.length <= 1}
                            onClick={() =>
                              applyProxyPoolText(
                                form.register_proxy_pool,
                                t('当前没有可整理的代理条目'),
                                t('代理池文本已去重压缩'),
                              )
                            }
                          >
                            {t('去重压缩文本')}
                          </Button>
                          <Button
                            size='small'
                            theme='light'
                            disabled={!proxyDraftVmessText}
                            onClick={() =>
                              applyProxyPoolText(
                                proxyDraftVmessText,
                                t('当前草稿中没有 vmess 条目'),
                                t('已仅保留 vmess 条目'),
                              )
                            }
                          >
                            {t('仅保留 vmess')}
                          </Button>
                          <Button
                            size='small'
                            theme='light'
                            disabled={!proxyDraftNativeText}
                            onClick={() =>
                              applyProxyPoolText(
                                proxyDraftNativeText,
                                t('当前草稿中没有可直接使用的原生代理'),
                                t('已仅保留原生代理'),
                              )
                            }
                          >
                            {t('仅保留原生代理')}
                          </Button>
                          <Button
                            size='small'
                            theme='light'
                            disabled={
                              validProxyRows.length === 0 ||
                              summaryDrivenActionsDisabled
                            }
                            onClick={() =>
                              applyProxyPoolText(
                                validProxyText,
                                t('当前没有可回填的可用代理'),
                                t('已将可用代理回填到文本框'),
                              )
                            }
                          >
                            {t('仅保留可用条目')}
                          </Button>
                          <Button
                            size='small'
                            theme='light'
                            disabled={
                              invalidProxyRows.length === 0 ||
                              summaryDrivenActionsDisabled
                            }
                            onClick={() =>
                              copyText(invalidProxyText, t('异常条目已复制'))
                            }
                          >
                            {t('复制异常条目')}
                          </Button>
                          <Button
                            size='small'
                            theme='light'
                            disabled={
                              validProxyRows.length === 0 ||
                              summaryDrivenActionsDisabled
                            }
                            onClick={() =>
                              copyText(validProxyText, t('可用条目已复制'))
                            }
                          >
                            {t('复制可用条目')}
                          </Button>
                          <Button
                            size='small'
                            theme='light'
                            disabled={
                              vmessProxyRows.length === 0 ||
                              summaryDrivenActionsDisabled
                            }
                            onClick={() =>
                              copyText(vmessProxyText, t('vmess 条目已复制'))
                            }
                          >
                            {t('复制 vmess 条目')}
                          </Button>
                        </Space>
                      </div>

                      <div
                        style={{
                          display: 'grid',
                          gridTemplateColumns: 'repeat(auto-fit, minmax(110px, 1fr))',
                          gap: 8,
                        }}
                      >
                        {proxySummaryCards.map((item) => (
                          <div
                            key={item.label}
                            style={{
                              padding: '10px 12px',
                              borderRadius: 10,
                              background: 'rgba(255,255,255,0.76)',
                              border: '1px solid rgba(148,163,184,0.24)',
                            }}
                          >
                            <Text type='secondary'>{item.label}</Text>
                            <div style={{ marginTop: 4, fontSize: 22, fontWeight: 700 }}>
                              {item.value}
                            </div>
                          </div>
                        ))}
                      </div>

                      <div
                        style={{
                          display: 'grid',
                          gridTemplateColumns: 'repeat(auto-fit, minmax(110px, 1fr))',
                          gap: 8,
                        }}
                      >
                        {[
                          {
                            label: t('草稿总数'),
                            value: proxyDraftMetrics.total,
                          },
                          {
                            label: t('草稿 vmess'),
                            value: proxyDraftMetrics.vmess,
                          },
                          {
                            label: t('草稿原生'),
                            value: proxyDraftMetrics.native,
                          },
                          {
                            label: t('草稿重复'),
                            value: proxyDraftMetrics.duplicated,
                          },
                        ].map((item) => (
                          <div
                            key={item.label}
                            style={{
                              padding: '10px 12px',
                              borderRadius: 10,
                              background: 'rgba(255,255,255,0.56)',
                              border: '1px dashed rgba(148,163,184,0.3)',
                            }}
                          >
                            <Text type='secondary'>{item.label}</Text>
                            <div style={{ marginTop: 4, fontSize: 20, fontWeight: 700 }}>
                              {item.value}
                            </div>
                          </div>
                        ))}
                      </div>

                      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                        <Tag
                          color={
                            !settingsState.proxy_pool_summary
                              ? 'grey'
                              : isProxyPoolSummaryStale
                                ? 'orange'
                                : 'green'
                          }
                        >
                          {!settingsState.proxy_pool_summary
                            ? t('尚无检测结果')
                            : isProxyPoolSummaryStale
                              ? t('检测结果已过期')
                              : t('检测结果与当前草稿一致')}
                        </Tag>
                        {proxyDraftMetrics.unknown > 0 ? (
                          <Tag color='red'>
                            {t('草稿中有 {{count}} 条待人工确认', {
                              count: proxyDraftMetrics.unknown,
                            })}
                          </Tag>
                        ) : null}
                        {proxyDraftMetrics.duplicated > 0 ? (
                          <Tag color='blue'>
                            {t('草稿中检测到 {{count}} 条重复', {
                              count: proxyDraftMetrics.duplicated,
                            })}
                          </Tag>
                        ) : null}
                      </div>

                      {settingsState.proxy_pool_summary ? (
                        <>
                          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                            {proxyRuntimeSummary.map((item) => (
                              <Tag key={item} color='cyan'>
                                {item}
                              </Tag>
                            ))}
                          </div>

                          <div
                            style={{
                              display: 'grid',
                              gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
                              gap: 10,
                            }}
                          >
                            <div>
                              <Text strong>{t('异常条目')}</Text>
                              <div
                                style={{
                                  marginTop: 8,
                                  display: 'flex',
                                  flexDirection: 'column',
                                  gap: 6,
                                }}
                              >
                                {invalidProxyRows.length === 0 ? (
                                  <Text type='secondary'>{t('未发现异常条目')}</Text>
                                ) : (
                                  invalidProxyRows.slice(0, 4).map((item) => (
                                    <div
                                      key={`invalid-${item.index}`}
                                      style={{
                                        padding: '8px 10px',
                                        borderRadius: 10,
                                        background: 'rgba(255,241,242,0.85)',
                                        border: '1px solid rgba(248,113,113,0.28)',
                                      }}
                                    >
                                      <Text strong>
                                        #{Number(item?.index) || 0}
                                      </Text>
                                      <div style={{ marginTop: 4 }}>
                                        <Text type='secondary'>
                                          {String(item?.raw || '').trim() || '-'}
                                        </Text>
                                      </div>
                                      <div style={{ marginTop: 4 }}>
                                        <Text>{String(item?.error || '').trim() || '-'}</Text>
                                      </div>
                                    </div>
                                  ))
                                )}
                              </div>
                            </div>

                            <div>
                              <Text strong>{t('vmess 节点')}</Text>
                              <div
                                style={{
                                  marginTop: 8,
                                  display: 'flex',
                                  flexDirection: 'column',
                                  gap: 6,
                                }}
                              >
                                {vmessProxyRows.length === 0 ? (
                                  <Text type='secondary'>{t('当前代理池未发现 vmess 条目')}</Text>
                                ) : (
                                  vmessProxyRows.slice(0, 4).map((item) => (
                                    <div
                                      key={`vmess-${item.index}`}
                                      style={{
                                        padding: '8px 10px',
                                        borderRadius: 10,
                                        background: 'rgba(236,253,245,0.85)',
                                        border: '1px solid rgba(16,185,129,0.24)',
                                      }}
                                    >
                                      <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                                        <Tag color='green'>
                                          #{Number(item?.index) || 0}
                                        </Tag>
                                        <Tag color='light-blue'>
                                          {String(item?.label || 'vmess').trim()}
                                        </Tag>
                                      </div>
                                      <div style={{ marginTop: 4 }}>
                                        <Text type='secondary'>
                                          {String(item?.raw || '').trim() || '-'}
                                        </Text>
                                      </div>
                                    </div>
                                  ))
                                )}
                              </div>
                            </div>
                          </div>
                        </>
                      ) : null}
                    </div>
                  ) : null}
                </div>
                <div>
                  <Text strong>{t('Register Proxy Gateway')}</Text>
                  <Input
                    value={form.register_proxy_gateway}
                    onChange={(value) =>
                      setField('register_proxy_gateway', value)
                    }
                    placeholder='socks5://127.0.0.1:7890'
                    style={{ marginTop: '8px' }}
                    disabled={
                      !settingsState.can_configure || form.mode !== 'local'
                    }
                  />
                  <div style={{ marginTop: '4px' }}>
                    <Text type='tertiary'>
                      {t(
                        '兼容旧模式：当 vmess 自动转换不可用时，可回退到这里填写本地 socks5/http 网关。',
                      )}
                    </Text>
                  </div>
                </div>
                <div>
                  <Text strong>{t('Register User-Agent')}</Text>
                  <Input
                    value={form.register_user_agent}
                    onChange={(value) => setField('register_user_agent', value)}
                    placeholder='可选，不填则使用浏览器默认 UA'
                    style={{ marginTop: '8px' }}
                    disabled={
                      !settingsState.can_configure || form.mode !== 'local'
                    }
                  />
                </div>
              </Space>
            </Card>

            <Card
              title={t('Cloudflare / TempMail / Gmail / CPA')}
              bodyStyle={{ padding: '14px' }}
            >
              <Space vertical style={{ width: '100%' }}>
                <div>
                  <Text strong>{t('Cloudflare API Token')}</Text>
                  <Input
                    mode='password'
                    value={form.cloudflare_api_token}
                    onChange={(value) =>
                      setField('cloudflare_api_token', value)
                    }
                    placeholder={t('留空则不更新')}
                    style={{ marginTop: '8px' }}
                    autoComplete='new-password'
                    disabled={!settingsState.can_configure}
                  />
                </div>
                <div>
                  <Text strong>
                    {t('Cloudflare API Email (可选，仅 Global Key 模式需要)')}
                  </Text>
                  <Input
                    value={form.cloudflare_api_email}
                    onChange={(value) =>
                      setField('cloudflare_api_email', value)
                    }
                    placeholder='name@example.com'
                    style={{ marginTop: '8px' }}
                    disabled={!settingsState.can_configure}
                  />
                </div>
                <div>
                  <Text strong>{t('Cloudflare Zone ID')}</Text>
                  <Input
                    value={form.cloudflare_zone_id}
                    onChange={(value) => setField('cloudflare_zone_id', value)}
                    style={{ marginTop: '8px' }}
                    disabled={!settingsState.can_configure}
                  />
                </div>
                <div>
                  <Text strong>{t('Cloudflare Domain')}</Text>
                  <Input
                    value={form.cloudflare_domain}
                    onChange={(value) => setField('cloudflare_domain', value)}
                    style={{ marginTop: '8px' }}
                    disabled={!settingsState.can_configure}
                  />
                </div>
                <div>
                  <Text strong>{t('TempMail Base URL')}</Text>
                  <Input
                    value={form.tempmail_base_url}
                    onChange={(value) => setField('tempmail_base_url', value)}
                    placeholder='https://mailbox.wizawn.com'
                    style={{ marginTop: '8px' }}
                    disabled={!settingsState.can_configure}
                  />
                  <div style={{ marginTop: '4px' }}>
                    <Text type='tertiary'>
                      {t(
                        '用于模块 1 的 TempMail 地址生成，以及自动创建的 Wizawn 邮箱轮询。',
                      )}
                    </Text>
                  </div>
                </div>
                <div>
                  <Text strong>{t('TempMail Admin Auth')}</Text>
                  <Input
                    mode='password'
                    value={form.tempmail_admin_auth}
                    onChange={(value) => setField('tempmail_admin_auth', value)}
                    placeholder={t('留空则不更新')}
                    style={{ marginTop: '8px' }}
                    autoComplete='new-password'
                    disabled={!settingsState.can_configure}
                  />
                  <div style={{ marginTop: '4px' }}>
                    <Text type='tertiary'>
                      {t(
                        '用于调用 worker 的 /admin/new_address，请填写 x-admin-auth 对应值。',
                      )}
                    </Text>
                  </div>
                </div>
                <div>
                  <Text strong>{t('TempMail Default Domain')}</Text>
                  <Input
                    value={form.tempmail_domain}
                    onChange={(value) => setField('tempmail_domain', value)}
                    placeholder='caowo.de'
                    style={{ marginTop: '8px' }}
                    disabled={!settingsState.can_configure}
                  />
                  <div style={{ marginTop: '4px' }}>
                    <Text type='tertiary'>
                      {t(
                        '模块 1 未显式指定 domain 时，将默认使用此域名生成地址。',
                      )}
                    </Text>
                  </div>
                </div>
                <div>
                  <Text strong>{t('Default Mailbox ID')}</Text>
                  <InputNumber
                    min={0}
                    step={1}
                    value={form.default_mailbox_id}
                    onChange={(value) =>
                      setField('default_mailbox_id', Number(value) || 0)
                    }
                    style={{ marginTop: '8px', width: '100%' }}
                    disabled={!settingsState.can_configure}
                  />
                </div>
                <div>
                  <Text strong>{t('CPA Endpoint')}</Text>
                  <Input
                    value={form.cpa_endpoint}
                    onChange={(value) => setField('cpa_endpoint', value)}
                    style={{ marginTop: '8px' }}
                    disabled={!settingsState.can_configure}
                  />
                  <div style={{ marginTop: '8px' }}>
                    <Banner
                      type={cpaStatusBanner.type}
                      closeIcon={null}
                      description={cpaStatusBanner.text}
                    />
                  </div>
                  <div style={{ marginTop: 8, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                    <Tag color='cyan'>
                      {cpaEndpointMode === 'cliproxy_management'
                        ? t('探测模式: CLIProxy Management')
                        : cpaEndpointMode === 'legacy_importer'
                          ? t('探测模式: Legacy Importer')
                          : cpaEndpointMode === 'generic'
                            ? t('探测模式: 通用 Endpoint')
                            : t('探测模式: 未识别')}
                    </Tag>
                    <Tag color={settingsState.cpa_configured ? 'green' : 'orange'}>
                      {settingsState.cpa_configured
                        ? t('已保存 Token')
                        : t('未检测到已保存 Token')}
                    </Tag>
                    {lastCPATestResult?.matched_probe ? (
                      <Tag color='green'>
                        {t('最近探测')}: {String(lastCPATestResult.matched_probe).trim()}
                      </Tag>
                    ) : null}
                  </div>
                  <div style={{ marginTop: 8 }}>
                    <Button
                      theme='outline'
                      size='small'
                      loading={testing}
                      onClick={onQuickTestCPA}
                      disabled={!settingsState.can_configure}
                    >
                      {t('测试 CPA 连通性')}
                    </Button>
                  </div>
                  <div style={{ marginTop: '4px' }}>
                    <Text type='tertiary'>
                      {cpaEndpointMode === 'cliproxy_management'
                        ? t('检测到 /v0/management 前缀，导入模块会优先按 auth-files 方式落 CPA。')
                        : t('未检测到标准 management 前缀时，会优先按传统导入接口或通用探测顺序处理。')}
                    </Text>
                  </div>
                </div>
                <div>
                  <Text strong>{t('CPA Token')}</Text>
                  <Input
                    mode='password'
                    value={form.cpa_token}
                    onChange={(value) => setField('cpa_token', value)}
                    placeholder={t('留空则不更新')}
                    style={{ marginTop: '8px' }}
                    autoComplete='new-password'
                    disabled={!settingsState.can_configure}
                  />
                </div>
              </Space>
            </Card>
          </div>

          <Card title={t('参数获取地址')} bodyStyle={{ padding: '14px' }}>
            <Space vertical style={{ width: '100%' }}>
              {AUTOMATION_DOC_LINKS.map((item) => (
                <a
                  key={item.url}
                  href={item.url}
                  target='_blank'
                  rel='noreferrer'
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: 6,
                  }}
                >
                  <Text>{item.label}</Text>
                  <ExternalLink size={14} />
                </a>
              ))}
            </Space>
          </Card>
        </div>
      </Card>
    </Spin>
  );
};

export default TeamAutomationSetting;
