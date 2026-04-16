import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Progress,
  Select,
  Space,
  Switch,
  Table,
  Tabs,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import {
  API,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import { Link } from 'react-router-dom';

const { Text } = Typography;
const { TabPane } = Tabs;

const FUSION_STABLE_PROMPT =
  'You are a production-grade coding assistant for API relay operations. Keep responses concise and deterministic. Never fabricate facts, endpoints, logs, or states. If data is missing, state it clearly and request verification. Prioritize reliability: quota-aware routing, graceful failover, explicit auth/quota/rate-limit signal handling, and actionable error reporting.';

const QUOTA_GUARD_PROMPT =
  'You are a quota-first coding assistant for API relay operations. Keep responses concise, deterministic, and fully grounded in observed data. Never fabricate logs or endpoint states. Prefer accounts with higher weekly remaining quota, then higher 5h remaining quota. When quota is low/exhausted, proactively suggest stable fallback models and explicit retry windows.';

const FAILOVER_FIRST_PROMPT =
  'You are a failover-first coding assistant for API relay operations. Never fabricate facts. On auth/quota/rate-limit/network failures, classify the signal clearly, skip unstable routes quickly, and provide deterministic next actions. Keep responses concise and operationally actionable.';

const DEFAULT_POOL_CONFIG = {
  enable_weighted: true,
  default_cooldown_second: 300,
  request_timeout_second: 60,
  max_retry: 3,
  health_check_interval: 300,
  entitlement_check_interval: 3600,
  default_instructions: FUSION_STABLE_PROMPT,
};

const RECOMMENDED_POOL_CONFIG = {
  enable_weighted: true,
  default_cooldown_second: 300,
  request_timeout_second: 60,
  max_retry: 3,
  health_check_interval: 300,
  entitlement_check_interval: 3600,
  default_instructions: FUSION_STABLE_PROMPT,
};

const POOL_CONFIG_PRESETS = [
  {
    key: 'fusion_stable',
    label: '融合稳态（推荐）',
    description: '均衡策略：稳定优先，兼顾配额与故障转移',
    values: {
      enable_weighted: true,
      default_cooldown_second: 300,
      request_timeout_second: 60,
      max_retry: 3,
      health_check_interval: 300,
      entitlement_check_interval: 1800,
      default_instructions: FUSION_STABLE_PROMPT,
    },
  },
  {
    key: 'quota_guard',
    label: '额度优先（Quota）',
    description: '优先保障 weekly/5h 可用额度，降低过载风险',
    values: {
      enable_weighted: true,
      default_cooldown_second: 360,
      request_timeout_second: 75,
      max_retry: 2,
      health_check_interval: 240,
      entitlement_check_interval: 900,
      default_instructions: QUOTA_GUARD_PROMPT,
    },
  },
  {
    key: 'failover_first',
    label: '故障转移优先',
    description: '更快剔除不稳定账号，优先保证可用性',
    values: {
      enable_weighted: true,
      default_cooldown_second: 180,
      request_timeout_second: 45,
      max_retry: 4,
      health_check_interval: 180,
      entitlement_check_interval: 1200,
      default_instructions: FAILOVER_FIRST_PROMPT,
    },
  },
];

const DEFAULT_INSTRUCTION_PRESETS = [
  {
    key: 'assistant',
    label: '通用助手',
    value: 'You are a helpful assistant.',
  },
  {
    key: 'coding',
    label: '代码代理',
    value:
      'You are an expert coding assistant. Prefer concise, correct, and testable solutions. State assumptions explicitly and avoid fabricated details.',
  },
  {
    key: 'strict',
    label: '严谨模式',
    value:
      'Answer with strict factual grounding. If information is missing, state what is missing instead of guessing.',
  },
  {
    key: 'router_stable',
    label: '路由稳态',
    value:
      'You are a deterministic coding assistant. Keep replies concise, avoid unnecessary verbosity, and prioritize stable tool-driven execution with explicit error reporting.',
  },
  {
    key: 'fusion_stable',
    label: '融合稳态（推荐）',
    value: FUSION_STABLE_PROMPT,
  },
  {
    key: 'quota_guard',
    label: '额度优先（Quota）',
    value: QUOTA_GUARD_PROMPT,
  },
  {
    key: 'failover_first',
    label: '故障转移优先',
    value: FAILOVER_FIRST_PROMPT,
  },
];

const CHATMOCK_POOL_CHANNEL_NAME = 'ChatMockPool';
const CHATMOCK_POOL_CHANNEL_BASELINE_MODELS = [
  'gpt-5',
  'gpt-5-codex',
  'gpt-5-codex-mini',
  'gpt-5.1',
  'gpt-5.1-codex',
  'gpt-5.1-codex-mini',
  'gpt-5.1-codex-max',
  'gpt-5.2',
  'gpt-5.2-codex',
  'gpt-5.3-codex',
  'gpt-5.4',
  'gpt-5.3-codex-openai-compact',
  'gpt-5.4-openai-compact',
];

const DEFAULT_ACCOUNT_SUMMARY = {
  accounts: 0,
  status_counts: {},
  plan_counts: {},
  account_status_counts: {},
  signal_counts: {},
  exhausted_accounts: 0,
  low_accounts: 0,
  error_accounts: 0,
  auth_issue_accounts: 0,
  quota_issue_accounts: 0,
  rate_limited_accounts: 0,
  network_issue_accounts: 0,
  exhausted_names: [],
  low_names: [],
  error_names: [],
  auth_issue_names: [],
  quota_issue_names: [],
  rate_limited_names: [],
  network_issue_names: [],
  signal_suggestions: [],
  free_equivalent_7d: 0,
  plus_equivalent_7d: 0,
  stale_entitlement_accounts: 0,
  reset_passed_accounts: 0,
  token_expiring_24h_accounts: 0,
};

const DEFAULT_POOL_CHANNEL_STATUS = {
  exists: false,
  enabled: false,
  need_sync: true,
  message: '',
  channel_id: 0,
  channel_status_text: '',
  models_count: 0,
  recommended_models_count: 0,
  missing_models: [],
  extra_models: [],
};

const DEFAULT_ROUTE_DIAGNOSE_MODEL = 'gpt-5.3-codex';
const CHATMOCK_OPERATION_MODE_KEY = 'chatmock_operation_mode';

const ACCOUNT_SORT_MODES = [
  { value: 'default', label: '默认排序（优先级/权重）' },
  { value: 'quota_inspector', label: 'Quota Inspector 排序（计划+7d剩余）' },
  {
    value: 'smart_switch',
    label: 'Smart Switch 排序（1week剩余高→低，5h剩余高→低）',
  },
  { value: 'weekly_remaining_asc', label: '7d剩余由低到高' },
  { value: 'route_score_desc', label: '路由评分由高到低' },
  { value: 'name_asc', label: '账号名称 A-Z' },
];

const ACCOUNT_PLAN_RANK = {
  free: 0,
  team: 1,
  plus: 2,
  pro: 3,
  unknown: 9,
};

const ACCOUNT_QUOTA_STATUS_RANK = {
  exhausted: 0,
  low: 1,
  medium: 2,
  high: 3,
  full: 4,
  unknown: 5,
  error: 6,
};

const BASIC_ACCOUNT_COLUMN_INDICES = [0, 1, 2, 3, 9, 11, 12, 19];

const normalizeOperationMode = (value) =>
  String(value || '').toLowerCase() === 'advanced' ? 'advanced' : 'basic';

const DEFAULT_FORM_VALUES = {
  name: '',
  account: '',
  api_key: '',
  id_token: '',
  refresh_token: '',
  base_url: 'https://api.openai.com',
  proxy_url: '',
  status: 'enabled',
  weight: 1,
  priority: 100,
  max_concurrency: 2,
  rpm: 60,
  daily_token_limit: 0,
  supported_models_text: '',
  metadata: '',
};

const DEFAULT_MAILBOX_FORM_VALUES = {
  provider_template: 'qq',
  name: '',
  email: '',
  username: '',
  password: '',
  imap_host: 'imap.qq.com',
  imap_port: 993,
  imap_security: 'ssl',
  smtp_host: 'smtp.qq.com',
  smtp_port: 465,
  smtp_security: 'ssl',
  status: 'enabled',
  poll_interval_second: 60,
  metadata: '',
};

const MAILBOX_PROVIDER_PRESETS = {
  qq: {
    label: 'QQ',
    values: {
      imap_host: 'imap.qq.com',
      imap_port: 993,
      imap_security: 'ssl',
      smtp_host: 'smtp.qq.com',
      smtp_port: 465,
      smtp_security: 'ssl',
    },
  },
  gmail: {
    label: 'Gmail',
    values: {
      imap_host: 'imap.gmail.com',
      imap_port: 993,
      imap_security: 'ssl',
      smtp_host: 'smtp.gmail.com',
      smtp_port: 465,
      smtp_security: 'ssl',
    },
  },
  outlook: {
    label: 'Outlook',
    values: {
      imap_host: 'outlook.office365.com',
      imap_port: 993,
      imap_security: 'ssl',
      smtp_host: 'smtp.office365.com',
      smtp_port: 587,
      smtp_security: 'starttls',
    },
  },
  '163': {
    label: '163',
    values: {
      imap_host: 'imap.163.com',
      imap_port: 993,
      imap_security: 'ssl',
      smtp_host: 'smtp.163.com',
      smtp_port: 465,
      smtp_security: 'ssl',
    },
  },
  tempmail_wizawn: {
    label: 'TempMail / Wizawn',
    values: {
      email: 'temp@caowo.de',
      username: 'temp@caowo.de',
      imap_host: '',
      imap_port: 993,
      imap_security: 'ssl',
      smtp_host: '',
      smtp_port: 465,
      smtp_security: 'ssl',
      metadata: JSON.stringify(
        {
          provider: 'wizawn_http',
          base_url: 'https://mailbox.wizawn.com',
          domains: ['wizawn.com', 'caowo.de'],
          default_domain: 'caowo.de',
        },
        null,
        2,
      ),
    },
  },
  custom: {
    label: 'Custom',
    values: {},
  },
};

const CHATMOCK_MAIL_EVENT_TYPE_OPTIONS = [
  { label: '全部类型', value: '' },
  { label: 'verification_code', value: 'verification_code' },
  { label: 'invite_link', value: 'invite_link' },
  { label: 'auth_alert', value: 'auth_alert' },
  { label: 'billing_alert', value: 'billing_alert' },
  { label: 'system_notice', value: 'system_notice' },
  { label: 'system', value: 'system' },
];

const CHATMOCK_MAIL_EVENT_CLASSIFICATION_OPTIONS = [
  { label: '全部分类', value: '' },
  { label: 'verification_code', value: 'verification_code' },
  { label: 'invite_link', value: 'invite_link' },
  { label: 'auth_alert', value: 'auth_alert' },
  { label: 'billing_alert', value: 'billing_alert' },
  { label: 'system_notice', value: 'system_notice' },
  { label: 'system', value: 'system' },
];

const csvDefaultHeaders = [
  'name',
  'account',
  'api_key',
  'id_token',
  'refresh_token',
  'type',
  'email',
  'base_url',
  'proxy_url',
  'status',
  'weight',
  'priority',
  'max_concurrency',
  'rpm',
  'daily_token_limit',
  'supported_models',
  'metadata',
];

const accountFieldAliases = {
  name: ['name', '账号名称', 'email', '邮箱'],
  account: [
    'account',
    'account_id',
    'chatgpt_account',
    'chatgpt_account_id',
    '账号',
    '账号标识',
  ],
  api_key: [
    'api_key',
    'apikey',
    'token',
    'key',
    '密钥',
    'access_token',
    'accesstoken',
  ],
  id_token: ['id_token', 'idtoken'],
  refresh_token: ['refresh_token', 'refreshtoken'],
  type: ['type', 'provider', '来源'],
  email: ['email', '邮箱'],
  base_url: ['base_url', 'baseurl', 'url'],
  proxy_url: ['proxy_url', 'proxy', 'proxyurl'],
  status: ['status', '状态'],
  weight: ['weight', '权重'],
  priority: ['priority', '优先级'],
  max_concurrency: ['max_concurrency', 'concurrency', '并发', '并发上限'],
  rpm: ['rpm'],
  daily_token_limit: ['daily_token_limit', 'daily_limit', 'daily_tokens', '日限额'],
  supported_models: ['supported_models', 'models', 'model', '支持模型'],
  metadata: ['metadata', '备注'],
};

const mailboxCsvDefaultHeaders = [
  'name',
  'email',
  'username',
  'password',
  'imap_host',
  'imap_port',
  'imap_security',
  'smtp_host',
  'smtp_port',
  'smtp_security',
  'status',
  'poll_interval_second',
  'metadata',
];

const mailboxFieldAliases = {
  name: ['name', '邮箱名称'],
  email: ['email', '邮箱地址', '邮箱'],
  username: ['username', 'user', '账号', '邮箱用户名'],
  password: ['password', 'pass', '授权码', 'smtp_password', 'imap_password'],
  imap_host: ['imap_host', 'imap', 'imap_server'],
  imap_port: ['imap_port'],
  imap_security: ['imap_security', 'imap_tls', 'imap_ssl'],
  smtp_host: ['smtp_host', 'smtp', 'smtp_server'],
  smtp_port: ['smtp_port'],
  smtp_security: ['smtp_security', 'smtp_tls', 'smtp_ssl'],
  status: ['status', '状态'],
  poll_interval_second: ['poll_interval_second', 'poll_interval', '轮询间隔', '轮询秒数'],
  metadata: ['metadata', '备注'],
  provider_template: ['provider_template', 'provider', '模板'],
};

const toSafeErrorMessage = (error, fallback) => {
  const message =
    error?.response?.data?.message ||
    error?.message ||
    (typeof error === 'string' ? error : '');
  return message || fallback;
};

const toNumberOrDefault = (value, defaultValue) => {
  const num = Number(value);
  if (Number.isFinite(num)) {
    return num;
  }
  return defaultValue;
};

const normalizeAccountStatus = (value) => {
  const text = String(value || '')
    .trim()
    .toLowerCase();
  if (
    text === 'disabled' ||
    text === 'cooling' ||
    text === 'abnormal' ||
    text === 'quota_low' ||
    text === 'quota_exhausted' ||
    text === 'auth_invalid' ||
    text === 'network_issue' ||
    text === 'manual_review'
  ) {
    return text;
  }
  return 'enabled';
};

const normalizeMailboxStatus = (value) => {
  const text = String(value || '')
    .trim()
    .toLowerCase();
  if (text === 'disabled') {
    return 'disabled';
  }
  return 'enabled';
};

const isTransientMailboxError = (value) => {
  const text = String(value || '')
    .trim()
    .toLowerCase();
  if (!text) return false;
  if (text.startsWith('transient:')) return true;
  return (
    text.includes('system busy') ||
    text.includes('temporarily unavailable') ||
    text.includes('try again later') ||
    text.includes('rate limit')
  );
};

const parseMailboxRetryAt = (mailbox) => {
  const errorText = String(mailbox?.last_error || '');
  const checkedAt = Number(mailbox?.last_checked_time) || 0;
  if (!errorText || checkedAt <= 0) return 0;
  const match = errorText.match(/retry in\s+(\d+)s/i);
  if (!match?.[1]) return 0;
  const retryIn = Number(match[1]);
  if (!Number.isFinite(retryIn) || retryIn <= 0) return 0;
  return checkedAt + retryIn;
};

const normalizeMailboxRecord = (mailbox) => {
  const transient = isTransientMailboxError(mailbox?.last_error);
  const retryAt = transient ? parseMailboxRetryAt(mailbox) : 0;
  return {
    ...mailbox,
    _transient_error: transient,
    _retry_at: retryAt,
  };
};

const splitSupportedModels = (value) => {
  if (!value) {
    return [];
  }
  if (Array.isArray(value)) {
    return value
      .map((item) => String(item || '').trim())
      .filter(Boolean);
  }
  const text = String(value).trim();
  if (!text) {
    return [];
  }
  if (text.startsWith('[')) {
    try {
      const parsed = JSON.parse(text);
      if (Array.isArray(parsed)) {
        return parsed
          .map((item) => String(item || '').trim())
          .filter(Boolean);
      }
    } catch {
      // fallback to delimiter split
    }
  }
  return text
    .split(/[;,|]/)
    .map((item) => item.trim())
    .filter(Boolean);
};

const dedupeModels = (models) => {
  const set = new Set();
  if (!Array.isArray(models)) {
    return [];
  }
  models.forEach((item) => {
    const normalized = String(item || '').trim();
    if (normalized) {
      set.add(normalized);
    }
  });
  return Array.from(set);
};

const collectSupportedModelsFromAccounts = (records) => {
  if (!Array.isArray(records)) {
    return [];
  }
  const allModels = [];
  records.forEach((record) => {
    allModels.push(...splitSupportedModels(record?.supported_models));
  });
  return dedupeModels(allModels);
};

const renderStatusTag = (status) => {
  switch ((status || '').toLowerCase()) {
    case 'enabled':
      return <Tag color='green'>enabled</Tag>;
    case 'disabled':
      return <Tag color='grey'>disabled</Tag>;
    case 'cooling':
      return <Tag color='orange'>cooling</Tag>;
    case 'abnormal':
      return <Tag color='red'>abnormal</Tag>;
    case 'quota_low':
      return <Tag color='yellow'>quota_low</Tag>;
    case 'quota_exhausted':
      return <Tag color='red'>quota_exhausted</Tag>;
    case 'auth_invalid':
      return <Tag color='red'>auth_invalid</Tag>;
    case 'network_issue':
      return <Tag color='orange'>network_issue</Tag>;
    case 'manual_review':
      return <Tag color='purple'>manual_review</Tag>;
    case 'ok':
      return <Tag color='green'>ok</Tag>;
    case 'warn':
      return <Tag color='orange'>warn</Tag>;
    case 'error':
      return <Tag color='red'>error</Tag>;
    default:
      return <Tag color='grey'>{status || '-'}</Tag>;
  }
};

const renderMailEventTypeTag = (eventType, status) => {
  const normalizedType = String(eventType || '')
    .trim()
    .toLowerCase();
  const normalizedStatus = String(status || '')
    .trim()
    .toLowerCase();
  switch (normalizedType) {
    case 'verification_code':
      return <Tag color='blue'>verification_code</Tag>;
    case 'invite_link':
      return <Tag color='teal'>invite_link</Tag>;
    case 'auth_alert':
      return <Tag color='red'>auth_alert</Tag>;
    case 'billing_alert':
      return <Tag color='orange'>billing_alert</Tag>;
    case 'system_notice':
      return <Tag color='indigo'>system_notice</Tag>;
    case 'system':
      if (normalizedStatus === 'warn') {
        return <Tag color='orange'>system</Tag>;
      }
      if (normalizedStatus === 'error') {
        return <Tag color='red'>system</Tag>;
      }
      return <Tag color='grey'>system</Tag>;
    default:
      if (normalizedStatus === 'warn') {
        return <Tag color='orange'>{normalizedType || '-'}</Tag>;
      }
      if (normalizedStatus === 'error') {
        return <Tag color='red'>{normalizedType || '-'}</Tag>;
      }
      return <Tag color='grey'>{normalizedType || '-'}</Tag>;
  }
};

const renderMailboxStatusTag = (status) => {
  if ((status || '').toLowerCase() === 'enabled') {
    return <Tag color='green'>enabled</Tag>;
  }
  return <Tag color='grey'>disabled</Tag>;
};

const renderValidityTag = (valid) =>
  valid ? <Tag color='green'>valid</Tag> : <Tag color='red'>invalid</Tag>;

const renderQuotaStatusTag = (status) => {
  const normalized = String(status || '')
    .trim()
    .toLowerCase();
  switch (normalized) {
    case 'full':
      return <Tag color='green'>full</Tag>;
    case 'high':
      return <Tag color='blue'>high</Tag>;
    case 'medium':
      return <Tag color='blue'>medium</Tag>;
    case 'low':
      return <Tag color='orange'>low</Tag>;
    case 'exhausted':
      return <Tag color='red'>exhausted</Tag>;
    case 'error':
      return <Tag color='red'>error</Tag>;
    case 'unknown':
      return <Tag color='grey'>unknown</Tag>;
    default:
      return <Tag color='grey'>{normalized || 'unknown'}</Tag>;
  }
};

const renderDiagnoseDecisionTag = (decision) => {
  const normalized = String(decision || '')
    .trim()
    .toLowerCase();
  switch (normalized) {
    case 'selected':
      return <Tag color='green'>selected</Tag>;
    case 'eligible':
      return <Tag color='blue'>eligible</Tag>;
    default:
      return <Tag color='red'>blocked</Tag>;
  }
};

const normalizePlanTier = (value) => {
  const tier = String(value || '')
    .trim()
    .toLowerCase();
  if (!tier) {
    return 'unknown';
  }
  return tier;
};

const getPlanRank = (tier) => {
  const normalized = normalizePlanTier(tier);
  if (Object.prototype.hasOwnProperty.call(ACCOUNT_PLAN_RANK, normalized)) {
    return ACCOUNT_PLAN_RANK[normalized];
  }
  return ACCOUNT_PLAN_RANK.unknown;
};

const isResetPassed = (resetAt) => {
  const ts = Number(resetAt);
  if (!Number.isFinite(ts) || ts <= 0) {
    return false;
  }
  return Math.floor(Date.now() / 1000) >= ts;
};

const getRemainingPercentByWindow = (usedRaw, limitRaw, resetAtRaw) => {
  const used = Number(usedRaw);
  const limit = Number(limitRaw);
  if (!Number.isFinite(used) || !Number.isFinite(limit) || limit <= 0) {
    return null;
  }
  if (isResetPassed(resetAtRaw)) {
    return 100;
  }
  const remaining = ((limit - used) * 100) / limit;
  if (!Number.isFinite(remaining)) {
    return null;
  }
  return Math.max(0, Math.min(100, remaining));
};

const getWeeklyRemainingPercent = (record) => {
  const explicit = Number(record?.weekly_remaining_percent);
  if (Number.isFinite(explicit)) {
    return Math.max(0, Math.min(100, explicit));
  }
  return getRemainingPercentByWindow(
    record?.usage_weekly_used,
    record?.usage_weekly_limit,
    record?.usage_weekly_reset_at,
  );
};

const getFiveHourRemainingPercent = (record) => {
  return getRemainingPercentByWindow(
    record?.usage_5h_used,
    record?.usage_5h_limit,
    record?.usage_5h_reset_at,
  );
};

const getCodeReviewRemainingPercent = (record) => {
  return getRemainingPercentByWindow(
    record?.usage_code_review_used,
    record?.usage_code_review_limit,
    record?.usage_code_review_reset_at,
  );
};

const deriveQuotaStatusForReport = (record) => {
  const explicit = String(record?.quota_status || '')
    .trim()
    .toLowerCase();
  if (explicit) {
    return explicit;
  }
  if (String(record?.entitlement_error || '').trim()) {
    return 'error';
  }
  const weeklyRemaining = getWeeklyRemainingPercent(record);
  if (!Number.isFinite(weeklyRemaining)) {
    return 'unknown';
  }
  if (weeklyRemaining <= 0) return 'exhausted';
  if (weeklyRemaining <= 30) return 'low';
  if (weeklyRemaining <= 70) return 'medium';
  if (weeklyRemaining < 100) return 'high';
  return 'full';
};

const getQuotaStatusRank = (status) => {
  const key = String(status || '')
    .trim()
    .toLowerCase();
  if (Object.prototype.hasOwnProperty.call(ACCOUNT_QUOTA_STATUS_RANK, key)) {
    return ACCOUNT_QUOTA_STATUS_RANK[key];
  }
  return ACCOUNT_QUOTA_STATUS_RANK.unknown;
};

const pickSmartSwitchCandidate = (items) => {
  const ranked = sortAccountsByMode(items, 'smart_switch');
  if (ranked.length === 0) {
    return null;
  }
  const preferred = ranked.find((item) => {
    const status = String(item?.status || '')
      .trim()
      .toLowerCase();
    if (status !== 'enabled') {
      return false;
    }
    const quotaStatus = deriveQuotaStatusForReport(item);
    return quotaStatus !== 'exhausted' && quotaStatus !== 'error';
  });
  return preferred || ranked[0] || null;
};

const sortAccountsByMode = (items, mode) => {
  const list = Array.isArray(items) ? [...items] : [];
  if (mode === 'default') {
    return list;
  }

  list.sort((a, b) => {
    if (mode === 'quota_inspector') {
      const planDelta = getPlanRank(a?.entitlement_tier) - getPlanRank(b?.entitlement_tier);
      if (planDelta !== 0) return planDelta;
      const aRemaining = getWeeklyRemainingPercent(a);
      const bRemaining = getWeeklyRemainingPercent(b);
      if (Number.isFinite(aRemaining) && Number.isFinite(bRemaining)) {
        if (aRemaining !== bRemaining) {
          return aRemaining - bRemaining;
        }
      } else if (Number.isFinite(aRemaining)) {
        return -1;
      } else if (Number.isFinite(bRemaining)) {
        return 1;
      }
      const statusDelta =
        getQuotaStatusRank(deriveQuotaStatusForReport(a)) -
        getQuotaStatusRank(deriveQuotaStatusForReport(b));
      if (statusDelta !== 0) return statusDelta;
      return String(a?.name || '').localeCompare(String(b?.name || ''));
    }

    if (mode === 'smart_switch') {
      const aWeekly = getWeeklyRemainingPercent(a);
      const bWeekly = getWeeklyRemainingPercent(b);
      if (Number.isFinite(aWeekly) && Number.isFinite(bWeekly)) {
        if (aWeekly !== bWeekly) {
          return bWeekly - aWeekly;
        }
      } else if (Number.isFinite(aWeekly)) {
        return -1;
      } else if (Number.isFinite(bWeekly)) {
        return 1;
      }

      const aFive = getFiveHourRemainingPercent(a);
      const bFive = getFiveHourRemainingPercent(b);
      if (Number.isFinite(aFive) && Number.isFinite(bFive)) {
        if (aFive !== bFive) {
          return bFive - aFive;
        }
      } else if (Number.isFinite(aFive)) {
        return -1;
      } else if (Number.isFinite(bFive)) {
        return 1;
      }

      const aScore = Number(a?.route_score);
      const bScore = Number(b?.route_score);
      const normalizedA = Number.isFinite(aScore) ? aScore : -1;
      const normalizedB = Number.isFinite(bScore) ? bScore : -1;
      if (normalizedA !== normalizedB) {
        return normalizedB - normalizedA;
      }

      return String(a?.name || '').localeCompare(String(b?.name || ''));
    }

    if (mode === 'weekly_remaining_asc') {
      const aRemaining = getWeeklyRemainingPercent(a);
      const bRemaining = getWeeklyRemainingPercent(b);
      if (Number.isFinite(aRemaining) && Number.isFinite(bRemaining)) {
        if (aRemaining !== bRemaining) {
          return aRemaining - bRemaining;
        }
      } else if (Number.isFinite(aRemaining)) {
        return -1;
      } else if (Number.isFinite(bRemaining)) {
        return 1;
      }
      return String(a?.name || '').localeCompare(String(b?.name || ''));
    }

    if (mode === 'route_score_desc') {
      const aScore = Number(a?.route_score);
      const bScore = Number(b?.route_score);
      const normalizedA = Number.isFinite(aScore) ? aScore : -1;
      const normalizedB = Number.isFinite(bScore) ? bScore : -1;
      if (normalizedA !== normalizedB) {
        return normalizedB - normalizedA;
      }
      return String(a?.name || '').localeCompare(String(b?.name || ''));
    }

    if (mode === 'name_asc') {
      return String(a?.name || '').localeCompare(String(b?.name || ''));
    }

    return 0;
  });

  return list;
};

const getWeeklyRemainingProgressColor = (percent) => {
  if (!Number.isFinite(percent)) {
    return 'var(--semi-color-border)';
  }
  if (percent <= 0) return 'var(--semi-color-danger)';
  if (percent <= 30) return 'var(--semi-color-warning)';
  if (percent <= 70) return 'var(--semi-color-info)';
  return 'var(--semi-color-success)';
};

const buildQuotaInspectorReport = (items) => {
  const normalized = Array.isArray(items)
    ? items.map((item) => {
        const weeklyRemainingPercent = getWeeklyRemainingPercent(item);
        const quotaStatus = deriveQuotaStatusForReport(item);
        return {
          id: Number(item?.id) || 0,
          name: item?.name || '',
          account: item?.account || '',
          status: item?.status || 'unknown',
          plan_type: normalizePlanTier(item?.entitlement_tier),
          quota_status: quotaStatus,
          weekly_remaining_percent: Number.isFinite(weeklyRemainingPercent)
            ? Number(weeklyRemainingPercent.toFixed(2))
            : null,
          usage_5h_used: Number(item?.usage_5h_used) || 0,
          usage_5h_limit: Number(item?.usage_5h_limit) || 0,
          usage_weekly_used: Number(item?.usage_weekly_used) || 0,
          usage_weekly_limit: Number(item?.usage_weekly_limit) || 0,
          usage_weekly_reset_at: Number(item?.usage_weekly_reset_at) || 0,
          usage_code_review_used: Number(item?.usage_code_review_used) || 0,
          usage_code_review_limit: Number(item?.usage_code_review_limit) || 0,
          usage_code_review_reset_at: Number(item?.usage_code_review_reset_at) || 0,
          entitlement_checked_at: Number(item?.entitlement_checked_at) || 0,
          entitlement_error: item?.entitlement_error || '',
          route_score: Number(item?.route_score) || 0,
        };
      })
    : [];

  const sorted = sortAccountsByMode(normalized, 'quota_inspector');
  const statusCounts = {};
  const planCounts = {};
  let freeEquivalent7D = 0;
  let plusEquivalent7D = 0;

  sorted.forEach((item) => {
    const status = String(item?.quota_status || 'unknown');
    const plan = String(item?.plan_type || 'unknown');
    statusCounts[status] = (statusCounts[status] || 0) + 1;
    planCounts[plan] = (planCounts[plan] || 0) + 1;
    const remaining = Number(item?.weekly_remaining_percent);
    if (Number.isFinite(remaining)) {
      if (plan === 'free') {
        freeEquivalent7D += remaining;
      }
      if (plan === 'plus') {
        plusEquivalent7D += remaining;
      }
    }
  });

  return {
    generated_at: Date.now(),
    accounts: sorted.length,
    status_counts: statusCounts,
    plan_counts: planCounts,
    exhausted_accounts: Number(statusCounts.exhausted || 0),
    low_accounts: Number(statusCounts.low || 0),
    error_accounts: Number(statusCounts.error || 0),
    free_equivalent_7d:
      planCounts.free > 0 ? Number((freeEquivalent7D / planCounts.free).toFixed(2)) : 0,
    plus_equivalent_7d:
      planCounts.plus > 0 ? Number((plusEquivalent7D / planCounts.plus).toFixed(2)) : 0,
    sort_mode: 'quota_inspector',
    items: sorted,
  };
};

const renderUsage = (used, limit) => {
  const safeUsed = Number.isFinite(Number(used)) ? Number(used) : 0;
  const safeLimit = Number.isFinite(Number(limit)) ? Number(limit) : 0;
  if (safeUsed === 0 && safeLimit === 0) {
    return '-';
  }
  if (safeLimit === 100 && safeUsed >= 0 && safeUsed <= 100) {
    return `${safeUsed}%`;
  }
  if (safeLimit > 0) {
    return `${safeUsed}/${safeLimit}`;
  }
  return `${safeUsed}`;
};

const triggerBlobDownload = (blob, filename) => {
  const url = window.URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
  window.URL.revokeObjectURL(url);
};

const getFilenameFromDisposition = (contentDisposition, fallbackName) => {
  const text = String(contentDisposition || '');
  const utf8Match = text.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1]);
    } catch {
      return fallbackName;
    }
  }
  const basicMatch = text.match(/filename="?([^";]+)"?/i);
  if (basicMatch?.[1]) {
    return basicMatch[1];
  }
  return fallbackName;
};

