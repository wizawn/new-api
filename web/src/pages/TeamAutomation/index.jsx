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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Banner,
  Button,
  Card,
  Empty,
  Input,
  InputNumber,
  Modal,
  Progress,
  Select,
  Space,
  Spin,
  Switch,
  Table,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { Link } from 'react-router-dom';
import {
  autoSelectTeamAutomationTeam,
  batchDeleteTeamAutomationAccounts,
  batchDeleteTeamAutomationTeams,
  batchUpdateTeamAutomationAccounts,
  batchUpdateTeamAutomationTeams,
  buildTeamAutomationTeamsExportURL,
  buildTeamAutomationAccountsExportURL,
  batchDeleteTeamAutomationRuns,
  createTeamAutomationAccount,
  createTeamAutomationTeam,
  deleteTeamAutomationAccount,
  deleteTeamAutomationTeam,
  getTeamAutomationAccountStats,
  getTeamAutomationMeta,
  getTeamAutomationModules,
  getTeamAutomationRun,
  getTeamAutomationRunHistory,
  getTeamAutomationRunLogs,
  getTeamAutomationTeamStats,
  importTeamAutomationTeams,
  importTeamAutomationAccountsCSV,
  importRegisteredTeamAutomationAccounts,
  importTeamAutomationRedeemCodes,
  listTeamAutomationAccounts,
  listTeamAutomationTeams,
  showError,
  showSuccess,
  startTeamAutomationRun,
  stopTeamAutomationRun,
  TEAM_AUTOMATION_IMPORT_MAX_BYTES,
  updateTeamAutomationAccount,
  updateTeamAutomationTeam,
} from '../../helpers';
import { useTranslation } from 'react-i18next';
import {
  CircleStop,
  Download,
  Eye,
  FileUp,
  Play,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  X,
} from 'lucide-react';

const { Text, Title } = Typography;

const STATUS_COLOR = {
  pending: 'grey',
  generated: 'blue',
  waiting_for_invite: 'cyan',
  invite_received: 'cyan',
  joined_team: 'indigo',
  login_captured: 'purple',
  converted: 'violet',
  imported: 'teal',
  completed: 'green',
  manual_review: 'yellow',
  failed: 'red',
  disabled: 'grey',
  queued: 'grey',
  running: 'blue',
  stopping: 'orange',
  stopped: 'grey',
  active: 'green',
  full: 'orange',
  expired: 'red',
  error: 'red',
  banned: 'grey',
};

const ACCOUNT_STATUS_OPTIONS = [
  { label: '全部账号', value: '' },
  { label: 'pending', value: 'pending' },
  { label: 'generated', value: 'generated' },
  { label: 'waiting_for_invite', value: 'waiting_for_invite' },
  { label: 'invite_received', value: 'invite_received' },
  { label: 'joined_team', value: 'joined_team' },
  { label: 'login_captured', value: 'login_captured' },
  { label: 'converted', value: 'converted' },
  { label: 'imported', value: 'imported' },
  { label: 'completed', value: 'completed' },
  { label: 'manual_review', value: 'manual_review' },
  { label: 'failed', value: 'failed' },
  { label: 'disabled', value: 'disabled' },
];

const DEFAULT_ACCOUNT_FORM = {
  id: 0,
  name: '',
  email: '',
  password: '',
  redeem_code: '',
  redeem_type: '',
  status: 'pending',
  redeem_site: '',
  mailbox_id: 0,
  metadata: '',
};

const DEFAULT_REGISTERED_IMPORT_FORM = {
  text: '',
  default_redeem_site: '',
};

const TEAM_STATUS_OPTIONS = [
  { label: '全部 Team', value: '' },
  { label: 'active', value: 'active' },
  { label: 'full', value: 'full' },
  { label: 'expired', value: 'expired' },
  { label: 'error', value: 'error' },
  { label: 'banned', value: 'banned' },
  { label: 'disabled', value: 'disabled' },
];

const EXECUTION_LOG_LEVEL_OPTIONS = [
  { label: '全部级别', value: '' },
  { label: 'info', value: 'info' },
  { label: 'warning', value: 'warning' },
  { label: 'error', value: 'error' },
];

const EXECUTION_STATUS_OPTIONS = [
  { label: '全部执行状态', value: '' },
  { label: 'queued', value: 'queued' },
  { label: 'running', value: 'running' },
  { label: 'stopping', value: 'stopping' },
  { label: 'stopped', value: 'stopped' },
  { label: 'completed', value: 'completed' },
  { label: 'failed', value: 'failed' },
];

const DEFAULT_REDEEM_IMPORT_FORM = {
  lines: '',
  default_redeem_site: '',
};

const DEFAULT_TEAM_FORM = {
  id: 0,
  email: '',
  access_token: '',
  refresh_token: '',
  session_token: '',
  client_id: '',
  account_id: '',
  team_name: '',
  plan_type: '',
  subscription_plan: '',
  expires_at: 0,
  current_members: 0,
  pending_invites: 0,
  max_members: 6,
  status: 'active',
  account_role: '',
  device_code_auth_enabled: false,
  error_count: 0,
  last_sync_time: 0,
  metadata: '',
};

const DEFAULT_TEAM_IMPORT_FORM = {
  content: '',
};

const FULL_PIPELINE_MODULE_ID = '';

const HISTORY_QUERY_DEFAULTS = {
  page: 1,
  page_size: 20,
  status: '',
  module: '',
  search: '',
  only_failed: false,
  only_proxy_retried: false,
};

const EXECUTION_LOG_PAGE_SIZE = 500;

const ACCOUNT_QUICK_FILTERS = [
  { key: 'waiting_for_invite', label: '待邀请', value: 'waiting_for_invite' },
  { key: 'converted', label: '待导入', value: 'converted' },
  { key: 'failed', label: '失败', value: 'failed' },
  { key: 'manual_review', label: '人工复核', value: 'manual_review' },
];

const TEAM_QUICK_FILTERS = [
  { key: 'active', label: '活跃 Team', value: 'active' },
  { key: 'full', label: '已满', value: 'full' },
  { key: 'expired', label: '过期', value: 'expired' },
  { key: 'error', label: '异常', value: 'error' },
];

const ACCOUNT_OVERVIEW_CARD_DEFINITIONS = [
  { key: 'all', label: '账号总数', status: '' },
  { key: 'waiting_for_invite', label: '等待邀请', status: 'waiting_for_invite' },
  { key: 'converted', label: '待导入', status: 'converted' },
  { key: 'failed', label: '失败', status: 'failed' },
  { key: 'manual_review', label: '人工复核', status: 'manual_review' },
];

const TEAM_STATUS_OVERVIEW_DEFINITIONS = [
  { key: 'all', label: '全部 Team', status: '' },
  { key: 'active', label: '活跃 Team', status: 'active' },
  { key: 'full', label: '已满', status: 'full' },
  { key: 'expired', label: '过期', status: 'expired' },
  { key: 'error', label: '异常', status: 'error' },
];

const RUN_TARGET_QUICK_FILTERS = [
  { key: 'all', label: '全部账号', status: '' },
  { key: 'waiting_for_invite', label: '待邀请', status: 'waiting_for_invite' },
  { key: 'converted', label: '待导入', status: 'converted' },
  { key: 'failed', label: '失败', status: 'failed' },
  { key: 'manual_review', label: '人工复核', status: 'manual_review' },
];

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

const PROXY_FIELD_KEYS = [
  'proxy_rotate_mode',
  'proxy_retry_count',
  'proxy_pool',
  'proxy_gateway',
  'proxy_switch_url',
  'proxy_switch_token',
  'disable_proxy',
];

const PROXY_PRESET_DEFINITIONS = [
  {
    key: 'gateway_only',
    label: 'gateway-only',
    description: '固定连外部网关，由 CPA 或网关侧自行轮询真实节点。',
    values: {
      proxy_rotate_mode: 'stable',
      proxy_retry_count: 1,
      proxy_pool: '',
      disable_proxy: false,
    },
  },
  {
    key: 'stable',
    label: 'stable',
    description: '同一账号尽量稳定命中同一节点，失败时再切换下一个节点。',
    values: {
      proxy_rotate_mode: 'stable',
      proxy_retry_count: 1,
      disable_proxy: false,
    },
  },
  {
    key: 'random',
    label: 'random',
    description: '每次执行随机选节点，适合快速打散流量。',
    values: {
      proxy_rotate_mode: 'random',
      proxy_retry_count: 1,
      disable_proxy: false,
    },
  },
  {
    key: 'vmess_xray',
    label: 'vmess + xray',
    description: '优先走本机 xray 转换 vmess，不再走外部换节点接管。',
    values: {
      proxy_rotate_mode: 'stable',
      proxy_retry_count: 1,
      proxy_gateway: '',
      proxy_switch_url: '',
      proxy_switch_token: '',
      disable_proxy: false,
    },
  },
  {
    key: 'direct',
    label: '直连',
    description: '本次模块执行不使用代理，直接走本地网络。',
    values: {
      disable_proxy: true,
      proxy_pool: '',
      proxy_gateway: '',
      proxy_switch_url: '',
      proxy_switch_token: '',
    },
  },
];

const getModulePipelineHint = (t) =>
  t('默认顺序：邮箱生成 -> Team 兑换 -> 邀请监听 -> 浏览器登录 -> 导入目标。');

const toOptionList = (options = []) => {
  if (!Array.isArray(options)) {
    return [];
  }
  return options
    .map((item) => {
      if (!item) {
        return null;
      }
      if (typeof item === 'string') {
        return { label: item, value: item };
      }
      const label = String(item.label || item.name || item.value || '').trim();
      const value = String(item.value || item.key || item.id || label).trim();
      if (!label && !value) {
        return null;
      }
      return {
        label: label || value,
        value: value || label,
      };
    })
    .filter(Boolean);
};

const normalizeModules = (payload) => {
  const items = Array.isArray(payload)
    ? payload
    : Array.isArray(payload?.items)
      ? payload.items
      : [];
  return items
    .map((item) => ({
      id: String(item?.id || '').trim(),
      name: String(item?.name || '').trim(),
      description: String(item?.description || '').trim(),
      fields: Array.isArray(item?.fields) ? item.fields : [],
      supported_statuses: Array.isArray(item?.supported_statuses)
        ? item.supported_statuses
        : [],
      bulk_supported: item?.bulk_supported !== false,
    }))
    .filter((item) => item.id);
};

const parseJSONPayload = (value) => {
  if (!value) {
    return null;
  }
  if (typeof value === 'object') {
    return value;
  }
  if (typeof value !== 'string') {
    return null;
  }
  try {
    return JSON.parse(value);
  } catch {
    return null;
  }
};

const formatTimestamp = (value) => {
  const timestamp = Number(value) || 0;
  if (timestamp <= 0) {
    return '-';
  }
  return new Date(timestamp * 1000).toLocaleString('zh-CN', {
    hour12: false,
  });
};

const shortenText = (value, maxLength = 160) => {
  const text = String(value || '').trim();
  if (!text) {
    return '';
  }
  if (text.length <= maxLength) {
    return text;
  }
  return `${text.slice(0, maxLength)}...`;
};

const extractFailureReasonFingerprint = (value) => {
  const text = String(value || '')
    .replace(/\s+/g, ' ')
    .trim();
  if (!text) {
    return '';
  }
  const primary = text.split(/\n+/)[0].split(/[|;；]/)[0].trim();
  return primary || text;
};

const summarizeExecutionResult = (summary) => {
  if (!summary || typeof summary !== 'object' || Array.isArray(summary)) {
    return {
      rows: [],
      failedRows: [],
    };
  }
  const rows = Object.entries(summary)
    .map(([key, value]) => {
      if (!value || typeof value !== 'object' || Array.isArray(value)) {
        return null;
      }
      const match = String(key).match(/^account_(\d+)$/);
      return {
        accountId: match ? Number(match[1]) || 0 : 0,
        key,
        status: String(value.status || '').trim(),
        module: String(value.module || '').trim(),
        error: String(value.error || '').trim(),
      };
    })
    .filter(Boolean)
    .sort((a, b) => a.accountId - b.accountId);
  return {
    rows,
    failedRows: rows.filter((item) => item.status === 'failed'),
  };
};

const buildExecutionHistoryFailureSummary = (record) => {
  const failedSummary = String(record?.failed_summary || '').trim();
  if (failedSummary) {
    return failedSummary;
  }
  const errorMessage = String(record?.error_message || '').trim();
  if (errorMessage) {
    return errorMessage;
  }
  return '';
};

const buildLogPayloadSummary = (payload) => {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
    return [];
  }
  const parts = [];
  const attempt = Number(payload.attempt) || 0;
  const nextAttempt = Number(payload.next_attempt) || 0;
  const maxAttempts = Number(payload.max_attempts) || 0;
  if (attempt > 0 && maxAttempts > 0) {
    parts.push(`attempt ${attempt}/${maxAttempts}`);
  } else if (attempt > 0) {
    parts.push(`attempt ${attempt}`);
  }
  if (nextAttempt > 0) {
    parts.push(`next ${nextAttempt}`);
  }
  const proxyNode = String(payload.proxy_node || payload.node || '').trim();
  const proxyValue = String(payload.proxy_value || payload.proxy || '').trim();
  if (proxyNode) {
    parts.push(`node ${shortenText(proxyNode, 40)}`);
  }
  if (proxyValue) {
    parts.push(`proxy ${shortenText(proxyValue, 60)}`);
  }
  const rotateMode = String(payload.rotate_mode || '').trim();
  if (rotateMode) {
    parts.push(`rotate ${rotateMode}`);
  }
  const targetURL = String(payload.target_url || '').trim();
  if (targetURL) {
    parts.push(`target ${shortenText(targetURL, 80)}`);
  }
  const retryReason = String(payload.retry_reason || payload.error || '').trim();
  if (retryReason) {
    parts.push(shortenText(retryReason, 120));
  }
  const responseText = String(payload.response || '').trim();
  if (responseText) {
    parts.push(`response ${shortenText(responseText, 100)}`);
  }
  return parts;
};

const isProxyRelatedLog = (item, payload) => {
  const fragments = [
    String(item?.step || '').trim(),
    String(item?.message || '').trim(),
    String(item?.payload || '').trim(),
    String(payload?.proxy || '').trim(),
    String(payload?.proxy_node || payload?.node || '').trim(),
    String(payload?.retry_reason || '').trim(),
  ]
    .join(' ')
    .toLowerCase();
  if (!fragments) {
    return false;
  }
  const patterns = [
    'proxy',
    'cloudflare',
    'add-phone',
    'retry',
    'gateway',
    'switch',
    'vmess',
    'xray',
    'captcha',
  ];
  return patterns.some((pattern) => fragments.includes(pattern));
};

const cleanupRunOptions = (fields = []) => {
  const next = {};
  Object.entries(fields).forEach(([key, value]) => {
    if (typeof value === 'string') {
      if (value.trim() !== '') {
        next[key] = value.trim();
      }
      return;
    }
    if (value !== null && value !== undefined) {
      next[key] = value;
    }
  });
  return next;
};

const StatusTag = ({ value }) => (
  <Tag color={STATUS_COLOR[value] || 'grey'}>{value || '-'}</Tag>
);