const parseCsvLine = (line, delimiter) => {
  const output = [];
  let current = '';
  let inQuotes = false;

  for (let i = 0; i < line.length; i++) {
    const char = line[i];
    if (char === '"') {
      if (inQuotes && line[i + 1] === '"') {
        current += '"';
        i++;
        continue;
      }
      inQuotes = !inQuotes;
      continue;
    }

    if (!inQuotes && char === delimiter) {
      output.push(current.trim());
      current = '';
      continue;
    }

    current += char;
  }

  output.push(current.trim());
  return output;
};

const pickAliasField = (source, aliases) => {
  for (const alias of aliases) {
    if (Object.prototype.hasOwnProperty.call(source, alias)) {
      return source[alias];
    }
  }
  return undefined;
};

const buildAccountPayloadFromAny = (raw) => {
  if (!raw || typeof raw !== 'object') {
    return null;
  }

  const normalized = {};
  Object.keys(raw).forEach((key) => {
    normalized[String(key).trim().toLowerCase()] = raw[key];
  });

  const nestedAuth =
    normalized.auth_json && typeof normalized.auth_json === 'object'
      ? normalized.auth_json
      : null;
  const nestedTokens =
    (normalized.tokens && typeof normalized.tokens === 'object'
      ? normalized.tokens
      : null) ||
    (normalized.token && typeof normalized.token === 'object'
      ? normalized.token
      : null) ||
    (nestedAuth?.tokens && typeof nestedAuth.tokens === 'object'
      ? nestedAuth.tokens
      : null) ||
    (nestedAuth?.token && typeof nestedAuth.token === 'object'
      ? nestedAuth.token
      : null);

  const nameRaw = pickAliasField(normalized, accountFieldAliases.name);
  const name = String(nameRaw || '').trim();
  const account = String(
    pickAliasField(normalized, accountFieldAliases.account) ||
      nestedAuth?.account_id ||
      nestedAuth?.chatgpt_account_id ||
      nestedAuth?.chatgpt_account ||
      '',
  ).trim();
  const apiKey = String(
    pickAliasField(normalized, accountFieldAliases.api_key) ||
      nestedTokens?.access_token ||
      nestedAuth?.access_token ||
      '',
  ).trim();
  const idToken = String(
    pickAliasField(normalized, accountFieldAliases.id_token) ||
      nestedTokens?.id_token ||
      nestedAuth?.id_token ||
      '',
  ).trim();
  const refreshToken = String(
    pickAliasField(normalized, accountFieldAliases.refresh_token) ||
      nestedTokens?.refresh_token ||
      nestedAuth?.refresh_token ||
      '',
  ).trim();

  const baseURLRaw = pickAliasField(normalized, accountFieldAliases.base_url);
  const importType = String(
    pickAliasField(normalized, accountFieldAliases.type) ||
      normalized.type ||
      nestedAuth?.type ||
      '',
  )
    .trim()
    .toLowerCase();
  const importEmail = String(
    pickAliasField(normalized, accountFieldAliases.email) || nestedAuth?.email || '',
  )
    .trim()
    .toLowerCase();
  const codexStyleToken = apiKey && !apiKey.toLowerCase().startsWith('sk-');
  const baseURL = String(
    baseURLRaw ||
      (importType === 'codex' || codexStyleToken
        ? 'https://chatgpt.com'
        : 'https://api.openai.com'),
  ).trim();

  const fallbackName =
    name ||
    String(importEmail || account || 'chatmock-account').trim();

  const payload = {
    name: fallbackName,
    account,
    api_key: apiKey,
    id_token: idToken,
    refresh_token: refreshToken,
    type: importType,
    email: importEmail,
    base_url: baseURL || 'https://api.openai.com',
    proxy_url: String(
      pickAliasField(normalized, accountFieldAliases.proxy_url) || '',
    ).trim(),
    status: normalizeAccountStatus(
      pickAliasField(normalized, accountFieldAliases.status),
    ),
    weight: toNumberOrDefault(
      pickAliasField(normalized, accountFieldAliases.weight),
      1,
    ),
    priority: toNumberOrDefault(
      pickAliasField(normalized, accountFieldAliases.priority),
      100,
    ),
    max_concurrency: toNumberOrDefault(
      pickAliasField(normalized, accountFieldAliases.max_concurrency),
      2,
    ),
    rpm: toNumberOrDefault(pickAliasField(normalized, accountFieldAliases.rpm), 60),
    daily_token_limit: toNumberOrDefault(
      pickAliasField(normalized, accountFieldAliases.daily_token_limit),
      0,
    ),
    supported_models: splitSupportedModels(
      pickAliasField(normalized, accountFieldAliases.supported_models),
    ),
    metadata: String(
      pickAliasField(normalized, accountFieldAliases.metadata) || '',
    ).trim(),
  };

  return payload;
};

const parseBatchAccountsInput = (inputText) => {
  const text = String(inputText || '').trim();
  if (!text) {
    return [];
  }

  if (text.startsWith('[') || text.startsWith('{')) {
    const parsed = JSON.parse(text);
    if (Array.isArray(parsed)) {
      return parsed
        .map((item) => buildAccountPayloadFromAny(item))
        .filter(Boolean);
    }

    if (parsed && typeof parsed === 'object') {
      const candidates = [parsed.items, parsed.accounts, parsed.data, parsed.records];
      for (const collection of candidates) {
        if (Array.isArray(collection)) {
          return collection
            .map((item) => buildAccountPayloadFromAny(item))
            .filter(Boolean);
        }
      }
      const single = buildAccountPayloadFromAny(parsed);
      return single ? [single] : [];
    }

    throw new Error('JSON 批量导入格式不正确');
  }

  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);

  if (lines.length === 0) {
    return [];
  }

  if (lines.every((line) => line.startsWith('{') && line.endsWith('}'))) {
    return lines
      .map((line) => JSON.parse(line))
      .map((item) => buildAccountPayloadFromAny(item))
      .filter(Boolean);
  }

  const delimiter = lines.some((line) => line.includes('\t')) ? '\t' : ',';
  const rows = lines.map((line) => parseCsvLine(line, delimiter));
  if (rows.length === 0) {
    return [];
  }

  const first = rows[0].map((value) => String(value || '').trim().toLowerCase());
  const knownHeaders = new Set(csvDefaultHeaders);
  const hasHeader = first.some((value) => knownHeaders.has(value));

  const headers = hasHeader ? first : csvDefaultHeaders;
  const dataRows = hasHeader ? rows.slice(1) : rows;

  return dataRows
    .map((row) => {
      const record = {};
      headers.forEach((header, index) => {
        record[header] = row[index];
      });
      return buildAccountPayloadFromAny(record);
    })
    .filter(Boolean);
};

const normalizeMailboxSecurityMode = (value, fallback = 'ssl') => {
  const text = String(value || '')
    .trim()
    .toLowerCase();
  if (text === 'starttls' || text === 'none' || text === 'ssl' || text === 'tls') {
    if (text === 'tls') {
      return 'ssl';
    }
    return text;
  }
  return fallback;
};

const buildMailboxPayloadFromAny = (raw) => {
  if (!raw || typeof raw !== 'object') {
    return null;
  }
  const normalized = {};
  Object.keys(raw).forEach((key) => {
    normalized[String(key).trim().toLowerCase()] = raw[key];
  });

  const providerRaw = pickAliasField(normalized, mailboxFieldAliases.provider_template);
  const provider = String(providerRaw || 'custom')
    .trim()
    .toLowerCase();
  const preset = MAILBOX_PROVIDER_PRESETS[provider] || MAILBOX_PROVIDER_PRESETS.custom;
  const presetValues = preset?.values || {};

  const email = String(pickAliasField(normalized, mailboxFieldAliases.email) || '')
    .trim()
    .toLowerCase();
  const username = String(
    pickAliasField(normalized, mailboxFieldAliases.username) || email,
  ).trim();
  const name = String(
    pickAliasField(normalized, mailboxFieldAliases.name) || email || username || 'mailbox',
  ).trim();

  const payload = {
    name,
    email,
    username,
    password: String(pickAliasField(normalized, mailboxFieldAliases.password) || '').trim(),
    imap_host: String(
      pickAliasField(normalized, mailboxFieldAliases.imap_host) ||
        presetValues.imap_host ||
        '',
    ).trim(),
    imap_port: toNumberOrDefault(
      pickAliasField(normalized, mailboxFieldAliases.imap_port),
      presetValues.imap_port || 993,
    ),
    imap_security: normalizeMailboxSecurityMode(
      pickAliasField(normalized, mailboxFieldAliases.imap_security),
      presetValues.imap_security || 'ssl',
    ),
    smtp_host: String(
      pickAliasField(normalized, mailboxFieldAliases.smtp_host) ||
        presetValues.smtp_host ||
        '',
    ).trim(),
    smtp_port: toNumberOrDefault(
      pickAliasField(normalized, mailboxFieldAliases.smtp_port),
      presetValues.smtp_port || 465,
    ),
    smtp_security: normalizeMailboxSecurityMode(
      pickAliasField(normalized, mailboxFieldAliases.smtp_security),
      presetValues.smtp_security || 'ssl',
    ),
    status: normalizeMailboxStatus(
      pickAliasField(normalized, mailboxFieldAliases.status),
    ),
    poll_interval_second: toNumberOrDefault(
      pickAliasField(normalized, mailboxFieldAliases.poll_interval_second),
      60,
    ),
    metadata: String(
      pickAliasField(normalized, mailboxFieldAliases.metadata) || '',
    ).trim(),
  };

  return payload;
};

const parseBatchMailboxesInput = (inputText) => {
  const text = String(inputText || '').trim();
  if (!text) {
    return [];
  }

  if (text.startsWith('[') || text.startsWith('{')) {
    const parsed = JSON.parse(text);
    if (Array.isArray(parsed)) {
      return parsed
        .map((item) => buildMailboxPayloadFromAny(item))
        .filter(Boolean);
    }
    if (parsed && typeof parsed === 'object') {
      const candidates = [parsed.items, parsed.mailboxes, parsed.data, parsed.records];
      for (const collection of candidates) {
        if (Array.isArray(collection)) {
          return collection
            .map((item) => buildMailboxPayloadFromAny(item))
            .filter(Boolean);
        }
      }
      const single = buildMailboxPayloadFromAny(parsed);
      return single ? [single] : [];
    }
    throw new Error('JSON 批量导入格式不正确');
  }

  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  if (lines.length === 0) {
    return [];
  }

  if (lines.every((line) => line.startsWith('{') && line.endsWith('}'))) {
    return lines
      .map((line) => JSON.parse(line))
      .map((item) => buildMailboxPayloadFromAny(item))
      .filter(Boolean);
  }

  const delimiter = lines.some((line) => line.includes('\t')) ? '\t' : ',';
  const rows = lines.map((line) => parseCsvLine(line, delimiter));
  if (rows.length === 0) {
    return [];
  }
  const first = rows[0].map((value) => String(value || '').trim().toLowerCase());
  const knownHeaders = new Set(mailboxCsvDefaultHeaders);
  const hasHeader = first.some((value) => knownHeaders.has(value));
  const headers = hasHeader ? first : mailboxCsvDefaultHeaders;
  const dataRows = hasHeader ? rows.slice(1) : rows;

  return dataRows
    .map((row) => {
      const record = {};
      headers.forEach((header, index) => {
        record[header] = row[index];
      });
      return buildMailboxPayloadFromAny(record);
    })
    .filter(Boolean);
};

const buildPoolConfigPayload = (poolConfig) => ({
  enable_weighted: !!poolConfig.enable_weighted,
  default_cooldown_second: Number(poolConfig.default_cooldown_second) || 300,
  request_timeout_second: Number(poolConfig.request_timeout_second) || 60,
  max_retry: Number(poolConfig.max_retry) || 3,
  health_check_interval: Number(poolConfig.health_check_interval) || 300,
  entitlement_check_interval: Number(poolConfig.entitlement_check_interval) || 0,
  default_instructions: String(poolConfig.default_instructions || '').trim(),
});