const TeamAutomationPage = () => {
  const { t } = useTranslation();
  const importRef = useRef(null);

  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [meta, setMeta] = useState({});
  const [modules, setModules] = useState([]);
  const [accountStats, setAccountStats] = useState(null);
  const [teamStats, setTeamStats] = useState(null);
  const [accounts, setAccounts] = useState([]);
  const [accountsTotal, setAccountsTotal] = useState(0);
  const [teams, setTeams] = useState([]);
  const [teamsTotal, setTeamsTotal] = useState(0);
  const [selectedAccountRowKeys, setSelectedAccountRowKeys] = useState([]);
  const [selectedTeamRowKeys, setSelectedTeamRowKeys] = useState([]);
  const [batchAccountStatus, setBatchAccountStatus] = useState('');
  const [batchAccountRedeemSite, setBatchAccountRedeemSite] = useState('');
  const [batchAccountMetadata, setBatchAccountMetadata] = useState('');
  const [batchTeamStatus, setBatchTeamStatus] = useState('');
  const [batchTeamMetadata, setBatchTeamMetadata] = useState('');
  const [history, setHistory] = useState([]);
  const [historyTotal, setHistoryTotal] = useState(0);
  const [selectedHistoryRowKeys, setSelectedHistoryRowKeys] = useState([]);
  const [selectedExecutionId, setSelectedExecutionId] = useState('');
  const [executionDetail, setExecutionDetail] = useState(null);
  const [executionLogs, setExecutionLogs] = useState([]);
  const [executionLogLevelFilter, setExecutionLogLevelFilter] = useState('');
  const [executionLogSearch, setExecutionLogSearch] = useState('');
  const [executionLogOnlyProxy, setExecutionLogOnlyProxy] = useState(false);
  const [executionLogTotal, setExecutionLogTotal] = useState(0);
  const [executionLogPage, setExecutionLogPage] = useState(1);
  const [executionLogLoading, setExecutionLogLoading] = useState(false);
  const [executionLogLoadingMore, setExecutionLogLoadingMore] = useState(false);
  const [teamAutoSelect, setTeamAutoSelect] = useState(null);
  const [teamAutoSelectExcludedIds, setTeamAutoSelectExcludedIds] = useState([]);

  const [runSubmitting, setRunSubmitting] = useState(false);
  const [stopSubmitting, setStopSubmitting] = useState(false);
  const [teamAutoSelectSubmitting, setTeamAutoSelectSubmitting] =
    useState(false);

  const [accountQuery, setAccountQuery] = useState({
    page: 1,
    page_size: 20,
    status: '',
    search: '',
  });
  const [teamQuery, setTeamQuery] = useState({
    page: 1,
    page_size: 20,
    status: '',
    search: '',
  });
  const [accountDraft, setAccountDraft] = useState({
    status: '',
    search: '',
  });
  const [teamDraft, setTeamDraft] = useState({
    status: '',
    search: '',
  });

  const [historyQuery, setHistoryQuery] = useState(HISTORY_QUERY_DEFAULTS);

  const [moduleId, setModuleId] = useState('');
  const [runFilterStatus, setRunFilterStatus] = useState('');
  const [runFilterSearch, setRunFilterSearch] = useState('');
  const [moduleFieldValues, setModuleFieldValues] = useState({});

  const [accountModal, setAccountModal] = useState({
    visible: false,
    mode: 'create',
    form: DEFAULT_ACCOUNT_FORM,
    submitting: false,
  });
  const [registeredImportModal, setRegisteredImportModal] = useState({
    visible: false,
    submitting: false,
    form: DEFAULT_REGISTERED_IMPORT_FORM,
  });
  const [redeemImportModal, setRedeemImportModal] = useState({
    visible: false,
    submitting: false,
    form: DEFAULT_REDEEM_IMPORT_FORM,
  });
  const [teamModal, setTeamModal] = useState({
    visible: false,
    mode: 'create',
    form: DEFAULT_TEAM_FORM,
    submitting: false,
  });
  const [teamImportModal, setTeamImportModal] = useState({
    visible: false,
    form: DEFAULT_TEAM_IMPORT_FORM,
    submitting: false,
  });

  const currentModule = useMemo(
    () => modules.find((item) => item.id === moduleId) || null,
    [modules, moduleId],
  );
  const executionSummary = useMemo(
    () => summarizeExecutionResult(executionDetail?.result_summary),
    [executionDetail?.result_summary],
  );
  const executionLogStats = useMemo(() => {
    const stats = {
      total: executionLogs.length,
      warning: 0,
      error: 0,
      proxy: 0,
      retry: 0,
    };
    executionLogs.forEach((item) => {
      const payload = parseJSONPayload(item?.payload);
      const level = String(item?.level || '').trim().toLowerCase();
      const message = String(item?.message || '').trim().toLowerCase();
      if (level === 'warning') {
        stats.warning += 1;
      }
      if (level === 'error') {
        stats.error += 1;
      }
      if (isProxyRelatedLog(item, payload)) {
        stats.proxy += 1;
      }
      if (message.includes('proxy retry scheduled')) {
        stats.retry += 1;
      }
    });
    return stats;
  }, [executionLogs]);
  const filteredExecutionLogs = useMemo(() => {
    const keyword = String(executionLogSearch || '').trim().toLowerCase();
    return executionLogs
      .filter((item) => {
        const payload = parseJSONPayload(item?.payload);
        const level = String(item?.level || '').trim().toLowerCase();
        if (executionLogLevelFilter && level !== executionLogLevelFilter) {
          return false;
        }
        if (executionLogOnlyProxy && !isProxyRelatedLog(item, payload)) {
          return false;
        }
        if (!keyword) {
          return true;
        }
        const haystack = [
          String(item?.step || '').trim(),
          String(item?.message || '').trim(),
          String(item?.payload || '').trim(),
          String(item?.account_id || '').trim(),
        ]
          .join(' ')
          .toLowerCase();
        return haystack.includes(keyword);
      });
  }, [
    executionLogs,
    executionLogLevelFilter,
    executionLogOnlyProxy,
    executionLogSearch,
  ]);
  const historyStats = useMemo(() => {
    const stats = {
      total: history.length,
      failed: 0,
      proxyRetried: 0,
      completed: 0,
    };
    history.forEach((item) => {
      const status = String(item?.status || '').trim().toLowerCase();
      if (status === 'failed') {
        stats.failed += 1;
      }
      if (status === 'completed') {
        stats.completed += 1;
      }
      if ((Number(item?.proxy_retry_count) || 0) > 0) {
        stats.proxyRetried += 1;
      }
    });
    return stats;
  }, [history]);
  const accountFilterSummary = useMemo(() => {
    const parts = [];
    if (String(accountQuery.status || '').trim()) {
      parts.push(`status=${String(accountQuery.status).trim()}`);
    }
    if (String(accountQuery.search || '').trim()) {
      parts.push(`search=${String(accountQuery.search).trim()}`);
    }
    return parts.join(' | ');
  }, [accountQuery.search, accountQuery.status]);
  const hasAccountFilters = useMemo(
    () => accountFilterSummary !== '',
    [accountFilterSummary],
  );
  const teamFilterSummary = useMemo(() => {
    const parts = [];
    if (String(teamQuery.status || '').trim()) {
      parts.push(`status=${String(teamQuery.status).trim()}`);
    }
    if (String(teamQuery.search || '').trim()) {
      parts.push(`search=${String(teamQuery.search).trim()}`);
    }
    return parts.join(' | ');
  }, [teamQuery.search, teamQuery.status]);
  const hasTeamFilters = useMemo(
    () => teamFilterSummary !== '',
    [teamFilterSummary],
  );
  const historyFilterSummary = useMemo(() => {
    const parts = [];
    if (String(historyQuery.status || '').trim()) {
      parts.push(`status=${String(historyQuery.status).trim()}`);
    }
    if (String(historyQuery.module || '').trim()) {
      parts.push(`module=${String(historyQuery.module).trim()}`);
    }
    if (String(historyQuery.search || '').trim()) {
      parts.push(`search=${String(historyQuery.search).trim()}`);
    }
    if (historyQuery.only_failed === true) {
      parts.push('only_failed=true');
    }
    if (historyQuery.only_proxy_retried === true) {
      parts.push('only_proxy_retried=true');
    }
    return parts.join(' | ');
  }, [
    historyQuery.module,
    historyQuery.only_failed,
    historyQuery.only_proxy_retried,
    historyQuery.search,
    historyQuery.status,
  ]);
  const hasHistoryFilters = useMemo(
    () => historyFilterSummary !== '',
    [historyFilterSummary],
  );
  const hasAccountBatchUpdateFields = useMemo(
    () =>
      String(batchAccountStatus || '').trim() !== '' ||
      String(batchAccountRedeemSite || '').trim() !== '' ||
      String(batchAccountMetadata || '').trim() !== '',
    [batchAccountMetadata, batchAccountRedeemSite, batchAccountStatus],
  );
  const accountBatchUpdateSummary = useMemo(() => {
    const parts = [];
    if (String(batchAccountStatus || '').trim()) {
      parts.push(`status->${String(batchAccountStatus).trim()}`);
    }
    if (String(batchAccountRedeemSite || '').trim()) {
      parts.push(`redeem_site->${String(batchAccountRedeemSite).trim()}`);
    }
    if (String(batchAccountMetadata || '').trim()) {
      parts.push(`metadata+=${String(batchAccountMetadata).trim()}`);
    }
    return parts.join(' | ');
  }, [batchAccountMetadata, batchAccountRedeemSite, batchAccountStatus]);
  const hasTeamBatchUpdateFields = useMemo(
    () =>
      String(batchTeamStatus || '').trim() !== '' ||
      String(batchTeamMetadata || '').trim() !== '',
    [batchTeamMetadata, batchTeamStatus],
  );
  const teamBatchUpdateSummary = useMemo(() => {
    const parts = [];
    if (String(batchTeamStatus || '').trim()) {
      parts.push(`status->${String(batchTeamStatus).trim()}`);
    }
    if (String(batchTeamMetadata || '').trim()) {
      parts.push(`metadata+=${String(batchTeamMetadata).trim()}`);
    }
    return parts.join(' | ');
  }, [batchTeamMetadata, batchTeamStatus]);
  const proxyPoolSummary = useMemo(() => {
    if (String(meta?.mode || '').trim() !== 'local') {
      return null;
    }
    return meta?.proxy_pool_summary || null;
  }, [meta]);
  const proxyPoolRows = useMemo(
    () =>
      Array.isArray(proxyPoolSummary?.items) ? proxyPoolSummary.items : [],
    [proxyPoolSummary],
  );
  const invalidProxyRows = useMemo(
    () => proxyPoolRows.filter((item) => item?.valid !== true),
    [proxyPoolRows],
  );
  const vmessProxyRows = useMemo(
    () => proxyPoolRows.filter((item) => String(item?.mode || '').trim() === 'vmess'),
    [proxyPoolRows],
  );
  const proxySummaryCards = useMemo(() => {
    if (!proxyPoolSummary) {
      return [];
    }
    return [
      {
        label: t('代理总数'),
        value: Number(proxyPoolSummary?.total_entries) || 0,
      },
      {
        label: t('可用'),
        value: Number(proxyPoolSummary?.valid_entries) || 0,
      },
      {
        label: t('无效'),
        value: Number(proxyPoolSummary?.invalid_entries) || 0,
      },
      {
        label: t('vmess'),
        value: Number(proxyPoolSummary?.vmess_entries) || 0,
      },
    ];
  }, [proxyPoolSummary, t]);
  const proxyStatusBanner = useMemo(() => {
    if (!proxyPoolSummary) {
      return null;
    }
    const warning = String(proxyPoolSummary?.warning || '').trim();
    if (warning) {
      return {
        type: 'warning',
        text: warning,
      };
    }
    if (Number(proxyPoolSummary?.invalid_entries) > 0) {
      return {
        type: 'warning',
        text: t('检测到无效代理条目，建议先进入设置页清洗后再启动任务。'),
      };
    }
    if (Number(proxyPoolSummary?.vmess_entries) > 0) {
      return {
        type: proxyPoolSummary?.vmess_ready === true ? 'success' : 'danger',
        text:
          proxyPoolSummary?.vmess_ready === true
            ? t('vmess 节点已就绪，可直接参与本机轮询。')
            : t('存在 vmess 节点，但当前缺少 xray 或可用 gateway。'),
      };
    }
    return {
      type: 'success',
      text: t('当前代理池状态正常，可直接用于本机自动化流程。'),
    };
  }, [proxyPoolSummary, t]);
  const proxyRuntimeTags = useMemo(() => {
    if (!proxyPoolSummary) {
      return [];
    }
    return [
      proxyPoolSummary?.xray_available === true
        ? `${t('xray')}: ${String(proxyPoolSummary?.xray_path || 'xray').trim()}`
        : t('xray 未检测到'),
      proxyPoolSummary?.gateway_valid === true
        ? `${t('gateway')}: ${String(proxyPoolSummary?.gateway_label || proxyPoolSummary?.gateway_mode || '-').trim()}`
        : String(proxyPoolSummary?.gateway_error || '').trim()
          ? `${t('gateway 异常')}: ${String(proxyPoolSummary?.gateway_error || '').trim()}`
          : t('gateway 未配置'),
    ];
  }, [proxyPoolSummary, t]);
  const cpaEndpointMode = useMemo(
    () => resolveCPAEndpointMode(meta?.cpa_endpoint),
    [meta?.cpa_endpoint],
  );
  const cpaStatusSummary = useMemo(() => {
    const endpoint = String(meta?.cpa_endpoint || '').trim();
    if (!endpoint) {
      return {
        type: 'info',
        text: t('当前未配置 CPA Endpoint；导入模块默认只适合落到 NewAPI。'),
      };
    }
    if (meta?.cpa_configured === true) {
      return {
        type: 'success',
        text:
          cpaEndpointMode === 'cliproxy_management'
            ? t('CPA 已配置完成，当前端点识别为 CLIProxy Management / auth-files 模式。')
            : cpaEndpointMode === 'legacy_importer'
              ? t('CPA 已配置完成，当前端点识别为传统导入接口模式。')
              : t('CPA 已配置完成，当前端点可用于通用探测与导入。'),
      };
    }
    return {
      type: 'warning',
      text: t('已填写 CPA Endpoint，但尚未检测到已保存 Token；导入目标若包含 CPA，任务会被拦截。'),
    };
  }, [cpaEndpointMode, meta?.cpa_configured, meta?.cpa_endpoint, t]);
  const teamAutoSelectCandidates = useMemo(
    () => (Array.isArray(teamAutoSelect?.candidates) ? teamAutoSelect.candidates : []),
    [teamAutoSelect],
  );
  const excludedTeamPreview = useMemo(() => {
    if (!Array.isArray(teamAutoSelectExcludedIds) || teamAutoSelectExcludedIds.length === 0) {
      return [];
    }
    const lookup = new Map();
    [teamAutoSelect?.selected]
      .concat(teamAutoSelectCandidates)
      .concat(teams)
      .forEach((item) => {
        const id = Number(item?.id) || 0;
        if (id > 0 && !lookup.has(id)) {
          lookup.set(id, item);
        }
      });
    return teamAutoSelectExcludedIds.map((id) => {
      const numericID = Number(id) || 0;
      return lookup.get(numericID) || { id: numericID };
    });
  }, [teamAutoSelect?.selected, teamAutoSelectCandidates, teamAutoSelectExcludedIds, teams]);
  const historyFailureReasonCards = useMemo(() => {
    const buckets = new Map();
    history.forEach((item) => {
      const raw = extractFailureReasonFingerprint(
        buildExecutionHistoryFailureSummary(item),
      );
      if (!raw) {
        return;
      }
      const term = shortenText(raw, 120);
      const key = term.toLowerCase();
      if (!buckets.has(key)) {
        buckets.set(key, {
          term,
          label: shortenText(term, 40),
          count: 0,
        });
      }
      buckets.get(key).count += 1;
    });
    return Array.from(buckets.values())
      .sort((a, b) => b.count - a.count || a.term.length - b.term.length)
      .slice(0, 8);
  }, [history]);

  const selectedExecutionStatus = String(executionDetail?.status || '').trim();
  const canStopSelectedExecution = ['queued', 'running', 'stopping'].includes(
    selectedExecutionStatus,
  );

  const moduleOptions = useMemo(
    () =>
      [{ label: t('全部模块流水线'), value: '' }].concat(
        modules.map((item) => ({ label: item.name, value: item.id })),
      ),
    [modules, t],
  );
  const currentModuleFieldKeys = useMemo(
    () =>
      new Set(
        Array.isArray(currentModule?.fields)
          ? currentModule.fields
              .map((field) => String(field?.key || '').trim())
              .filter(Boolean)
          : [],
      ),
    [currentModule],
  );
  const historyModuleOptions = useMemo(
    () =>
      [{ label: t('全部模块'), value: '' }].concat(
        modules.map((item) => ({ label: item.name, value: item.id })),
      ),
    [modules, t],
  );
  const hasProxyControls = useMemo(
    () => PROXY_FIELD_KEYS.some((key) => currentModuleFieldKeys.has(key)),
    [currentModuleFieldKeys],
  );
  const visibleProxyPresets = useMemo(
    () =>
      PROXY_PRESET_DEFINITIONS.filter((preset) =>
        Object.keys(preset.values).some((key) => currentModuleFieldKeys.has(key)),
      ),
    [currentModuleFieldKeys],
  );
  const accountByStatus = accountStats?.by_status || {};
  const teamByStatus = teamStats?.by_status || {};

  const connectorState = useMemo(() => {
    const mode = String(meta?.mode || 'local');
    const enabled = meta?.enabled !== false;
    const configured = meta?.configured !== false;
    const reachable = meta?.reachable !== false;
    const warning = String(meta?.warning || '').trim();

    if (!enabled || mode === 'disabled') {
      return {
        type: 'warning',
        text:
          warning ||
          t('Automation 已禁用，可在设置中启用 local 或 remote 模式。'),
      };
    }
    if (!configured) {
      return {
        type: 'warning',
        text: warning || t('Automation 未完成配置，请先配置后再执行任务。'),
      };
    }
    if (!reachable) {
      return {
        type: 'danger',
        text: warning || t('Automation 当前不可达，请检查连接参数。'),
      };
    }
    return {
      type: 'info',
      text:
        mode === 'remote'
          ? t('当前为 remote 兼容模式。')
          : t('当前为 local 本机执行模式。'),
    };
  }, [meta, t]);

  const isRunReady = useMemo(() => {
    if (meta?.enabled === false || String(meta?.mode || '') === 'disabled') {
      return false;
    }
    if (meta?.configured === false) {
      return false;
    }
    if (meta?.reachable === false) {
      return false;
    }
    return true;
  }, [meta]);
  const runFilterSummary = useMemo(() => {
    const parts = [];
    if (String(runFilterStatus || '').trim()) {
      parts.push(`status=${String(runFilterStatus).trim()}`);
    }
    if (String(runFilterSearch || '').trim()) {
      parts.push(`search=${String(runFilterSearch).trim()}`);
    }
    return parts.join(' | ');
  }, [runFilterSearch, runFilterStatus]);
  const runTargetEstimate = useMemo(() => {
    const nextStatus = String(runFilterStatus || '').trim();
    const nextSearch = String(runFilterSearch || '').trim();
    const accountQueryStatus = String(accountQuery.status || '').trim();
    const accountQuerySearch = String(accountQuery.search || '').trim();
    if (!nextStatus && !nextSearch) {
      return Number(accountStats?.total) || accountsTotal;
    }
    if (nextStatus && !nextSearch) {
      return Number(accountByStatus?.[nextStatus]) || 0;
    }
    if (nextStatus === accountQueryStatus && nextSearch === accountQuerySearch) {
      return accountsTotal;
    }
    return null;
  }, [
    accountByStatus,
    accountQuery.search,
    accountQuery.status,
    accountStats?.total,
    accountsTotal,
    runFilterSearch,
    runFilterStatus,
  ]);
  const currentImportTarget = useMemo(() => {
    if (String(currentModule?.id || '').trim() !== 'import_target') {
      return '';
    }
    return String(moduleFieldValues?.target || 'newapi').trim() || 'newapi';
  }, [currentModule?.id, moduleFieldValues]);
  const runBlockedReason = useMemo(() => {
    if (
      String(currentModule?.id || '').trim() === 'import_target' &&
      ['cpa', 'both'].includes(currentImportTarget) &&
      meta?.cpa_configured !== true
    ) {
      return t('当前导入目标包含 CPA，但设置页尚未完成 CPA Endpoint / Token 配置。');
    }
    return '';
  }, [currentImportTarget, currentModule?.id, meta?.cpa_configured, t]);
  const canStartRun = useMemo(
    () => isRunReady && !runBlockedReason,
    [isRunReady, runBlockedReason],
  );

  const fetchRunDetail = async (executionId) => {
    if (!executionId) {
      setExecutionDetail(null);
      setExecutionLogs([]);
      setExecutionLogTotal(0);
      setExecutionLogPage(1);
      return;
    }
    try {
      setExecutionLogLoading(true);
      const [detail, logs] = await Promise.all([
        getTeamAutomationRun(executionId),
        getTeamAutomationRunLogs(executionId, {
          page: 1,
          page_size: EXECUTION_LOG_PAGE_SIZE,
        }),
      ]);
      setExecutionDetail(detail || null);
      setExecutionLogs(Array.isArray(logs?.items) ? logs.items : []);
      setExecutionLogTotal(Number(logs?.total) || 0);
      setExecutionLogPage(Number(logs?.page) || 1);
    } catch (error) {
      showError(error?.message || t('加载执行详情失败'));
    } finally {
      setExecutionLogLoading(false);
    }
  };

  const loadMoreExecutionLogs = async (loadAll = false) => {
    if (!selectedExecutionId || executionLogLoading || executionLogLoadingMore) {
      return;
    }
    if (executionLogs.length >= executionLogTotal && executionLogTotal > 0) {
      return;
    }
    const currentPage = Math.max(1, Number(executionLogPage) || 1);
    const currentTotal = Math.max(0, Number(executionLogTotal) || 0);
    const startPage = currentPage + 1;
    const loadingStateSetter = loadAll ? setExecutionLogLoading : setExecutionLogLoadingMore;
    loadingStateSetter(true);
    try {
      let nextPage = startPage;
      let merged = [...executionLogs];
      let latestTotal = currentTotal;
      while (true) {
        const logs = await getTeamAutomationRunLogs(selectedExecutionId, {
          page: nextPage,
          page_size: EXECUTION_LOG_PAGE_SIZE,
        });
        const items = Array.isArray(logs?.items) ? logs.items : [];
        latestTotal = Number(logs?.total) || latestTotal;
        merged = merged.concat(items);
        setExecutionLogs(merged);
        setExecutionLogTotal(latestTotal);
        setExecutionLogPage(Number(logs?.page) || nextPage);
        if (!loadAll || items.length === 0 || merged.length >= latestTotal) {
          break;
        }
        nextPage += 1;
      }
    } catch (error) {
      showError(error?.message || t('加载更多日志失败'));
    } finally {
      loadingStateSetter(false);
    }
  };

  const fetchAccounts = async (nextQuery = accountQuery) => {
    const data = await listTeamAutomationAccounts(nextQuery);
    const rows = Array.isArray(data?.items)
      ? data.items
      : Array.isArray(data?.accounts)
        ? data.accounts
        : [];
    setAccounts(rows);
    setAccountsTotal(Number(data?.total) || 0);
    setSelectedAccountRowKeys((prev) =>
      prev.filter((key) =>
        rows.some((item) => String(item?.id) === String(key)),
      ),
    );
  };

  const fetchTeams = async (nextQuery = teamQuery) => {
    const data = await listTeamAutomationTeams(nextQuery);
    const rows = Array.isArray(data?.items)
      ? data.items
      : Array.isArray(data?.teams)
        ? data.teams
        : [];
    setTeams(rows);
    setTeamsTotal(Number(data?.total) || 0);
    setSelectedTeamRowKeys((prev) =>
      prev.filter((key) =>
        rows.some((item) => String(item?.id) === String(key)),
      ),
    );
  };

  const applyAccountQuickStatusFilter = async (status) => {
    const nextStatus = String(status || '').trim();
    const next = {
      ...accountQuery,
      page: 1,
      status: nextStatus,
    };
    setAccountDraft((prev) => ({ ...prev, status: nextStatus }));
    setAccountQuery(next);
    await fetchAccounts(next);
  };

  const applyTeamQuickStatusFilter = async (status) => {
    const nextStatus = String(status || '').trim();
    const next = {
      ...teamQuery,
      page: 1,
      status: nextStatus,
    };
    setTeamDraft((prev) => ({ ...prev, status: nextStatus }));
    setTeamQuery(next);
    await fetchTeams(next);
  };

  const fetchHistory = async (nextQuery = historyQuery) => {
    const data = await getTeamAutomationRunHistory(nextQuery);
    const rows = Array.isArray(data?.items) ? data.items : [];
    setHistory(rows);
    setHistoryTotal(Number(data?.total) || 0);
    setSelectedHistoryRowKeys((prev) =>
      prev.filter((key) =>
        rows.some((item) => String(item?.execution_id || '') === String(key)),
      ),
    );
  };

  const applyHistoryQuery = async (patch = {}) => {
    const next = {
      ...historyQuery,
      ...patch,
    };
    if (
      Object.prototype.hasOwnProperty.call(patch, 'status') ||
      Object.prototype.hasOwnProperty.call(patch, 'module') ||
      Object.prototype.hasOwnProperty.call(patch, 'search') ||
      Object.prototype.hasOwnProperty.call(patch, 'only_failed') ||
      Object.prototype.hasOwnProperty.call(patch, 'only_proxy_retried')
    ) {
      next.page = 1;
    }
    setHistoryQuery(next);
    await fetchHistory(next);
  };

  const resetHistoryQuery = async () => {
    const next = {
      ...HISTORY_QUERY_DEFAULTS,
      page_size: historyQuery.page_size,
    };
    setHistoryQuery(next);
    await fetchHistory(next);
  };

  const applyProxyPreset = (preset) => {
    if (!preset) {
      return;
    }
    setModuleFieldValues((prev) => {
      const next = { ...prev };
      Object.entries(preset.values || {}).forEach(([key, value]) => {
        if (currentModuleFieldKeys.has(key)) {
          next[key] = value;
        }
      });
      return next;
    });
    showSuccess(
      t('已应用代理模板：{{name}}', {
        name: preset.label,
      }),
    );
  };

  const handleDeletedHistoryResult = async (result, fallbackMessage) => {
    const deletedIDs = Array.isArray(result?.deleted_execution_ids)
      ? result.deleted_execution_ids.map((item) => String(item || '').trim()).filter(Boolean)
      : [];
    const deletedCount = Number(result?.deleted_count) || 0;
    const activeCount = Number(result?.skipped_active_count) || 0;
    const missingCount = Number(result?.skipped_missing_count) || 0;
    const deletedArtifactFiles = Number(result?.deleted_artifact_file_count) || 0;
    const errorCount = Number(result?.artifact_delete_error_count) || 0;
    showSuccess(
      t(fallbackMessage || '执行历史清理完成：删除 {{deleted}}，跳过活动 {{active}}，缺失 {{missing}}，清理产物文件 {{artifacts}}', {
        deleted: deletedCount,
        active: activeCount,
        missing: missingCount,
        artifacts: deletedArtifactFiles,
      }),
    );
    if (errorCount > 0) {
      showError(
        t('部分产物文件清理失败，共 {{count}} 条，请查看返回详情', {
          count: errorCount,
        }),
      );
    }
    setSelectedHistoryRowKeys((prev) =>
      prev.filter((key) => !deletedIDs.includes(String(key))),
    );
    if (
      selectedExecutionId &&
      deletedIDs.includes(String(selectedExecutionId || '').trim())
    ) {
      setSelectedExecutionId('');
      setExecutionDetail(null);
      setExecutionLogs([]);
    }
    await fetchHistory(historyQuery);
  };

  const onBatchDeleteHistorySelected = () => {
    if (selectedHistoryRowKeys.length === 0) {
      showError(t('请先选择执行记录'));
      return;
    }
    Modal.confirm({
      title: t('批量删除执行历史'),
      content: t(
        '确认删除已选中的 {{count}} 条执行记录？会同时清理关联日志、产物记录，并尝试删除 artifact_root 下的产物文件。',
        {
          count: selectedHistoryRowKeys.length,
        },
      ),
      okType: 'danger',
      onOk: async () => {
        try {
          const result = await batchDeleteTeamAutomationRuns({
            execution_ids: selectedHistoryRowKeys.map((item) => String(item)),
            delete_artifact_files: true,
          });
          await handleDeletedHistoryResult(result);
        } catch (error) {
          showError(error?.message || t('批量删除执行历史失败'));
        }
      },
    });
  };

  const onBatchDeleteHistoryByFilter = () => {
    if (historyTotal <= 0) {
      showError(t('当前没有可清理的执行记录'));
      return;
    }
    Modal.confirm({
      title: t('清理当前筛选结果'),
      content: t(
        '确认按当前筛选条件清理 {{count}} 条执行记录？会跳过正在运行中的任务，并同时清理关联日志与产物文件。',
        {
          count: historyTotal,
        },
      ),
      okType: 'danger',
      onOk: async () => {
        try {
          const result = await batchDeleteTeamAutomationRuns({
            status: historyQuery.status,
            module: historyQuery.module,
            search: historyQuery.search,
            only_failed: historyQuery.only_failed === true,
            only_proxy_retried: historyQuery.only_proxy_retried === true,
            delete_artifact_files: true,
          });
          await handleDeletedHistoryResult(result);
        } catch (error) {
          showError(error?.message || t('按筛选条件清理执行历史失败'));
        }
      },
    });
  };

  const fetchAll = async ({ silent = false } = {}) => {
    if (silent) {
      setRefreshing(true);
    } else {
      setLoading(true);
    }
    try {
      const [metaPayload, modulePayload] = await Promise.all([
        getTeamAutomationMeta(),
        getTeamAutomationModules(),
      ]);
      const moduleList = normalizeModules(modulePayload);
      setMeta(metaPayload || {});
      setModules(moduleList);

      if (
        metaPayload?.enabled === false ||
        String(metaPayload?.mode || '') === 'disabled' ||
        metaPayload?.configured === false ||
        metaPayload?.reachable === false
      ) {
        setAccountStats(null);
        setTeamStats(null);
        setAccounts([]);
        setAccountsTotal(0);
        setTeams([]);
        setTeamsTotal(0);
        setHistory([]);
        setHistoryTotal(0);
        setSelectedExecutionId('');
        setExecutionDetail(null);
        setExecutionLogs([]);
        setTeamAutoSelect(null);
        setTeamAutoSelectExcludedIds([]);
        return;
      }

      const [statsPayload, teamStatsPayload] = await Promise.all([
        getTeamAutomationAccountStats(),
        getTeamAutomationTeamStats(),
        fetchAccounts(accountQuery),
        fetchTeams(teamQuery),
        fetchHistory(historyQuery),
      ]);
      setAccountStats(statsPayload || null);
      setTeamStats(teamStatsPayload || null);

      if (selectedExecutionId) {
        await fetchRunDetail(selectedExecutionId);
      }
    } catch (error) {
      showError(error?.message || t('加载 Team Automation 页面失败'));
    } finally {
      if (silent) {
        setRefreshing(false);
      } else {
        setLoading(false);
      }
    }
  };

  useEffect(() => {
    fetchAll({ silent: false });
  }, []);

  useEffect(() => {
    if (!currentModule) {
      setModuleFieldValues({});
      return;
    }
    const defaults = {};
    currentModule.fields.forEach((field) => {
      const key = String(field?.key || '').trim();
      if (!key) {
        return;
      }
      if (field.default !== undefined) {
        defaults[key] = field.default;
      } else if (field.type === 'boolean') {
        defaults[key] = false;
      } else {
        defaults[key] = '';
      }
    });
    setModuleFieldValues(defaults);
  }, [currentModule?.id]);

  useEffect(() => {
    if (!selectedExecutionId) {
      return;
    }
    fetchRunDetail(selectedExecutionId);
  }, [selectedExecutionId]);

  useEffect(() => {
    setExecutionLogLevelFilter('');
    setExecutionLogSearch('');
    setExecutionLogOnlyProxy(false);
  }, [selectedExecutionId]);

  const onStartRun = async () => {
    try {
      if (!canStartRun) {
        showError(
          runBlockedReason || t('Automation 当前不可执行，请先检查配置状态'),
        );
        return;
      }
      setRunSubmitting(true);
      const payload = {
        module: moduleId,
        account_status: runFilterStatus,
        search: runFilterSearch,
        run_options: cleanupRunOptions(moduleFieldValues),
      };
      const result = await startTeamAutomationRun(payload);
      showSuccess(t('任务已启动'));
      const nextId = String(result?.execution_id || result?.id || '').trim();
      if (nextId) {
        setSelectedExecutionId(nextId);
      }
      await Promise.all([
        fetchHistory(historyQuery),
        fetchAccounts(accountQuery),
      ]);
      if (nextId) {
        await fetchRunDetail(nextId);
      }
    } catch (error) {
      showError(error?.message || t('启动任务失败'));
    } finally {
      setRunSubmitting(false);
    }
  };

  const applyRunQuickFilter = (status) => {
    setRunFilterStatus(String(status || '').trim());
  };

  const syncRunFiltersFromAccountQuery = () => {
    setRunFilterStatus(String(accountQuery.status || '').trim());
    setRunFilterSearch(String(accountQuery.search || '').trim());
    showSuccess(t('运行筛选已同步为当前账号列表筛选'));
  };

  const clearRunFilters = () => {
    setRunFilterStatus('');
    setRunFilterSearch('');
  };

  const onStopRun = async () => {
    if (!selectedExecutionId) {
      return;
    }
    try {
      setStopSubmitting(true);
      await stopTeamAutomationRun(selectedExecutionId);
      showSuccess(t('停止请求已提交'));
      await Promise.all([
        fetchHistory(historyQuery),
        fetchRunDetail(selectedExecutionId),
      ]);
    } catch (error) {
      showError(error?.message || t('停止任务失败'));
    } finally {
      setStopSubmitting(false);
    }
  };

  const onImportAccounts = async (event) => {
    const file = event?.target?.files?.[0];
    event.target.value = '';
    if (!file) {
      return;
    }
    if (file.size > TEAM_AUTOMATION_IMPORT_MAX_BYTES) {
      showError(
        t('CSV 文件过大，最大 {{size}} MB', {
          size: Math.floor(TEAM_AUTOMATION_IMPORT_MAX_BYTES / 1024 / 1024),
        }),
      );
      return;
    }
    try {
      const formData = new FormData();
      formData.append('file', file);
      const payload = await importTeamAutomationAccountsCSV(formData);
      const created = Number(payload?.created) || 0;
      const updated = Number(payload?.updated) || 0;
      const failed = Number(payload?.failed) || 0;
      showSuccess(
        t('导入完成：新增 {{created}}，更新 {{updated}}，失败 {{failed}}', {
          created,
          updated,
          failed,
        }),
      );
      await Promise.all([
        fetchAccounts(accountQuery),
        getTeamAutomationAccountStats().then((v) => setAccountStats(v || null)),
      ]);
    } catch (error) {
      showError(error?.message || t('导入 CSV 失败'));
    }
  };

  const openRegisteredImportModal = () => {
    setRegisteredImportModal({
      visible: true,
      submitting: false,
      form: {
        ...DEFAULT_REGISTERED_IMPORT_FORM,
      },
    });
  };

  const openRedeemImportModal = () => {
    setRedeemImportModal({
      visible: true,
      submitting: false,
      form: {
        ...DEFAULT_REDEEM_IMPORT_FORM,
      },
    });
  };

  const onImportRegisteredAccounts = async () => {
    try {
      const raw = String(registeredImportModal.form.text || '').trim();
      if (!raw) {
        showError(t('请提供账号数据'));
        return;
      }
      setRegisteredImportModal((prev) => ({ ...prev, submitting: true }));
      const payload = {
        text: raw,
        start_from: 'team_redeem',
        default_redeem_site: String(
          registeredImportModal.form.default_redeem_site || '',
        ).trim(),
        continue_from_module: true,
        auto_run: false,
      };
      const result = await importRegisteredTeamAutomationAccounts(payload);
      const created = Number(result?.created) || 0;
      const updated = Number(result?.updated) || 0;
      const failed = Number(result?.failed) || 0;
      showSuccess(
        t('导入完成：新增 {{created}}，更新 {{updated}}，失败 {{failed}}', {
          created,
          updated,
          failed,
        }),
      );
      setRegisteredImportModal({
        visible: false,
        submitting: false,
        form: DEFAULT_REGISTERED_IMPORT_FORM,
      });
      await Promise.all([
        fetchAccounts(accountQuery),
        getTeamAutomationAccountStats().then((v) => setAccountStats(v || null)),
        fetchHistory(historyQuery),
      ]);
    } catch (error) {
      showError(error?.message || t('导入已注册账号失败'));
      setRegisteredImportModal((prev) => ({ ...prev, submitting: false }));
    }
  };

  const onImportRedeemCodes = async () => {
    try {
      const lines = String(redeemImportModal.form.lines || '').trim();
      if (!lines) {
        showError(t('请提供兑换码数据'));
        return;
      }
      setRedeemImportModal((prev) => ({ ...prev, submitting: true }));
      const payload = {
        lines,
        start_from: 'team_redeem',
        default_redeem_site: String(
          redeemImportModal.form.default_redeem_site || '',
        ).trim(),
        continue_from_module: true,
        auto_run: false,
      };
      const result = await importTeamAutomationRedeemCodes(payload);
      const created = Number(result?.created) || 0;
      const updated = Number(result?.updated) || 0;
      const failed = Number(result?.failed) || 0;
      showSuccess(
        t(
          '兑换码导入完成：新增 {{created}}，更新 {{updated}}，失败 {{failed}}',
          {
            created,
            updated,
            failed,
          },
        ),
      );
      setRedeemImportModal({
        visible: false,
        submitting: false,
        form: DEFAULT_REDEEM_IMPORT_FORM,
      });
      await Promise.all([
        fetchAccounts(accountQuery),
        getTeamAutomationAccountStats().then((v) => setAccountStats(v || null)),
        fetchHistory(historyQuery),
      ]);
    } catch (error) {
      showError(error?.message || t('导入兑换码失败'));
      setRedeemImportModal((prev) => ({ ...prev, submitting: false }));
    }
  };

  const onDeleteAccount = (record) => {
    Modal.confirm({
      title: t('删除账号'),
      content: t('确认删除 {{email}} ?', { email: record?.email || '-' }),
      okType: 'danger',
      onOk: async () => {
        try {
          await deleteTeamAutomationAccount(
            String(record?.id || record?.email || ''),
          );
          showSuccess(t('账号已删除'));
          await Promise.all([
            fetchAccounts(accountQuery),
            getTeamAutomationAccountStats().then((v) =>
              setAccountStats(v || null),
            ),
          ]);
        } catch (error) {
          showError(error?.message || t('删除账号失败'));
        }
      },
    });
  };

  const onBatchDeleteAccounts = () => {
    if (selectedAccountRowKeys.length === 0) {
      showError(t('请先选择账号'));
      return;
    }
    Modal.confirm({
      title: t('批量删除账号'),
      content: t('确认删除已选中的 {{count}} 个账号?', {
        count: selectedAccountRowKeys.length,
      }),
      okType: 'danger',
      onOk: async () => {
        try {
          const result = await batchDeleteTeamAutomationAccounts({
            ids: selectedAccountRowKeys
              .map((key) => Number(key))
              .filter(Boolean),
          });
          const deletedCount = Number(result?.deleted_count) || 0;
          const failedCount = Number(result?.failed_count) || 0;
          showSuccess(
            t('批量删除完成：成功 {{deleted}}，失败 {{failed}}', {
              deleted: deletedCount,
              failed: failedCount,
            }),
          );
          setSelectedAccountRowKeys([]);
          await Promise.all([
            fetchAccounts(accountQuery),
            getTeamAutomationAccountStats().then((v) =>
              setAccountStats(v || null),
            ),
          ]);
        } catch (error) {
          showError(error?.message || t('批量删除账号失败'));
        }
      },
    });
  };

  const onBatchDeleteAccountsByFilter = () => {
    const matchedCount = Number(accountsTotal) || 0;
    if (matchedCount <= 0) {
      showError(t('当前没有可清理的账号'));
      return;
    }
    const hasFilters =
      String(accountQuery.status || '').trim() !== '' ||
      String(accountQuery.search || '').trim() !== '';
    Modal.confirm({
      title: t('清理当前账号筛选'),
      content: hasFilters
        ? t('确认删除当前筛选命中的 {{count}} 个账号?', {
            count: matchedCount,
          })
        : t('当前未设置筛选条件，确认删除全部 {{count}} 个账号?', {
            count: matchedCount,
          }),
      okType: 'danger',
      onOk: async () => {
        try {
          const result = await batchDeleteTeamAutomationAccounts({
            status: accountQuery.status,
            search: accountQuery.search,
            all: !hasFilters,
          });
          const deletedCount = Number(result?.deleted_count) || 0;
          const failedCount = Number(result?.failed_count) || 0;
          showSuccess(
            t('按筛选清理完成：成功 {{deleted}}，失败 {{failed}}', {
              deleted: deletedCount,
              failed: failedCount,
            }),
          );
          setSelectedAccountRowKeys([]);
          const nextQuery = { ...accountQuery, page: 1 };
          setAccountQuery(nextQuery);
          await Promise.all([
            fetchAccounts(nextQuery),
            getTeamAutomationAccountStats().then((v) =>
              setAccountStats(v || null),
            ),
          ]);
        } catch (error) {
          showError(error?.message || t('按筛选清理账号失败'));
        }
      },
    });
  };

  const onBatchUpdateAccounts = () => {
    if (selectedAccountRowKeys.length === 0) {
      showError(t('请先选择账号'));
      return;
    }
    const payload = {
      ids: selectedAccountRowKeys.map((key) => Number(key)).filter(Boolean),
    };
    const nextStatus = String(batchAccountStatus || '').trim();
    const nextRedeemSite = String(batchAccountRedeemSite || '').trim();
    const nextMetadata = String(batchAccountMetadata || '').trim();
    if (!hasAccountBatchUpdateFields) {
      showError(t('请先设置批量更新字段'));
      return;
    }
    if (nextStatus) {
      payload.status = nextStatus;
    }
    if (nextRedeemSite) {
      payload.redeem_site = nextRedeemSite;
    }
    if (nextMetadata) {
      payload.metadata = nextMetadata;
    }
    Modal.confirm({
      title: t('批量更新账号'),
      content: t('确认更新已选中的 {{count}} 个账号?', {
        count: selectedAccountRowKeys.length,
      }),
      onOk: async () => {
        try {
          const result = await batchUpdateTeamAutomationAccounts(payload);
          const updatedCount = Number(result?.updated_count) || 0;
          const failedCount = Number(result?.failed_count) || 0;
          showSuccess(
            t('批量更新完成：成功 {{updated}}，失败 {{failed}}', {
              updated: updatedCount,
              failed: failedCount,
            }),
          );
          await Promise.all([
            fetchAccounts(accountQuery),
            getTeamAutomationAccountStats().then((v) =>
              setAccountStats(v || null),
            ),
          ]);
        } catch (error) {
          showError(error?.message || t('批量更新账号失败'));
        }
      },
    });
  };

  const onBatchUpdateAccountsByFilter = () => {
    const matchedCount = Number(accountsTotal) || 0;
    if (matchedCount <= 0) {
      showError(t('当前没有可更新的账号'));
      return;
    }
    if (!hasAccountBatchUpdateFields) {
      showError(t('请先设置批量更新字段'));
      return;
    }
    const hasFilters =
      String(accountQuery.status || '').trim() !== '' ||
      String(accountQuery.search || '').trim() !== '';
    const payload = {
      filter_status: accountQuery.status,
      search: accountQuery.search,
      all: !hasFilters,
    };
    const nextStatus = String(batchAccountStatus || '').trim();
    const nextRedeemSite = String(batchAccountRedeemSite || '').trim();
    const nextMetadata = String(batchAccountMetadata || '').trim();
    if (nextStatus) {
      payload.status = nextStatus;
    }
    if (nextRedeemSite) {
      payload.redeem_site = nextRedeemSite;
    }
    if (nextMetadata) {
      payload.metadata = nextMetadata;
    }
    Modal.confirm({
      title: t('按筛选批量更新账号'),
      content: hasFilters
        ? t('确认将当前筛选命中的 {{count}} 个账号应用待更新字段？', {
            count: matchedCount,
          })
        : t('当前未设置筛选条件，确认将全部 {{count}} 个账号应用待更新字段？', {
            count: matchedCount,
          }),
      onOk: async () => {
        try {
          const result = await batchUpdateTeamAutomationAccounts(payload);
          const updatedCount = Number(result?.updated_count) || 0;
          const failedCount = Number(result?.failed_count) || 0;
          showSuccess(
            t('按筛选批量更新完成：成功 {{updated}}，失败 {{failed}}', {
              updated: updatedCount,
              failed: failedCount,
            }),
          );
          setSelectedAccountRowKeys([]);
          const nextQuery = { ...accountQuery, page: 1 };
          setAccountQuery(nextQuery);
          await Promise.all([
            fetchAccounts(nextQuery),
            getTeamAutomationAccountStats().then((v) =>
              setAccountStats(v || null),
            ),
          ]);
        } catch (error) {
          showError(error?.message || t('按筛选批量更新账号失败'));
        }
      },
    });
  };

  const openCreateAccount = () => {
    setAccountModal({
      visible: true,
      mode: 'create',
      submitting: false,
      form: { ...DEFAULT_ACCOUNT_FORM },
    });
  };

  const openEditAccount = (record) => {
    setAccountModal({
      visible: true,
      mode: 'edit',
      submitting: false,
      form: {
        ...DEFAULT_ACCOUNT_FORM,
        id: Number(record?.id) || 0,
        name: record?.name || '',
        email: record?.email || '',
        redeem_code: record?.redeem_code || '',
        redeem_type: record?.redeem_type || '',
        status: record?.status || 'pending',
        redeem_site: record?.redeem_site || '',
        mailbox_id: Number(record?.mailbox_id) || 0,
        metadata: record?.metadata || '',
      },
    });
  };

  const saveAccount = async () => {
    const form = accountModal.form;
    const email = String(form?.email || '')
      .trim()
      .toLowerCase();
    if (!email) {
      showError(t('邮箱不能为空'));
      return;
    }
    const payload = {
      name: String(form?.name || '').trim() || email,
      email,
      password: String(form?.password || '').trim(),
      redeem_code: String(form?.redeem_code || '').trim(),
      redeem_type: String(form?.redeem_type || '').trim(),
      status: String(form?.status || '').trim(),
      redeem_site: String(form?.redeem_site || '').trim(),
      mailbox_id: Number(form?.mailbox_id) || 0,
      metadata: String(form?.metadata || '').trim(),
    };
    try {
      setAccountModal((prev) => ({ ...prev, submitting: true }));
      if (accountModal.mode === 'create') {
        await createTeamAutomationAccount(payload);
        showSuccess(t('账号已创建'));
      } else {
        await updateTeamAutomationAccount(String(form?.id || email), payload);
        showSuccess(t('账号已更新'));
      }
      setAccountModal({
        visible: false,
        mode: 'create',
        submitting: false,
        form: DEFAULT_ACCOUNT_FORM,
      });
      await Promise.all([
        fetchAccounts(accountQuery),
        getTeamAutomationAccountStats().then((v) => setAccountStats(v || null)),
      ]);
    } catch (error) {
      showError(error?.message || t('保存账号失败'));
      setAccountModal((prev) => ({ ...prev, submitting: false }));
    }
  };

  const openCreateTeam = () => {
    setTeamModal({
      visible: true,
      mode: 'create',
      form: { ...DEFAULT_TEAM_FORM },
      submitting: false,
    });
  };

  const openEditTeam = (record) => {
    setTeamModal({
      visible: true,
      mode: 'edit',
      form: {
        ...DEFAULT_TEAM_FORM,
        id: Number(record?.id) || 0,
        email: record?.email || '',
        client_id: record?.client_id || '',
        account_id: record?.account_id || '',
        team_name: record?.team_name || '',
        plan_type: record?.plan_type || '',
        subscription_plan: record?.subscription_plan || '',
        expires_at: Number(record?.expires_at) || 0,
        current_members: Number(record?.current_members) || 0,
        pending_invites: Number(record?.pending_invites) || 0,
        max_members: Number(record?.max_members) || 6,
        status: record?.status || 'active',
        account_role: record?.account_role || '',
        device_code_auth_enabled: record?.device_code_auth_enabled === true,
        error_count: Number(record?.error_count) || 0,
        last_sync_time: Number(record?.last_sync_time) || 0,
        metadata: record?.metadata || '',
      },
      submitting: false,
    });
  };

  const onDeleteTeam = (record) => {
    Modal.confirm({
      title: t('删除 Team 资产'),
      content: t('确认删除 {{email}} ?', { email: record?.email || '-' }),
      okType: 'danger',
      onOk: async () => {
        try {
          await deleteTeamAutomationTeam(
            String(record?.id || record?.email || ''),
          );
          showSuccess(t('Team 资产已删除'));
          await Promise.all([
            fetchTeams(teamQuery),
            getTeamAutomationTeamStats().then((v) => setTeamStats(v || null)),
          ]);
        } catch (error) {
          showError(error?.message || t('删除 Team 资产失败'));
        }
      },
    });
  };

  const onBatchDeleteTeams = () => {
    if (selectedTeamRowKeys.length === 0) {
      showError(t('请先选择 Team'));
      return;
    }
    Modal.confirm({
      title: t('批量删除 Team 资产'),
      content: t('确认删除已选中的 {{count}} 个 Team 资产?', {
        count: selectedTeamRowKeys.length,
      }),
      okType: 'danger',
      onOk: async () => {
        try {
          const result = await batchDeleteTeamAutomationTeams({
            ids: selectedTeamRowKeys.map((key) => Number(key)).filter(Boolean),
          });
          const deletedCount = Number(result?.deleted_count) || 0;
          const failedCount = Number(result?.failed_count) || 0;
          showSuccess(
            t('批量删除完成：成功 {{deleted}}，失败 {{failed}}', {
              deleted: deletedCount,
              failed: failedCount,
            }),
          );
          setSelectedTeamRowKeys([]);
          await Promise.all([
            fetchTeams(teamQuery),
            getTeamAutomationTeamStats().then((v) => setTeamStats(v || null)),
          ]);
        } catch (error) {
          showError(error?.message || t('批量删除 Team 资产失败'));
        }
      },
    });
  };

  const onBatchDeleteTeamsByFilter = () => {
    const matchedCount = Number(teamsTotal) || 0;
    if (matchedCount <= 0) {
      showError(t('当前没有可清理的 Team 资产'));
      return;
    }
    const hasFilters =
      String(teamQuery.status || '').trim() !== '' ||
      String(teamQuery.search || '').trim() !== '';
    Modal.confirm({
      title: t('清理当前 Team 筛选'),
      content: hasFilters
        ? t('确认删除当前筛选命中的 {{count}} 个 Team 资产?', {
            count: matchedCount,
          })
        : t('当前未设置筛选条件，确认删除全部 {{count}} 个 Team 资产?', {
            count: matchedCount,
          }),
      okType: 'danger',
      onOk: async () => {
        try {
          const result = await batchDeleteTeamAutomationTeams({
            status: teamQuery.status,
            search: teamQuery.search,
            all: !hasFilters,
          });
          const deletedCount = Number(result?.deleted_count) || 0;
          const failedCount = Number(result?.failed_count) || 0;
          showSuccess(
            t('按筛选清理完成：成功 {{deleted}}，失败 {{failed}}', {
              deleted: deletedCount,
              failed: failedCount,
            }),
          );
          setSelectedTeamRowKeys([]);
          const nextQuery = { ...teamQuery, page: 1 };
          setTeamQuery(nextQuery);
          await Promise.all([
            fetchTeams(nextQuery),
            getTeamAutomationTeamStats().then((v) => setTeamStats(v || null)),
          ]);
        } catch (error) {
          showError(error?.message || t('按筛选清理 Team 资产失败'));
        }
      },
    });
  };

  const onBatchUpdateTeams = () => {
    if (selectedTeamRowKeys.length === 0) {
      showError(t('请先选择 Team'));
      return;
    }
    const nextStatus = String(batchTeamStatus || '').trim();
    const nextMetadata = String(batchTeamMetadata || '').trim();
    if (!hasTeamBatchUpdateFields) {
      showError(t('请先设置 Team 批量更新字段'));
      return;
    }
    Modal.confirm({
      title: t('批量更新 Team 资产'),
      content: t('确认更新已选中的 {{count}} 个 Team 资产?', {
        count: selectedTeamRowKeys.length,
      }),
      onOk: async () => {
        try {
          const payload = {
            ids: selectedTeamRowKeys.map((key) => Number(key)).filter(Boolean),
          };
          if (nextStatus) {
            payload.status = nextStatus;
          }
          if (nextMetadata) {
            payload.metadata = nextMetadata;
          }
          const result = await batchUpdateTeamAutomationTeams(payload);
          const updatedCount = Number(result?.updated_count) || 0;
          const failedCount = Number(result?.failed_count) || 0;
          showSuccess(
            t('批量更新完成：成功 {{updated}}，失败 {{failed}}', {
              updated: updatedCount,
              failed: failedCount,
            }),
          );
          await Promise.all([
            fetchTeams(teamQuery),
            getTeamAutomationTeamStats().then((v) => setTeamStats(v || null)),
          ]);
        } catch (error) {
          showError(error?.message || t('批量更新 Team 资产失败'));
        }
      },
    });
  };

  const onBatchUpdateTeamsByFilter = () => {
    const matchedCount = Number(teamsTotal) || 0;
    if (matchedCount <= 0) {
      showError(t('当前没有可更新的 Team 资产'));
      return;
    }
    if (!hasTeamBatchUpdateFields) {
      showError(t('请先设置 Team 批量更新字段'));
      return;
    }
    const hasFilters =
      String(teamQuery.status || '').trim() !== '' ||
      String(teamQuery.search || '').trim() !== '';
    const payload = {
      filter_status: teamQuery.status,
      search: teamQuery.search,
      all: !hasFilters,
    };
    const nextStatus = String(batchTeamStatus || '').trim();
    const nextMetadata = String(batchTeamMetadata || '').trim();
    if (nextStatus) {
      payload.status = nextStatus;
    }
    if (nextMetadata) {
      payload.metadata = nextMetadata;
    }
    Modal.confirm({
      title: t('按筛选批量更新 Team 资产'),
      content: hasFilters
        ? t('确认将当前筛选命中的 {{count}} 个 Team 资产应用待更新字段？', {
            count: matchedCount,
          })
        : t('当前未设置筛选条件，确认将全部 {{count}} 个 Team 资产应用待更新字段？', {
            count: matchedCount,
          }),
      onOk: async () => {
        try {
          const result = await batchUpdateTeamAutomationTeams(payload);
          const updatedCount = Number(result?.updated_count) || 0;
          const failedCount = Number(result?.failed_count) || 0;
          showSuccess(
            t('按筛选批量更新完成：成功 {{updated}}，失败 {{failed}}', {
              updated: updatedCount,
              failed: failedCount,
            }),
          );
          setSelectedTeamRowKeys([]);
          const nextQuery = { ...teamQuery, page: 1 };
          setTeamQuery(nextQuery);
          await Promise.all([
            fetchTeams(nextQuery),
            getTeamAutomationTeamStats().then((v) => setTeamStats(v || null)),
          ]);
        } catch (error) {
          showError(error?.message || t('按筛选批量更新 Team 资产失败'));
        }
      },
    });
  };

  const saveTeam = async () => {
    const form = teamModal.form;
    const email = String(form?.email || '')
      .trim()
      .toLowerCase();
    if (!email) {
      showError(t('邮箱不能为空'));
      return;
    }
    const payload = {
      email,
      access_token: String(form?.access_token || '').trim(),
      refresh_token: String(form?.refresh_token || '').trim(),
      session_token: String(form?.session_token || '').trim(),
      client_id: String(form?.client_id || '').trim(),
      account_id: String(form?.account_id || '').trim(),
      team_name: String(form?.team_name || '').trim(),
      plan_type: String(form?.plan_type || '').trim(),
      subscription_plan: String(form?.subscription_plan || '').trim(),
      expires_at: Number(form?.expires_at) || 0,
      current_members: Number(form?.current_members) || 0,
      pending_invites: Number(form?.pending_invites) || 0,
      max_members: Number(form?.max_members) || 6,
      status: String(form?.status || '').trim(),
      account_role: String(form?.account_role || '').trim(),
      device_code_auth_enabled: form?.device_code_auth_enabled === true,
      error_count: Number(form?.error_count) || 0,
      last_sync_time: Number(form?.last_sync_time) || 0,
      metadata: String(form?.metadata || '').trim(),
    };
    if (
      teamModal.mode === 'create' &&
      !payload.access_token &&
      !payload.session_token &&
      (!payload.refresh_token || !payload.client_id)
    ) {
      showError(
        t(
          '至少提供 access_token，或 session_token，或 refresh_token + client_id',
        ),
      );
      return;
    }
    try {
      setTeamModal((prev) => ({ ...prev, submitting: true }));
      if (teamModal.mode === 'create') {
        await createTeamAutomationTeam(payload);
        showSuccess(t('Team 资产已创建'));
      } else {
        await updateTeamAutomationTeam(String(form?.id || email), payload);
        showSuccess(t('Team 资产已更新'));
      }
      setTeamModal({
        visible: false,
        mode: 'create',
        form: DEFAULT_TEAM_FORM,
        submitting: false,
      });
      await Promise.all([
        fetchTeams(teamQuery),
        getTeamAutomationTeamStats().then((v) => setTeamStats(v || null)),
      ]);
    } catch (error) {
      showError(error?.message || t('保存 Team 资产失败'));
      setTeamModal((prev) => ({ ...prev, submitting: false }));
    }
  };

  const openTeamImportModal = () => {
    setTeamImportModal({
      visible: true,
      form: { ...DEFAULT_TEAM_IMPORT_FORM },
      submitting: false,
    });
  };

  const onImportTeams = async () => {
    const content = String(teamImportModal.form.content || '').trim();
    if (!content) {
      showError(t('请提供 Team 资产数据'));
      return;
    }
    try {
      setTeamImportModal((prev) => ({ ...prev, submitting: true }));
      const result = await importTeamAutomationTeams({
        import_type: 'batch',
        content,
      });
      const created = Number(result?.created) || 0;
      const updated = Number(result?.updated) || 0;
      const failed = Number(result?.failed) || 0;
      showSuccess(
        t('导入完成：新增 {{created}}，更新 {{updated}}，失败 {{failed}}', {
          created,
          updated,
          failed,
        }),
      );
      setTeamImportModal({
        visible: false,
        form: DEFAULT_TEAM_IMPORT_FORM,
        submitting: false,
      });
      await Promise.all([
        fetchTeams(teamQuery),
        getTeamAutomationTeamStats().then((v) => setTeamStats(v || null)),
      ]);
    } catch (error) {
      showError(error?.message || t('导入 Team 资产失败'));
      setTeamImportModal((prev) => ({ ...prev, submitting: false }));
    }
  };

  const focusTeamCandidate = async (record) => {
    const keyword = String(
      record?.team_name || record?.email || record?.account_id || '',
    ).trim();
    const next = {
      ...teamQuery,
      page: 1,
      search: keyword,
    };
    setTeamDraft((prev) => ({
      ...prev,
      search: keyword,
    }));
    setTeamQuery(next);
    await fetchTeams(next);
  };

  const requestAutoSelectTeam = async (excludeIDs = []) => {
    const normalized = Array.isArray(excludeIDs)
      ? excludeIDs
          .map((item) => Number(item) || 0)
          .filter((item) => item > 0)
      : [];
    try {
      setTeamAutoSelectSubmitting(true);
      const result = await autoSelectTeamAutomationTeam({
        exclude_team_ids: normalized,
      });
      setTeamAutoSelect(result || null);
      setTeamAutoSelectExcludedIds(normalized);
      if (result?.selected?.email) {
        showSuccess(
          t('已选出最优 Team：{{email}}', { email: result.selected.email }),
        );
      } else {
        showError(t('当前没有可用 Team'));
      }
    } catch (error) {
      showError(error?.message || t('自动选队失败'));
    } finally {
      setTeamAutoSelectSubmitting(false);
    }
  };

  const onAutoSelectTeam = async () => {
    await requestAutoSelectTeam([]);
  };

  const onRetryAutoSelectExcludingCurrent = async () => {
    const currentID = Number(teamAutoSelect?.selected?.id) || 0;
    if (currentID <= 0) {
      showError(t('当前没有可排除的推荐 Team'));
      return;
    }
    const nextExcluded = Array.from(
      new Set(teamAutoSelectExcludedIds.concat([currentID])),
    );
    await requestAutoSelectTeam(nextExcluded);
  };

  const removeExcludedAutoSelectTeam = async (teamID) => {
    const numericID = Number(teamID) || 0;
    if (numericID <= 0) {
      return;
    }
    const nextExcluded = teamAutoSelectExcludedIds.filter(
      (item) => Number(item) !== numericID,
    );
    await requestAutoSelectTeam(nextExcluded);
  };

  const excludeTeamCandidateAndRecalculate = async (record) => {
    const currentID = Number(record?.id) || 0;
    if (currentID <= 0) {
      showError(t('当前 Team 缺少可排除的 ID'));
      return;
    }
    const nextExcluded = Array.from(
      new Set(teamAutoSelectExcludedIds.concat([currentID])),
    );
    await requestAutoSelectTeam(nextExcluded);
  };

  const applyHistoryFailureReason = async (term) => {
    const keyword = String(term || '').trim();
    if (!keyword) {
      return;
    }
    await applyHistoryQuery({
      search: keyword,
      status: 'failed',
      only_failed: true,
    });
  };

  const filterExecutionLogsByAccount = (accountID) => {
    const keyword = String(accountID || '').trim();
    if (!keyword) {
      showError(t('当前失败条目缺少账号 ID'));
      return;
    }
    setExecutionLogSearch(keyword);
    showSuccess(
      t('已按账号 {{id}} 过滤执行日志', {
        id: keyword,
      }),
    );
  };

  const renderModuleField = (field) => {
    const key = String(field?.key || '').trim();
    if (!key) {
      return null;
    }
    const type = String(field?.type || 'text').toLowerCase();
    const label = field?.label || key;
    const description = field?.description || '';
    const required = field?.required === true;
    const value = moduleFieldValues[key];

    const setValue = (next) => {
      setModuleFieldValues((prev) => ({
        ...prev,
        [key]: next,
      }));
    };

    let control = null;
    if (type === 'select') {
      control = (
        <Select
          style={{ width: '100%' }}
          optionList={toOptionList(field?.options)}
          value={value}
          onChange={setValue}
          placeholder={field?.placeholder || ''}
        />
      );
    } else if (type === 'number') {
      control = (
        <InputNumber
          style={{ width: '100%' }}
          value={Number(value) || 0}
          onChange={(next) => setValue(Number(next) || 0)}
          placeholder={field?.placeholder || ''}
        />
      );
    } else if (type === 'boolean') {
      control = (
        <Switch
          checked={value === true}
          onChange={(next) => setValue(next === true)}
        />
      );
    } else if (type === 'textarea') {
      control = (
        <TextArea
          autosize={{ minRows: 3, maxRows: 8 }}
          value={String(value || '')}
          onChange={(next) => setValue(next)}
          placeholder={field?.placeholder || ''}
        />
      );
    } else if (type === 'password') {
      control = (
        <Input
          mode='password'
          value={String(value || '')}
          onChange={(next) => setValue(next)}
          placeholder={field?.placeholder || ''}
          autoComplete='new-password'
        />
      );
    } else {
      control = (
        <Input
          value={String(value || '')}
          onChange={(next) => setValue(next)}
          placeholder={field?.placeholder || ''}
        />
      );
    }

    return (
      <div
        key={key}
        data-testid={`team-automation-field-${key}`}
        style={{ minWidth: 220 }}
      >
        <Text strong>
          {label}
          {required ? ' *' : ''}
        </Text>
        {description ? (
          <div style={{ marginTop: 2 }}>
            <Text type='tertiary'>{description}</Text>
          </div>
        ) : null}
        <div style={{ marginTop: 8 }}>{control}</div>
      </div>
    );
  };

  const accountColumns = [
    {
      title: t('ID'),
      dataIndex: 'id',
      width: 70,
    },
    {
      title: t('名称'),
      dataIndex: 'name',
      render: (_, record) => record?.name || '-',
    },
    {
      title: t('邮箱'),
      dataIndex: 'email',
      render: (_, record) => record?.email || '-',
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 150,
      render: (_, record) => <StatusTag value={record?.status} />,
    },
    {
      title: t('兑换码'),
      dataIndex: 'redeem_code',
      render: (_, record) => record?.redeem_code || '-',
    },
    {
      title: t('更新时间'),
      dataIndex: 'last_update_time',
      width: 180,
      render: (_, record) => record?.last_update_time || '-',
    },
    {
      title: t('操作'),
      dataIndex: 'action',
      width: 160,
      render: (_, record) => (
        <Space>
          <Button
            theme='borderless'
            icon={<Eye size={14} />}
            onClick={() => openEditAccount(record)}
          />
          <Button
            theme='borderless'
            type='danger'
            icon={<Trash2 size={14} />}
            onClick={() => onDeleteAccount(record)}
          />
        </Space>
      ),
    },
  ];

  const teamColumns = [
    {
      title: t('ID'),
      dataIndex: 'id',
      width: 70,
    },
    {
      title: t('Team'),
      dataIndex: 'team_name',
      render: (_, record) => record?.team_name || '-',
    },
    {
      title: t('邮箱'),
      dataIndex: 'email',
      render: (_, record) => record?.email || '-',
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 120,
      render: (_, record) => <StatusTag value={record?.status} />,
    },
    {
      title: t('席位'),
      dataIndex: 'slots',
      width: 180,
      render: (_, record) =>
        `${Number(record?.current_members) || 0} + ${Number(record?.pending_invites) || 0} / ${Number(record?.max_members) || 0}`,
    },
    {
      title: t('剩余'),
      dataIndex: 'remaining_slots',
      width: 100,
      render: (_, record) => Number(record?.remaining_slots) || 0,
    },
    {
      title: t('账户ID'),
      dataIndex: 'account_id',
      render: (_, record) => record?.account_id || '-',
    },
    {
      title: t('更新时间'),
      dataIndex: 'last_update_time',
      width: 180,
      render: (_, record) => record?.last_update_time || '-',
    },
    {
      title: t('操作'),
      dataIndex: 'action',
      width: 160,
      render: (_, record) => (
        <Space>
          <Button
            theme='borderless'
            icon={<Eye size={14} />}
            onClick={() => openEditTeam(record)}
          />
          <Button
            theme='borderless'
            type='danger'
            icon={<Trash2 size={14} />}
            onClick={() => onDeleteTeam(record)}
          />
        </Space>
      ),
    },
  ];

  const historyColumns = [
    {
      title: t('执行ID'),
      dataIndex: 'execution_id',
      render: (_, record) => record?.execution_id || '-',
    },
    {
      title: t('模块'),
      dataIndex: 'module',
      width: 180,
      render: (_, record) => record?.module || '-',
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 140,
      render: (_, record) => <StatusTag value={record?.status} />,
    },
    {
      title: t('进度'),
      dataIndex: 'progress',
      width: 180,
      render: (_, record) => (
        <Progress
          percent={Number(record?.progress) || 0}
          showInfo
          size='small'
          aria-label={`progress-${record?.execution_id || ''}`}
        />
      ),
    },
    {
      title: t('成功/失败'),
      dataIndex: 'result',
      width: 150,
      render: (_, record) =>
        `${Number(record?.success_count) || 0}/${Number(record?.failed_count) || 0}`,
    },
    {
      title: t('代理重试'),
      dataIndex: 'proxy_retry_count',
      width: 110,
      render: (_, record) => {
        const count = Number(record?.proxy_retry_count) || 0;
        if (count <= 0) {
          return <Text type='tertiary'>0</Text>;
        }
        return <Tag color='blue'>{count}</Tag>;
      },
    },
    {
      title: t('失败摘要'),
      dataIndex: 'failed_summary',
      width: 320,
      render: (_, record) => {
        const text = buildExecutionHistoryFailureSummary(record);
        if (!text) {
          return <Text type='tertiary'>-</Text>;
        }
        const isFailed = String(record?.status || '').trim().toLowerCase() === 'failed';
        return (
          <Text type={isFailed ? 'danger' : 'secondary'}>
            {shortenText(text, 160)}
          </Text>
        );
      },
    },
    {
      title: t('创建时间'),
      dataIndex: 'created_at',
      width: 160,
      render: (_, record) => {
        const ts = Number(record?.created_at) || 0;
        if (!ts) return '-';
        return new Date(ts * 1000).toLocaleString();
      },
    },
    {
      title: t('操作'),
      dataIndex: 'op',
      width: 120,
      render: (_, record) => (
        <Button
          theme='light'
          size='small'
          onClick={() =>
            setSelectedExecutionId(String(record?.execution_id || ''))
          }
        >
          {t('查看')}
        </Button>
      ),
    },
  ];

  const accountOverviewCards = useMemo(
    () =>
      ACCOUNT_OVERVIEW_CARD_DEFINITIONS.map((item) => ({
        ...item,
        value:
          item.status === ''
            ? Number(accountStats?.total) || accountsTotal
            : Number(accountByStatus?.[item.status]) || 0,
      })),
    [accountByStatus, accountStats?.total, accountsTotal],
  );
  const teamStatusOverviewCards = useMemo(
    () =>
      TEAM_STATUS_OVERVIEW_DEFINITIONS.map((item) => ({
        ...item,
        value:
          item.status === ''
            ? Number(teamStats?.total) || teamsTotal
            : Number(teamByStatus?.[item.status]) || 0,
      })),
    [teamByStatus, teamStats?.total, teamsTotal],
  );
  const accountRowSelection = {
    selectedRowKeys: selectedAccountRowKeys,
    onChange: (selectedRowKeys) => setSelectedAccountRowKeys(selectedRowKeys),
  };
  const teamRowSelection = {
    selectedRowKeys: selectedTeamRowKeys,
    onChange: (selectedRowKeys) => setSelectedTeamRowKeys(selectedRowKeys),
  };
  const historyRowSelection = {
    selectedRowKeys: selectedHistoryRowKeys,
    onChange: (selectedRowKeys) => setSelectedHistoryRowKeys(selectedRowKeys),
    getCheckboxProps: (record) => {
      const status = String(record?.status || '').trim().toLowerCase();
      return {
        disabled: ['queued', 'running', 'stopping'].includes(status),
      };
    },
  };

  return (
    <div className='mt-[60px] px-2'>
      <input
        ref={importRef}
        type='file'
        accept='.csv,text/csv'
        style={{ display: 'none' }}
        onChange={onImportAccounts}
      />

      <Spin spinning={loading} size='large'>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <Banner
            type={connectorState.type}
            closeIcon={null}
            description={
              <div>
                <div data-testid='team-automation-capability-source'>
                  {connectorState.text}
                </div>
                {meta?.can_configure ? (
                  <div style={{ marginTop: 8 }}>
                    <Link
                      data-testid='team-automation-open-setting'
                      to='/console/setting?tab=team-automation'
                    >
                      {t('前往 Automation 设置')}
                    </Link>
                  </div>
                ) : (
                  <div style={{ marginTop: 8 }}>
                    <Text type='secondary'>
                      {t('当前账号无配置权限，请联系 root 用户。')}
                    </Text>
                  </div>
                )}
              </div>
            }
          />

          <Card bodyStyle={{ padding: '14px' }}>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                gap: '12px',
                alignItems: 'center',
                flexWrap: 'wrap',
              }}
            >
              <div>
                <Title heading={5} style={{ margin: 0 }}>
                  {t('Automation 控制台')}
                </Title>
                <Text type='secondary'>
                  {t(
                    '本地执行优先，支持远端兼容模式；执行日志和产物均可本地追踪。',
                  )}
                </Text>
              </div>
              <Space>
                <Button
                  icon={<RefreshCw size={15} />}
                  onClick={() => fetchAll({ silent: true })}
                  loading={refreshing}
                >
                  {t('刷新')}
                </Button>
                <Button
                  icon={<CircleStop size={15} />}
                  type='warning'
                  disabled={!selectedExecutionId || !canStopSelectedExecution}
                  loading={stopSubmitting}
                  onClick={onStopRun}
                >
                  {t('停止任务')}
                </Button>
              </Space>
            </div>
          </Card>

          {String(meta?.cpa_endpoint || '').trim() ? (
            <Card bodyStyle={{ padding: '14px' }}>
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  gap: '12px',
                  alignItems: 'center',
                  flexWrap: 'wrap',
                }}
              >
                <div>
                  <Title heading={6} style={{ margin: 0 }}>
                    {t('CPA / 导入链路')}
                  </Title>
                  <Text type='secondary'>{cpaStatusSummary.text}</Text>
                </div>
                <Link to='/console/setting?tab=team-automation'>
                  {t('前往 CPA 设置')}
                </Link>
              </div>
              <div
                style={{
                  marginTop: 10,
                  display: 'flex',
                  gap: 8,
                  flexWrap: 'wrap',
                }}
              >
                <Tag color={meta?.cpa_configured ? 'green' : 'orange'}>
                  {meta?.cpa_configured ? t('CPA 已配置') : t('CPA Token 未配置')}
                </Tag>
                <Tag color='cyan'>
                  {cpaEndpointMode === 'cliproxy_management'
                    ? t('模式: CLIProxy Management')
                    : cpaEndpointMode === 'legacy_importer'
                      ? t('模式: Legacy Importer')
                      : t('模式: 通用 Endpoint')}
                </Tag>
                <Tag color='blue'>{String(meta?.cpa_endpoint || '').trim()}</Tag>
              </div>
            </Card>
          ) : null}

          {proxyPoolSummary ? (
            <Card bodyStyle={{ padding: '14px' }}>
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  gap: '12px',
                  alignItems: 'center',
                  flexWrap: 'wrap',
                  marginBottom: 12,
                }}
              >
                <div>
                  <Title heading={6} style={{ margin: 0 }}>
                    {t('代理池健康摘要')}
                  </Title>
                  <Text type='secondary'>
                    {t('运行前快速确认 vmess / gateway / 无效条目状态，避免任务启动后才发现代理池异常。')}
                  </Text>
                </div>
                <Link to='/console/setting?tab=team-automation'>
                  {t('前往 Automation 设置')}
                </Link>
              </div>

              {proxyStatusBanner ? (
                <Banner
                  type={proxyStatusBanner.type}
                  closeIcon={null}
                  description={proxyStatusBanner.text}
                  style={{ marginBottom: 12 }}
                />
              ) : null}

              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))',
                  gap: '10px',
                  marginBottom: 12,
                }}
              >
                {proxySummaryCards.map((item) => (
                  <div
                    key={item.label}
                    style={{
                      padding: '12px 14px',
                      borderRadius: 12,
                      border: '1px solid rgba(148,163,184,0.2)',
                      background:
                        'linear-gradient(135deg, rgba(15,23,42,0.03), rgba(56,189,248,0.08))',
                    }}
                  >
                    <Text type='secondary'>{item.label}</Text>
                    <div style={{ marginTop: 4, fontSize: 24, fontWeight: 700 }}>
                      {item.value}
                    </div>
                  </div>
                ))}
              </div>

              <div
                style={{
                  display: 'flex',
                  gap: '8px',
                  flexWrap: 'wrap',
                  marginBottom: invalidProxyRows.length > 0 ? 12 : 0,
                }}
              >
                {proxyRuntimeTags.map((item) => (
                  <Tag key={item} color='cyan'>
                    {item}
                  </Tag>
                ))}
              </div>

              {invalidProxyRows.length > 0 ? (
                <div
                  style={{
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 8,
                  }}
                >
                  <Text strong>{t('异常条目预览')}</Text>
                  {invalidProxyRows.slice(0, 3).map((item) => (
                    <div
                      key={`invalid-proxy-${item.index}`}
                      style={{
                        padding: '10px 12px',
                        borderRadius: 10,
                        background: 'rgba(255,241,242,0.88)',
                        border: '1px solid rgba(248,113,113,0.24)',
                      }}
                    >
                      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                        <Tag color='red'>#{Number(item?.index) || 0}</Tag>
                        <Text>{String(item?.raw || '').trim() || '-'}</Text>
                      </div>
                      <div style={{ marginTop: 4 }}>
                        <Text type='secondary'>
                          {String(item?.error || '').trim() || '-'}
                        </Text>
                      </div>
                    </div>
                  ))}
                  {invalidProxyRows.length > 3 ? (
                    <Text type='tertiary'>
                      {t('还有 {{count}} 条异常条目，请到设置页查看并复制清洗。', {
                        count: invalidProxyRows.length - 3,
                      })}
                    </Text>
                  ) : null}
                </div>
              ) : vmessProxyRows.length > 0 ? (
                <Text type='tertiary'>
                  {t('已识别 {{count}} 条 vmess 节点；如需复制或清洗条目，可在设置页使用诊断面板。', {
                    count: vmessProxyRows.length,
                  })}
                </Text>
              ) : null}
            </Card>
          ) : null}

          <Card title={t('运行任务')} bodyStyle={{ padding: '14px' }}>
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
                gap: '12px',
              }}
            >
              <div>
                <Text strong>{t('模块')}</Text>
                <Select
                  data-testid='team-automation-run-module'
                  style={{ marginTop: 8, width: '100%' }}
                  optionList={moduleOptions}
                  value={moduleId}
                  onChange={setModuleId}
                />
                {!currentModule && moduleId === FULL_PIPELINE_MODULE_ID ? (
                  <div style={{ marginTop: 4 }}>
                    <Text type='tertiary'>{getModulePipelineHint(t)}</Text>
                  </div>
                ) : null}
              </div>
              <div>
                <Text strong>{t('账号状态过滤')}</Text>
                <Select
                  data-testid='team-automation-run-account-status'
                  style={{ marginTop: 8, width: '100%' }}
                  optionList={ACCOUNT_STATUS_OPTIONS}
                  value={runFilterStatus}
                  onChange={setRunFilterStatus}
                />
              </div>
              <div>
                <Text strong>{t('搜索关键词')}</Text>
                <Input
                  style={{ marginTop: 8 }}
                  value={runFilterSearch}
                  onChange={setRunFilterSearch}
                  placeholder={t('邮箱 / 兑换码 / 名称')}
                />
              </div>
            </div>

            <div
              style={{
                marginTop: 12,
                padding: '10px 12px',
                borderRadius: 10,
                border: '1px solid rgba(148,163,184,0.18)',
                background:
                  'linear-gradient(135deg, rgba(15,23,42,0.03), rgba(34,197,94,0.06))',
                display: 'flex',
                justifyContent: 'space-between',
                gap: '12px',
                flexWrap: 'wrap',
                alignItems: 'center',
              }}
            >
              <div>
                <Text strong>{t('运行目标速选')}</Text>
                <div style={{ marginTop: 4 }}>
                  <Text type='secondary'>
                    {t('可直接套用账号列表筛选，避免在启动任务前重复设置状态和搜索关键词。')}
                  </Text>
                </div>
              </div>
              <Space wrap>
                {RUN_TARGET_QUICK_FILTERS.map((item) => (
                  <Button
                    key={item.key}
                    size='small'
                    theme={
                      String(runFilterStatus || '').trim() === item.status
                        ? 'solid'
                        : 'light'
                    }
                    type={
                      item.status === 'failed' || item.status === 'manual_review'
                        ? 'warning'
                        : 'tertiary'
                    }
                    onClick={() => applyRunQuickFilter(item.status)}
                  >
                    {t(item.label)}
                  </Button>
                ))}
                <Button size='small' theme='outline' onClick={syncRunFiltersFromAccountQuery}>
                  {t('沿用账号筛选')}
                </Button>
                <Button size='small' theme='light' onClick={clearRunFilters}>
                  {t('清空运行筛选')}
                </Button>
              </Space>
            </div>

            <div
              style={{
                marginTop: 12,
                display: 'flex',
                gap: '8px',
                flexWrap: 'wrap',
                alignItems: 'center',
                padding: '10px 12px',
                border: '1px solid var(--semi-color-border)',
                borderRadius: 10,
                background: 'var(--semi-color-fill-0)',
              }}
            >
              <Text>
                {t('当前运行筛选')}: <b>{runFilterSummary || t('全部账号')}</b>
              </Text>
              <Text type='secondary'>
                {runTargetEstimate === null
                  ? t('目标数量将在执行时按实时搜索结果决定')
                  : t('预计命中 {{count}} 个账号', {
                      count: runTargetEstimate,
                    })}
              </Text>
              {String(currentModule?.id || '').trim() === 'import_target' ? (
                <Tag color={meta?.cpa_configured ? 'green' : 'orange'}>
                  {['cpa', 'both'].includes(currentImportTarget)
                    ? meta?.cpa_configured
                      ? t('CPA 导入已就绪')
                      : t('CPA 导入未配置完成')
                    : t('当前仅导入 NewAPI')}
                </Tag>
              ) : null}
            </div>

            {runBlockedReason ? (
              <Banner
                type='warning'
                closeIcon={null}
                style={{ marginTop: 12 }}
                description={runBlockedReason}
              />
            ) : null}

            {currentModule ? (
              <div style={{ marginTop: 14 }}>
                <Text strong>{currentModule.name}</Text>
                {currentModule.description ? (
                  <div style={{ marginTop: 4 }}>
                    <Text type='secondary'>{currentModule.description}</Text>
                  </div>
                ) : null}
                {hasProxyControls ? (
                  <div
                    style={{
                      marginTop: 12,
                      padding: '12px',
                      borderRadius: 10,
                      background: 'var(--semi-color-fill-0)',
                      border: '1px solid var(--semi-color-border)',
                    }}
                  >
                    <Text strong>{t('代理模板')}</Text>
                    <div style={{ marginTop: 4 }}>
                      <Text type='secondary'>
                        {t(
                          '按当前模块字段一键填充代理轮换策略，适合 gateway-only、随机轮询和 vmess+xray 场景。',
                        )}
                      </Text>
                    </div>
                    <Space wrap style={{ marginTop: 10 }}>
                      {visibleProxyPresets.map((preset) => (
                        <Button
                          key={preset.key}
                          theme='light'
                          size='small'
                          onClick={() => applyProxyPreset(preset)}
                        >
                          {preset.label}
                        </Button>
                      ))}
                    </Space>
                    <div
                      style={{
                        marginTop: 8,
                        display: 'flex',
                        flexDirection: 'column',
                        gap: '4px',
                      }}
                    >
                      {visibleProxyPresets.map((preset) => (
                        <Text key={`${preset.key}-desc`} type='tertiary'>
                          {preset.label}: {t(preset.description)}
                        </Text>
                      ))}
                    </div>
                  </div>
                ) : null}
                <div
                  style={{
                    marginTop: 10,
                    display: 'grid',
                    gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
                    gap: '12px',
                  }}
                >
                  {currentModule.fields.map((field) =>
                    renderModuleField(field),
                  )}
                </div>
              </div>
            ) : null}

            <Space style={{ marginTop: 14 }}>
              <Button
                data-testid='team-automation-start-run'
                theme='solid'
                type='primary'
                icon={<Play size={15} />}
                onClick={onStartRun}
                loading={runSubmitting}
                disabled={!canStartRun}
              >
                {t('启动任务')}
              </Button>
            </Space>
          </Card>

          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
              gap: '12px',
            }}
          >
            {accountOverviewCards.map((item) => {
              const active = String(accountQuery.status || '').trim() === item.status;
              return (
                <Card
                  key={item.key}
                  bodyStyle={{ padding: '14px' }}
                  style={{
                    cursor: 'pointer',
                    border: active
                      ? '1px solid var(--semi-color-primary)'
                      : '1px solid var(--semi-color-border)',
                    background: active
                      ? 'linear-gradient(135deg, rgba(16,185,129,0.08), rgba(59,130,246,0.04))'
                      : undefined,
                  }}
                  onClick={() => applyAccountQuickStatusFilter(item.status)}
                >
                  <Text type='secondary'>{t(item.label)}</Text>
                  <div style={{ fontSize: 28, fontWeight: 700 }}>
                    {item.value}
                  </div>
                  <div style={{ marginTop: 4 }}>
                    <Text type='tertiary'>
                      {active ? t('当前状态筛选') : t('点击直接筛选')}
                    </Text>
                  </div>
                </Card>
              );
            })}
          </div>

          <Card title={t('Team 资产池')} bodyStyle={{ padding: '14px' }}>
            <Banner
              type='info'
              closeIcon={null}
              style={{ marginBottom: 12 }}
              description={
                <div
                  style={{ display: 'flex', flexDirection: 'column', gap: 4 }}
                >
                  <Text>
                    {t(
                      '兼容 team-manage 的核心资产语义：Team owner、token、席位、待邀请数、自动选队。',
                    )}
                  </Text>
                  <Text>
                    {t(
                      '批量导入格式：`邮箱,AccessToken,RefreshToken,SessionToken,ClientID[,AccountID,TeamName,CurrentMembers,PendingInvites,MaxMembers,Status]`。',
                    )}
                  </Text>
                  <Text>
                    {t('自动选队会优先选择占用席位更少且仍有空位的 Team。')}
                  </Text>
                </div>
              }
            />

            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
                gap: '12px',
                marginBottom: 12,
              }}
            >
              <Card bodyStyle={{ padding: '14px' }}>
                <Text type='secondary'>{t('Team 总数')}</Text>
                <div style={{ fontSize: 28, fontWeight: 700 }}>
                  {Number(teamStats?.total) || teamsTotal}
                </div>
              </Card>
              <Card bodyStyle={{ padding: '14px' }}>
                <Text type='secondary'>{t('可用 Team')}</Text>
                <div style={{ fontSize: 28, fontWeight: 700 }}>
                  {Number(teamStats?.available) || 0}
                </div>
              </Card>
              <Card bodyStyle={{ padding: '14px' }}>
                <Text type='secondary'>{t('剩余席位')}</Text>
                <div style={{ fontSize: 28, fontWeight: 700 }}>
                  {Number(teamStats?.total_remaining_slots) || 0}
                </div>
              </Card>
              <Card bodyStyle={{ padding: '14px' }}>
                <Text type='secondary'>{t('待邀请数')}</Text>
                <div style={{ fontSize: 28, fontWeight: 700 }}>
                  {Number(teamStats?.total_pending_invites) || 0}
                </div>
              </Card>
            </div>

            {teamAutoSelect?.selected ? (
              <div
                style={{
                  marginBottom: 12,
                  padding: '12px',
                  borderRadius: 12,
                  border: '1px solid rgba(34,197,94,0.2)',
                  background:
                    'linear-gradient(135deg, rgba(236,253,245,0.9), rgba(219,234,254,0.85))',
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
                    <Text strong>
                      {t('当前推荐 Team')}:{' '}
                      {teamAutoSelect.selected.team_name ||
                        teamAutoSelect.selected.email}
                    </Text>
                    <div style={{ marginTop: 4 }}>
                      <Text type='secondary'>
                        {t('邮箱')}: {teamAutoSelect.selected.email} |{' '}
                        {t('剩余席位')}:{' '}
                        {Number(teamAutoSelect.selected.remaining_slots) || 0} |{' '}
                        {t('候选数')}: {teamAutoSelectCandidates.length}
                      </Text>
                    </div>
                  </div>
                  <Space wrap>
                    <Button
                      size='small'
                      theme='outline'
                      onClick={() => focusTeamCandidate(teamAutoSelect.selected)}
                    >
                      {t('定位到列表')}
                    </Button>
                    <Button
                      size='small'
                      theme='light'
                      type='warning'
                      loading={teamAutoSelectSubmitting}
                      onClick={onRetryAutoSelectExcludingCurrent}
                    >
                      {t('排除当前推荐重算')}
                    </Button>
                    <Button
                      size='small'
                      theme='light'
                      onClick={() => requestAutoSelectTeam([])}
                    >
                      {t('清空排除后重算')}
                    </Button>
                  </Space>
                </div>
                {teamAutoSelectExcludedIds.length > 0 ? (
                  <div
                    style={{
                      display: 'flex',
                      flexDirection: 'column',
                      gap: 8,
                    }}
                  >
                    <Text type='tertiary'>
                      {t('本轮已排除 {{count}} 个 Team', {
                        count: teamAutoSelectExcludedIds.length,
                      })}
                    </Text>
                    <div
                      style={{
                        display: 'flex',
                        gap: 8,
                        flexWrap: 'wrap',
                      }}
                    >
                      {excludedTeamPreview.map((item) => (
                        <div
                          key={`excluded-team-${item?.id || item?.email || 'unknown'}`}
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 6,
                            padding: '6px 8px',
                            borderRadius: 999,
                            background: 'rgba(255,255,255,0.72)',
                            border: '1px solid rgba(148,163,184,0.22)',
                          }}
                        >
                          <Text type='secondary'>
                            {item?.team_name || item?.email || `#${item?.id || '-'}`}
                          </Text>
                          <Button
                            size='small'
                            theme='borderless'
                            icon={<X size={12} />}
                            onClick={() => removeExcludedAutoSelectTeam(item?.id)}
                          />
                        </div>
                      ))}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : null}

            {teamAutoSelectCandidates.length > 0 ? (
              <div
                style={{
                  marginBottom: 12,
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
                  gap: '10px',
                }}
              >
                {teamAutoSelectCandidates.slice(0, 4).map((item) => {
                  const selected =
                    String(item?.id || '') ===
                    String(teamAutoSelect?.selected?.id || '');
                  return (
                    <Card
                      key={`candidate-${item?.id || item?.email}`}
                      bodyStyle={{ padding: '12px' }}
                      style={{
                        border: selected
                          ? '1px solid rgba(34,197,94,0.45)'
                          : '1px solid var(--semi-color-border)',
                        background: selected
                          ? 'rgba(236,253,245,0.8)'
                          : 'var(--semi-color-bg-1)',
                      }}
                    >
                      <div
                        style={{
                          display: 'flex',
                          justifyContent: 'space-between',
                          gap: 8,
                        }}
                      >
                        <Text strong>{item?.team_name || item?.email || '-'}</Text>
                        {selected ? <Tag color='green'>{t('推荐')}</Tag> : null}
                      </div>
                      <div style={{ marginTop: 6 }}>
                        <Text type='secondary'>{item?.email || '-'}</Text>
                      </div>
                      <div
                        style={{
                          marginTop: 8,
                          display: 'flex',
                          gap: 8,
                          flexWrap: 'wrap',
                        }}
                      >
                        <Tag color='blue'>
                          {t('剩余席位')}: {Number(item?.remaining_slots) || 0}
                        </Tag>
                        <Tag color='cyan'>
                          {t('状态')}: {String(item?.status || '-')}
                        </Tag>
                      </div>
                      <div style={{ marginTop: 8 }}>
                        <Text type='tertiary'>
                          {Number(item?.current_members) || 0} +{' '}
                          {Number(item?.pending_invites) || 0} /{' '}
                          {Number(item?.max_members) || 0}
                        </Text>
                      </div>
                      <div style={{ marginTop: 10 }}>
                        <Space wrap>
                          <Button
                            size='small'
                            theme='light'
                            onClick={() => focusTeamCandidate(item)}
                          >
                            {t('查看该 Team')}
                          </Button>
                          <Button
                            size='small'
                            theme='outline'
                            type='warning'
                            disabled={
                              String(item?.id || '') ===
                              String(teamAutoSelect?.selected?.id || '')
                            }
                            onClick={() => excludeTeamCandidateAndRecalculate(item)}
                          >
                            {t('加入排除并重算')}
                          </Button>
                        </Space>
                      </div>
                    </Card>
                  );
                })}
              </div>
            ) : null}

            <div
              style={{
                marginBottom: 12,
                padding: '10px 12px',
                borderRadius: 10,
                background:
                  'linear-gradient(135deg, rgba(15,23,42,0.03), rgba(59,130,246,0.06))',
                border: '1px solid rgba(148,163,184,0.18)',
                display: 'flex',
                justifyContent: 'space-between',
                gap: '10px',
                flexWrap: 'wrap',
                alignItems: 'center',
              }}
            >
              <div>
                <Text strong>{t('Team 快捷筛选')}</Text>
                <div style={{ marginTop: 4 }}>
                  <Text type='secondary'>
                    {t('快速定位已满、过期或异常 Team，减少手动切状态排查。')}
                  </Text>
                </div>
              </div>
              <Space wrap>
                {TEAM_QUICK_FILTERS.map((item) => (
                  <Button
                    key={item.key}
                    size='small'
                    theme={
                      String(teamQuery.status || '').trim() === item.value
                        ? 'solid'
                        : 'light'
                    }
                    type={
                      item.value === 'error' || item.value === 'expired'
                        ? 'warning'
                        : 'tertiary'
                    }
                    onClick={() => applyTeamQuickStatusFilter(item.value)}
                  >
                    {t(item.label)}
                  </Button>
                ))}
                <Button
                  size='small'
                  theme='outline'
                  onClick={() => applyTeamQuickStatusFilter('')}
                >
                  {t('全部 Team')}
                </Button>
              </Space>
            </div>

            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))',
                gap: '10px',
                marginBottom: 12,
              }}
            >
              {teamStatusOverviewCards.map((item) => {
                const active = String(teamQuery.status || '').trim() === item.status;
                return (
                  <Card
                    key={item.key}
                    bodyStyle={{ padding: '12px 14px' }}
                    style={{
                      cursor: 'pointer',
                      border: active
                        ? '1px solid var(--semi-color-primary)'
                        : '1px solid var(--semi-color-border)',
                      background: active
                        ? 'linear-gradient(135deg, rgba(59,130,246,0.08), rgba(14,165,233,0.05))'
                        : undefined,
                    }}
                    onClick={() => applyTeamQuickStatusFilter(item.status)}
                  >
                    <Text type='secondary'>{t(item.label)}</Text>
                    <div style={{ marginTop: 4, fontSize: 24, fontWeight: 700 }}>
                      {item.value}
                    </div>
                    <div style={{ marginTop: 4 }}>
                      <Text type='tertiary'>
                        {active ? t('当前状态筛选') : t('点击直接筛选')}
                      </Text>
                    </div>
                  </Card>
                );
              })}
            </div>

            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                gap: '12px',
                flexWrap: 'wrap',
                marginBottom: 12,
              }}
            >
              <Space>
                <Button
                  theme='solid'
                  type='primary'
                  icon={<Plus size={15} />}
                  onClick={openCreateTeam}
                >
                  {t('新增 Team')}
                </Button>
                <Button
                  icon={<FileUp size={15} />}
                  onClick={openTeamImportModal}
                >
                  {t('导入 Team')}
                </Button>
                <Button
                  icon={<Download size={15} />}
                  onClick={() => {
                    const url = buildTeamAutomationTeamsExportURL(teamQuery);
                    window.open(url, '_blank');
                  }}
                >
                  {t('导出 CSV')}
                </Button>
                <Button
                  loading={teamAutoSelectSubmitting}
                  onClick={onAutoSelectTeam}
                >
                  {t('自动选队预览')}
                </Button>
                <Button
                  type='danger'
                  icon={<Trash2 size={15} />}
                  disabled={selectedTeamRowKeys.length === 0}
                  onClick={onBatchDeleteTeams}
                >
                  {t('批量删除')}
                </Button>
                <Button
                  theme='light'
                  type='warning'
                  disabled={teamsTotal <= 0}
                  onClick={onBatchDeleteTeamsByFilter}
                >
                  {t('清理当前筛选')}
                </Button>
              </Space>
              <Space>
                <Select
                  style={{ width: 180 }}
                  optionList={TEAM_STATUS_OPTIONS.filter(
                    (item) => item.value !== '',
                  )}
                  value={batchTeamStatus}
                  onChange={setBatchTeamStatus}
                  placeholder={t('批量设置状态')}
                />
                <Input
                  style={{ width: 180 }}
                  value={batchTeamMetadata}
                  onChange={setBatchTeamMetadata}
                  placeholder={t('批量追加标记')}
                />
                <Button
                  disabled={
                    selectedTeamRowKeys.length === 0 || !hasTeamBatchUpdateFields
                  }
                  onClick={onBatchUpdateTeams}
                >
                  {t('批量更新')}
                </Button>
                <Button
                  theme='light'
                  type='warning'
                  disabled={teamsTotal <= 0 || !hasTeamBatchUpdateFields}
                  onClick={onBatchUpdateTeamsByFilter}
                >
                  {t('更新当前筛选')}
                </Button>
                <Select
                  style={{ width: 180 }}
                  optionList={TEAM_STATUS_OPTIONS}
                  value={teamDraft.status}
                  onChange={(value) =>
                    setTeamDraft((prev) => ({ ...prev, status: value }))
                  }
                />
                <Input
                  prefix={<Search size={14} />}
                  value={teamDraft.search}
                  onChange={(value) =>
                    setTeamDraft((prev) => ({ ...prev, search: value }))
                  }
                  placeholder={t('搜索 Team / 邮箱 / Account ID')}
                />
                <Button
                  icon={<RefreshCw size={15} />}
                  onClick={async () => {
                    const next = {
                      ...teamQuery,
                      page: 1,
                      status: teamDraft.status,
                      search: teamDraft.search,
                    };
                    setTeamQuery(next);
                    await fetchTeams(next);
                  }}
                >
                  {t('筛选')}
                </Button>
                <Button
                  theme='light'
                  onClick={async () => {
                    const next = {
                      ...teamQuery,
                      page: 1,
                      status: '',
                      search: '',
                    };
                    setTeamDraft({ status: '', search: '' });
                    setTeamQuery(next);
                    await fetchTeams(next);
                  }}
                >
                  {t('清空筛选')}
                </Button>
              </Space>
            </div>
            <div
              style={{
                marginBottom: 12,
                display: 'flex',
                gap: '8px',
                flexWrap: 'wrap',
                alignItems: 'center',
                padding: '10px 12px',
                border: '1px solid var(--semi-color-border)',
                borderRadius: 10,
                background: 'var(--semi-color-fill-0)',
              }}
            >
              <Text>
                {t('当前筛选命中')}: <b>{teamsTotal}</b>
              </Text>
              <Text>
                {t('已选中')}: <b>{selectedTeamRowKeys.length}</b>
              </Text>
              <Tag color={hasTeamFilters ? 'blue' : 'red'}>
                {hasTeamFilters ? t('当前为筛选范围') : t('当前为全量范围')}
              </Tag>
              <Text type='secondary'>
                {teamFilterSummary || t('未设置筛选条件，将作用于整个 Team 资产池')}
              </Text>
              <Text type='tertiary'>
                {teamBatchUpdateSummary
                  ? t('待应用字段') + ': ' + teamBatchUpdateSummary
                  : t('未设置待应用的批量更新字段')}
              </Text>
              <Button
                size='small'
                theme='light'
                disabled={selectedTeamRowKeys.length === 0}
                onClick={() => setSelectedTeamRowKeys([])}
              >
                {t('清空已选')}
              </Button>
            </div>

            <Table
              rowKey='id'
              rowSelection={teamRowSelection}
              columns={teamColumns}
              dataSource={teams}
              empty={<Empty description={t('暂无 Team 资产')} />}
              pagination={{
                currentPage: teamQuery.page,
                pageSize: teamQuery.page_size,
                total: teamsTotal,
                onPageChange: async (page) => {
                  const next = { ...teamQuery, page };
                  setTeamQuery(next);
                  await fetchTeams(next);
                },
                onPageSizeChange: async (pageSize) => {
                  const next = { ...teamQuery, page: 1, page_size: pageSize };
                  setTeamQuery(next);
                  await fetchTeams(next);
                },
              }}
            />

            <div style={{ marginTop: 10 }}>
              <Text type='tertiary'>
                {t('状态分布')}: active {Number(teamByStatus?.active) || 0} /
                full {Number(teamByStatus?.full) || 0} / expired{' '}
                {Number(teamByStatus?.expired) || 0} / error{' '}
                {Number(teamByStatus?.error) || 0}
              </Text>
            </div>
          </Card>

          <Card title={t('执行历史')} bodyStyle={{ padding: '14px' }}>
            <div
              style={{
                marginBottom: 12,
                display: 'flex',
                flexDirection: 'column',
                gap: '12px',
              }}
            >
              <div
                style={{
                  display: 'flex',
                  gap: '8px',
                  flexWrap: 'wrap',
                }}
              >
                <Card
                  bodyStyle={{ padding: '10px 12px' }}
                  style={{
                    cursor: 'pointer',
                    border:
                      !historyQuery.status &&
                      !historyQuery.module &&
                      !historyQuery.search &&
                      historyQuery.only_failed !== true &&
                      historyQuery.only_proxy_retried !== true
                        ? '1px solid var(--semi-color-primary)'
                        : undefined,
                  }}
                  onClick={() => resetHistoryQuery()}
                >
                  <Text type='secondary'>{t('当前页总数')}</Text>
                  <div style={{ fontSize: 18, fontWeight: 700 }}>
                    {historyStats.total}
                  </div>
                </Card>
                <Card
                  bodyStyle={{ padding: '10px 12px' }}
                  style={{
                    cursor: 'pointer',
                    border:
                      historyQuery.only_failed || historyQuery.status === 'failed'
                        ? '1px solid var(--semi-color-danger)'
                        : undefined,
                  }}
                  onClick={() =>
                    applyHistoryQuery({
                      status: 'failed',
                      only_failed: true,
                      only_proxy_retried: false,
                    })
                  }
                >
                  <Text type='secondary'>{t('失败执行')}</Text>
                  <div style={{ fontSize: 18, fontWeight: 700 }}>
                    {historyStats.failed}
                  </div>
                </Card>
                <Card
                  bodyStyle={{ padding: '10px 12px' }}
                  style={{
                    cursor: 'pointer',
                    border: historyQuery.only_proxy_retried
                      ? '1px solid var(--semi-color-primary)'
                      : undefined,
                  }}
                  onClick={() =>
                    applyHistoryQuery({
                      only_proxy_retried: true,
                      only_failed: false,
                      status: '',
                    })
                  }
                >
                  <Text type='secondary'>{t('触发代理重试')}</Text>
                  <div style={{ fontSize: 18, fontWeight: 700 }}>
                    {historyStats.proxyRetried}
                  </div>
                </Card>
                <Card
                  bodyStyle={{ padding: '10px 12px' }}
                  style={{
                    cursor: 'pointer',
                    border: historyQuery.status === 'completed'
                      ? '1px solid var(--semi-color-success)'
                      : undefined,
                  }}
                  onClick={() =>
                    applyHistoryQuery({
                      status: 'completed',
                      only_failed: false,
                    })
                  }
                >
                  <Text type='secondary'>{t('已完成')}</Text>
                  <div style={{ fontSize: 18, fontWeight: 700 }}>
                    {historyStats.completed}
                  </div>
                </Card>
              </div>
              {historyFailureReasonCards.length > 0 ? (
                <div
                  style={{
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 8,
                  }}
                >
                  <Text strong>{t('失败原因聚合')}</Text>
                  <div
                    style={{
                      display: 'flex',
                      gap: '8px',
                      flexWrap: 'wrap',
                    }}
                  >
                    {historyFailureReasonCards.map((item) => (
                      <Button
                        key={`history-failure-${item.term}`}
                        size='small'
                        theme='light'
                        type='warning'
                        onClick={() => applyHistoryFailureReason(item.term)}
                      >
                        {item.label} x{item.count}
                      </Button>
                    ))}
                    <Button
                      size='small'
                      theme='outline'
                      onClick={() => resetHistoryQuery()}
                    >
                      {t('清空失败筛选')}
                    </Button>
                  </div>
                </div>
              ) : null}
              <Space wrap>
                <Button
                  type='danger'
                  icon={<Trash2 size={14} />}
                  disabled={selectedHistoryRowKeys.length === 0}
                  onClick={onBatchDeleteHistorySelected}
                >
                  {t('删除所选')}
                </Button>
                <Button
                  theme='light'
                  type='warning'
                  disabled={historyTotal <= 0}
                  onClick={onBatchDeleteHistoryByFilter}
                >
                  {t('清理当前筛选')}
                </Button>
                <Select
                  style={{ width: 180 }}
                  optionList={EXECUTION_STATUS_OPTIONS}
                  value={historyQuery.status}
                  onChange={(value) =>
                    applyHistoryQuery({
                      status: value,
                      only_failed: value === 'failed',
                    })
                  }
                />
                <Select
                  style={{ width: 220 }}
                  optionList={historyModuleOptions}
                  value={historyQuery.module}
                  onChange={(value) => applyHistoryQuery({ module: value })}
                />
                <Input
                  style={{ width: 260 }}
                  value={historyQuery.search}
                  onChange={(value) =>
                    setHistoryQuery((prev) => ({ ...prev, search: value }))
                  }
                  placeholder={t('搜索执行ID / 模块 / 失败摘要')}
                  prefix={<Search size={14} />}
                  onEnterPress={() =>
                    applyHistoryQuery({ search: historyQuery.search })
                  }
                />
                <Button
                  icon={<Search size={14} />}
                  onClick={() =>
                    applyHistoryQuery({ search: historyQuery.search })
                  }
                >
                  {t('查询')}
                </Button>
                <Button theme='light' onClick={() => resetHistoryQuery()}>
                  {t('清空筛选')}
                </Button>
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    padding: '0 4px',
                  }}
                >
                  <Text type='secondary'>{t('只看失败')}</Text>
                  <Switch
                    checked={historyQuery.only_failed === true}
                    onChange={(checked) =>
                      applyHistoryQuery({
                        only_failed: checked,
                        status: checked ? 'failed' : '',
                      })
                    }
                  />
                </div>
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    padding: '0 4px',
                  }}
                >
                  <Text type='secondary'>{t('只看代理重试')}</Text>
                  <Switch
                    checked={historyQuery.only_proxy_retried === true}
                    onChange={(checked) =>
                      applyHistoryQuery({ only_proxy_retried: checked })
                    }
                  />
                </div>
                <Text type='tertiary'>
                  {t('当前页')}: {history.length} / {t('总匹配')}: {historyTotal}
                </Text>
              </Space>
              <div
                style={{
                  display: 'flex',
                  gap: '8px',
                  flexWrap: 'wrap',
                  alignItems: 'center',
                  padding: '10px 12px',
                  border: '1px solid var(--semi-color-border)',
                  borderRadius: 10,
                  background: 'var(--semi-color-fill-0)',
                }}
              >
                <Text>
                  {t('已选中')}: <b>{selectedHistoryRowKeys.length}</b>
                </Text>
                <Tag color={hasHistoryFilters ? 'blue' : 'red'}>
                  {hasHistoryFilters ? t('当前为筛选范围') : t('当前为全量范围')}
                </Tag>
                <Text type='secondary'>
                  {historyFilterSummary ||
                    t('未设置筛选条件，清理当前筛选会作用于全部执行历史')}
                </Text>
                <Button
                  size='small'
                  theme='light'
                  disabled={selectedHistoryRowKeys.length === 0}
                  onClick={() => setSelectedHistoryRowKeys([])}
                >
                  {t('清空已选')}
                </Button>
              </div>
            </div>
            <Table
              rowKey='execution_id'
              rowSelection={historyRowSelection}
              columns={historyColumns}
              dataSource={history}
              empty={<Empty description={t('暂无执行历史')} />}
              pagination={{
                currentPage: historyQuery.page,
                pageSize: historyQuery.page_size,
                total: historyTotal,
                onPageChange: async (page) => {
                  const next = { ...historyQuery, page };
                  setHistoryQuery(next);
                  await fetchHistory(next);
                },
                onPageSizeChange: async (pageSize) => {
                  const next = {
                    ...historyQuery,
                    page: 1,
                    page_size: pageSize,
                  };
                  setHistoryQuery(next);
                  await fetchHistory(next);
                },
              }}
            />
          </Card>

          <Card title={t('执行详情 / 日志')} bodyStyle={{ padding: '14px' }}>
            {!selectedExecutionId ? (
              <Empty description={t('请选择一条执行记录')} />
            ) : (
              <div
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '12px',
                }}
              >
                <div>
                  <Text strong>
                    {t('执行ID')}: {selectedExecutionId}
                  </Text>
                </div>
                {executionDetail ? (
                  <div
                    style={{
                      display: 'grid',
                      gridTemplateColumns:
                        'repeat(auto-fit, minmax(220px, 1fr))',
                      gap: '12px',
                    }}
                  >
                    <div>
                      <Text type='secondary'>{t('状态')}</Text>
                      <div style={{ marginTop: 4 }}>
                        <StatusTag value={executionDetail?.status} />
                      </div>
                    </div>
                    <div>
                      <Text type='secondary'>{t('模块')}</Text>
                      <div style={{ marginTop: 4 }}>
                        {executionDetail?.module || '-'}
                      </div>
                    </div>
                    <div>
                      <Text type='secondary'>{t('成功/失败')}</Text>
                      <div style={{ marginTop: 4 }}>
                        {Number(executionDetail?.success_count) || 0}/
                        {Number(executionDetail?.failed_count) || 0}
                      </div>
                    </div>
                    <div>
                      <Text type='secondary'>{t('进度')}</Text>
                      <div style={{ marginTop: 4 }}>
                        <Progress
                          percent={Number(executionDetail?.progress) || 0}
                          showInfo
                          size='small'
                        />
                      </div>
                    </div>
                    <div>
                      <Text type='secondary'>{t('开始时间')}</Text>
                      <div style={{ marginTop: 4 }}>
                        {formatTimestamp(executionDetail?.started_at)}
                      </div>
                    </div>
                    <div>
                      <Text type='secondary'>{t('结束时间')}</Text>
                      <div style={{ marginTop: 4 }}>
                        {formatTimestamp(executionDetail?.finished_at)}
                      </div>
                    </div>
                  </div>
                ) : null}
                {executionSummary.rows.length > 0 ? (
                  <div>
                    <Text strong>{t('执行摘要')}</Text>
                    <div
                      style={{
                        marginTop: 8,
                        display: 'grid',
                        gridTemplateColumns:
                          'repeat(auto-fit, minmax(220px, 1fr))',
                        gap: '12px',
                      }}
                    >
                      <Card bodyStyle={{ padding: '12px' }}>
                        <Text type='secondary'>{t('账号条目')}</Text>
                        <div style={{ fontSize: 24, fontWeight: 700 }}>
                          {executionSummary.rows.length}
                        </div>
                      </Card>
                      <Card bodyStyle={{ padding: '12px' }}>
                        <Text type='secondary'>{t('失败条目')}</Text>
                        <div style={{ fontSize: 24, fontWeight: 700 }}>
                          {executionSummary.failedRows.length}
                        </div>
                      </Card>
                    </div>
                    {executionSummary.failedRows.length > 0 ? (
                      <div
                        style={{
                          marginTop: 10,
                          display: 'flex',
                          flexDirection: 'column',
                          gap: '8px',
                        }}
                      >
                        {executionSummary.failedRows
                          .slice(0, 12)
                          .map((item) => (
                            <div
                              key={item.key}
                              style={{
                                border: '1px solid var(--semi-color-border)',
                                borderRadius: 8,
                                padding: 10,
                                background: 'var(--semi-color-bg-1)',
                              }}
                            >
                              <div
                                style={{
                                  display: 'flex',
                                  justifyContent: 'space-between',
                                  gap: 8,
                                  alignItems: 'flex-start',
                                  flexWrap: 'wrap',
                                }}
                              >
                                <div>
                                  <Text>
                                    #{item.accountId || '-'} [{item.module || '-'}]{' '}
                                    {item.error || t('执行失败')}
                                  </Text>
                                  <div style={{ marginTop: 4 }}>
                                    <Tag color='red'>{t('失败账号')}</Tag>
                                    <Tag color='blue'>
                                      {t('账号 ID')}: {item.accountId || '-'}
                                    </Tag>
                                  </div>
                                </div>
                                <Space wrap>
                                  <Button
                                    size='small'
                                    theme='light'
                                    onClick={() =>
                                      filterExecutionLogsByAccount(item.accountId)
                                    }
                                  >
                                    {t('筛日志')}
                                  </Button>
                                </Space>
                              </div>
                            </div>
                          ))}
                      </div>
                    ) : (
                      <div style={{ marginTop: 8 }}>
                        <Text type='success'>{t('当前执行未发现失败条目。')}</Text>
                      </div>
                    )}
                  </div>
                ) : null}
                <div>
                  <Text strong>{t('最近日志')}</Text>
                  <div
                    style={{
                      marginTop: 8,
                      marginBottom: 8,
                      display: 'flex',
                      gap: '8px',
                      flexWrap: 'wrap',
                      alignItems: 'center',
                    }}
                  >
                    <Card bodyStyle={{ padding: '10px 12px' }}>
                      <Text type='secondary'>{t('总日志')}</Text>
                      <div style={{ fontSize: 18, fontWeight: 700 }}>
                        {executionLogTotal || executionLogStats.total}
                      </div>
                    </Card>
                    <Card bodyStyle={{ padding: '10px 12px' }}>
                      <Text type='secondary'>{t('Warning')}</Text>
                      <div style={{ fontSize: 18, fontWeight: 700 }}>
                        {executionLogStats.warning}
                      </div>
                    </Card>
                    <Card bodyStyle={{ padding: '10px 12px' }}>
                      <Text type='secondary'>{t('Error')}</Text>
                      <div style={{ fontSize: 18, fontWeight: 700 }}>
                        {executionLogStats.error}
                      </div>
                    </Card>
                    <Card bodyStyle={{ padding: '10px 12px' }}>
                      <Text type='secondary'>{t('代理相关')}</Text>
                      <div style={{ fontSize: 18, fontWeight: 700 }}>
                        {executionLogStats.proxy}
                      </div>
                    </Card>
                    <Card bodyStyle={{ padding: '10px 12px' }}>
                      <Text type='secondary'>{t('重试次数')}</Text>
                      <div style={{ fontSize: 18, fontWeight: 700 }}>
                        {executionLogStats.retry}
                      </div>
                    </Card>
                  </div>
                  <Space wrap>
                    <Select
                      style={{ width: 160 }}
                      optionList={EXECUTION_LOG_LEVEL_OPTIONS}
                      value={executionLogLevelFilter}
                      onChange={setExecutionLogLevelFilter}
                    />
                    <Input
                      style={{ width: 220 }}
                      value={executionLogSearch}
                      onChange={setExecutionLogSearch}
                      placeholder={t('搜索日志 / 代理 / 账号')}
                      prefix={<Search size={14} />}
                    />
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '8px',
                        padding: '0 4px',
                      }}
                    >
                      <Text type='secondary'>{t('只看代理相关')}</Text>
                      <Switch
                        checked={executionLogOnlyProxy}
                        onChange={setExecutionLogOnlyProxy}
                      />
                    </div>
                    <Text type='tertiary'>
                      {t('已加载')}: {executionLogs.length} /{' '}
                      {executionLogTotal || executionLogs.length}
                    </Text>
                    <Button
                      theme='light'
                      disabled={
                        executionLogLoading ||
                        executionLogLoadingMore ||
                        executionLogs.length >= executionLogTotal
                      }
                      loading={executionLogLoadingMore}
                      onClick={() => loadMoreExecutionLogs(false)}
                    >
                      {t('加载更多')}
                    </Button>
                    <Button
                      theme='outline'
                      disabled={
                        executionLogLoading ||
                        executionLogLoadingMore ||
                        executionLogs.length >= executionLogTotal
                      }
                      loading={executionLogLoading}
                      onClick={() => loadMoreExecutionLogs(true)}
                    >
                      {t('拉取全部')}
                    </Button>
                  </Space>
                  <div
                    style={{
                      marginTop: 8,
                      maxHeight: 260,
                      overflow: 'auto',
                      border: '1px solid var(--semi-color-border)',
                      borderRadius: 8,
                      padding: 10,
                      background: 'var(--semi-color-fill-0)',
                    }}
                  >
                    {filteredExecutionLogs.length === 0 ? (
                      <Text type='tertiary'>{t('暂无日志')}</Text>
                    ) : (
                      filteredExecutionLogs.map((item) => {
                        const payload = parseJSONPayload(item.payload);
                        const payloadSummary = buildLogPayloadSummary(payload);
                        const proxyRelated = isProxyRelatedLog(item, payload);
                        const level = String(item.level || 'info').trim();
                        let itemBorder = '1px solid var(--semi-color-border)';
                        let itemBackground = 'transparent';
                        if (level === 'error') {
                          itemBorder = '1px solid rgba(var(--semi-red-5), 0.45)';
                          itemBackground = 'rgba(var(--semi-red-0), 1)';
                        } else if (level === 'warning') {
                          itemBorder =
                            '1px solid rgba(var(--semi-orange-5), 0.45)';
                          itemBackground = 'rgba(var(--semi-orange-0), 1)';
                        } else if (proxyRelated) {
                          itemBorder =
                            '1px solid rgba(var(--semi-blue-5), 0.35)';
                          itemBackground = 'rgba(var(--semi-blue-0), 1)';
                        }
                        return (
                          <div
                            key={item.id}
                            style={{
                              marginBottom: 8,
                              padding: 8,
                              borderRadius: 8,
                              border: itemBorder,
                              background: itemBackground,
                              paddingBottom: 8,
                            }}
                          >
                            <Text>
                              [{formatTimestamp(item.created_at)}] [
                              {item.level || 'info'}] [{item.step || '-'}]{' '}
                              {item.message || '-'}
                            </Text>
                            <div style={{ marginTop: 2 }}>
                              <Text type='tertiary'>
                                account #{Number(item.account_id) || 0}
                              </Text>
                            </div>
                            {payloadSummary.length > 0 ? (
                              <div style={{ marginTop: 2 }}>
                                <Text type='secondary'>
                                  {payloadSummary.join(' | ')}
                                </Text>
                              </div>
                            ) : null}
                            {!payloadSummary.length && payload ? (
                              <div style={{ marginTop: 2 }}>
                                <Text type='tertiary'>
                                  {shortenText(JSON.stringify(payload), 200)}
                                </Text>
                              </div>
                            ) : null}
                          </div>
                        );
                      })
                    )}
                  </div>
                </div>
              </div>
            )}
          </Card>

          <Card title={t('账号管理')} bodyStyle={{ padding: '14px' }}>
            <Banner
              type='info'
              closeIcon={null}
              style={{ marginBottom: 12 }}
              description={
                <div
                  style={{ display: 'flex', flexDirection: 'column', gap: 4 }}
                >
                  <Text>
                    {t(
                      '导入账号：每行 `邮箱,密码,兑换码[,站点]`，导入后进入号池。',
                    )}
                  </Text>
                  <Text>
                    {t(
                      '补充兑换码：每行 `已有账号邮箱或ID,兑换码[,站点]`，用于后补兑换码。',
                    )}
                  </Text>
                  <Text>
                    {t('新增账号：适合手动补单个账号；CSV 适合从旧表迁移。')}
                  </Text>
                </div>
              }
            />

            <div
              style={{
                marginBottom: 12,
                padding: '10px 12px',
                borderRadius: 10,
                background:
                  'linear-gradient(135deg, rgba(15,23,42,0.03), rgba(16,185,129,0.07))',
                border: '1px solid rgba(148,163,184,0.18)',
                display: 'flex',
                justifyContent: 'space-between',
                gap: '10px',
                flexWrap: 'wrap',
                alignItems: 'center',
              }}
            >
              <div>
                <Text strong>{t('账号快捷筛选')}</Text>
                <div style={{ marginTop: 4 }}>
                  <Text type='secondary'>
                    {t('快速切到待邀请、待导入、失败和人工复核账号，减少手动翻页排查。')}
                  </Text>
                </div>
              </div>
              <Space wrap>
                {ACCOUNT_QUICK_FILTERS.map((item) => (
                  <Button
                    key={item.key}
                    size='small'
                    theme={
                      String(accountQuery.status || '').trim() === item.value
                        ? 'solid'
                        : 'light'
                    }
                    type={
                      item.value === 'failed' ||
                      item.value === 'manual_review'
                        ? 'warning'
                        : 'tertiary'
                    }
                    onClick={() => applyAccountQuickStatusFilter(item.value)}
                  >
                    {t(item.label)}
                  </Button>
                ))}
                <Button
                  size='small'
                  theme='outline'
                  onClick={() => applyAccountQuickStatusFilter('')}
                >
                  {t('全部账号')}
                </Button>
              </Space>
            </div>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                gap: '12px',
                flexWrap: 'wrap',
                marginBottom: 12,
              }}
            >
              <Space>
                <Button
                  theme='solid'
                  type='primary'
                  icon={<Plus size={15} />}
                  onClick={openCreateAccount}
                >
                  {t('新增账号')}
                </Button>
                <Button
                  icon={<FileUp size={15} />}
                  onClick={() => importRef.current?.click()}
                >
                  {t('导入 CSV')}
                </Button>
                <Button
                  icon={<FileUp size={15} />}
                  onClick={openRedeemImportModal}
                >
                  {t('补充兑换码')}
                </Button>
                <Button
                  icon={<FileUp size={15} />}
                  onClick={openRegisteredImportModal}
                >
                  {t('导入账号')}
                </Button>
                <Button
                  icon={<Download size={15} />}
                  onClick={() => {
                    const url =
                      buildTeamAutomationAccountsExportURL(accountQuery);
                    window.open(url, '_blank');
                  }}
                >
                  {t('导出 CSV')}
                </Button>
                <Button
                  type='danger'
                  icon={<Trash2 size={15} />}
                  disabled={selectedAccountRowKeys.length === 0}
                  onClick={onBatchDeleteAccounts}
                >
                  {t('批量删除')}
                </Button>
                <Button
                  theme='light'
                  type='warning'
                  disabled={accountsTotal <= 0}
                  onClick={onBatchDeleteAccountsByFilter}
                >
                  {t('清理当前筛选')}
                </Button>
              </Space>
              <Space>
                <Select
                  style={{ width: 180 }}
                  optionList={ACCOUNT_STATUS_OPTIONS.filter(
                    (item) => item.value !== '',
                  )}
                  value={batchAccountStatus}
                  onChange={setBatchAccountStatus}
                  placeholder={t('批量设置状态')}
                />
                <Input
                  style={{ width: 180 }}
                  value={batchAccountRedeemSite}
                  onChange={setBatchAccountRedeemSite}
                  placeholder={t('批量设置兑换站点')}
                />
                <Input
                  style={{ width: 180 }}
                  value={batchAccountMetadata}
                  onChange={setBatchAccountMetadata}
                  placeholder={t('批量追加标记')}
                />
                <Button
                  disabled={
                    selectedAccountRowKeys.length === 0 ||
                    !hasAccountBatchUpdateFields
                  }
                  onClick={onBatchUpdateAccounts}
                >
                  {t('批量更新')}
                </Button>
                <Button
                  theme='light'
                  type='warning'
                  disabled={accountsTotal <= 0 || !hasAccountBatchUpdateFields}
                  onClick={onBatchUpdateAccountsByFilter}
                >
                  {t('更新当前筛选')}
                </Button>
                <Select
                  style={{ width: 180 }}
                  optionList={ACCOUNT_STATUS_OPTIONS}
                  value={accountDraft.status}
                  onChange={(value) =>
                    setAccountDraft((prev) => ({ ...prev, status: value }))
                  }
                />
                <Input
                  prefix={<Search size={14} />}
                  value={accountDraft.search}
                  onChange={(value) =>
                    setAccountDraft((prev) => ({ ...prev, search: value }))
                  }
                  placeholder={t('搜索邮箱/兑换码')}
                />
                <Button
                  icon={<RefreshCw size={15} />}
                  onClick={async () => {
                    const next = {
                      ...accountQuery,
                      page: 1,
                      status: accountDraft.status,
                      search: accountDraft.search,
                    };
                    setAccountQuery(next);
                    await fetchAccounts(next);
                  }}
                >
                  {t('筛选')}
                </Button>
                <Button
                  theme='light'
                  onClick={async () => {
                    const next = {
                      ...accountQuery,
                      page: 1,
                      status: '',
                      search: '',
                    };
                    setAccountDraft({ status: '', search: '' });
                    setAccountQuery(next);
                    await fetchAccounts(next);
                  }}
                >
                  {t('清空筛选')}
                </Button>
              </Space>
            </div>
            <div
              style={{
                marginBottom: 12,
                display: 'flex',
                gap: '8px',
                flexWrap: 'wrap',
                alignItems: 'center',
                padding: '10px 12px',
                border: '1px solid var(--semi-color-border)',
                borderRadius: 10,
                background: 'var(--semi-color-fill-0)',
              }}
            >
              <Text>
                {t('当前筛选命中')}: <b>{accountsTotal}</b>
              </Text>
              <Text>
                {t('已选中')}: <b>{selectedAccountRowKeys.length}</b>
              </Text>
              <Tag color={hasAccountFilters ? 'blue' : 'red'}>
                {hasAccountFilters ? t('当前为筛选范围') : t('当前为全量范围')}
              </Tag>
              <Text type='secondary'>
                {accountFilterSummary || t('未设置筛选条件，将作用于整个账号池')}
              </Text>
              <Text type='tertiary'>
                {accountBatchUpdateSummary
                  ? t('待应用字段') + ': ' + accountBatchUpdateSummary
                  : t('未设置待应用的批量更新字段')}
              </Text>
              <Button
                size='small'
                theme='light'
                disabled={selectedAccountRowKeys.length === 0}
                onClick={() => setSelectedAccountRowKeys([])}
              >
                {t('清空已选')}
              </Button>
            </div>

            <Table
              rowKey='id'
              rowSelection={accountRowSelection}
              columns={accountColumns}
              dataSource={accounts}
              empty={<Empty description={t('暂无账号')} />}
              pagination={{
                currentPage: accountQuery.page,
                pageSize: accountQuery.page_size,
                total: accountsTotal,
                onPageChange: async (page) => {
                  const next = { ...accountQuery, page };
                  setAccountQuery(next);
                  await fetchAccounts(next);
                },
                onPageSizeChange: async (pageSize) => {
                  const next = {
                    ...accountQuery,
                    page: 1,
                    page_size: pageSize,
                  };
                  setAccountQuery(next);
                  await fetchAccounts(next);
                },
              }}
            />
          </Card>
        </div>
      </Spin>

      <Modal
        title={accountModal.mode === 'create' ? t('新增账号') : t('编辑账号')}
        visible={accountModal.visible}
        onCancel={() =>
          setAccountModal({
            visible: false,
            mode: 'create',
            submitting: false,
            form: DEFAULT_ACCOUNT_FORM,
          })
        }
        onOk={saveAccount}
        confirmLoading={accountModal.submitting}
        okText={t('保存')}
        cancelText={t('取消')}
      >
        <div
          style={{ display: 'grid', gridTemplateColumns: '1fr', gap: '10px' }}
        >
          <div>
            <Text strong>{t('名称')}</Text>
            <Input
              style={{ marginTop: 6 }}
              value={accountModal.form.name}
              onChange={(value) =>
                setAccountModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, name: value },
                }))
              }
            />
          </div>
          <div>
            <Text strong>{t('邮箱')}</Text>
            <Input
              style={{ marginTop: 6 }}
              value={accountModal.form.email}
              onChange={(value) =>
                setAccountModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, email: value },
                }))
              }
              disabled={accountModal.mode === 'edit'}
            />
          </div>
          <div>
            <Text strong>{t('密码')}</Text>
            <Input
              style={{ marginTop: 6 }}
              mode='password'
              autoComplete='new-password'
              value={accountModal.form.password}
              onChange={(value) =>
                setAccountModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, password: value },
                }))
              }
              placeholder={
                accountModal.mode === 'edit' ? t('留空则不更新') : ''
              }
            />
          </div>
          <div>
            <Text strong>{t('兑换码')}</Text>
            <Input
              style={{ marginTop: 6 }}
              value={accountModal.form.redeem_code}
              onChange={(value) =>
                setAccountModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, redeem_code: value },
                }))
              }
            />
          </div>
          <div>
            <Text strong>{t('状态')}</Text>
            <Select
              style={{ marginTop: 6, width: '100%' }}
              optionList={ACCOUNT_STATUS_OPTIONS.filter(
                (item) => item.value !== '',
              )}
              value={accountModal.form.status}
              onChange={(value) =>
                setAccountModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, status: value },
                }))
              }
            />
          </div>
          <div>
            <Text strong>{t('兑换站点')}</Text>
            <Input
              style={{ marginTop: 6 }}
              value={accountModal.form.redeem_site}
              onChange={(value) =>
                setAccountModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, redeem_site: value },
                }))
              }
            />
          </div>
          <div>
            <Text strong>{t('Mailbox ID')}</Text>
            <InputNumber
              style={{ marginTop: 6, width: '100%' }}
              min={0}
              value={Number(accountModal.form.mailbox_id) || 0}
              onChange={(value) =>
                setAccountModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, mailbox_id: Number(value) || 0 },
                }))
              }
            />
          </div>
          <div>
            <Text strong>{t('Metadata')}</Text>
            <TextArea
              autosize={{ minRows: 3, maxRows: 6 }}
              style={{ marginTop: 6 }}
              value={accountModal.form.metadata}
              onChange={(value) =>
                setAccountModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, metadata: value },
                }))
              }
            />
          </div>
        </div>
      </Modal>

      <Modal
        title={t('补充兑换码')}
        visible={redeemImportModal.visible}
        onCancel={() =>
          setRedeemImportModal({
            visible: false,
            submitting: false,
            form: DEFAULT_REDEEM_IMPORT_FORM,
          })
        }
        onOk={onImportRedeemCodes}
        confirmLoading={redeemImportModal.submitting}
        okText={t('导入到号池')}
        cancelText={t('取消')}
      >
        <div style={{ display: 'grid', gridTemplateColumns: '1fr', gap: 10 }}>
          <div>
            <Text strong>{t('默认兑换站点（可选）')}</Text>
            <Input
              style={{ marginTop: 6 }}
              value={redeemImportModal.form.default_redeem_site}
              onChange={(value) =>
                setRedeemImportModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, default_redeem_site: value },
                }))
              }
              placeholder='helloteam.store'
            />
          </div>
          <div>
            <Text strong>{t('兑换码数据')}</Text>
            <TextArea
              autosize={{ minRows: 8, maxRows: 14 }}
              style={{ marginTop: 6 }}
              value={redeemImportModal.form.lines}
              onChange={(value) =>
                setRedeemImportModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, lines: value },
                }))
              }
              placeholder={`foo@example.com,TEAM-AAA
foo2@example.com,TEAM-BBB,helloteam.store
12,TEAM-CCC,teamxz.store`}
            />
            <div style={{ marginTop: 6 }}>
              <Text type='tertiary'>
                {t(
                  '每行一条：已有账号邮箱,兑换码[,站点] 或 账号ID,兑换码[,站点]',
                )}
              </Text>
            </div>
          </div>
        </div>
      </Modal>

      <Modal
        title={
          teamModal.mode === 'create'
            ? t('新增 Team 资产')
            : t('编辑 Team 资产')
        }
        visible={teamModal.visible}
        onCancel={() =>
          setTeamModal({
            visible: false,
            mode: 'create',
            form: DEFAULT_TEAM_FORM,
            submitting: false,
          })
        }
        onOk={saveTeam}
        confirmLoading={teamModal.submitting}
        okText={t('保存')}
        cancelText={t('取消')}
      >
        <div style={{ display: 'grid', gridTemplateColumns: '1fr', gap: 10 }}>
          <div>
            <Text strong>{t('邮箱')}</Text>
            <Input
              style={{ marginTop: 6 }}
              value={teamModal.form.email}
              onChange={(value) =>
                setTeamModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, email: value },
                }))
              }
              disabled={teamModal.mode === 'edit'}
            />
          </div>
          <div>
            <Text strong>{t('Access Token')}</Text>
            <TextArea
              autosize={{ minRows: 2, maxRows: 4 }}
              style={{ marginTop: 6 }}
              value={teamModal.form.access_token}
              onChange={(value) =>
                setTeamModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, access_token: value },
                }))
              }
              placeholder={teamModal.mode === 'edit' ? t('留空则不更新') : ''}
            />
          </div>
          <div>
            <Text strong>{t('Session Token')}</Text>
            <Input
              style={{ marginTop: 6 }}
              value={teamModal.form.session_token}
              onChange={(value) =>
                setTeamModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, session_token: value },
                }))
              }
              placeholder={teamModal.mode === 'edit' ? t('留空则不更新') : ''}
            />
          </div>
          <div>
            <Text strong>{t('Refresh Token')}</Text>
            <Input
              style={{ marginTop: 6 }}
              value={teamModal.form.refresh_token}
              onChange={(value) =>
                setTeamModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, refresh_token: value },
                }))
              }
              placeholder={teamModal.mode === 'edit' ? t('留空则不更新') : ''}
            />
          </div>
          <div>
            <Text strong>{t('Client ID')}</Text>
            <Input
              style={{ marginTop: 6 }}
              value={teamModal.form.client_id}
              onChange={(value) =>
                setTeamModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, client_id: value },
                }))
              }
              placeholder={teamModal.mode === 'edit' ? t('留空则不更新') : ''}
            />
          </div>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(2, 1fr)',
              gap: 10,
            }}
          >
            <div>
              <Text strong>{t('Team 名称')}</Text>
              <Input
                style={{ marginTop: 6 }}
                value={teamModal.form.team_name}
                onChange={(value) =>
                  setTeamModal((prev) => ({
                    ...prev,
                    form: { ...prev.form, team_name: value },
                  }))
                }
              />
            </div>
            <div>
              <Text strong>{t('Account ID')}</Text>
              <Input
                style={{ marginTop: 6 }}
                value={teamModal.form.account_id}
                onChange={(value) =>
                  setTeamModal((prev) => ({
                    ...prev,
                    form: { ...prev.form, account_id: value },
                  }))
                }
              />
            </div>
          </div>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(3, 1fr)',
              gap: 10,
            }}
          >
            <div>
              <Text strong>{t('当前成员')}</Text>
              <InputNumber
                style={{ marginTop: 6, width: '100%' }}
                min={0}
                value={Number(teamModal.form.current_members) || 0}
                onChange={(value) =>
                  setTeamModal((prev) => ({
                    ...prev,
                    form: { ...prev.form, current_members: Number(value) || 0 },
                  }))
                }
              />
            </div>
            <div>
              <Text strong>{t('待邀请')}</Text>
              <InputNumber
                style={{ marginTop: 6, width: '100%' }}
                min={0}
                value={Number(teamModal.form.pending_invites) || 0}
                onChange={(value) =>
                  setTeamModal((prev) => ({
                    ...prev,
                    form: { ...prev.form, pending_invites: Number(value) || 0 },
                  }))
                }
              />
            </div>
            <div>
              <Text strong>{t('最大成员')}</Text>
              <InputNumber
                style={{ marginTop: 6, width: '100%' }}
                min={1}
                value={Number(teamModal.form.max_members) || 6}
                onChange={(value) =>
                  setTeamModal((prev) => ({
                    ...prev,
                    form: { ...prev.form, max_members: Number(value) || 6 },
                  }))
                }
              />
            </div>
          </div>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(3, 1fr)',
              gap: 10,
            }}
          >
            <div>
              <Text strong>{t('状态')}</Text>
              <Select
                style={{ marginTop: 6, width: '100%' }}
                optionList={TEAM_STATUS_OPTIONS.filter(
                  (item) => item.value !== '',
                )}
                value={teamModal.form.status}
                onChange={(value) =>
                  setTeamModal((prev) => ({
                    ...prev,
                    form: { ...prev.form, status: value },
                  }))
                }
              />
            </div>
            <div>
              <Text strong>{t('Plan Type')}</Text>
              <Input
                style={{ marginTop: 6 }}
                value={teamModal.form.plan_type}
                onChange={(value) =>
                  setTeamModal((prev) => ({
                    ...prev,
                    form: { ...prev.form, plan_type: value },
                  }))
                }
              />
            </div>
            <div>
              <Text strong>{t('Subscription')}</Text>
              <Input
                style={{ marginTop: 6 }}
                value={teamModal.form.subscription_plan}
                onChange={(value) =>
                  setTeamModal((prev) => ({
                    ...prev,
                    form: { ...prev.form, subscription_plan: value },
                  }))
                }
              />
            </div>
          </div>
          <div>
            <Text strong>{t('Metadata')}</Text>
            <TextArea
              autosize={{ minRows: 3, maxRows: 6 }}
              style={{ marginTop: 6 }}
              value={teamModal.form.metadata}
              onChange={(value) =>
                setTeamModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, metadata: value },
                }))
              }
            />
          </div>
        </div>
      </Modal>

      <Modal
        title={t('导入 Team 资产')}
        visible={teamImportModal.visible}
        onCancel={() =>
          setTeamImportModal({
            visible: false,
            form: DEFAULT_TEAM_IMPORT_FORM,
            submitting: false,
          })
        }
        onOk={onImportTeams}
        confirmLoading={teamImportModal.submitting}
        okText={t('导入')}
        cancelText={t('取消')}
      >
        <div style={{ display: 'grid', gridTemplateColumns: '1fr', gap: 10 }}>
          <div>
            <Text strong>{t('批量内容')}</Text>
            <TextArea
              autosize={{ minRows: 8, maxRows: 14 }}
              style={{ marginTop: 6 }}
              value={teamImportModal.form.content}
              onChange={(value) =>
                setTeamImportModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, content: value },
                }))
              }
              placeholder={`owner1@example.com,eyJ...,rt-xxx,st-xxx,client-id-1
owner2@example.com,eyJ...,,,client-id-2,account_123,Team Alpha,3,1,6,active`}
            />
            <div style={{ marginTop: 6 }}>
              <Text type='tertiary'>
                {t(
                  '支持逗号 / Tab / 竖线 / ---- 分隔；兼容 team-manage 的 single / batch 导入语义。',
                )}
              </Text>
            </div>
          </div>
        </div>
      </Modal>

      <Modal
        title={t('导入账号')}
        visible={registeredImportModal.visible}
        onCancel={() =>
          setRegisteredImportModal({
            visible: false,
            submitting: false,
            form: DEFAULT_REGISTERED_IMPORT_FORM,
          })
        }
        onOk={onImportRegisteredAccounts}
        confirmLoading={registeredImportModal.submitting}
        okText={t('导入到号池')}
        cancelText={t('取消')}
      >
        <div style={{ display: 'grid', gridTemplateColumns: '1fr', gap: 10 }}>
          <div>
            <Text strong>{t('默认兑换站点（可选）')}</Text>
            <Input
              style={{ marginTop: 6 }}
              value={registeredImportModal.form.default_redeem_site}
              onChange={(value) =>
                setRegisteredImportModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, default_redeem_site: value },
                }))
              }
              placeholder='helloteam.store'
            />
          </div>
          <div>
            <Text strong>{t('账号数据')}</Text>
            <TextArea
              autosize={{ minRows: 8, maxRows: 14 }}
              style={{ marginTop: 6 }}
              value={registeredImportModal.form.text}
              onChange={(value) =>
                setRegisteredImportModal((prev) => ({
                  ...prev,
                  form: { ...prev.form, text: value },
                }))
              }
              placeholder={`foo@example.com,Password123!,TEAM-AAA
foo2@example.com,Password456!,TEAM-BBB,helloteam.store`}
            />
            <div style={{ marginTop: 6 }}>
              <Text type='tertiary'>
                {t(
                  '每行一条：邮箱,密码,兑换码[,站点]。支持逗号、Tab、竖线分隔。',
                )}
              </Text>
            </div>
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default TeamAutomationPage;