const ChatMockPage = () => {
  const { t } = useTranslation();

  const [activeTab, setActiveTab] = useState('accounts');
  const [operationMode, setOperationMode] = useState(() => {
    if (typeof window === 'undefined') {
      return 'basic';
    }
    return normalizeOperationMode(
      window.localStorage.getItem(CHATMOCK_OPERATION_MODE_KEY),
    );
  });
  const isAdvancedMode = operationMode === 'advanced';

  const [loading, setLoading] = useState(false);
  const [savingConfig, setSavingConfig] = useState(false);
  const [accounts, setAccounts] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [accountKeyword, setAccountKeyword] = useState('');
  const [accountStatusFilter, setAccountStatusFilter] = useState('');
  const [accountTierFilter, setAccountTierFilter] = useState('');
  const [accountQuotaStatusFilter, setAccountQuotaStatusFilter] = useState('');
  const [accountSortMode, setAccountSortMode] = useState('default');
  const [accountSummary, setAccountSummary] = useState(DEFAULT_ACCOUNT_SUMMARY);
  const [selectedAccountRowKeys, setSelectedAccountRowKeys] = useState([]);

  const [poolConfig, setPoolConfig] = useState(DEFAULT_POOL_CONFIG);
  const [selectedPoolPreset, setSelectedPoolPreset] = useState('fusion_stable');
  const [poolChannelStatus, setPoolChannelStatus] = useState(
    DEFAULT_POOL_CHANNEL_STATUS,
  );

  const [formVisible, setFormVisible] = useState(false);
  const [editingAccount, setEditingAccount] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  const [formApi, setFormApi] = useState(null);

  const [batchImportVisible, setBatchImportVisible] = useState(false);
  const [batchImportText, setBatchImportText] = useState('');
  const [batchImportSubmitting, setBatchImportSubmitting] = useState(false);
  const [batchImportUploading, setBatchImportUploading] = useState(false);
  const [batchImportFileName, setBatchImportFileName] = useState('');
  const [batchImportResult, setBatchImportResult] = useState(null);
  const [batchCheckingHealth, setBatchCheckingHealth] = useState(false);
  const [batchCheckingEntitlement, setBatchCheckingEntitlement] = useState(false);
  const [batchRefreshingAuth, setBatchRefreshingAuth] = useState(false);
  const [batchInspecting, setBatchInspecting] = useState(false);
  const [batchInspectAndSyncing, setBatchInspectAndSyncing] = useState(false);
  const [stabilizingPool, setStabilizingPool] = useState(false);
  const [exportingAccounts, setExportingAccounts] = useState(false);
  const [includeSensitiveExport, setIncludeSensitiveExport] = useState(false);
  const [syncingPoolChannel, setSyncingPoolChannel] = useState(false);
  const [batchMutatingAccounts, setBatchMutatingAccounts] = useState(false);
  const [batchConcurrency, setBatchConcurrency] = useState(6);
  const [batchRetryAttempts, setBatchRetryAttempts] = useState(2);
  const [diagnoseModel, setDiagnoseModel] = useState(DEFAULT_ROUTE_DIAGNOSE_MODEL);
  const [diagnoseLoading, setDiagnoseLoading] = useState(false);
  const [diagnoseVisible, setDiagnoseVisible] = useState(false);
  const [diagnoseResult, setDiagnoseResult] = useState(null);
  const batchImportFileInputRef = useRef(null);

  const [mailboxEnabled, setMailboxEnabled] = useState(true);
  const [mailboxHint, setMailboxHint] = useState('');
  const [mailboxesLoading, setMailboxesLoading] = useState(false);
  const [mailboxes, setMailboxes] = useState([]);
  const [mailboxTotal, setMailboxTotal] = useState(0);
  const [mailboxPage, setMailboxPage] = useState(1);
  const [mailboxPageSize, setMailboxPageSize] = useState(10);
  const [mailboxKeyword, setMailboxKeyword] = useState('');
  const [mailboxStatusFilter, setMailboxStatusFilter] = useState('');

  const [mailboxFormVisible, setMailboxFormVisible] = useState(false);
  const [editingMailbox, setEditingMailbox] = useState(null);
  const [mailboxSubmitting, setMailboxSubmitting] = useState(false);
  const [mailboxFormApi, setMailboxFormApi] = useState(null);
  const [, setMailboxProviderPreset] = useState('qq');
  const [mailboxBatchImportVisible, setMailboxBatchImportVisible] = useState(false);
  const [mailboxBatchImportText, setMailboxBatchImportText] = useState('');
  const [mailboxBatchImportSubmitting, setMailboxBatchImportSubmitting] =
    useState(false);
  const [mailboxBatchImportUploading, setMailboxBatchImportUploading] =
    useState(false);
  const [mailboxBatchImportFileName, setMailboxBatchImportFileName] =
    useState('');
  const [mailboxBatchImportResult, setMailboxBatchImportResult] = useState(null);
  const [exportingMailboxes, setExportingMailboxes] = useState(false);
  const mailboxBatchImportFileInputRef = useRef(null);
  const [mailboxBatchTesting, setMailboxBatchTesting] = useState(false);

  const [eventsLoading, setEventsLoading] = useState(false);
  const [events, setEvents] = useState([]);
  const [eventsTotal, setEventsTotal] = useState(0);
  const [eventsPage, setEventsPage] = useState(1);
  const [eventsPageSize, setEventsPageSize] = useState(10);
  const [eventsMailboxId, setEventsMailboxId] = useState(0);
  const [eventsType, setEventsType] = useState('');
  const [eventsClassification, setEventsClassification] = useState('');
  const [eventsKeyword, setEventsKeyword] = useState('');

  const [routeLogsLoading, setRouteLogsLoading] = useState(false);
  const [routeLogs, setRouteLogs] = useState([]);
  const [routeLogsTotal, setRouteLogsTotal] = useState(0);
  const [routeLogsPage, setRouteLogsPage] = useState(1);
  const [routeLogsPageSize, setRouteLogsPageSize] = useState(10);
  const [routeLogsAccountId, setRouteLogsAccountId] = useState(0);
  const [routeLogsModel, setRouteLogsModel] = useState('');
  const [routeLogsSuccess, setRouteLogsSuccess] = useState('');
  const [routeLogsStrategy, setRouteLogsStrategy] = useState('');
  const [routeLogsKeyword, setRouteLogsKeyword] = useState('');

  const [snapshotsLoading, setSnapshotsLoading] = useState(false);
  const [snapshots, setSnapshots] = useState([]);
  const [snapshotsTotal, setSnapshotsTotal] = useState(0);
  const [snapshotsPage, setSnapshotsPage] = useState(1);
  const [snapshotsPageSize, setSnapshotsPageSize] = useState(10);
  const [snapshotsAccountId, setSnapshotsAccountId] = useState(0);
  const [snapshotsType, setSnapshotsType] = useState('');
  const [snapshotsStatus, setSnapshotsStatus] = useState('');
  const [snapshotsKeyword, setSnapshotsKeyword] = useState('');

  const handleMailboxDisabled = useCallback((message) => {
    setMailboxEnabled(false);
    setMailboxHint(message || 'CHATMOCK_MAILBOX_ENABLED=false，邮箱中心未启用');
    setMailboxes([]);
    setMailboxTotal(0);
    setEvents([]);
    setEventsTotal(0);
  }, []);

  const loadPoolConfig = useCallback(async () => {
    try {
      const res = await API.get('/api/admin/chatmock/pool-config');
      const { success, data, message } = res.data;
      if (!success) {
        showError(message || t('加载账号池配置失败'));
        return;
      }
      setPoolConfig({
        ...DEFAULT_POOL_CONFIG,
        ...(data || {}),
      });
    } catch (error) {
      showError(toSafeErrorMessage(error, t('加载账号池配置失败')));
    }
  }, [t]);

  const loadPoolChannelStatus = useCallback(async () => {
    try {
      const res = await API.get('/api/admin/chatmock/channel-status');
      const { success, data, message } = res.data;
      if (!success) {
        showError(message || t('加载渠道接入状态失败'));
        setPoolChannelStatus(DEFAULT_POOL_CHANNEL_STATUS);
        return;
      }
      setPoolChannelStatus({
        ...DEFAULT_POOL_CHANNEL_STATUS,
        ...(data || {}),
      });
    } catch (error) {
      if (Number(error?.response?.status) === 404) {
        setPoolChannelStatus(DEFAULT_POOL_CHANNEL_STATUS);
        return;
      }
      showError(toSafeErrorMessage(error, t('加载渠道接入状态失败')));
      setPoolChannelStatus(DEFAULT_POOL_CHANNEL_STATUS);
    }
  }, [t]);

  const loadAccountSummary = useCallback(async () => {
    try {
      const res = await API.get('/api/admin/chatmock/accounts/summary', {
        params: {
          keyword: accountKeyword || undefined,
          status: accountStatusFilter || undefined,
          tier: accountTierFilter || undefined,
          quota_status: accountQuotaStatusFilter || undefined,
        },
      });
      const { success, data, message } = res.data;
      if (!success) {
        showError(message || t('加载账号池汇总失败'));
        setAccountSummary(DEFAULT_ACCOUNT_SUMMARY);
        return;
      }
      setAccountSummary({
        ...DEFAULT_ACCOUNT_SUMMARY,
        ...(data || {}),
      });
    } catch (error) {
      showError(toSafeErrorMessage(error, t('加载账号池汇总失败')));
      setAccountSummary(DEFAULT_ACCOUNT_SUMMARY);
    }
  }, [
    accountKeyword,
    accountStatusFilter,
    accountTierFilter,
    accountQuotaStatusFilter,
    t,
  ]);

  const loadAccounts = useCallback(
    async (nextPage = page, nextPageSize = pageSize) => {
      setLoading(true);
      try {
        const res = await API.get('/api/admin/chatmock/accounts', {
          params: {
            p: nextPage,
            page_size: nextPageSize,
            keyword: accountKeyword || undefined,
            status: accountStatusFilter || undefined,
            tier: accountTierFilter || undefined,
            quota_status: accountQuotaStatusFilter || undefined,
          },
        });
        const { success, data, message } = res.data;
        if (!success) {
          showError(message || t('加载账号池失败'));
          setAccounts([]);
          setTotal(0);
          return;
        }
        const items = Array.isArray(data?.items) ? data.items : [];
        setAccounts(items);
        setTotal(data?.total || 0);
        setPage(data?.page || nextPage);
        setSelectedAccountRowKeys((prev) =>
          prev.filter((key) =>
            items.some((item) => Number(item?.id) === Number(key)),
          ),
        );
      } catch (error) {
        showError(toSafeErrorMessage(error, t('加载账号池失败')));
        setAccounts([]);
        setTotal(0);
        setSelectedAccountRowKeys([]);
      } finally {
        setLoading(false);
        loadAccountSummary();
      }
    },
    [
      page,
      pageSize,
      accountKeyword,
      accountStatusFilter,
      accountTierFilter,
      accountQuotaStatusFilter,
      loadAccountSummary,
      t,
    ],
  );

  const loadMailboxes = useCallback(
    async (nextPage = mailboxPage, nextPageSize = mailboxPageSize) => {
      setMailboxesLoading(true);
      try {
        const res = await API.get('/api/admin/chatmock/mailboxes', {
          params: {
            p: nextPage,
            page_size: nextPageSize,
            keyword: mailboxKeyword || undefined,
            status: mailboxStatusFilter || undefined,
          },
        });
        const { success, data, message } = res.data;
        if (!success) {
          if (String(message || '').includes('CHATMOCK_MAILBOX_ENABLED=false')) {
            handleMailboxDisabled(message);
            return;
          }
          showError(message || t('加载邮箱列表失败'));
          return;
        }
        setMailboxEnabled(true);
        setMailboxHint('');
        const items = Array.isArray(data?.items) ? data.items : [];
        setMailboxes(items.map((item) => normalizeMailboxRecord(item)));
        setMailboxTotal(data?.total || 0);
        setMailboxPage(data?.page || nextPage);
      } catch (error) {
        const message = toSafeErrorMessage(error, t('加载邮箱列表失败'));
        if (String(message).includes('CHATMOCK_MAILBOX_ENABLED=false')) {
          handleMailboxDisabled(message);
          return;
        }
        showError(message);
      } finally {
        setMailboxesLoading(false);
      }
    },
    [mailboxPage, mailboxPageSize, mailboxKeyword, mailboxStatusFilter, t, handleMailboxDisabled],
  );

  const loadMailEvents = useCallback(
    async (
      nextPage = eventsPage,
      nextPageSize = eventsPageSize,
      mailboxID = eventsMailboxId,
      eventType = eventsType,
      classification = eventsClassification,
      keyword = eventsKeyword,
    ) => {
      if (!mailboxEnabled) {
        return;
      }
      setEventsLoading(true);
      try {
        const res = await API.get('/api/admin/chatmock/mail-events', {
          params: {
            mailbox_id: mailboxID || undefined,
            event_type: eventType || undefined,
            classification: classification || undefined,
            keyword: keyword || undefined,
            p: nextPage,
            page_size: nextPageSize,
          },
        });
        const { success, data, message } = res.data;
        if (!success) {
          if (String(message || '').includes('CHATMOCK_MAILBOX_ENABLED=false')) {
            handleMailboxDisabled(message);
            return;
          }
          showError(message || t('加载邮件事件失败'));
          return;
        }
        const items = Array.isArray(data?.items) ? data.items : [];
        setEvents(items);
        setEventsTotal(data?.total || 0);
        setEventsPage(data?.page || nextPage);
      } catch (error) {
        const message = toSafeErrorMessage(error, t('加载邮件事件失败'));
        if (String(message).includes('CHATMOCK_MAILBOX_ENABLED=false')) {
          handleMailboxDisabled(message);
          return;
        }
        showError(message);
      } finally {
        setEventsLoading(false);
      }
    },
    [
      eventsPage,
      eventsPageSize,
      eventsMailboxId,
      eventsType,
      eventsClassification,
      eventsKeyword,
      mailboxEnabled,
      t,
      handleMailboxDisabled,
    ],
  );

  const loadRouteLogs = useCallback(
    async (
      nextPage = routeLogsPage,
      nextPageSize = routeLogsPageSize,
      accountID = routeLogsAccountId,
      modelName = routeLogsModel,
      success = routeLogsSuccess,
      strategy = routeLogsStrategy,
      keyword = routeLogsKeyword,
    ) => {
      setRouteLogsLoading(true);
      try {
        const res = await API.get('/api/admin/chatmock/route-logs', {
          params: {
            p: nextPage,
            page_size: nextPageSize,
            account_id: accountID || undefined,
            model: modelName || undefined,
            success: success || undefined,
            strategy: strategy || undefined,
            keyword: keyword || undefined,
          },
        });
        const { success: ok, data, message } = res.data;
        if (!ok) {
          showError(message || t('加载路由日志失败'));
          setRouteLogs([]);
          setRouteLogsTotal(0);
          return;
        }
        setRouteLogs(Array.isArray(data?.items) ? data.items : []);
        setRouteLogsTotal(Number(data?.total) || 0);
        setRouteLogsPage(Number(data?.page) || nextPage);
      } catch (error) {
        showError(toSafeErrorMessage(error, t('加载路由日志失败')));
        setRouteLogs([]);
        setRouteLogsTotal(0);
      } finally {
        setRouteLogsLoading(false);
      }
    },
    [
      routeLogsPage,
      routeLogsPageSize,
      routeLogsAccountId,
      routeLogsModel,
      routeLogsSuccess,
      routeLogsStrategy,
      routeLogsKeyword,
      t,
    ],
  );

  const loadAccountSnapshots = useCallback(
    async (
      nextPage = snapshotsPage,
      nextPageSize = snapshotsPageSize,
      accountID = snapshotsAccountId,
      snapshotType = snapshotsType,
      status = snapshotsStatus,
      keyword = snapshotsKeyword,
    ) => {
      setSnapshotsLoading(true);
      try {
        const res = await API.get('/api/admin/chatmock/account-snapshots', {
          params: {
            p: nextPage,
            page_size: nextPageSize,
            account_id: accountID || undefined,
            snapshot_type: snapshotType || undefined,
            status: status || undefined,
            keyword: keyword || undefined,
          },
        });
        const { success: ok, data, message } = res.data;
        if (!ok) {
          showError(message || t('加载账号快照失败'));
          setSnapshots([]);
          setSnapshotsTotal(0);
          return;
        }
        setSnapshots(Array.isArray(data?.items) ? data.items : []);
        setSnapshotsTotal(Number(data?.total) || 0);
        setSnapshotsPage(Number(data?.page) || nextPage);
      } catch (error) {
        showError(toSafeErrorMessage(error, t('加载账号快照失败')));
        setSnapshots([]);
        setSnapshotsTotal(0);
      } finally {
        setSnapshotsLoading(false);
      }
    },
    [
      snapshotsPage,
      snapshotsPageSize,
      snapshotsAccountId,
      snapshotsType,
      snapshotsStatus,
      snapshotsKeyword,
      t,
    ],
  );

  useEffect(() => {
    loadPoolConfig();
  }, [loadPoolConfig]);

  useEffect(() => {
    loadPoolChannelStatus();
  }, [loadPoolChannelStatus]);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    window.localStorage.setItem(
      CHATMOCK_OPERATION_MODE_KEY,
      normalizeOperationMode(operationMode),
    );
  }, [operationMode]);

  useEffect(() => {
    loadAccounts(page, pageSize);
  }, [loadAccounts, page, pageSize]);

  useEffect(() => {
    loadMailboxes(mailboxPage, mailboxPageSize);
  }, [loadMailboxes, mailboxPage, mailboxPageSize]);

  useEffect(() => {
    loadMailEvents(
      eventsPage,
      eventsPageSize,
      eventsMailboxId,
      eventsType,
      eventsClassification,
      eventsKeyword,
    );
  }, [
    loadMailEvents,
    eventsPage,
    eventsPageSize,
    eventsMailboxId,
    eventsType,
    eventsClassification,
    eventsKeyword,
  ]);

  useEffect(() => {
    if (!isAdvancedMode) {
      return;
    }
    loadRouteLogs(
      routeLogsPage,
      routeLogsPageSize,
      routeLogsAccountId,
      routeLogsModel,
      routeLogsSuccess,
      routeLogsStrategy,
      routeLogsKeyword,
    );
  }, [
    isAdvancedMode,
    loadRouteLogs,
    routeLogsPage,
    routeLogsPageSize,
    routeLogsAccountId,
    routeLogsModel,
    routeLogsSuccess,
    routeLogsStrategy,
    routeLogsKeyword,
  ]);

  useEffect(() => {
    if (!isAdvancedMode) {
      return;
    }
    loadAccountSnapshots(
      snapshotsPage,
      snapshotsPageSize,
      snapshotsAccountId,
      snapshotsType,
      snapshotsStatus,
      snapshotsKeyword,
    );
  }, [
    isAdvancedMode,
    loadAccountSnapshots,
    snapshotsPage,
    snapshotsPageSize,
    snapshotsAccountId,
    snapshotsType,
    snapshotsStatus,
    snapshotsKeyword,
  ]);

  const openCreateModal = () => {
    setEditingAccount(null);
    setFormVisible(true);
    setTimeout(() => {
      formApi?.setValues(DEFAULT_FORM_VALUES, { isOverride: true });
    }, 0);
  };

  const openEditModal = (record) => {
    setEditingAccount(record);
    setFormVisible(true);
    setTimeout(() => {
      formApi?.setValues(
        {
          name: record?.name || '',
          account: record?.account || '',
          api_key: '',
          id_token: '',
          refresh_token: '',
          base_url: record?.base_url || 'https://api.openai.com',
          proxy_url: record?.proxy_url || '',
          status: record?.status || 'enabled',
          weight: record?.weight ?? 1,
          priority: record?.priority ?? 100,
          max_concurrency: record?.max_concurrency ?? 2,
          rpm: record?.rpm ?? 60,
          daily_token_limit: record?.daily_token_limit ?? 0,
          supported_models_text: Array.isArray(record?.supported_models)
            ? record.supported_models.join(',')
            : '',
          metadata: record?.metadata || '',
        },
        { isOverride: true },
      );
    }, 0);
  };

  const savePoolConfig = async () => {
    setSavingConfig(true);
    try {
      const payload = buildPoolConfigPayload(poolConfig);
      const res = await API.put('/api/admin/chatmock/pool-config', payload);
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t('保存账号池配置失败'));
        return;
      }
      setPoolConfig({
        ...DEFAULT_POOL_CONFIG,
        ...(data || {}),
      });
      showSuccess(t('账号池配置已保存'));
    } catch (error) {
      showError(toSafeErrorMessage(error, t('保存账号池配置失败')));
    } finally {
      setSavingConfig(false);
    }
  };

  const applyRecommendedPoolConfig = () => {
    setPoolConfig((prev) => ({
      ...prev,
      ...RECOMMENDED_POOL_CONFIG,
    }));
    showSuccess(t('已应用推荐参数，请点击“保存账号池配置”生效'));
  };

  const applyPoolPreset = (presetKey) => {
    const preset =
      POOL_CONFIG_PRESETS.find((item) => item.key === String(presetKey || '').trim()) ||
      POOL_CONFIG_PRESETS[0];
    setSelectedPoolPreset(preset.key);
    setPoolConfig((prev) => ({
      ...prev,
      ...(preset?.values || {}),
    }));
    showInfo(
      t('已应用策略档位：{{name}}，请点击“保存账号池配置”生效', {
        name: t(preset.label),
      }),
    );
  };

  const applyPoolPresetAndSave = async () => {
    const preset =
      POOL_CONFIG_PRESETS.find(
        (item) => item.key === String(selectedPoolPreset || '').trim(),
      ) || POOL_CONFIG_PRESETS[0];
    const nextConfig = {
      ...poolConfig,
      ...(preset?.values || {}),
    };
    setPoolConfig(nextConfig);
    setSavingConfig(true);
    try {
      const payload = buildPoolConfigPayload(nextConfig);
      const res = await API.put('/api/admin/chatmock/pool-config', payload);
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t('保存账号池配置失败'));
        return;
      }
      setPoolConfig({
        ...DEFAULT_POOL_CONFIG,
        ...(data || {}),
      });
      showSuccess(
        t('已应用并保存策略档位：{{name}}', {
          name: t(preset.label),
        }),
      );
    } catch (error) {
      showError(toSafeErrorMessage(error, t('保存账号池配置失败')));
    } finally {
      setSavingConfig(false);
    }
  };

  const applyInstructionPreset = (preset) => {
    const text = String(preset?.value || '').trim();
    if (!text) {
      return;
    }
    setPoolConfig((prev) => ({
      ...prev,
      default_instructions: text,
    }));
    showInfo(t('已填入默认提示词模板：{{name}}', { name: preset?.label || '-' }));
  };

  const syncPoolChannelToNewAPI = async () => {
    setSyncingPoolChannel(true);
    try {
      // Prefer server-side sync: it uses full account set and keeps behavior consistent for UI/API callers.
      try {
        const directRes = await API.post('/api/admin/chatmock/sync-channel');
        const directPayload = directRes?.data || {};
        if (directPayload?.success) {
          const data = directPayload?.data || {};
          showSuccess(
            t('已同步 ChatMock 渠道（ID: {{id}}），模型数 {{count}}', {
              id: data?.channel_id ?? '-',
              count: data?.models_count ?? 0,
            }),
          );
          loadPoolChannelStatus();
          return;
        }
        if (String(directPayload?.message || '').trim()) {
          showError(directPayload?.message || t('同步 ChatMock 渠道失败'));
          return;
        }
      } catch (directError) {
        // fallback for older backend builds without this endpoint
        if (Number(directError?.response?.status) !== 404) {
          throw directError;
        }
      }

      const accountPageSize = 500;
      const accountModels = [];
      let accountPage = 1;
      let accountTotalPages = 1;

      while (accountPage <= accountTotalPages) {
        const accountRes = await API.get('/api/admin/chatmock/accounts', {
          params: {
            p: accountPage,
            page_size: accountPageSize,
          },
        });
        const accountPayload = accountRes?.data || {};
        if (!accountPayload?.success) {
          showError(accountPayload?.message || t('读取账号池失败'));
          return;
        }
        const accountItems = Array.isArray(accountPayload?.data?.items)
          ? accountPayload.data.items
          : [];
        accountModels.push(...collectSupportedModelsFromAccounts(accountItems));
        const total = Number(accountPayload?.data?.total) || 0;
        accountTotalPages = Math.max(1, Math.ceil(total / accountPageSize));
        accountPage += 1;
      }

      const mergedModels = dedupeModels([
        ...CHATMOCK_POOL_CHANNEL_BASELINE_MODELS,
        ...accountModels,
      ]).sort((a, b) => a.localeCompare(b));

      const listRes = await API.get('/api/channel/', {
        params: {
          p: 1,
          page_size: 500,
        },
      });
      const listPayload = listRes?.data || {};
      if (!listPayload?.success) {
        showError(listPayload?.message || t('读取渠道列表失败'));
        return;
      }
      const channels = Array.isArray(listPayload?.data?.items)
        ? listPayload.data.items
        : [];
      const existing = channels.find((item) => Number(item?.type) === 58);

      const channelPayload = {
        name: CHATMOCK_POOL_CHANNEL_NAME,
        type: 58,
        key: 'chatmock_pool',
        status: 1,
        group: 'default',
        models: mergedModels.join(','),
        priority: 10,
        weight: 0,
        auto_ban: 1,
        other: '',
        model_mapping: '',
        setting: '{}',
      };

      let res;
      if (existing?.id) {
        res = await API.put('/api/channel/', {
          ...channelPayload,
          id: Number(existing.id),
        });
      } else {
        res = await API.post('/api/channel/', {
          mode: 'single',
          channel: channelPayload,
        });
      }
      const payload = res?.data || {};
      if (!payload?.success) {
        showError(payload?.message || t('同步 ChatMock 渠道失败'));
        return;
      }
      showSuccess(
        existing?.id
          ? t('已更新 ChatMock 渠道，已同步 {{count}} 个模型', { count: mergedModels.length })
          : t('已创建 ChatMock 渠道，已同步 {{count}} 个模型', { count: mergedModels.length }),
      );
      showInfo(
        t('账号池识别 {{dynamicCount}} 个模型，合并基线后共 {{totalCount}} 个', {
          dynamicCount: dedupeModels(accountModels).length,
          totalCount: mergedModels.length,
        }),
      );
      loadPoolChannelStatus();
    } catch (error) {
      showError(toSafeErrorMessage(error, t('同步 ChatMock 渠道失败')));
    } finally {
      setSyncingPoolChannel(false);
    }
  };

  const handleDiagnoseRoute = async () => {
    const model = String(diagnoseModel || '').trim();
    setDiagnoseLoading(true);
    try {
      const res = await API.post('/api/admin/chatmock/accounts/route-diagnose', {
        model,
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t('路由诊断失败'));
        return;
      }
      setDiagnoseResult(data || null);
      setDiagnoseVisible(true);
      showSuccess(t('路由诊断完成'));
    } catch (error) {
      showError(toSafeErrorMessage(error, t('路由诊断失败')));
    } finally {
      setDiagnoseLoading(false);
    }
  };

  const handleSubmitForm = async () => {
    if (!formApi) {
      return;
    }
    const values = formApi.getValues();
    if (!values?.name || !values?.account || !values?.base_url) {
      showError(t('请补全 name/account/base_url'));
      return;
    }
    if (!editingAccount && !values?.api_key) {
      showError(t('新建账号时必须填写 API Key'));
      return;
    }

    const payload = {
      name: values.name,
      account: values.account,
      api_key: values.api_key || undefined,
      id_token: values.id_token || undefined,
      refresh_token: values.refresh_token || undefined,
      base_url: values.base_url,
      proxy_url: values.proxy_url || '',
      status: values.status || 'enabled',
      weight: Number(values.weight) || 1,
      priority: Number(values.priority) || 100,
      max_concurrency: Number(values.max_concurrency) || 2,
      rpm: Number(values.rpm) || 60,
      daily_token_limit: Number(values.daily_token_limit) || 0,
      supported_models: splitSupportedModels(values.supported_models_text),
      metadata: values.metadata || '',
    };

    setSubmitting(true);
    try {
      let res;
      if (editingAccount?.id) {
        res = await API.put(`/api/admin/chatmock/accounts/${editingAccount.id}`, payload);
      } else {
        res = await API.post('/api/admin/chatmock/accounts', payload);
      }
      const { success, message } = res.data;
      if (!success) {
        showError(message || t('保存账号失败'));
        return;
      }
      showSuccess(t('账号保存成功'));
      setFormVisible(false);
      loadAccounts(page, pageSize);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('保存账号失败')));
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (record) => {
    Modal.confirm({
      title: t('确认删除账号'),
      content: `${record?.name || '-'} (${record?.account || '-'})`,
      type: 'warning',
      onOk: async () => {
        try {
          const res = await API.delete(`/api/admin/chatmock/accounts/${record.id}`);
          const { success, message } = res.data;
          if (!success) {
            showError(message || t('删除失败'));
            return;
          }
          showSuccess(t('删除成功'));
          loadAccounts(page, pageSize);
        } catch (error) {
          showError(toSafeErrorMessage(error, t('删除失败')));
        }
      },
    });
  };

  const handleSwitchStatus = async (record) => {
    const nextStatus = record?.status === 'enabled' ? 'disabled' : 'enabled';
    try {
      const res = await API.post(`/api/admin/chatmock/accounts/${record.id}/status`, {
        status: nextStatus,
      });
      const { success, message } = res.data;
      if (!success) {
        showError(message || t('切换状态失败'));
        return;
      }
      showSuccess(t('状态已更新'));
      loadAccounts(page, pageSize);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('切换状态失败')));
    }
  };

  const handleBatchSetSelectedAccountsStatus = async (nextStatus) => {
    const ids = selectedAccountRowKeys.map((key) => Number(key)).filter(Boolean);
    if (ids.length === 0) {
      showInfo(t('请先选择账号'));
      return;
    }
    const actionText = nextStatus === 'enabled' ? t('启用') : t('禁用');
    Modal.confirm({
      title: t('确认批量{{action}}', { action: actionText }),
      content: t('确认将已选中的 {{count}} 个账号批量设为 {{status}}？', {
        count: ids.length,
        status: nextStatus,
      }),
      type: 'warning',
      onOk: async () => {
        setBatchMutatingAccounts(true);
        try {
          const res = await API.post('/api/admin/chatmock/accounts/batch-status', {
            ids,
            target_status: nextStatus,
          });
          const { success, message, data } = res.data;
          if (!success) {
            showError(message || t('批量更新状态失败'));
            return;
          }
          showSuccess(
            t('批量状态更新完成：{{count}} 个账号已设为 {{status}}', {
              count: Number(data?.updated_count) || 0,
              status: nextStatus,
            }),
          );
          setSelectedAccountRowKeys([]);
          await loadAccounts(page, pageSize);
        } catch (error) {
          showError(toSafeErrorMessage(error, t('批量更新状态失败')));
        } finally {
          setBatchMutatingAccounts(false);
        }
      },
    });
  };

  const handleBatchDeleteSelectedAccounts = async () => {
    const ids = selectedAccountRowKeys.map((key) => Number(key)).filter(Boolean);
    if (ids.length === 0) {
      showInfo(t('请先选择账号'));
      return;
    }
    Modal.confirm({
      title: t('确认批量删除账号'),
      content: t('确认删除已选中的 {{count}} 个账号？会同时清理关联快照与路由日志。', {
        count: ids.length,
      }),
      type: 'warning',
      onOk: async () => {
        setBatchMutatingAccounts(true);
        try {
          const res = await API.post('/api/admin/chatmock/accounts/batch-delete', {
            ids,
          });
          const { success, message, data } = res.data;
          if (!success) {
            showError(message || t('批量删除失败'));
            return;
          }
          showSuccess(
            t(
              '批量删除完成：删除 {{deleted}} 个账号，清理 {{snapshots}} 条快照、{{logs}} 条路由日志',
              {
                deleted: Number(data?.deleted_count) || 0,
                snapshots: Number(data?.deleted_snapshot_count) || 0,
                logs: Number(data?.deleted_route_log_count) || 0,
              },
            ),
          );
          setSelectedAccountRowKeys([]);
          const nextPage = page > 1 && ids.length >= accounts.length ? page - 1 : page;
          setPage(Math.max(1, nextPage));
          await loadAccounts(Math.max(1, nextPage), pageSize);
        } catch (error) {
          showError(toSafeErrorMessage(error, t('批量删除失败')));
        } finally {
          setBatchMutatingAccounts(false);
        }
      },
    });
  };

  const handleBatchDeleteFilteredAccounts = async () => {
    if (Number(total) <= 0) {
      showInfo(t('当前没有可清理的账号'));
      return;
    }
    const filters = {
      keyword: String(accountKeyword || '').trim(),
      status: String(accountStatusFilter || '').trim(),
      tier: String(accountTierFilter || '').trim(),
      quota_status: String(accountQuotaStatusFilter || '').trim(),
    };
    const hasFilters = Object.values(filters).some((value) => String(value || '').trim() !== '');
    Modal.confirm({
      title: t('确认清理当前筛选'),
      content: hasFilters
        ? t('确认删除当前筛选命中的 {{count}} 个账号？会同时清理关联快照与路由日志。', {
            count: total,
          })
        : t('当前未设置筛选条件，确认清空整个账号池（共 {{count}} 个账号）？', {
            count: total,
          }),
      type: 'warning',
      onOk: async () => {
        setBatchMutatingAccounts(true);
        try {
          const res = await API.post('/api/admin/chatmock/accounts/batch-delete', {
            filters,
            all: !hasFilters,
          });
          const { success, message, data } = res.data;
          if (!success) {
            showError(message || t('清理当前筛选失败'));
            return;
          }
          showSuccess(
            t(
              '清理完成：删除 {{deleted}} 个账号，清理 {{snapshots}} 条快照、{{logs}} 条路由日志',
              {
                deleted: Number(data?.deleted_count) || 0,
                snapshots: Number(data?.deleted_snapshot_count) || 0,
                logs: Number(data?.deleted_route_log_count) || 0,
              },
            ),
          );
          setSelectedAccountRowKeys([]);
          setPage(1);
          await loadAccounts(1, pageSize);
        } catch (error) {
          showError(toSafeErrorMessage(error, t('清理当前筛选失败')));
        } finally {
          setBatchMutatingAccounts(false);
        }
      },
    });
  };

  const handleBatchSetFilteredAccountsStatus = async (nextStatus) => {
    if (Number(total) <= 0) {
      showInfo(t('当前没有可操作的账号'));
      return;
    }
    const filters = {
      keyword: String(accountKeyword || '').trim(),
      status: String(accountStatusFilter || '').trim(),
      tier: String(accountTierFilter || '').trim(),
      quota_status: String(accountQuotaStatusFilter || '').trim(),
    };
    const hasFilters = Object.values(filters).some(
      (value) => String(value || '').trim() !== '',
    );
    const actionText = nextStatus === 'enabled' ? t('启用') : t('禁用');
    Modal.confirm({
      title: t('确认批量{{action}}当前筛选', { action: actionText }),
      content: hasFilters
        ? t('确认将当前筛选命中的 {{count}} 个账号批量设为 {{status}}？', {
            count: total,
            status: nextStatus,
          })
        : t('当前未设置筛选条件，确认将整个账号池（共 {{count}} 个账号）批量设为 {{status}}？', {
            count: total,
            status: nextStatus,
          }),
      type: 'warning',
      onOk: async () => {
        setBatchMutatingAccounts(true);
        try {
          const res = await API.post('/api/admin/chatmock/accounts/batch-status', {
            filters,
            all: !hasFilters,
            target_status: nextStatus,
          });
          const { success, message, data } = res.data;
          if (!success) {
            showError(message || t('批量更新筛选账号状态失败'));
            return;
          }
          showSuccess(
            t('批量状态更新完成：{{count}} 个账号已设为 {{status}}', {
              count: Number(data?.updated_count) || 0,
              status: nextStatus,
            }),
          );
          setSelectedAccountRowKeys([]);
          await loadAccounts(1, pageSize);
          setPage(1);
        } catch (error) {
          showError(toSafeErrorMessage(error, t('批量更新筛选账号状态失败')));
        } finally {
          setBatchMutatingAccounts(false);
        }
      },
    });
  };

  const handleHealthCheck = async (record) => {
    try {
      const res = await API.post(`/api/admin/chatmock/accounts/${record.id}/health-check`);
      const { success, message } = res.data;
      if (!success) {
        showError(message || t('健康检查失败'));
        return;
      }
      showSuccess(t('健康检查成功'));
      loadAccounts(page, pageSize);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('健康检查失败')));
    }
  };

  const handleEntitlementCheck = async (record) => {
    try {
      const res = await API.post(`/api/admin/chatmock/accounts/${record.id}/entitlement-check`);
      const { success, message } = res.data;
      if (!success) {
        showError(message || t('权益检查失败'));
        return;
      }
      showSuccess(t('权益检查已完成'));
      loadAccounts(page, pageSize);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('权益检查失败')));
    }
  };

  const handleRefreshCredential = async (record) => {
    try {
      const res = await API.post(
        `/api/admin/chatmock/accounts/${record.id}/refresh-token`,
      );
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t('授权刷新失败'));
        return;
      }
      if (data?.refreshed) {
        showSuccess(t('授权刷新成功'));
      } else {
        showInfo(t('无需刷新或缺少 refresh_token'));
      }
      loadAccounts(page, pageSize);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('授权刷新失败')));
    }
  };

  const handleBatchHealthCheckAll = async () => {
    setBatchCheckingHealth(true);
    try {
      const res = await API.post(
        '/api/admin/chatmock/accounts/health-check-all',
        {},
        {
          params: {
            concurrency: Number(batchConcurrency) || 6,
            retry_attempts: Number(batchRetryAttempts) || 2,
          },
        },
      );
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t('批量健康检查失败'));
        return;
      }
      const successCount = Number(data?.success) || 0;
      const failedCount = Number(data?.failed) || 0;
      if (failedCount > 0) {
        showInfo(
          t('批量健康检查完成：成功 {{success}} 条，失败 {{failed}} 条', {
            success: successCount,
            failed: failedCount,
          }),
        );
      } else {
        showSuccess(
          t('批量健康检查完成：成功 {{success}} 条', {
            success: successCount,
          }),
        );
      }
      loadAccounts(page, pageSize);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('批量健康检查失败')));
    } finally {
      setBatchCheckingHealth(false);
    }
  };

  const handleBatchRefreshTokenAll = async () => {
    setBatchRefreshingAuth(true);
    try {
      const res = await API.post(
        '/api/admin/chatmock/accounts/refresh-token-all',
        {},
        {
          params: {
            concurrency: Number(batchConcurrency) || 6,
            retry_attempts: Number(batchRetryAttempts) || 2,
          },
        },
      );
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t('批量刷新授权失败'));
        return;
      }
      const successCount = Number(data?.success) || 0;
      const refreshedCount = Number(data?.refreshed) || 0;
      const failedCount = Number(data?.failed) || 0;
      if (failedCount > 0) {
        showInfo(
          t('批量刷新授权完成：成功 {{success}} 条，实际刷新 {{refreshed}} 条，失败 {{failed}} 条', {
            success: successCount,
            refreshed: refreshedCount,
            failed: failedCount,
          }),
        );
      } else {
        showSuccess(
          t('批量刷新授权完成：成功 {{success}} 条，实际刷新 {{refreshed}} 条', {
            success: successCount,
            refreshed: refreshedCount,
          }),
        );
      }
      loadAccounts(page, pageSize);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('批量刷新授权失败')));
    } finally {
      setBatchRefreshingAuth(false);
    }
  };

  const handleBatchEntitlementCheckAll = async () => {
    setBatchCheckingEntitlement(true);
    try {
      const res = await API.post(
        '/api/admin/chatmock/accounts/entitlement-check-all',
        {},
        {
          params: {
            concurrency: Number(batchConcurrency) || 6,
            retry_attempts: Number(batchRetryAttempts) || 2,
          },
        },
      );
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t('批量权益检查失败'));
        return;
      }
      const successCount = Number(data?.success) || 0;
      const failedCount = Number(data?.failed) || 0;
      if (failedCount > 0) {
        showInfo(
          t('批量权益检查完成：成功 {{success}} 条，失败 {{failed}} 条', {
            success: successCount,
            failed: failedCount,
          }),
        );
      } else {
        showSuccess(
          t('批量权益检查完成：成功 {{success}} 条', {
            success: successCount,
          }),
        );
      }
      loadAccounts(page, pageSize);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('批量权益检查失败')));
    } finally {
      setBatchCheckingEntitlement(false);
    }
  };

  const handleBatchInspectAll = async () => {
    setBatchInspecting(true);
    try {
      const res = await API.post(
        '/api/admin/chatmock/accounts/inspect-all',
        {},
        {
          params: {
            concurrency: Number(batchConcurrency) || 6,
            retry_attempts: Number(batchRetryAttempts) || 2,
          },
        },
      );
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t('全量巡检失败'));
        return;
      }
      const healthFailed = Number(data?.health?.failed) || 0;
      const entitlementFailed = Number(data?.entitlement?.failed) || 0;
      const text = t(
        '巡检完成：健康检查成功 {{healthSuccess}} / 失败 {{healthFailed}}，权益检查成功 {{entitlementSuccess}} / 失败 {{entitlementFailed}}',
        {
          healthSuccess: Number(data?.health?.success) || 0,
          healthFailed,
          entitlementSuccess: Number(data?.entitlement?.success) || 0,
          entitlementFailed,
        },
      );
      if (healthFailed > 0 || entitlementFailed > 0) {
        showInfo(text);
      } else {
        showSuccess(text);
      }
      loadAccounts(page, pageSize);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('全量巡检失败')));
    } finally {
      setBatchInspecting(false);
    }
  };

  const handleBatchInspectAndSync = async () => {
    setBatchInspectAndSyncing(true);
    try {
      const inspectRes = await API.post(
        '/api/admin/chatmock/accounts/inspect-all',
        {},
        {
          params: {
            concurrency: Number(batchConcurrency) || 6,
            retry_attempts: Number(batchRetryAttempts) || 2,
          },
        },
      );
      const inspectPayload = inspectRes?.data || {};
      if (!inspectPayload?.success) {
        showError(inspectPayload?.message || t('巡检执行失败，未进入同步'));
        return;
      }
      const healthFailed = Number(inspectPayload?.data?.health?.failed) || 0;
      const entitlementFailed = Number(inspectPayload?.data?.entitlement?.failed) || 0;
      if (healthFailed > 0 || entitlementFailed > 0) {
        showInfo(
          t(
            '巡检完成（健康失败 {{healthFailed}} / 权益失败 {{entitlementFailed}}），继续尝试同步渠道',
            {
              healthFailed,
              entitlementFailed,
            },
          ),
        );
      }

      const syncRes = await API.post('/api/admin/chatmock/sync-channel');
      const syncPayload = syncRes?.data || {};
      if (!syncPayload?.success) {
        showError(syncPayload?.message || t('渠道同步失败'));
        return;
      }
      const channelID = Number(syncPayload?.data?.channel_id) || 0;
      const modelCount = Number(syncPayload?.data?.models_count) || 0;
      showSuccess(
        t('巡检并同步完成：渠道ID {{id}}，模型数 {{count}}', {
          id: channelID > 0 ? channelID : '-',
          count: modelCount,
        }),
      );
      loadAccounts(page, pageSize);
      loadPoolChannelStatus();
    } catch (error) {
      showError(toSafeErrorMessage(error, t('巡检并同步失败')));
    } finally {
      setBatchInspectAndSyncing(false);
    }
  };

  const handleStabilizePool = async () => {
    setStabilizingPool(true);
    try {
      const params = {
        concurrency: Number(batchConcurrency) || 6,
        retry_attempts: Number(batchRetryAttempts) || 2,
      };
      const stabilizeRes = await API.post(
        '/api/admin/chatmock/accounts/stabilize',
        {},
        { params },
      );
      const stabilizePayload = stabilizeRes?.data || {};
      if (!stabilizePayload?.success) {
        showError(stabilizePayload?.message || t('一键稳态维护失败'));
        return;
      }
      const stabilizeData = stabilizePayload?.data || {};
      const refreshData = stabilizeData?.refresh || {};
      const inspectData = stabilizeData?.inspect || {};
      const syncData = stabilizeData?.channel || {};

      const refreshFailed = Number(refreshData?.failed) || 0;
      const healthFailed = Number(inspectData?.health?.failed) || 0;
      const entitlementFailed = Number(inspectData?.entitlement?.failed) || 0;
      const hasFailure = refreshFailed > 0 || healthFailed > 0 || entitlementFailed > 0;

      const summary = t(
        '稳态维护完成：授权刷新 {{refreshed}}/{{refreshTotal}}，健康通过 {{healthSuccess}}/{{inspectTotal}}，权益通过 {{entitlementSuccess}}/{{inspectTotal}}，渠道模型 {{modelsCount}}',
        {
          refreshed: Number(refreshData?.refreshed) || 0,
          refreshTotal: Number(refreshData?.total) || 0,
          healthSuccess: Number(inspectData?.health?.success) || 0,
          entitlementSuccess: Number(inspectData?.entitlement?.success) || 0,
          inspectTotal: Number(inspectData?.total) || 0,
          modelsCount: Number(syncData?.models_count) || 0,
        },
      );
      if (hasFailure) {
        showInfo(summary);
      } else {
        showSuccess(summary);
      }

      loadAccounts(page, pageSize);
      loadPoolChannelStatus();
      loadAccountSummary();
    } catch (error) {
      showError(toSafeErrorMessage(error, t('一键稳态维护失败')));
    } finally {
      setStabilizingPool(false);
    }
  };

  const handleExportAccounts = async (format = 'json') => {
    setExportingAccounts(true);
    try {
      const res = await API.get('/api/admin/chatmock/accounts/export', {
        params: {
          format,
          keyword: accountKeyword || undefined,
          status: accountStatusFilter || undefined,
          tier: accountTierFilter || undefined,
          quota_status: accountQuotaStatusFilter || undefined,
          include_sensitive: includeSensitiveExport ? 'true' : undefined,
          confirm_sensitive_export: includeSensitiveExport ? 'true' : undefined,
        },
        responseType: 'blob',
      });
      const fallbackName = `chatmock-accounts.${format === 'csv' ? 'csv' : 'json'}`;
      const fileName = getFilenameFromDisposition(
        res?.headers?.['content-disposition'],
        fallbackName,
      );
      triggerBlobDownload(res.data, fileName);
      showSuccess(t('账号导出成功'));
    } catch (error) {
      showError(toSafeErrorMessage(error, t('账号导出失败')));
    } finally {
      setExportingAccounts(false);
    }
  };

  const handleExportQuotaInspectorJSON = async () => {
    setExportingAccounts(true);
    try {
      const res = await API.get('/api/admin/chatmock/accounts/export', {
        params: {
          format: 'json',
          keyword: accountKeyword || undefined,
          status: accountStatusFilter || undefined,
          tier: accountTierFilter || undefined,
          quota_status: accountQuotaStatusFilter || undefined,
          include_sensitive: includeSensitiveExport ? 'true' : undefined,
          confirm_sensitive_export: includeSensitiveExport ? 'true' : undefined,
        },
        responseType: 'blob',
      });
      const payloadText = await res.data.text();
      const parsed = JSON.parse(payloadText);
      const report = buildQuotaInspectorReport(parsed?.items);
      const blob = new Blob([JSON.stringify(report, null, 2)], {
        type: 'application/json;charset=utf-8',
      });
      const fallbackName = `chatmock-quota-inspector-${Date.now()}.json`;
      triggerBlobDownload(blob, fallbackName);
      showSuccess(t('巡检报告导出成功'));
    } catch (error) {
      showError(toSafeErrorMessage(error, t('巡检报告导出失败')));
    } finally {
      setExportingAccounts(false);
    }
  };

  const handleExportMailboxes = async (format = 'json') => {
    setExportingMailboxes(true);
    try {
      const res = await API.get('/api/admin/chatmock/mailboxes/export', {
        params: {
          format,
          keyword: mailboxKeyword || undefined,
          status: mailboxStatusFilter || undefined,
          include_sensitive: includeSensitiveExport ? 'true' : undefined,
          confirm_sensitive_export: includeSensitiveExport ? 'true' : undefined,
        },
        responseType: 'blob',
      });
      const fallbackName = `chatmock-mailboxes.${format === 'csv' ? 'csv' : 'json'}`;
      const fileName = getFilenameFromDisposition(
        res?.headers?.['content-disposition'],
        fallbackName,
      );
      triggerBlobDownload(res.data, fileName);
      showSuccess(t('邮箱导出成功'));
    } catch (error) {
      showError(toSafeErrorMessage(error, t('邮箱导出失败')));
    } finally {
      setExportingMailboxes(false);
    }
  };

  const fillAccountImportTemplate = () => {
    const template = [
      {
        name: 'codex-demo',
        account: 'acct-demo',
        api_key: 'access_or_sk_token',
        base_url: 'https://chatgpt.com',
        status: 'enabled',
        weight: 1,
        priority: 100,
        max_concurrency: 2,
        rpm: 60,
        daily_token_limit: 0,
        supported_models: ['gpt-5.2', 'gpt-5.3-codex'],
      },
    ];
    setBatchImportText(JSON.stringify(template, null, 2));
    setBatchImportResult(null);
  };

  const fillMailboxImportTemplate = () => {
    const template = [
      {
        name: 'qq-mailbox-1',
        email: 'demo@example.com',
        username: 'demo@example.com',
        password: 'app-password',
        imap_host: 'imap.qq.com',
        imap_port: 993,
        imap_security: 'ssl',
        smtp_host: 'smtp.qq.com',
        smtp_port: 465,
        smtp_security: 'ssl',
        status: 'enabled',
        poll_interval_second: 60,
      },
    ];
    setMailboxBatchImportText(JSON.stringify(template, null, 2));
    setMailboxBatchImportResult(null);
  };

  const handleBatchImport = async () => {
    let parsedAccounts = [];
    try {
      parsedAccounts = parseBatchAccountsInput(batchImportText);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('批量导入内容格式错误')));
      return;
    }

    if (parsedAccounts.length === 0) {
      showError(t('未解析到可导入账号，请检查输入内容'));
      return;
    }

    setBatchImportSubmitting(true);
    try {
      const res = await API.post('/api/admin/chatmock/accounts/batch', {
        items: parsedAccounts,
        upsert: true,
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t('批量导入失败'));
        return;
      }

      const details = Array.isArray(data?.details)
        ? data.details
            .map((item) => {
              const index = Number(item?.index) || 0;
              const account = item?.account ? `(${item.account})` : '';
              const msg = item?.message || t('保存失败');
              return `第 ${index || '?'} 行${account}：${msg}`;
            })
            .filter(Boolean)
        : [];
      const successCount = Number(data?.success) || 0;
      const failedCount = Number(data?.failed) || 0;
      const totalCount = Number(data?.total) || parsedAccounts.length;

      setBatchImportResult({
        total: totalCount,
        success: successCount,
        failed: failedCount,
        details,
      });

      if (successCount > 0) {
        setPage(1);
        loadAccounts(1, pageSize);
      }

      if (failedCount > 0) {
        showInfo(
          t('批量导入完成：成功 {{success}} 条，失败 {{failed}} 条', {
            success: successCount,
            failed: failedCount,
          }),
        );
      } else {
        showSuccess(
          t('批量导入完成：成功 {{success}} 条', {
            success: successCount,
          }),
        );
      }
    } catch (error) {
      showError(toSafeErrorMessage(error, t('批量导入失败')));
    } finally {
      setBatchImportSubmitting(false);
    }
  };

  const handleBatchImportFileChange = async (event) => {
    const files = Array.from(event?.target?.files || []);
    if (files.length === 0) {
      return;
    }
    setBatchImportUploading(true);
    try {
      const mergedAccounts = [];
      for (const file of files) {
        const text = await file.text();
        const parsed = parseBatchAccountsInput(text);
        mergedAccounts.push(...parsed);
      }
      if (mergedAccounts.length === 0) {
        showError(t('未从所选文件解析到可导入账号'));
        return;
      }
      setBatchImportText(JSON.stringify(mergedAccounts, null, 2));
      setBatchImportFileName(
        files.length === 1
          ? files[0].name
          : t('{{count}} 个文件', { count: files.length }),
      );
      setBatchImportResult(null);
      showSuccess(
        t('已读取 {{count}} 个文件，共解析 {{total}} 条记录，请确认后点击开始导入', {
          count: files.length,
          total: mergedAccounts.length,
        }),
      );
    } catch (error) {
      showError(toSafeErrorMessage(error, t('读取导入文件失败')));
    } finally {
      setBatchImportUploading(false);
      if (event?.target) {
        event.target.value = '';
      }
    }
  };

  const handleMailboxBatchImport = async () => {
    let parsedMailboxes = [];
    try {
      parsedMailboxes = parseBatchMailboxesInput(mailboxBatchImportText);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('批量导入内容格式错误')));
      return;
    }
    if (parsedMailboxes.length === 0) {
      showError(t('未解析到可导入邮箱，请检查输入内容'));
      return;
    }

    setMailboxBatchImportSubmitting(true);
    try {
      const res = await API.post('/api/admin/chatmock/mailboxes/batch', {
        items: parsedMailboxes,
        upsert: true,
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t('批量导入失败'));
        return;
      }

      const details = Array.isArray(data?.details)
        ? data.details
            .map((item) => {
              const index = Number(item?.index) || 0;
              const email = item?.email ? `(${item.email})` : '';
              const msg = item?.message || t('保存失败');
              return `第 ${index || '?'} 行${email}：${msg}`;
            })
            .filter(Boolean)
        : [];
      const successCount = Number(data?.success) || 0;
      const failedCount = Number(data?.failed) || 0;
      const totalCount = Number(data?.total) || parsedMailboxes.length;

      setMailboxBatchImportResult({
        total: totalCount,
        success: successCount,
        failed: failedCount,
        details,
      });

      if (successCount > 0) {
        setMailboxPage(1);
        loadMailboxes(1, mailboxPageSize);
      }

      if (failedCount > 0) {
        showInfo(
          t('批量导入完成：成功 {{success}} 条，失败 {{failed}} 条', {
            success: successCount,
            failed: failedCount,
          }),
        );
      } else {
        showSuccess(
          t('批量导入完成：成功 {{success}} 条', {
            success: successCount,
          }),
        );
      }
    } catch (error) {
      showError(toSafeErrorMessage(error, t('批量导入失败')));
    } finally {
      setMailboxBatchImportSubmitting(false);
    }
  };

  const handleMailboxBatchImportFileChange = async (event) => {
    const files = Array.from(event?.target?.files || []);
    if (files.length === 0) {
      return;
    }
    setMailboxBatchImportUploading(true);
    try {
      const mergedMailboxes = [];
      for (const file of files) {
        const text = await file.text();
        const parsed = parseBatchMailboxesInput(text);
        mergedMailboxes.push(...parsed);
      }
      if (mergedMailboxes.length === 0) {
        showError(t('未从所选文件解析到可导入邮箱'));
        return;
      }
      setMailboxBatchImportText(JSON.stringify(mergedMailboxes, null, 2));
      setMailboxBatchImportFileName(
        files.length === 1
          ? files[0].name
          : t('{{count}} 个文件', { count: files.length }),
      );
      setMailboxBatchImportResult(null);
      showSuccess(
        t('已读取 {{count}} 个文件，共解析 {{total}} 条记录，请确认后点击开始导入', {
          count: files.length,
          total: mergedMailboxes.length,
        }),
      );
    } catch (error) {
      showError(toSafeErrorMessage(error, t('读取导入文件失败')));
    } finally {
      setMailboxBatchImportUploading(false);
      if (event?.target) {
        event.target.value = '';
      }
    }
  };

  const openCreateMailboxModal = () => {
    setEditingMailbox(null);
    setMailboxProviderPreset('qq');
    setMailboxFormVisible(true);
    setTimeout(() => {
      mailboxFormApi?.setValues(DEFAULT_MAILBOX_FORM_VALUES, { isOverride: true });
    }, 0);
  };

  const openEditMailboxModal = (record) => {
    setEditingMailbox(record);
    setMailboxProviderPreset('custom');
    setMailboxFormVisible(true);
    setTimeout(() => {
      mailboxFormApi?.setValues(
        {
          provider_template: 'custom',
          name: record?.name || '',
          email: record?.email || '',
          username: record?.username || '',
          password: '',
          imap_host: record?.imap_host || '',
          imap_port: record?.imap_port || 993,
          imap_security: record?.imap_security || 'ssl',
          smtp_host: record?.smtp_host || '',
          smtp_port: record?.smtp_port || 465,
          smtp_security: record?.smtp_security || 'ssl',
          status: record?.status || 'enabled',
          poll_interval_second: record?.poll_interval_second || 60,
          metadata: record?.metadata || '',
        },
        { isOverride: true },
      );
    }, 0);
  };

  const applyMailboxPreset = (nextPreset) => {
    const key = String(nextPreset || 'custom').toLowerCase();
    const preset = MAILBOX_PROVIDER_PRESETS[key] || MAILBOX_PROVIDER_PRESETS.custom;
    setMailboxProviderPreset(key in MAILBOX_PROVIDER_PRESETS ? key : 'custom');
    if (!mailboxFormApi || !preset?.values) {
      return;
    }
    const current = mailboxFormApi.getValues() || {};
    mailboxFormApi.setValues(
      {
        ...current,
        provider_template: key,
        ...preset.values,
      },
      { isOverride: true },
    );
  };

  const handleSubmitMailboxForm = async () => {
    if (!mailboxFormApi) {
      return;
    }
    const values = mailboxFormApi.getValues();
    const providerTemplate = String(values?.provider_template || 'custom').toLowerCase();
    const isProviderMailbox = providerTemplate === 'tempmail_wizawn';
    if (
      !values?.name ||
      !values?.email ||
      !values?.username ||
      (!isProviderMailbox && !values?.imap_host)
    ) {
      showError(
        isProviderMailbox
          ? t('请补全 name/email/username/password')
          : t('请补全 name/email/username/imap_host（smtp_host 可选）'),
      );
      return;
    }
    if (!editingMailbox?.id && !values?.password) {
      showError(t('新建邮箱时必须填写密码或授权码'));
      return;
    }

    const payload = {
      name: values.name,
      email: values.email,
      username: values.username,
      password: values.password || undefined,
      imap_host: values.imap_host,
      imap_port: Number(values.imap_port) || 993,
      imap_security: values.imap_security || 'ssl',
      smtp_host: String(values.smtp_host || '').trim() || undefined,
      smtp_port: String(values.smtp_host || '').trim()
        ? Number(values.smtp_port) || 465
        : undefined,
      smtp_security: String(values.smtp_host || '').trim()
        ? values.smtp_security || 'ssl'
        : undefined,
      status: normalizeMailboxStatus(values.status),
      poll_interval_second: Number(values.poll_interval_second) || 60,
      metadata: values.metadata || '',
    };

    setMailboxSubmitting(true);
    try {
      let res;
      if (editingMailbox?.id) {
        res = await API.put(`/api/admin/chatmock/mailboxes/${editingMailbox.id}`, payload);
      } else {
        res = await API.post('/api/admin/chatmock/mailboxes', payload);
      }
      const { success, message } = res.data;
      if (!success) {
        if (String(message || '').includes('CHATMOCK_MAILBOX_ENABLED=false')) {
          handleMailboxDisabled(message);
          return;
        }
        showError(message || t('保存邮箱失败'));
        return;
      }
      showSuccess(t('邮箱保存成功'));
      setMailboxFormVisible(false);
      loadMailboxes(mailboxPage, mailboxPageSize);
    } catch (error) {
      const message = toSafeErrorMessage(error, t('保存邮箱失败'));
      if (String(message).includes('CHATMOCK_MAILBOX_ENABLED=false')) {
        handleMailboxDisabled(message);
        return;
      }
      showError(message);
    } finally {
      setMailboxSubmitting(false);
    }
  };

  const handleDeleteMailbox = async (record) => {
    Modal.confirm({
      title: t('确认删除邮箱'),
      content: `${record?.name || '-'} (${record?.email || '-'})`,
      type: 'warning',
      onOk: async () => {
        try {
          const res = await API.delete(`/api/admin/chatmock/mailboxes/${record.id}`);
          const { success, message } = res.data;
          if (!success) {
            showError(message || t('删除邮箱失败'));
            return;
          }
          showSuccess(t('删除成功'));
          loadMailboxes(mailboxPage, mailboxPageSize);
      loadMailEvents(
        eventsPage,
        eventsPageSize,
        eventsMailboxId,
        eventsType,
        eventsClassification,
        eventsKeyword,
      );
        } catch (error) {
          showError(toSafeErrorMessage(error, t('删除邮箱失败')));
        }
      },
    });
  };

  const handleToggleMailboxStatus = async (record) => {
    const nextStatus = record?.status === 'enabled' ? 'disabled' : 'enabled';
    try {
      const res = await API.put(`/api/admin/chatmock/mailboxes/${record.id}`, {
        status: nextStatus,
      });
      const { success, message } = res.data;
      if (!success) {
        showError(message || t('切换邮箱状态失败'));
        return;
      }
      showSuccess(t('状态已更新'));
      loadMailboxes(mailboxPage, mailboxPageSize);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('切换邮箱状态失败')));
    }
  };

  const handleTestMailbox = async (record) => {
    try {
      const res = await API.post(`/api/admin/chatmock/mailboxes/${record.id}/test`);
      const { success, message } = res.data;
      if (!success) {
        showError(message || t('邮箱连接测试失败'));
        return;
      }
      showSuccess(t('邮箱连接测试成功'));
      loadMailboxes(mailboxPage, mailboxPageSize);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('邮箱连接测试失败')));
    }
  };

  const handleBatchTestMailboxes = async () => {
    setMailboxBatchTesting(true);
    try {
      const res = await API.post('/api/admin/chatmock/mailboxes/test-all');
      const { success, data, message } = res.data;
      if (!success) {
        showError(message || t('批量邮箱连接测试失败'));
        return;
      }
      const successCount = Number(data?.success) || 0;
      const failedCount = Number(data?.failed) || 0;
      if (failedCount > 0) {
        showInfo(
          t('批量邮箱连接测试完成：成功 {{success}} 条，失败 {{failed}} 条', {
            success: successCount,
            failed: failedCount,
          }),
        );
      } else {
        showSuccess(
          t('批量邮箱连接测试完成：成功 {{success}} 条', {
            success: successCount,
          }),
        );
      }
      loadMailboxes(mailboxPage, mailboxPageSize);
    } catch (error) {
      showError(toSafeErrorMessage(error, t('批量邮箱连接测试失败')));
    } finally {
      setMailboxBatchTesting(false);
    }
  };

  const handlePullMailbox = async (record) => {
    try {
      const res = await API.post(`/api/admin/chatmock/mailboxes/${record.id}/pull`, {
        limit: 30,
        timeout_sec: 20,
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t('拉取邮件失败'));
        return;
      }
      const count = Array.isArray(data) ? data.length : 0;
      showSuccess(
        t('邮件拉取完成，共处理 {{count}} 条新事件', {
          count,
        }),
      );
      loadMailboxes(mailboxPage, mailboxPageSize);
      loadMailEvents(
        1,
        eventsPageSize,
        record.id,
        eventsType,
        eventsClassification,
        eventsKeyword,
      );
      setEventsPage(1);
      setEventsMailboxId(record.id);
      setActiveTab('events');
    } catch (error) {
      showError(toSafeErrorMessage(error, t('拉取邮件失败')));
    }
  };

  const handleShowLatestCode = async (record) => {
    try {
      const res = await API.get(`/api/admin/chatmock/mailboxes/${record.id}/latest-code`);
      const { success, message, data } = res.data;
      if (!success) {
        showError(message || t('获取最新验证码失败'));
        return;
      }
      Modal.info({
        title: t('最新验证码'),
        content: (
          <div>
            <div>{t('邮箱')}: {record?.email || '-'}</div>
            <div>{t('验证码')}: <b>{data?.code || '-'}</b></div>
            <div>{t('主题')}: {data?.subject || '-'}</div>
            <div>{t('接收时间')}: {data?.received_at ? timestamp2string(data.received_at) : '-'}</div>
          </div>
        ),
      });
    } catch (error) {
      showError(toSafeErrorMessage(error, t('获取最新验证码失败')));
    }
  };

  const handleReparseMailEvent = async (record) => {
    if (!record?.id) {
      return;
    }
    try {
      const res = await API.post(`/api/admin/chatmock/mail-events/${record.id}/reparse`);
      const { success, message } = res.data;
      if (!success) {
        showError(message || t('重解析邮件事件失败'));
        return;
      }
      showSuccess(t('邮件事件已重解析'));
      loadMailEvents(
        eventsPage,
        eventsPageSize,
        eventsMailboxId,
        eventsType,
        eventsClassification,
        eventsKeyword,
      );
    } catch (error) {
      showError(toSafeErrorMessage(error, t('重解析邮件事件失败')));
    }
  };

  const accountColumns = useMemo(
    () => [
      {
        title: t('账号名称'),
        dataIndex: 'name',
        render: (text) => <Text strong>{text || '-'}</Text>,
      },
      {
        title: t('账号标识'),
        dataIndex: 'account',
        render: (text) => <Text>{text || '-'}</Text>,
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        width: 120,
        render: (status) => renderStatusTag(status),
      },
      {
        title: t('权益等级'),
        dataIndex: 'entitlement_tier',
        width: 120,
        render: (tier) => <Tag color='blue'>{tier || 'unknown'}</Tag>,
      },
      {
        title: t('权益有效性'),
        dataIndex: 'entitlement_valid',
        width: 120,
        render: (valid) => renderValidityTag(!!valid),
      },
      {
        title: t('5h使用量'),
        width: 120,
        render: (_, record) =>
          renderUsage(record?.usage_5h_used, record?.usage_5h_limit),
      },
      {
        title: t('5h重置时间'),
        dataIndex: 'usage_5h_reset_at',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('weekly使用量'),
        width: 140,
        render: (_, record) =>
          renderUsage(record?.usage_weekly_used, record?.usage_weekly_limit),
      },
      {
        title: t('code-review使用量'),
        width: 170,
        render: (_, record) =>
          renderUsage(
            record?.usage_code_review_used,
            record?.usage_code_review_limit,
          ),
      },
      {
        title: t('weekly剩余'),
        dataIndex: 'weekly_remaining_percent',
        width: 170,
        render: (value) => {
          const num = Number(value);
          if (!Number.isFinite(num)) {
            return '-';
          }
          const percent = Math.max(0, Math.min(100, num));
          return (
            <div className='min-w-[120px]'>
              <div className='text-xs text-gray-700'>{percent.toFixed(2)}%</div>
              <Progress
                percent={percent}
                stroke={getWeeklyRemainingProgressColor(percent)}
                size='small'
                showInfo={false}
                aria-label='weekly remaining progress'
              />
            </div>
          );
        },
      },
      {
        title: t('code-review剩余'),
        width: 170,
        render: (_, record) => {
          const percent = getCodeReviewRemainingPercent(record);
          if (!Number.isFinite(percent)) {
            return '-';
          }
          return (
            <div className='min-w-[120px]'>
              <div className='text-xs text-gray-700'>{percent.toFixed(2)}%</div>
              <Progress
                percent={percent}
                stroke={getWeeklyRemainingProgressColor(percent)}
                size='small'
                showInfo={false}
                aria-label='code review remaining progress'
              />
            </div>
          );
        },
      },
      {
        title: t('额度状态'),
        dataIndex: 'quota_status',
        width: 120,
        render: (status) => renderQuotaStatusTag(status),
      },
      {
        title: t('路由评分'),
        dataIndex: 'route_score',
        width: 120,
        render: (value) => {
          const num = Number(value);
          if (!Number.isFinite(num) || num <= 0) {
            return '-';
          }
          return num.toLocaleString();
        },
      },
      {
        title: t('weekly重置时间'),
        dataIndex: 'usage_weekly_reset_at',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('code-review重置时间'),
        dataIndex: 'usage_code_review_reset_at',
        width: 190,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('最近权益检查'),
        dataIndex: 'entitlement_checked_at',
        width: 180,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('Token 到期时间'),
        dataIndex: 'token_expired_at',
        width: 180,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('Token 最近刷新'),
        dataIndex: 'token_last_refresh_at',
        width: 180,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('错误信息'),
        dataIndex: 'entitlement_error',
        width: 240,
        render: (text, record) => {
          const merged = text || record?.last_error || '';
          if (!merged) return '-';
          return (
            <Text
              ellipsis={{
                showTooltip: {
                  opts: {
                    content: merged,
                  },
                },
              }}
            >
              {merged}
            </Text>
          );
        },
      },
      {
        title: t('操作'),
        width: 430,
        fixed: 'right',
        render: (_, record) => (
          <Space wrap>
            <Button size='small' type='tertiary' onClick={() => openEditModal(record)}>
              {t('编辑')}
            </Button>
            <Button size='small' onClick={() => handleSwitchStatus(record)}>
              {record?.status === 'enabled' ? t('禁用') : t('启用')}
            </Button>
            <Button size='small' type='primary' onClick={() => handleHealthCheck(record)}>
              {t('健康检查')}
            </Button>
            <Button
              size='small'
              type='primary'
              theme='outline'
              onClick={() => handleEntitlementCheck(record)}
            >
              {t('权益检查')}
            </Button>
            <Button
              size='small'
              type='primary'
              theme='outline'
              onClick={() => handleRefreshCredential(record)}
            >
              {t('刷新授权')}
            </Button>
            <Button size='small' type='danger' onClick={() => handleDelete(record)}>
              {t('删除')}
            </Button>
          </Space>
        ),
      },
    ],
    [t],
  );

  const diagnoseColumns = useMemo(
    () => [
      {
        title: t('账号'),
        width: 220,
        render: (_, record) => (
          <div>
            <div className='font-semibold text-gray-900'>{record?.account?.name || '-'}</div>
            <div className='text-xs text-gray-600'>{record?.account?.account || '-'}</div>
          </div>
        ),
      },
      {
        title: t('决策'),
        dataIndex: 'decision',
        width: 120,
        render: (value) => renderDiagnoseDecisionTag(value),
      },
      {
        title: t('排序/权重'),
        width: 180,
        render: (_, record) => {
          const rank = Number(record?.selection_rank) || 0;
          const weight = Number(record?.effective_weight) || 0;
          return (
            <div>
              <div className='text-xs text-gray-700'>
                {rank > 0 ? `${t('排序')}: #${rank}` : '-'}
              </div>
              <div className='text-xs text-gray-600'>
                {weight > 0 ? `${t('权重')}: ${weight.toLocaleString()}` : '-'}
              </div>
            </div>
          );
        },
      },
      {
        title: t('路由评分'),
        dataIndex: 'route_score',
        width: 120,
        render: (value) => {
          const num = Number(value);
          if (!Number.isFinite(num) || num <= 0) {
            return '-';
          }
          return num.toLocaleString();
        },
      },
      {
        title: t('运行态'),
        width: 180,
        render: (_, record) => {
          const inFlight = Number(record?.runtime?.in_flight) || 0;
          const requestCnt = Number(record?.runtime?.request_cnt) || 0;
          const windowStart = Number(record?.runtime?.window_start) || 0;
          return (
            <div>
              <div className='text-xs text-gray-700'>{`in-flight: ${inFlight}`}</div>
              <div className='text-xs text-gray-600'>{`rpm: ${requestCnt}`}</div>
              <div className='text-xs text-gray-500'>
                {windowStart > 0
                  ? `${t('窗口起点')}: ${timestamp2string(windowStart)}`
                  : `${t('窗口起点')}: -`}
              </div>
            </div>
          );
        },
      },
      {
        title: t('额度'),
        width: 170,
        render: (_, record) => {
          const weekly = Number(record?.weekly_remaining_percent);
          const fiveHour = Number(record?.five_hour_remaining_percent);
          return (
            <div>
              <div className='text-xs text-gray-700'>
                {Number.isFinite(weekly)
                  ? `weekly: ${weekly.toFixed(0)}%`
                  : 'weekly: -'}
              </div>
              <div className='text-xs text-gray-600'>
                {Number.isFinite(fiveHour)
                  ? `5h: ${fiveHour.toFixed(0)}%`
                  : '5h: -'}
              </div>
              {record?.entitlement_quota_exhausted ? (
                <div className='text-xs text-red-700'>{t('权益窗口已耗尽')}</div>
              ) : null}
            </div>
          );
        },
      },
      {
        title: t('诊断信息'),
        width: 360,
        render: (_, record) => {
          const reasons = Array.isArray(record?.reasons)
            ? record.reasons.filter(Boolean)
            : [];
          const notes = Array.isArray(record?.notes)
            ? record.notes.filter(Boolean)
            : [];
          const suggestions = Array.isArray(record?.suggestions)
            ? record.suggestions.filter(Boolean)
            : [];
          if (
            reasons.length === 0 &&
            notes.length === 0 &&
            suggestions.length === 0
          ) {
            return '-';
          }
          return (
            <div className='space-y-1'>
              {reasons.map((reason, idx) => (
                <div key={`r-${idx}`} className='text-xs text-red-700'>
                  {`• ${reason}`}
                </div>
              ))}
              {suggestions.map((suggestion, idx) => (
                <div key={`s-${idx}`} className='text-xs text-blue-700'>
                  {`• 建议：${suggestion}`}
                </div>
              ))}
              {notes.map((note, idx) => (
                <div key={`n-${idx}`} className='text-xs text-gray-600'>
                  {`• ${note}`}
                </div>
              ))}
            </div>
          );
        },
      },
    ],
    [t],
  );

  const mailboxColumns = useMemo(
    () => [
      {
        title: t('邮箱名称'),
        dataIndex: 'name',
        render: (text) => <Text strong>{text || '-'}</Text>,
      },
      {
        title: t('邮箱地址'),
        dataIndex: 'email',
        render: (text) => <Text>{text || '-'}</Text>,
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        width: 120,
        render: (status, record) => {
          if (record?._transient_error) {
            return <Tag color='orange'>transient</Tag>;
          }
          return renderMailboxStatusTag(status);
        },
      },
      {
        title: 'IMAP',
        width: 200,
        render: (_, record) => `${record?.imap_host || '-'}:${record?.imap_port || '-'}`,
      },
      {
        title: 'SMTP',
        width: 200,
        render: (_, record) => `${record?.smtp_host || '-'}:${record?.smtp_port || '-'}`,
      },
      {
        title: t('轮询间隔(秒)'),
        dataIndex: 'poll_interval_second',
        width: 130,
        render: (value) => value || 60,
      },
      {
        title: t('最近检查'),
        dataIndex: 'last_checked_time',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('下次重试'),
        dataIndex: '_retry_at',
        width: 170,
        render: (value, record) => {
          if (!record?._transient_error || !value) {
            return '-';
          }
          return timestamp2string(value);
        },
      },
      {
        title: t('最后错误'),
        dataIndex: 'last_error',
        width: 260,
        render: (text, record) => {
          if (!text) return '-';
          const content = record?._transient_error
            ? `⚠ ${text}`
            : text;
          return (
            <Text
              ellipsis={{
                showTooltip: {
                  opts: {
                    content,
                  },
                },
              }}
            >
              {content}
            </Text>
          );
        },
      },
      {
        title: t('操作'),
        width: 500,
        fixed: 'right',
        render: (_, record) => (
          <Space wrap>
            <Button size='small' type='tertiary' onClick={() => openEditMailboxModal(record)}>
              {t('编辑')}
            </Button>
            <Button size='small' onClick={() => handleToggleMailboxStatus(record)}>
              {record?.status === 'enabled' ? t('禁用') : t('启用')}
            </Button>
            <Button size='small' type='primary' onClick={() => handleTestMailbox(record)}>
              {t('连接测试')}
            </Button>
            <Button size='small' type='primary' theme='outline' onClick={() => handlePullMailbox(record)}>
              {t('拉取邮件')}
            </Button>
            <Button size='small' type='primary' theme='outline' onClick={() => handleShowLatestCode(record)}>
              {t('最新验证码')}
            </Button>
            <Button size='small' type='danger' onClick={() => handleDeleteMailbox(record)}>
              {t('删除')}
            </Button>
          </Space>
        ),
      },
    ],
    [t],
  );

  const eventColumns = useMemo(
    () => [
      {
        title: 'ID',
        dataIndex: 'id',
        width: 80,
      },
      {
        title: t('邮箱ID'),
        dataIndex: 'mailbox_id',
        width: 100,
      },
      {
        title: t('事件类型'),
        dataIndex: 'event_type',
        width: 160,
        render: (value, record) => renderMailEventTypeTag(value, record?.status),
      },
      {
        title: t('分类'),
        dataIndex: 'classification',
        width: 160,
        render: (value) => (value ? <Tag color='light-blue'>{value}</Tag> : '-'),
      },
      {
        title: t('主题'),
        dataIndex: 'subject',
        width: 220,
        render: (text) => text || '-',
      },
      {
        title: t('发件人'),
        dataIndex: 'sender',
        width: 220,
        render: (text) => text || '-',
      },
      {
        title: t('验证码'),
        dataIndex: 'code',
        width: 120,
        render: (text) => {
          if (!text) return '-';
          return <Tag color='green'>{text}</Tag>;
        },
      },
      {
        title: t('关联标识'),
        dataIndex: 'related_identifier',
        width: 180,
        render: (text) => text || '-',
      },
      {
        title: t('置信度'),
        dataIndex: 'confidence',
        width: 100,
        render: (value) => {
          const confidence = Number(value);
          if (!Number.isFinite(confidence) || confidence <= 0) {
            return '-';
          }
          return `${confidence.toFixed(2)}`;
        },
      },
      {
        title: t('内容摘要'),
        dataIndex: 'raw_snippet',
        width: 260,
        render: (text) => {
          if (!text) return '-';
          return (
            <Text
              ellipsis={{
                showTooltip: {
                  opts: {
                    content: text,
                  },
                },
              }}
            >
              {text}
            </Text>
          );
        },
      },
      {
        title: t('解析结果'),
        dataIndex: 'parsed_payload',
        width: 260,
        render: (text) => {
          if (!text) {
            return '-';
          }
          let formatted = text;
          try {
            formatted = JSON.stringify(JSON.parse(text), null, 2);
          } catch {
            // ignore
          }
          return (
            <Text
              ellipsis={{
                showTooltip: {
                  opts: {
                    content: <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{formatted}</pre>,
                  },
                },
              }}
            >
              {formatted}
            </Text>
          );
        },
      },
      {
        title: t('接收时间'),
        dataIndex: 'received_at',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('记录时间'),
        dataIndex: 'created_time',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('操作'),
        width: 120,
        fixed: 'right',
        render: (_, record) => (
          <Button
            size='small'
            type='tertiary'
            onClick={() => handleReparseMailEvent(record)}
          >
            {t('重解析')}
          </Button>
        ),
      },
    ],
    [handleReparseMailEvent, t],
  );

  const routeLogColumns = useMemo(
    () => [
      {
        title: 'ID',
        dataIndex: 'id',
        width: 80,
      },
      {
        title: t('账号ID'),
        dataIndex: 'account_id',
        width: 100,
      },
      {
        title: t('账号名称'),
        dataIndex: 'account_name',
        width: 180,
        render: (text) => text || '-',
      },
      {
        title: t('模型'),
        dataIndex: 'model',
        width: 180,
        render: (text) => text || '-',
      },
      {
        title: t('策略'),
        dataIndex: 'strategy',
        width: 160,
        render: (text) => text || '-',
      },
      {
        title: t('路由评分'),
        dataIndex: 'route_score',
        width: 120,
        render: (value) => {
          const score = Number(value);
          if (!Number.isFinite(score)) {
            return '-';
          }
          return score.toLocaleString();
        },
      },
      {
        title: t('耗时(ms)'),
        dataIndex: 'latency_ms',
        width: 120,
        render: (value) => {
          const latency = Number(value);
          if (!Number.isFinite(latency)) {
            return '-';
          }
          return latency;
        },
      },
      {
        title: t('结果'),
        dataIndex: 'success',
        width: 100,
        render: (value) =>
          value ? <Tag color='green'>success</Tag> : <Tag color='red'>failed</Tag>,
      },
      {
        title: t('失败原因'),
        dataIndex: 'failover_reason',
        width: 260,
        render: (text) => {
          if (!text) {
            return '-';
          }
          return (
            <Text
              ellipsis={{
                showTooltip: {
                  opts: {
                    content: text,
                  },
                },
              }}
            >
              {text}
            </Text>
          );
        },
      },
      {
        title: t('请求ID'),
        dataIndex: 'request_id',
        width: 210,
        render: (text) => text || '-',
      },
      {
        title: t('记录时间'),
        dataIndex: 'created_time',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
    ],
    [t],
  );

  const accountSnapshotColumns = useMemo(
    () => [
      {
        title: 'ID',
        dataIndex: 'id',
        width: 80,
      },
      {
        title: t('账号ID'),
        dataIndex: 'account_id',
        width: 100,
      },
      {
        title: t('快照类型'),
        dataIndex: 'snapshot_type',
        width: 160,
        render: (text) => text || '-',
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        width: 120,
        render: (value) => renderStatusTag(value),
      },
      {
        title: t('摘要'),
        dataIndex: 'summary',
        width: 280,
        render: (text) => text || '-',
      },
      {
        title: t('详情'),
        dataIndex: 'payload',
        width: 320,
        render: (text) => {
          if (!text) {
            return '-';
          }
          let formatted = text;
          try {
            formatted = JSON.stringify(JSON.parse(text), null, 2);
          } catch {
            // keep raw
          }
          return (
            <Text
              ellipsis={{
                showTooltip: {
                  opts: {
                    content: <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{formatted}</pre>,
                  },
                },
              }}
            >
              {formatted}
            </Text>
          );
        },
      },
      {
        title: t('记录时间'),
        dataIndex: 'created_time',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
    ],
    [t],
  );

  const mailboxSelectOptions = useMemo(() => {
    const options = [{ label: t('全部邮箱'), value: 0 }];
    mailboxes.forEach((item) => {
      options.push({
        label: `${item?.name || 'Mailbox'} (${item?.email || item?.id})`,
        value: item?.id,
      });
    });
    return options;
  }, [mailboxes, t]);

  const diagnoseRows = useMemo(() => {
    if (!Array.isArray(diagnoseResult?.items)) {
      return [];
    }
    return diagnoseResult.items.map((item, idx) => ({
      ...item,
      id: item?.account?.id || idx + 1,
    }));
  }, [diagnoseResult]);

  const accountOverview = useMemo(() => {
    const accountStatusCounts = accountSummary?.account_status_counts || {};
    const signalCounts = accountSummary?.signal_counts || {};
    const toInt = (value) => {
      const num = Number(value);
      return Number.isFinite(num) ? num : 0;
    };
    const toFloat = (value) => {
      const num = Number(value);
      return Number.isFinite(num) ? num : 0;
    };
    return {
      total: toInt(accountSummary?.accounts),
      enabled: toInt(accountStatusCounts.enabled),
      cooling: toInt(accountStatusCounts.cooling),
      exhausted: toInt(accountSummary?.exhausted_accounts),
      low: toInt(accountSummary?.low_accounts),
      error: toInt(accountSummary?.error_accounts),
      authIssue: toInt(accountSummary?.auth_issue_accounts ?? signalCounts?.auth),
      quotaIssue: toInt(accountSummary?.quota_issue_accounts ?? signalCounts?.quota),
      rateLimited: toInt(
        accountSummary?.rate_limited_accounts ?? signalCounts?.rate_limit,
      ),
      networkIssue: toInt(accountSummary?.network_issue_accounts ?? signalCounts?.network),
      freeEquivalent7D: toFloat(accountSummary?.free_equivalent_7d),
      plusEquivalent7D: toFloat(accountSummary?.plus_equivalent_7d),
      staleEntitlement: toInt(accountSummary?.stale_entitlement_accounts),
      resetPassed: toInt(accountSummary?.reset_passed_accounts),
      tokenExpiring24h: toInt(accountSummary?.token_expiring_24h_accounts),
    };
  }, [accountSummary]);
  const poolChannelSummary = useMemo(() => {
    const exists = !!poolChannelStatus?.exists;
    const enabled = !!poolChannelStatus?.enabled;
    const needSync = !!poolChannelStatus?.need_sync;
    const channelID = Number(poolChannelStatus?.channel_id) || 0;
    const modelsCount = Number(poolChannelStatus?.models_count) || 0;
    const missingCount = Array.isArray(poolChannelStatus?.missing_models)
      ? poolChannelStatus.missing_models.length
      : 0;
    const extraCount = Array.isArray(poolChannelStatus?.extra_models)
      ? poolChannelStatus.extra_models.length
      : 0;

    if (!exists) {
      return {
        color: 'red',
        label: t('未接入'),
        detail: t('未检测到 ChatMockPool 渠道，请点击“一键接入 NewAPI 渠道”。'),
      };
    }
    if (!enabled) {
      return {
        color: 'orange',
        label: t('未启用'),
        detail: t('渠道 #{{id}} 已存在但未启用，请到渠道管理页启用。', {
          id: channelID > 0 ? channelID : '-',
        }),
      };
    }
    if (needSync || missingCount > 0) {
      return {
        color: 'orange',
        label: t('需同步'),
        detail: t(
          '渠道 #{{id}} 已启用，当前模型 {{modelsCount}} 个，缺失 {{missingCount}} 个，建议执行同步。',
          {
            id: channelID > 0 ? channelID : '-',
            modelsCount,
            missingCount,
          },
        ),
      };
    }
    return {
      color: extraCount > 0 ? 'blue' : 'green',
      label: extraCount > 0 ? t('已接入(含扩展模型)') : t('已接入并启用'),
      detail: t('渠道 #{{id}} 正常可用，模型 {{modelsCount}} 个。', {
        id: channelID > 0 ? channelID : '-',
        modelsCount,
      }),
    };
  }, [poolChannelStatus, t]);

  const sortedAccounts = useMemo(
    () => sortAccountsByMode(accounts, accountSortMode),
    [accounts, accountSortMode],
  );
  const smartSwitchCandidate = useMemo(
    () => pickSmartSwitchCandidate(accounts),
    [accounts],
  );
  const smartSwitchCandidateSummary = useMemo(() => {
    if (!smartSwitchCandidate) {
      return null;
    }
    const weekly = getWeeklyRemainingPercent(smartSwitchCandidate);
    const fiveHour = getFiveHourRemainingPercent(smartSwitchCandidate);
    const renderPct = (value) =>
      Number.isFinite(value) ? `${Number(value).toFixed(1)}%` : '-';
    return {
      name: smartSwitchCandidate?.name || '-',
      account: smartSwitchCandidate?.account || '-',
      weekly: renderPct(weekly),
      fiveHour: renderPct(fiveHour),
      quotaStatus: deriveQuotaStatusForReport(smartSwitchCandidate),
    };
  }, [smartSwitchCandidate]);
  const displayAccountColumns = useMemo(() => {
    if (isAdvancedMode) {
      return accountColumns;
    }
    return BASIC_ACCOUNT_COLUMN_INDICES.map((idx) => accountColumns[idx]).filter(Boolean);
  }, [accountColumns, isAdvancedMode]);
  const hasActiveAccountFilters = useMemo(
    () =>
      [
        accountKeyword,
        accountStatusFilter,
        accountTierFilter,
        accountQuotaStatusFilter,
      ].some((value) => String(value || '').trim() !== ''),
    [
      accountKeyword,
      accountStatusFilter,
      accountTierFilter,
      accountQuotaStatusFilter,
    ],
  );
  const accountFilterSummaryText = useMemo(() => {
    const parts = [];
    if (String(accountKeyword || '').trim()) {
      parts.push(`keyword=${String(accountKeyword).trim()}`);
    }
    if (String(accountStatusFilter || '').trim()) {
      parts.push(`status=${String(accountStatusFilter).trim()}`);
    }
    if (String(accountTierFilter || '').trim()) {
      parts.push(`tier=${String(accountTierFilter).trim()}`);
    }
    if (String(accountQuotaStatusFilter || '').trim()) {
      parts.push(`quota=${String(accountQuotaStatusFilter).trim()}`);
    }
    if (String(accountSortMode || '').trim() && accountSortMode !== 'default') {
      parts.push(`sort=${String(accountSortMode).trim()}`);
    }
    return parts.join(' | ');
  }, [
    accountKeyword,
    accountStatusFilter,
    accountTierFilter,
    accountQuotaStatusFilter,
    accountSortMode,
  ]);
  const applyAccountQuickFilter = useCallback((patch = {}) => {
    if (Object.prototype.hasOwnProperty.call(patch, 'keyword')) {
      setAccountKeyword(String(patch.keyword || ''));
    }
    if (Object.prototype.hasOwnProperty.call(patch, 'status')) {
      setAccountStatusFilter(String(patch.status || ''));
    }
    if (Object.prototype.hasOwnProperty.call(patch, 'tier')) {
      setAccountTierFilter(String(patch.tier || ''));
    }
    if (Object.prototype.hasOwnProperty.call(patch, 'quota_status')) {
      setAccountQuotaStatusFilter(String(patch.quota_status || ''));
    }
    if (Object.prototype.hasOwnProperty.call(patch, 'sort_mode')) {
      setAccountSortMode(String(patch.sort_mode || 'default'));
    }
    setPage(1);
    setActiveTab('accounts');
  }, []);
  const clearAccountFilters = useCallback(() => {
    setAccountKeyword('');
    setAccountStatusFilter('');
    setAccountTierFilter('');
    setAccountQuotaStatusFilter('');
    setAccountSortMode('default');
    setPage(1);
  }, []);
  const accountRowSelection = {
    selectedRowKeys: selectedAccountRowKeys,
    onChange: (selectedRowKeys) => setSelectedAccountRowKeys(selectedRowKeys),
  };

  return (
    <div className='mt-[60px] px-2'>
      <Card style={{ marginBottom: 12 }}>
        <div className='flex flex-wrap items-center justify-between gap-3 rounded-xl border border-gray-200 bg-gray-50 px-4 py-3'>
          <div>
            <div className='text-sm font-semibold text-gray-900'>{t('操作模式')}</div>
            <div className='mt-1 text-xs text-gray-700'>
              {t('基础模式仅展示日常高频操作；高级模式展示全部巡检、导出与诊断能力。')}
            </div>
          </div>
          <Space wrap>
            <Button
              data-testid='chatmock-mode-basic'
              type={isAdvancedMode ? 'tertiary' : 'primary'}
              theme={isAdvancedMode ? 'outline' : 'solid'}
              onClick={() => setOperationMode('basic')}
            >
              {t('基础模式')}
            </Button>
            <Button
              data-testid='chatmock-mode-advanced'
              type={isAdvancedMode ? 'primary' : 'tertiary'}
              theme={isAdvancedMode ? 'solid' : 'outline'}
              onClick={() => setOperationMode('advanced')}
            >
              {t('高级模式')}
            </Button>
          </Space>
        </div>
      </Card>

      <Card
        title={t('ChatMock 账号池管理')}
        style={{ marginBottom: 12 }}
        headerExtraContent={
          <Space>
            <Button onClick={() => loadPoolConfig()}>{t('刷新配置')}</Button>
            <Button onClick={applyRecommendedPoolConfig}>
              {t('应用推荐参数')}
            </Button>
            <Button loading={savingConfig} onClick={savePoolConfig} type='primary'>
              {t('保存账号池配置')}
            </Button>
          </Space>
        }
      >
        <div className='mb-3 rounded-lg border border-indigo-200 bg-indigo-50 px-3 py-2'>
          <div className='text-xs font-semibold text-indigo-900'>
            {t('策略档位（一键应用）')}
          </div>
          <div className='mt-1 text-xs text-indigo-800'>
            {t('融合 CPA / CLIProxy / codex-tools 运维思路，先应用档位，再按需微调。')}
          </div>
          <div className='mt-2 flex flex-wrap items-end gap-2'>
            <div data-testid='chatmock-pool-preset-select' style={{ width: 320 }}>
              <Select
                style={{ width: '100%' }}
                value={selectedPoolPreset}
                optionList={POOL_CONFIG_PRESETS.map((item) => ({
                  value: item.key,
                  label: `${t(item.label)} · ${t(item.description)}`,
                }))}
                onChange={(value) => setSelectedPoolPreset(String(value || 'fusion_stable'))}
              />
            </div>
            <Button
              data-testid='chatmock-pool-preset-apply'
              theme='outline'
              onClick={() => applyPoolPreset(selectedPoolPreset)}
            >
              {t('应用档位')}
            </Button>
            <Button
              data-testid='chatmock-pool-preset-apply-save'
              type='primary'
              loading={savingConfig}
              onClick={applyPoolPresetAndSave}
            >
              {t('应用并保存')}
            </Button>
          </div>
        </div>
        <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3'>
          <div>
            <div className='text-sm mb-1'>{t('加权调度')}</div>
            <Switch
              checked={!!poolConfig.enable_weighted}
              onChange={(value) =>
                setPoolConfig((prev) => ({ ...prev, enable_weighted: value }))
              }
            />
          </div>
          {isAdvancedMode ? (
            <div>
              <div className='text-sm mb-1'>{t('默认冷却秒数')}</div>
              <InputNumber
                min={1}
                value={poolConfig.default_cooldown_second}
                onChange={(value) =>
                  setPoolConfig((prev) => ({
                    ...prev,
                    default_cooldown_second: Number(value) || 300,
                  }))
                }
              />
            </div>
          ) : null}
          {isAdvancedMode ? (
            <div>
              <div className='text-sm mb-1'>{t('请求超时秒数')}</div>
              <InputNumber
                min={5}
                value={poolConfig.request_timeout_second}
                onChange={(value) =>
                  setPoolConfig((prev) => ({
                    ...prev,
                    request_timeout_second: Number(value) || 60,
                  }))
                }
              />
            </div>
          ) : null}
          {isAdvancedMode ? (
            <div>
              <div className='text-sm mb-1'>{t('最大重试次数')}</div>
              <InputNumber
                min={1}
                value={poolConfig.max_retry}
                onChange={(value) =>
                  setPoolConfig((prev) => ({
                    ...prev,
                    max_retry: Number(value) || 3,
                  }))
                }
              />
            </div>
          ) : null}
          {isAdvancedMode ? (
            <div>
              <div className='text-sm mb-1'>{t('健康检查间隔(秒)')}</div>
              <InputNumber
                min={30}
                value={poolConfig.health_check_interval}
                onChange={(value) =>
                  setPoolConfig((prev) => ({
                    ...prev,
                    health_check_interval: Number(value) || 300,
                  }))
                }
              />
            </div>
          ) : null}
          <div>
            <div className='text-sm mb-1'>{t('权益检查间隔(秒，0=关闭)')}</div>
            <InputNumber
              min={0}
              value={poolConfig.entitlement_check_interval}
              onChange={(value) =>
                setPoolConfig((prev) => ({
                  ...prev,
                  entitlement_check_interval: Number(value) || 0,
                }))
              }
            />
            <div className='mt-1 text-xs text-gray-600'>
              {t(
                '系统会按该间隔自动检查账号权益（team/plus/pro/free）以及 5h / weekly / code-review 用量。',
              )}
            </div>
            {!isAdvancedMode ? (
              <div className='mt-1 text-xs text-gray-600'>
                {t('更多调度参数（冷却/超时/重试/健康检查）请切换到高级模式调整。')}
              </div>
            ) : null}
          </div>
          <div className='md:col-span-2 lg:col-span-3'>
            <div className='text-sm mb-1'>{t('默认系统提示词（Codex）')}</div>
            <TextArea
              data-testid='chatmock-default-instructions'
              rows={3}
              autosize={false}
              value={poolConfig.default_instructions || ''}
              onChange={(value) =>
                setPoolConfig((prev) => ({
                  ...prev,
                  default_instructions: String(value || ''),
                }))
              }
              placeholder={t('当请求中未显式提供 instructions 时，将使用这里的默认提示词')}
            />
            <div className='mt-2 flex flex-wrap gap-2'>
              {DEFAULT_INSTRUCTION_PRESETS.map((preset) => (
                <Button
                  key={preset.key}
                  size='small'
                  theme='outline'
                  onClick={() => applyInstructionPreset(preset)}
                >
                  {t(preset.label)}
                </Button>
              ))}
            </div>
            <div className='mt-1 text-xs text-gray-600'>
              {t('建议先选择模板，再按业务场景微调后保存。')}
            </div>
          </div>
        </div>
      </Card>

      <Card style={{ marginBottom: 12 }}>
        <div className='flex flex-wrap items-center justify-between gap-3 rounded-xl border border-sky-200 bg-sky-50 px-4 py-3'>
          <div>
            <div className='text-sm font-semibold text-sky-900'>
              {t('将账号池接入 NewAPI 转发渠道')}
            </div>
            <div className='mt-1 text-xs text-sky-800'>
              {t('点击后一键创建或更新 type=58 的 ChatMockPool 渠道，省去手工配置步骤。')}
            </div>
            <div className='mt-1 text-xs text-sky-800'>
              {t('实验模型（如 spark）默认不自动纳入，需在账号 supported_models 中显式声明。')}
            </div>
            <div
              data-testid='chatmock-pool-channel-status'
              className='mt-2 inline-flex items-center gap-2 rounded-md border border-sky-200 bg-white/70 px-2 py-1 text-xs text-sky-900'
            >
              <Tag color={poolChannelSummary.color}>{poolChannelSummary.label}</Tag>
              <span>{poolChannelSummary.detail}</span>
            </div>
            {Array.isArray(poolChannelStatus?.missing_models) &&
            poolChannelStatus.missing_models.length > 0 ? (
              <div className='mt-1 text-xs text-sky-800'>
                {t('缺失模型')}: {poolChannelStatus.missing_models.slice(0, 8).join(', ')}
                {poolChannelStatus.missing_models.length > 8 ? ' ...' : ''}
              </div>
            ) : null}
          </div>
          <Space wrap>
            <Button
              data-testid='chatmock-sync-channel'
              type='primary'
              loading={syncingPoolChannel}
              onClick={syncPoolChannelToNewAPI}
            >
              {t('一键接入 NewAPI 渠道')}
            </Button>
            <Button theme='outline' type='tertiary'>
              <Link className='text-current no-underline' to='/console/channel'>
                {t('前往渠道管理')}
              </Link>
            </Button>
          </Space>
        </div>
      </Card>

      <Card>
        <div className='mb-3 grid grid-cols-2 gap-2 md:grid-cols-4 xl:grid-cols-8'>
          <div className='rounded border border-gray-200 bg-white px-3 py-2'>
            <div className='text-xs text-gray-600'>{t('账号总数')}</div>
            <div className='text-lg font-semibold text-gray-900'>{accountOverview.total}</div>
          </div>
          <div className='rounded border border-emerald-200 bg-emerald-50 px-3 py-2'>
            <div className='text-xs text-emerald-700'>{t('启用中')}</div>
            <div className='text-lg font-semibold text-emerald-900'>{accountOverview.enabled}</div>
          </div>
          <div className='rounded border border-amber-200 bg-amber-50 px-3 py-2'>
            <div className='text-xs text-amber-700'>{t('冷却中')}</div>
            <div className='text-lg font-semibold text-amber-900'>{accountOverview.cooling}</div>
          </div>
          <div className='rounded border border-red-200 bg-red-50 px-3 py-2'>
            <div className='text-xs text-red-700'>{t('额度耗尽')}</div>
            <div className='text-lg font-semibold text-red-900'>{accountOverview.exhausted}</div>
          </div>
          <div className='rounded border border-orange-200 bg-orange-50 px-3 py-2'>
            <div className='text-xs text-orange-700'>{t('低额度预警')}</div>
            <div className='text-lg font-semibold text-orange-900'>{accountOverview.low}</div>
          </div>
          <div className='rounded border border-rose-200 bg-rose-50 px-3 py-2'>
            <div className='text-xs text-rose-700'>{t('异常账号')}</div>
            <div className='text-lg font-semibold text-rose-900'>{accountOverview.error}</div>
          </div>
          <div className='rounded border border-sky-200 bg-sky-50 px-3 py-2'>
            <div className='text-xs text-sky-700'>{t('Free当量(7d)')}</div>
            <div className='text-lg font-semibold text-sky-900'>
              {accountOverview.freeEquivalent7D.toFixed(1)}%
            </div>
          </div>
          <div className='rounded border border-indigo-200 bg-indigo-50 px-3 py-2'>
            <div className='text-xs text-indigo-700'>{t('Plus当量(7d)')}</div>
            <div className='text-lg font-semibold text-indigo-900'>
              {accountOverview.plusEquivalent7D.toFixed(1)}%
            </div>
          </div>
        </div>
        <div className='mb-2 flex flex-wrap items-center gap-2 text-xs'>
          <Tag color={accountOverview.staleEntitlement > 0 ? 'orange' : 'green'}>
            {t('权益数据待刷新')}: {accountOverview.staleEntitlement}
          </Tag>
          <Tag color={accountOverview.resetPassed > 0 ? 'orange' : 'green'}>
            {t('重置后待回填')}: {accountOverview.resetPassed}
          </Tag>
          <Tag color={accountOverview.tokenExpiring24h > 0 ? 'red' : 'green'}>
            {t('24h内授权到期')}: {accountOverview.tokenExpiring24h}
          </Tag>
          <Tag color={accountOverview.authIssue > 0 ? 'red' : 'green'}>
            {t('鉴权异常')}: {accountOverview.authIssue}
          </Tag>
          <Tag color={accountOverview.quotaIssue > 0 ? 'orange' : 'green'}>
            {t('额度问题')}: {accountOverview.quotaIssue}
          </Tag>
          <Tag color={accountOverview.rateLimited > 0 ? 'orange' : 'green'}>
            {t('限流问题')}: {accountOverview.rateLimited}
          </Tag>
          <Tag color={accountOverview.networkIssue > 0 ? 'orange' : 'green'}>
            {t('网络问题')}: {accountOverview.networkIssue}
          </Tag>
        </div>
        {Array.isArray(accountSummary?.signal_suggestions) &&
        accountSummary.signal_suggestions.length > 0 ? (
          <div
            data-testid='chatmock-signal-suggestions'
            className='mb-2 rounded border border-orange-200 bg-orange-50 px-3 py-2 text-xs text-orange-900'
          >
            <div className='font-semibold'>{t('维护建议')}</div>
            {accountSummary.signal_suggestions.slice(0, 3).map((tip, idx) => (
              <div key={`signal-tip-${idx}`} style={{ marginTop: idx === 0 ? 4 : 2 }}>
                {idx + 1}. {tip}
              </div>
            ))}
            <div className='mt-2 flex flex-wrap gap-2'>
              {accountOverview.authIssue > 0 ? (
                <Button
                  size='small'
                  theme='outline'
                  onClick={() =>
                    applyAccountQuickFilter({
                      status: 'auth_invalid',
                      quota_status: '',
                    })
                  }
                >
                  {t('筛选鉴权异常')}
                </Button>
              ) : null}
              {accountOverview.networkIssue > 0 ? (
                <Button
                  size='small'
                  theme='outline'
                  onClick={() =>
                    applyAccountQuickFilter({
                      status: 'network_issue',
                      quota_status: '',
                    })
                  }
                >
                  {t('筛选网络异常')}
                </Button>
              ) : null}
              {accountOverview.quotaIssue > 0 ? (
                <Button
                  size='small'
                  theme='outline'
                  onClick={() =>
                    applyAccountQuickFilter({
                      status: '',
                      quota_status: 'exhausted',
                    })
                  }
                >
                  {t('筛选额度耗尽')}
                </Button>
              ) : null}
              <Button
                size='small'
                theme='outline'
                onClick={() =>
                  applyAccountQuickFilter({
                    status: '',
                    quota_status: '',
                    sort_mode: 'smart_switch',
                  })
                }
              >
                {t('切到 Smart Switch 排序')}
              </Button>
            </div>
          </div>
        ) : null}
        <div className='mb-3 text-xs text-gray-700'>
          {t('套餐分布：free {{free}} / plus {{plus}} / pro {{pro}} / team {{team}} / unknown {{unknown}}', {
            free: Number(accountSummary?.plan_counts?.free || 0),
            plus: Number(accountSummary?.plan_counts?.plus || 0),
            pro: Number(accountSummary?.plan_counts?.pro || 0),
            team: Number(accountSummary?.plan_counts?.team || 0),
            unknown: Number(accountSummary?.plan_counts?.unknown || 0),
          })}
        </div>
        {smartSwitchCandidateSummary ? (
          <div
            data-testid='chatmock-smart-switch-candidate'
            className='mb-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs text-emerald-900'
          >
            <span className='font-semibold'>{t('Smart Switch 推荐账号')}</span>
            <span>{`: ${smartSwitchCandidateSummary.name} (${smartSwitchCandidateSummary.account})`}</span>
            <span>{` · weekly ${smartSwitchCandidateSummary.weekly}`}</span>
            <span>{` · 5h ${smartSwitchCandidateSummary.fiveHour}`}</span>
            <span>{` · ${t('额度状态')}: ${smartSwitchCandidateSummary.quotaStatus}`}</span>
          </div>
        ) : null}
        {!isAdvancedMode ? (
          <div className='mb-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-700'>
            <div>
              {t(
                '基础模式操作建议：1) 新增/批量导入账号 2) 一键巡检 3) 巡检并同步渠道 4) 确认接入状态为“已接入并启用”',
              )}
            </div>
            <div className='mt-1'>
              {t('如需导出、路由诊断、全量健康检查等能力，请切换到高级模式。')}
            </div>
          </div>
        ) : null}
        <Tabs activeKey={activeTab} onChange={setActiveTab} type='line'>
          <TabPane tab={t('账号列表')} itemKey='accounts'>
            <div className='mb-3 flex flex-wrap items-end gap-2'>
              <Input
                style={{ width: 260 }}
                value={accountKeyword}
                placeholder={t('按名称/账号/Base URL 搜索')}
                onChange={(value) => setAccountKeyword(String(value || ''))}
              />
              <Select
                style={{ width: 180 }}
                value={accountStatusFilter}
                optionList={[
                  { label: t('全部状态'), value: '' },
                  { label: 'enabled', value: 'enabled' },
                  { label: 'cooling', value: 'cooling' },
                  { label: 'disabled', value: 'disabled' },
                  { label: 'abnormal', value: 'abnormal' },
                  { label: 'quota_low', value: 'quota_low' },
                  { label: 'quota_exhausted', value: 'quota_exhausted' },
                  { label: 'auth_invalid', value: 'auth_invalid' },
                  { label: 'network_issue', value: 'network_issue' },
                  { label: 'manual_review', value: 'manual_review' },
                ]}
                onChange={(value) => setAccountStatusFilter(String(value || ''))}
              />
              <Select
                style={{ width: 180 }}
                value={accountTierFilter}
                optionList={[
                  { label: t('全部等级'), value: '' },
                  { label: 'free', value: 'free' },
                  { label: 'plus', value: 'plus' },
                  { label: 'pro', value: 'pro' },
                  { label: 'team', value: 'team' },
                  { label: 'unknown', value: 'unknown' },
                ]}
                onChange={(value) => setAccountTierFilter(String(value || ''))}
              />
              <Select
                style={{ width: 200 }}
                value={accountQuotaStatusFilter}
                optionList={[
                  { label: t('全部额度状态'), value: '' },
                  { label: 'full', value: 'full' },
                  { label: 'high', value: 'high' },
                  { label: 'medium', value: 'medium' },
                  { label: 'low', value: 'low' },
                  { label: 'exhausted', value: 'exhausted' },
                  { label: 'error', value: 'error' },
                  { label: 'unknown', value: 'unknown' },
                ]}
                onChange={(value) => setAccountQuotaStatusFilter(String(value || ''))}
              />
              <Select
                style={{ width: 260 }}
                value={accountSortMode}
                optionList={ACCOUNT_SORT_MODES.map((item) => ({
                  label: t(item.label),
                  value: item.value,
                }))}
                onChange={(value) => setAccountSortMode(String(value || 'default'))}
              />
              <Button
                onClick={() => {
                  setPage(1);
                  loadAccounts(1, pageSize);
                }}
              >
                {t('查询')}
              </Button>
              <Button
                onClick={() => {
                  clearAccountFilters();
                }}
              >
                {t('清空筛选')}
              </Button>
            </div>
            <div className='mb-3 flex flex-wrap items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs'>
              <Text>
                {t('当前筛选命中')}: <b>{Number(total) || 0}</b>
              </Text>
              <Text>
                {t('已选账号')}: <b>{selectedAccountRowKeys.length}</b>
              </Text>
              <Text type='secondary'>
                {hasActiveAccountFilters
                  ? accountFilterSummaryText
                  : t('未设置筛选条件，将作用于整个账号池')}
              </Text>
              {hasActiveAccountFilters ? (
                <Button size='small' theme='borderless' onClick={clearAccountFilters}>
                  {t('一键清空筛选')}
                </Button>
              ) : null}
            </div>
            {isAdvancedMode ? (
              <div className='mb-3 flex flex-wrap items-end gap-2 rounded-lg border border-sky-200 bg-sky-50 px-3 py-2'>
                <div className='mr-2'>
                  <div className='text-xs font-semibold text-sky-900'>{t('路由诊断')}</div>
                  <div className='text-xs text-sky-800'>
                    {t('输入模型名后可看到每个账号的选中/跳过原因（状态、冷却、并发、RPM、额度等）')}
                  </div>
                </div>
                <Input
                  style={{ width: 260 }}
                  value={diagnoseModel}
                  placeholder={t('例如：gpt-5.3-codex')}
                  onChange={(value) => setDiagnoseModel(String(value || ''))}
                />
                <Button
                  data-testid='chatmock-route-diagnose'
                  type='primary'
                  loading={diagnoseLoading}
                  onClick={handleDiagnoseRoute}
                >
                  {t('诊断路由')}
                </Button>
                {diagnoseResult ? (
                  <Button theme='outline' onClick={() => setDiagnoseVisible(true)}>
                    {t('查看上次结果')}
                  </Button>
                ) : null}
              </div>
            ) : (
              <div className='mb-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs text-emerald-800'>
                {t('当前为基础模式：已隐藏路由诊断和批量运维高级参数，确保日常操作更简洁。')}
              </div>
            )}
            <div className='mb-3 flex flex-wrap gap-2'>
              <Button
                theme='outline'
                size='small'
                onClick={() => setAccountQuotaStatusFilter('exhausted')}
              >
                {t('仅看额度耗尽')}
              </Button>
              <Button
                theme='outline'
                size='small'
                onClick={() => setAccountQuotaStatusFilter('low')}
              >
                {t('仅看低额度')}
              </Button>
              <Button
                theme='outline'
                size='small'
                onClick={() => setAccountQuotaStatusFilter('error')}
              >
                {t('仅看异常')}
              </Button>
              <Button
                theme='outline'
                size='small'
                onClick={() => setAccountSortMode('quota_inspector')}
              >
                {t('启用巡检排序')}
              </Button>
              <Button
                theme='outline'
                size='small'
                data-testid='chatmock-smart-switch-sort'
                onClick={() => setAccountSortMode('smart_switch')}
              >
                {t('启用Smart Switch排序')}
              </Button>
            </div>
            <div className='mb-3 flex flex-wrap gap-2'>
              <Button data-testid='chatmock-add-account' type='primary' onClick={openCreateModal}>
                {t('新增账号')}
              </Button>
              <Button data-testid='chatmock-batch-import' onClick={() => setBatchImportVisible(true)}>
                {t('批量导入')}
              </Button>
              {!isAdvancedMode ? (
                <Button
                  data-testid='chatmock-stabilize'
                  type='primary'
                  loading={stabilizingPool}
                  onClick={handleStabilizePool}
                >
                  {t('一键稳态维护')}
                </Button>
              ) : (
                <>
                  <Button
                    data-testid='chatmock-inspect-all'
                    loading={batchInspecting}
                    onClick={handleBatchInspectAll}
                  >
                    {t('一键巡检')}
                  </Button>
                  <Button
                    data-testid='chatmock-inspect-sync'
                    loading={batchInspectAndSyncing}
                    onClick={handleBatchInspectAndSync}
                  >
                    {t('巡检并同步渠道')}
                  </Button>
                </>
              )}
              <Button data-testid='chatmock-refresh-list' onClick={() => loadAccounts(page, pageSize)}>
                {t('刷新列表')}
              </Button>
              {isAdvancedMode ? (
                <>
                  <div
                    data-testid='chatmock-advanced-batch-config'
                    className='flex items-center gap-2 rounded border border-gray-200 bg-gray-50 px-2 py-1'
                  >
                    <span className='text-xs text-gray-700'>{t('巡检并发')}</span>
                    <InputNumber
                      min={1}
                      max={32}
                      value={batchConcurrency}
                      style={{ width: 88 }}
                      onChange={(value) =>
                        setBatchConcurrency(
                          Math.max(1, Math.min(32, Number(value) || 6)),
                        )
                      }
                    />
                    <span className='text-xs text-gray-700'>{t('重试')}</span>
                    <InputNumber
                      min={1}
                      max={5}
                      value={batchRetryAttempts}
                      style={{ width: 76 }}
                      onChange={(value) =>
                        setBatchRetryAttempts(
                          Math.max(1, Math.min(5, Number(value) || 2)),
                        )
                      }
                    />
                  </div>
                  <Button
                    data-testid='chatmock-batch-health'
                    loading={batchCheckingHealth}
                    onClick={handleBatchHealthCheckAll}
                  >
                    {t('全量健康检查')}
                  </Button>
                  <Button
                    data-testid='chatmock-batch-refresh-token'
                    loading={batchRefreshingAuth}
                    onClick={handleBatchRefreshTokenAll}
                  >
                    {t('全量刷新授权')}
                  </Button>
                  <Button
                    data-testid='chatmock-batch-entitlement'
                    loading={batchCheckingEntitlement}
                    onClick={handleBatchEntitlementCheckAll}
                  >
                    {t('全量权益检查')}
                  </Button>
                  <Button
                    data-testid='chatmock-export-json'
                    loading={exportingAccounts}
                    onClick={() => handleExportAccounts('json')}
                  >
                    {t('导出JSON')}
                  </Button>
                  <Button
                    data-testid='chatmock-export-csv'
                    loading={exportingAccounts}
                    onClick={() => handleExportAccounts('csv')}
                  >
                    {t('导出CSV')}
                  </Button>
                  <div className='flex items-center gap-2 rounded border border-amber-200 bg-amber-50 px-2 py-1 text-xs text-amber-900'>
                    <Switch
                      size='small'
                      checked={includeSensitiveExport}
                      onChange={(value) => setIncludeSensitiveExport(!!value)}
                    />
                    <span>{t('导出敏感字段(root+确认)')}</span>
                  </div>
                  <Button
                    data-testid='chatmock-export-inspector'
                    loading={exportingAccounts}
                    onClick={handleExportQuotaInspectorJSON}
                  >
                    {t('导出巡检JSON')}
                  </Button>
                </>
              ) : (
                <Button
                  data-testid='chatmock-switch-advanced-inline'
                  theme='outline'
                  onClick={() => setOperationMode('advanced')}
                >
                  {t('显示高级操作')}
                </Button>
              )}
            </div>
            <div className='mb-3 flex flex-wrap items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2'>
              <Text>{t('已选账号')}: <b>{selectedAccountRowKeys.length}</b></Text>
              <Text type='secondary'>{t('当前筛选命中')}: <b>{Number(total) || 0}</b></Text>
              <Button
                size='small'
                theme='outline'
                disabled={selectedAccountRowKeys.length === 0}
                loading={batchMutatingAccounts}
                onClick={() => handleBatchSetSelectedAccountsStatus('enabled')}
              >
                {t('启用所选')}
              </Button>
              <Button
                size='small'
                theme='outline'
                disabled={selectedAccountRowKeys.length === 0}
                loading={batchMutatingAccounts}
                onClick={() => handleBatchSetSelectedAccountsStatus('disabled')}
              >
                {t('禁用所选')}
              </Button>
              <Button
                size='small'
                theme='outline'
                disabled={Number(total) <= 0}
                loading={batchMutatingAccounts}
                onClick={() => handleBatchSetFilteredAccountsStatus('enabled')}
              >
                {t('启用当前筛选')}
              </Button>
              <Button
                size='small'
                theme='outline'
                disabled={Number(total) <= 0}
                loading={batchMutatingAccounts}
                onClick={() => handleBatchSetFilteredAccountsStatus('disabled')}
              >
                {t('禁用当前筛选')}
              </Button>
              <Button
                size='small'
                type='danger'
                disabled={selectedAccountRowKeys.length === 0}
                loading={batchMutatingAccounts}
                onClick={handleBatchDeleteSelectedAccounts}
              >
                {t('删除所选')}
              </Button>
              <Button
                size='small'
                theme='light'
                type='warning'
                disabled={Number(total) <= 0}
                loading={batchMutatingAccounts}
                onClick={handleBatchDeleteFilteredAccounts}
              >
                {t('清理当前筛选')}
              </Button>
            </div>
            <Table
              rowKey='id'
              rowSelection={accountRowSelection}
              loading={loading}
              columns={displayAccountColumns}
              dataSource={sortedAccounts}
              scroll={{ x: isAdvancedMode ? 3000 : 1700 }}
              pagination={{
                currentPage: page,
                pageSize,
                total,
                showSizeChanger: true,
                pageSizeOptions: [10, 20, 50, 100],
                onPageChange: (nextPage) => {
                  setPage(nextPage);
                  loadAccounts(nextPage, pageSize);
                },
                onPageSizeChange: (nextSize) => {
                  setPageSize(nextSize);
                  setPage(1);
                  loadAccounts(1, nextSize);
                },
              }}
            />
          </TabPane>

          <TabPane tab={t('邮箱管理')} itemKey='mailboxes'>
            {!mailboxEnabled ? (
              <div className='rounded-lg border border-amber-300 bg-amber-50 px-4 py-3 text-amber-900'>
                <div className='font-semibold'>{t('邮箱中心未启用')}</div>
                <div className='mt-1 text-sm'>{mailboxHint || 'CHATMOCK_MAILBOX_ENABLED=false'}</div>
              </div>
            ) : (
              <>
                <div className='mb-3 flex flex-wrap items-end gap-2'>
                  <Input
                    style={{ width: 260 }}
                    value={mailboxKeyword}
                    placeholder={t('按名称/邮箱/用户名搜索')}
                    onChange={(value) => setMailboxKeyword(String(value || ''))}
                  />
                  <Select
                    style={{ width: 180 }}
                    value={mailboxStatusFilter}
                    optionList={[
                      { label: t('全部状态'), value: '' },
                      { label: 'enabled', value: 'enabled' },
                      { label: 'disabled', value: 'disabled' },
                    ]}
                    onChange={(value) => setMailboxStatusFilter(String(value || ''))}
                  />
                  <Button
                    onClick={() => {
                      setMailboxPage(1);
                      loadMailboxes(1, mailboxPageSize);
                    }}
                  >
                    {t('查询')}
                  </Button>
                  <Button
                    onClick={() => {
                      setMailboxKeyword('');
                      setMailboxStatusFilter('');
                      setMailboxPage(1);
                    }}
                  >
                    {t('清空筛选')}
                  </Button>
                </div>
                <div className='mb-3 flex flex-wrap gap-2'>
                  <Button type='primary' onClick={openCreateMailboxModal}>
                    {t('新增邮箱')}
                  </Button>
                  <Button onClick={() => setMailboxBatchImportVisible(true)}>
                    {t('批量导入')}
                  </Button>
                  <Button loading={mailboxBatchTesting} onClick={handleBatchTestMailboxes}>
                    {t('全量连接测试')}
                  </Button>
                  <Button
                    loading={exportingMailboxes}
                    onClick={() => handleExportMailboxes('json')}
                  >
                    {t('导出JSON')}
                  </Button>
                  <Button
                    loading={exportingMailboxes}
                    onClick={() => handleExportMailboxes('csv')}
                  >
                    {t('导出CSV')}
                  </Button>
                  <Button onClick={() => loadMailboxes(mailboxPage, mailboxPageSize)}>
                    {t('刷新邮箱列表')}
                  </Button>
                </div>
                <Table
                  rowKey='id'
                  loading={mailboxesLoading}
                  columns={mailboxColumns}
                  dataSource={mailboxes}
                  scroll={{ x: 2600 }}
                  pagination={{
                    currentPage: mailboxPage,
                    pageSize: mailboxPageSize,
                    total: mailboxTotal,
                    showSizeChanger: true,
                    pageSizeOptions: [10, 20, 50, 100],
                    onPageChange: (nextPage) => {
                      setMailboxPage(nextPage);
                      loadMailboxes(nextPage, mailboxPageSize);
                    },
                    onPageSizeChange: (nextSize) => {
                      setMailboxPageSize(nextSize);
                      setMailboxPage(1);
                      loadMailboxes(1, nextSize);
                    },
                  }}
                />
              </>
            )}
          </TabPane>

          <TabPane tab={t('邮件事件')} itemKey='events'>
            {!mailboxEnabled ? (
              <div className='rounded-lg border border-amber-300 bg-amber-50 px-4 py-3 text-amber-900'>
                <div className='font-semibold'>{t('邮箱中心未启用')}</div>
                <div className='mt-1 text-sm'>{mailboxHint || 'CHATMOCK_MAILBOX_ENABLED=false'}</div>
              </div>
            ) : (
              <>
                <div className='mb-3 flex flex-wrap items-center gap-2'>
                  <Select
                    style={{ width: 280 }}
                    optionList={mailboxSelectOptions}
                    value={eventsMailboxId}
                    onChange={(value) => {
                      const next = Number(value) || 0;
                      setEventsMailboxId(next);
                      setEventsPage(1);
                      loadMailEvents(
                        1,
                        eventsPageSize,
                        next,
                        eventsType,
                        eventsClassification,
                        eventsKeyword,
                      );
                    }}
                  />
                  <Select
                    style={{ width: 220 }}
                    optionList={CHATMOCK_MAIL_EVENT_TYPE_OPTIONS}
                    value={eventsType}
                    onChange={(value) => {
                      const next = String(value || '');
                      setEventsType(next);
                      setEventsPage(1);
                      loadMailEvents(
                        1,
                        eventsPageSize,
                        eventsMailboxId,
                        next,
                        eventsClassification,
                        eventsKeyword,
                      );
                    }}
                  />
                  <Select
                    style={{ width: 220 }}
                    optionList={CHATMOCK_MAIL_EVENT_CLASSIFICATION_OPTIONS}
                    value={eventsClassification}
                    onChange={(value) => {
                      const next = String(value || '');
                      setEventsClassification(next);
                      setEventsPage(1);
                      loadMailEvents(
                        1,
                        eventsPageSize,
                        eventsMailboxId,
                        eventsType,
                        next,
                        eventsKeyword,
                      );
                    }}
                  />
                  <Input
                    style={{ width: 260 }}
                    value={eventsKeyword}
                    placeholder={t('按主题/发件人/内容/关联标识搜索')}
                    onChange={(value) => setEventsKeyword(String(value || ''))}
                  />
                  <Button
                    onClick={() =>
                      loadMailEvents(
                        eventsPage,
                        eventsPageSize,
                        eventsMailboxId,
                        eventsType,
                        eventsClassification,
                        eventsKeyword,
                      )
                    }
                  >
                    {t('刷新事件')}
                  </Button>
                  <Button
                    type='tertiary'
                    onClick={() => {
                      setEventsType('');
                      setEventsClassification('');
                      setEventsKeyword('');
                      setEventsPage(1);
                    }}
                  >
                    {t('清空筛选')}
                  </Button>
                </div>
                <Table
                  rowKey='id'
                  loading={eventsLoading}
                  columns={eventColumns}
                  dataSource={events}
                  scroll={{ x: 2600 }}
                  pagination={{
                    currentPage: eventsPage,
                    pageSize: eventsPageSize,
                    total: eventsTotal,
                    showSizeChanger: true,
                    pageSizeOptions: [10, 20, 50, 100],
                    onPageChange: (nextPage) => {
                      setEventsPage(nextPage);
                      loadMailEvents(
                        nextPage,
                        eventsPageSize,
                        eventsMailboxId,
                        eventsType,
                        eventsClassification,
                        eventsKeyword,
                      );
                    },
                    onPageSizeChange: (nextSize) => {
                      setEventsPageSize(nextSize);
                      setEventsPage(1);
                      loadMailEvents(
                        1,
                        nextSize,
                        eventsMailboxId,
                        eventsType,
                        eventsClassification,
                        eventsKeyword,
                      );
                    },
                  }}
                />
              </>
            )}
          </TabPane>

          {isAdvancedMode ? (
            <TabPane tab={t('路由日志')} itemKey='route_logs'>
              <div className='mb-3 flex flex-wrap items-end gap-2'>
                <InputNumber
                  min={0}
                  style={{ width: 140 }}
                  value={routeLogsAccountId}
                  onChange={(value) => setRouteLogsAccountId(Number(value) || 0)}
                  placeholder={t('账号ID')}
                />
                <Input
                  style={{ width: 220 }}
                  value={routeLogsModel}
                  onChange={(value) => setRouteLogsModel(String(value || ''))}
                  placeholder={t('模型名')}
                />
                <Select
                  style={{ width: 150 }}
                  value={routeLogsSuccess}
                  optionList={[
                    { label: t('全部结果'), value: '' },
                    { label: 'success', value: 'true' },
                    { label: 'failed', value: 'false' },
                  ]}
                  onChange={(value) => setRouteLogsSuccess(String(value || ''))}
                />
                <Input
                  style={{ width: 220 }}
                  value={routeLogsStrategy}
                  onChange={(value) => setRouteLogsStrategy(String(value || ''))}
                  placeholder={t('策略，如 weighted_random_with_quota_bias')}
                />
                <Input
                  style={{ width: 260 }}
                  value={routeLogsKeyword}
                  onChange={(value) => setRouteLogsKeyword(String(value || ''))}
                  placeholder={t('关键词')}
                />
                <Button
                  onClick={() => {
                    setRouteLogsPage(1);
                    loadRouteLogs(
                      1,
                      routeLogsPageSize,
                      routeLogsAccountId,
                      routeLogsModel,
                      routeLogsSuccess,
                      routeLogsStrategy,
                      routeLogsKeyword,
                    );
                  }}
                >
                  {t('查询')}
                </Button>
                <Button
                  type='tertiary'
                  onClick={() => {
                    setRouteLogsAccountId(0);
                    setRouteLogsModel('');
                    setRouteLogsSuccess('');
                    setRouteLogsStrategy('');
                    setRouteLogsKeyword('');
                    setRouteLogsPage(1);
                  }}
                >
                  {t('清空筛选')}
                </Button>
              </div>
              <Table
                rowKey='id'
                loading={routeLogsLoading}
                columns={routeLogColumns}
                dataSource={routeLogs}
                scroll={{ x: 2200 }}
                pagination={{
                  currentPage: routeLogsPage,
                  pageSize: routeLogsPageSize,
                  total: routeLogsTotal,
                  showSizeChanger: true,
                  pageSizeOptions: [10, 20, 50, 100],
                  onPageChange: (nextPage) => {
                    setRouteLogsPage(nextPage);
                    loadRouteLogs(
                      nextPage,
                      routeLogsPageSize,
                      routeLogsAccountId,
                      routeLogsModel,
                      routeLogsSuccess,
                      routeLogsStrategy,
                      routeLogsKeyword,
                    );
                  },
                  onPageSizeChange: (nextSize) => {
                    setRouteLogsPageSize(nextSize);
                    setRouteLogsPage(1);
                    loadRouteLogs(
                      1,
                      nextSize,
                      routeLogsAccountId,
                      routeLogsModel,
                      routeLogsSuccess,
                      routeLogsStrategy,
                      routeLogsKeyword,
                    );
                  },
                }}
              />
            </TabPane>
          ) : null}

          {isAdvancedMode ? (
            <TabPane tab={t('账号快照')} itemKey='account_snapshots'>
              <div className='mb-3 flex flex-wrap items-end gap-2'>
                <InputNumber
                  min={0}
                  style={{ width: 140 }}
                  value={snapshotsAccountId}
                  onChange={(value) => setSnapshotsAccountId(Number(value) || 0)}
                  placeholder={t('账号ID')}
                />
                <Select
                  style={{ width: 200 }}
                  value={snapshotsType}
                  optionList={[
                    { label: t('全部类型'), value: '' },
                    { label: 'health_check', value: 'health_check' },
                    { label: 'entitlement_check', value: 'entitlement_check' },
                    { label: 'token_refresh', value: 'token_refresh' },
                    { label: 'route_result', value: 'route_result' },
                  ]}
                  onChange={(value) => setSnapshotsType(String(value || ''))}
                />
                <Select
                  style={{ width: 160 }}
                  value={snapshotsStatus}
                  optionList={[
                    { label: t('全部状态'), value: '' },
                    { label: 'ok', value: 'ok' },
                    { label: 'warn', value: 'warn' },
                    { label: 'error', value: 'error' },
                  ]}
                  onChange={(value) => setSnapshotsStatus(String(value || ''))}
                />
                <Input
                  style={{ width: 260 }}
                  value={snapshotsKeyword}
                  onChange={(value) => setSnapshotsKeyword(String(value || ''))}
                  placeholder={t('摘要/详情关键词')}
                />
                <Button
                  onClick={() => {
                    setSnapshotsPage(1);
                    loadAccountSnapshots(
                      1,
                      snapshotsPageSize,
                      snapshotsAccountId,
                      snapshotsType,
                      snapshotsStatus,
                      snapshotsKeyword,
                    );
                  }}
                >
                  {t('查询')}
                </Button>
                <Button
                  type='tertiary'
                  onClick={() => {
                    setSnapshotsAccountId(0);
                    setSnapshotsType('');
                    setSnapshotsStatus('');
                    setSnapshotsKeyword('');
                    setSnapshotsPage(1);
                  }}
                >
                  {t('清空筛选')}
                </Button>
              </div>
              <Table
                rowKey='id'
                loading={snapshotsLoading}
                columns={accountSnapshotColumns}
                dataSource={snapshots}
                scroll={{ x: 1900 }}
                pagination={{
                  currentPage: snapshotsPage,
                  pageSize: snapshotsPageSize,
                  total: snapshotsTotal,
                  showSizeChanger: true,
                  pageSizeOptions: [10, 20, 50, 100],
                  onPageChange: (nextPage) => {
                    setSnapshotsPage(nextPage);
                    loadAccountSnapshots(
                      nextPage,
                      snapshotsPageSize,
                      snapshotsAccountId,
                      snapshotsType,
                      snapshotsStatus,
                      snapshotsKeyword,
                    );
                  },
                  onPageSizeChange: (nextSize) => {
                    setSnapshotsPageSize(nextSize);
                    setSnapshotsPage(1);
                    loadAccountSnapshots(
                      1,
                      nextSize,
                      snapshotsAccountId,
                      snapshotsType,
                      snapshotsStatus,
                      snapshotsKeyword,
                    );
                  },
                }}
              />
            </TabPane>
          ) : null}
        </Tabs>
      </Card>

      <Modal
        title={t('账号池路由诊断结果')}
        visible={diagnoseVisible}
        onCancel={() => setDiagnoseVisible(false)}
        footer={
          <Button type='primary' onClick={() => setDiagnoseVisible(false)}>
            {t('关闭')}
          </Button>
        }
        width={1100}
      >
        {diagnoseResult ? (
          <div className='space-y-3'>
            <div className='grid grid-cols-1 gap-2 rounded border border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-700 md:grid-cols-2'>
              <div>
                {t('请求模型')}: <b>{diagnoseResult?.model || '-'}</b>
              </div>
              <div>
                {t('调度策略')}: <b>{diagnoseResult?.selection_policy || '-'}</b>
              </div>
              <div>
                {t('账号总数')}: <b>{Number(diagnoseResult?.total_count || 0)}</b>
              </div>
              <div>
                {t('可路由账号')}: <b>{Number(diagnoseResult?.eligible_count || 0)}</b>
              </div>
              <div>
                {t('本次选中')}: <b>{diagnoseResult?.selected_account_name || '-'}</b>
              </div>
              <div>
                {t('诊断时间')}:{' '}
                <b>
                  {diagnoseResult?.timestamp
                    ? timestamp2string(diagnoseResult.timestamp)
                    : '-'}
                </b>
              </div>
              {diagnoseResult?.selection_note ? (
                <div className='md:col-span-2 text-gray-800'>
                  {t('说明')}: {diagnoseResult.selection_note}
                </div>
              ) : null}
            </div>
            <Table
              rowKey='id'
              columns={diagnoseColumns}
              dataSource={diagnoseRows}
              pagination={false}
              scroll={{ x: 1700, y: 420 }}
            />
          </div>
        ) : (
          <div className='text-sm text-gray-700'>{t('暂无诊断数据')}</div>
        )}
      </Modal>

      <Modal
        title={editingAccount ? t('编辑账号') : t('新增账号')}
        visible={formVisible}
        onCancel={() => setFormVisible(false)}
        onOk={handleSubmitForm}
        confirmLoading={submitting}
        width={720}
      >
        <Form
          initValues={DEFAULT_FORM_VALUES}
          getFormApi={(api) => setFormApi(api)}
          labelPosition='left'
          labelAlign='left'
          style={{ marginTop: 8 }}
        >
          <Form.Input field='name' label={t('账号名称')} />
          <Form.Input field='account' label={t('账号标识')} />
          <Form.Input
            field='api_key'
            label={t('API Key')}
            placeholder={editingAccount ? t('留空表示不更新当前密钥') : t('请输入 API Key')}
          />
          <Form.Input
            field='id_token'
            label='ID Token'
            placeholder={editingAccount ? t('留空表示不更新当前 ID Token') : t('可选')}
          />
          <Form.Input
            field='refresh_token'
            label='Refresh Token'
            placeholder={editingAccount ? t('留空表示不更新当前 Refresh Token') : t('可选')}
          />
          <Form.Input field='base_url' label={t('Base URL')} />
          <Form.Input field='proxy_url' label={t('Proxy URL')} />

          <Form.Select
            field='status'
            label={t('状态')}
            optionList={[
              { value: 'enabled', label: 'enabled' },
              { value: 'disabled', label: 'disabled' },
              { value: 'cooling', label: 'cooling' },
              { value: 'abnormal', label: 'abnormal' },
              { value: 'quota_low', label: 'quota_low' },
              { value: 'quota_exhausted', label: 'quota_exhausted' },
              { value: 'auth_invalid', label: 'auth_invalid' },
              { value: 'network_issue', label: 'network_issue' },
              { value: 'manual_review', label: 'manual_review' },
            ]}
          />

          <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
            <Form.InputNumber field='weight' label={t('权重')} min={1} />
            <Form.InputNumber field='priority' label={t('优先级')} min={1} />
          </div>

          <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
            <Form.InputNumber field='max_concurrency' label={t('并发上限')} min={0} />
            <Form.InputNumber field='rpm' label='RPM' min={0} />
          </div>

          <Form.InputNumber field='daily_token_limit' label={t('每日Token上限')} min={0} />

          <Form.TextArea field='supported_models_text' label={t('支持模型(逗号分隔)')} autosize />

          <Form.TextArea field='metadata' label={t('Metadata')} autosize />
        </Form>
      </Modal>

      <Modal
        title={t('批量导入账号')}
        visible={batchImportVisible}
        onCancel={() => setBatchImportVisible(false)}
        onOk={handleBatchImport}
        confirmLoading={batchImportSubmitting}
        okText={t('开始导入')}
        width={860}
      >
        <div className='space-y-3'>
          <Text className='block text-sm'>
            {t('支持三种格式：JSON数组、逐行JSON、CSV/TSV。CSV 默认列顺序：name,account,api_key,id_token,refresh_token,type,email,base_url,proxy_url,status,weight,priority,max_concurrency,rpm,daily_token_limit,supported_models,metadata')}
          </Text>
          <div className='flex flex-wrap items-center gap-2'>
            <input
              ref={batchImportFileInputRef}
              type='file'
              multiple
              accept='.json,.jsonl,.csv,.tsv,.txt'
              className='hidden'
              onChange={handleBatchImportFileChange}
            />
            <Button
              loading={batchImportUploading}
              onClick={() => batchImportFileInputRef.current?.click()}
            >
              {t('选择文件导入')}
            </Button>
            <Button type='tertiary' onClick={fillAccountImportTemplate}>
              {t('填充模板')}
            </Button>
            {batchImportFileName ? (
              <Text type='secondary'>{t('已选择：{{name}}', { name: batchImportFileName })}</Text>
            ) : null}
          </div>
          <TextArea
            rows={14}
            autosize={false}
            value={batchImportText}
            onChange={(value) => setBatchImportText(value)}
            placeholder='[{"name":"acct-1","account":"acct-1","api_key":"sk-xxx","id_token":"...","refresh_token":"...","base_url":"https://api.openai.com"}]'
          />
          {batchImportResult && (
            <div className='rounded-lg border border-gray-200 bg-gray-50 p-3'>
              <div className='font-medium'>
                {t('导入结果：总计 {{total}}，成功 {{success}}，失败 {{failed}}', {
                  total: batchImportResult.total,
                  success: batchImportResult.success,
                  failed: batchImportResult.failed,
                })}
              </div>
              {batchImportResult.details.length > 0 && (
                <div className='mt-2 max-h-40 overflow-auto whitespace-pre-wrap text-xs text-gray-700'>
                  {batchImportResult.details.join('\n')}
                </div>
              )}
            </div>
          )}
        </div>
      </Modal>

      <Modal
        title={t('批量导入邮箱')}
        visible={mailboxBatchImportVisible}
        onCancel={() => setMailboxBatchImportVisible(false)}
        onOk={handleMailboxBatchImport}
        confirmLoading={mailboxBatchImportSubmitting}
        okText={t('开始导入')}
        width={860}
      >
        <div className='space-y-3'>
          <Text className='block text-sm'>
            {t('支持三种格式：JSON数组、逐行JSON、CSV/TSV。CSV 默认列顺序：name,email,username,password,imap_host,imap_port,imap_security,smtp_host(可选),smtp_port(可选),smtp_security(可选),status,poll_interval_second,metadata')}
          </Text>
          <Text type='secondary' className='block text-xs'>
            {t('建议使用邮箱授权码（而非登录密码），并先在邮箱后台开启 IMAP/SMTP 服务。')}
          </Text>
          <div className='flex flex-wrap items-center gap-2'>
            <input
              ref={mailboxBatchImportFileInputRef}
              type='file'
              multiple
              accept='.json,.jsonl,.csv,.tsv,.txt'
              className='hidden'
              onChange={handleMailboxBatchImportFileChange}
            />
            <Button
              loading={mailboxBatchImportUploading}
              onClick={() => mailboxBatchImportFileInputRef.current?.click()}
            >
              {t('选择文件导入')}
            </Button>
            <Button type='tertiary' onClick={fillMailboxImportTemplate}>
              {t('填充模板')}
            </Button>
            {mailboxBatchImportFileName ? (
              <Text type='secondary'>
                {t('已选择：{{name}}', { name: mailboxBatchImportFileName })}
              </Text>
            ) : null}
          </div>
          <TextArea
            rows={14}
            autosize={false}
            value={mailboxBatchImportText}
            onChange={(value) => setMailboxBatchImportText(value)}
            placeholder='[{"name":"mailbox-1","email":"demo@example.com","username":"demo@example.com","password":"app-pass","imap_host":"imap.qq.com"}]'
          />
          {mailboxBatchImportResult && (
            <div className='rounded-lg border border-gray-200 bg-gray-50 p-3'>
              <div className='font-medium'>
                {t('导入结果：总计 {{total}}，成功 {{success}}，失败 {{failed}}', {
                  total: mailboxBatchImportResult.total,
                  success: mailboxBatchImportResult.success,
                  failed: mailboxBatchImportResult.failed,
                })}
              </div>
              {mailboxBatchImportResult.details.length > 0 && (
                <div className='mt-2 max-h-40 overflow-auto whitespace-pre-wrap text-xs text-gray-700'>
                  {mailboxBatchImportResult.details.join('\n')}
                </div>
              )}
            </div>
          )}
        </div>
      </Modal>

      <Modal
        title={editingMailbox ? t('编辑邮箱') : t('新增邮箱')}
        visible={mailboxFormVisible}
        onCancel={() => setMailboxFormVisible(false)}
        onOk={handleSubmitMailboxForm}
        confirmLoading={mailboxSubmitting}
        width={760}
      >
        <Form
          initValues={DEFAULT_MAILBOX_FORM_VALUES}
          getFormApi={(api) => setMailboxFormApi(api)}
          labelPosition='left'
          labelAlign='left'
          style={{ marginTop: 8 }}
        >
          <Form.Select
            field='provider_template'
            label={t('邮箱服务商模板')}
            optionList={[
              { value: 'qq', label: 'QQ' },
              { value: 'gmail', label: 'Gmail' },
              { value: 'outlook', label: 'Outlook' },
              { value: '163', label: '163' },
              { value: 'tempmail_wizawn', label: 'TempMail / Wizawn' },
              { value: 'custom', label: 'Custom' },
            ]}
            onChange={(value) => applyMailboxPreset(value)}
          />
          <Form.Input field='name' label={t('邮箱名称')} />
          <Form.Input field='email' label={t('邮箱地址')} />
          <Form.Input field='username' label={t('用户名')} />
          <Form.Input
            field='password'
            label={t('密码/授权码')}
            placeholder={editingMailbox ? t('留空表示不更新') : t('请输入密码或授权码')}
          />

          <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
            <Form.Input field='imap_host' label='IMAP Host' />
            <Form.InputNumber field='imap_port' label='IMAP Port' min={1} max={65535} />
          </div>

          <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
            <Form.Select
              field='imap_security'
              label='IMAP Security'
              optionList={[
                { value: 'ssl', label: 'ssl' },
                { value: 'starttls', label: 'starttls' },
                { value: 'none', label: 'none' },
              ]}
            />
            <Form.Select
              field='status'
              label={t('状态')}
              optionList={[
                { value: 'enabled', label: 'enabled' },
                { value: 'disabled', label: 'disabled' },
              ]}
            />
          </div>

          <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
            <Form.Input field='smtp_host' label='SMTP Host（可选）' />
            <Form.InputNumber field='smtp_port' label='SMTP Port（可选）' min={1} max={65535} />
          </div>

          <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
            <Form.Select
              field='smtp_security'
              label='SMTP Security'
              optionList={[
                { value: 'ssl', label: 'ssl' },
                { value: 'starttls', label: 'starttls' },
                { value: 'none', label: 'none' },
              ]}
            />
            <Form.InputNumber
              field='poll_interval_second'
              label={t('轮询间隔(秒)')}
              min={30}
            />
          </div>

          <Form.TextArea field='metadata' label={t('Metadata')} autosize />
        </Form>
      </Modal>
    </div>
  );
};

export default ChatMockPage;
