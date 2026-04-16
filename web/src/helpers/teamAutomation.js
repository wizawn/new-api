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

import { API } from './api';

export const TEAM_AUTOMATION_IMPORT_MAX_BYTES = 2 * 1024 * 1024;

export const unwrapTeamAutomationResponse = (
  response,
  fallbackMessage = '请求失败',
) => {
  const { success, message, data } = response.data;
  if (!success) {
    throw new Error(message || fallbackMessage);
  }
  return data;
};

const withSkipErrorHandler = (config = {}) => ({
  skipErrorHandler: true,
  ...config,
});

export const getTeamAutomationSettings = async () =>
  unwrapTeamAutomationResponse(
    await API.get('/api/team-automation/settings', withSkipErrorHandler()),
    '加载 Team 自动化配置失败',
  );

export const saveTeamAutomationSettings = async (payload) =>
  unwrapTeamAutomationResponse(
    await API.put(
      '/api/team-automation/settings',
      payload,
      withSkipErrorHandler(),
    ),
    '保存 Team 自动化配置失败',
  );

export const testTeamAutomationConnection = async (payload) =>
  unwrapTeamAutomationResponse(
    await API.post(
      '/api/team-automation/settings/test-connection',
      payload,
      withSkipErrorHandler(),
    ),
    '连接测试失败',
  );

export const getTeamAutomationHealth = async () =>
  unwrapTeamAutomationResponse(
    await API.get('/api/team-automation/health', withSkipErrorHandler()),
  );

export const getTeamAutomationMeta = async () =>
  unwrapTeamAutomationResponse(
    await API.get('/api/team-automation/meta', withSkipErrorHandler()),
  );

export const getTeamAutomationModules = async () =>
  unwrapTeamAutomationResponse(
    await API.get('/api/team-automation/modules', withSkipErrorHandler()),
  );

export const getTeamAutomationCapabilities = async () =>
  unwrapTeamAutomationResponse(
    await API.get('/api/team-automation/capabilities', withSkipErrorHandler()),
  );

export const getTeamAutomationStats = async () =>
  unwrapTeamAutomationResponse(
    await API.get('/api/team-automation/stats', withSkipErrorHandler()),
  );

export const getTeamAutomationCacheStats = async () =>
  unwrapTeamAutomationResponse(
    await API.get('/api/team-automation/cache/stats', withSkipErrorHandler()),
  );

export const getTeamAutomationAccountStats = async () =>
  unwrapTeamAutomationResponse(
    await API.get(
      '/api/team-automation/accounts/stats',
      withSkipErrorHandler(),
    ),
  );

export const getTeamAutomationTeamStats = async () =>
  unwrapTeamAutomationResponse(
    await API.get('/api/team-automation/teams/stats', withSkipErrorHandler()),
  );

export const listTeamAutomationTeams = async (params = {}) =>
  unwrapTeamAutomationResponse(
    await API.get(
      '/api/team-automation/teams',
      withSkipErrorHandler({ params }),
    ),
  );

export const getTeamAutomationTeam = async (identity) =>
  unwrapTeamAutomationResponse(
    await API.get(
      `/api/team-automation/teams/${encodeURIComponent(identity)}`,
      withSkipErrorHandler(),
    ),
  );

export const createTeamAutomationTeam = async (payload) =>
  unwrapTeamAutomationResponse(
    await API.post(
      '/api/team-automation/teams',
      payload,
      withSkipErrorHandler(),
    ),
  );

export const updateTeamAutomationTeam = async (identity, payload) =>
  unwrapTeamAutomationResponse(
    await API.put(
      `/api/team-automation/teams/${encodeURIComponent(identity)}`,
      payload,
      withSkipErrorHandler(),
    ),
  );

export const deleteTeamAutomationTeam = async (identity) =>
  unwrapTeamAutomationResponse(
    await API.delete(
      `/api/team-automation/teams/${encodeURIComponent(identity)}`,
      withSkipErrorHandler(),
    ),
  );

export const batchDeleteTeamAutomationTeams = async (payload) =>
  unwrapTeamAutomationResponse(
    await API.post(
      '/api/team-automation/teams/batch-delete',
      payload,
      withSkipErrorHandler(),
    ),
  );

export const batchUpdateTeamAutomationTeams = async (payload) =>
  unwrapTeamAutomationResponse(
    await API.post(
      '/api/team-automation/teams/batch-update',
      payload,
      withSkipErrorHandler(),
    ),
  );

export const buildTeamAutomationTeamsExportURL = (filters = {}) => {
  const params = new URLSearchParams();
  if (filters.status) {
    params.set('status', filters.status);
  }
  if (filters.search) {
    params.set('search', filters.search);
  }
  const query = params.toString();
  return `/api/team-automation/teams/export/csv${query ? `?${query}` : ''}`;
};

export const importTeamAutomationTeams = async (payload) =>
  unwrapTeamAutomationResponse(
    await API.post(
      '/api/team-automation/teams/import',
      payload,
      withSkipErrorHandler(),
    ),
  );

export const autoSelectTeamAutomationTeam = async (payload = {}) =>
  unwrapTeamAutomationResponse(
    await API.post(
      '/api/team-automation/teams/auto-select',
      payload,
      withSkipErrorHandler(),
    ),
  );

export const listTeamAutomationAccounts = async (params = {}) =>
  unwrapTeamAutomationResponse(
    await API.get(
      '/api/team-automation/accounts',
      withSkipErrorHandler({ params }),
    ),
  );

export const getTeamAutomationAccount = async (email) =>
  unwrapTeamAutomationResponse(
    await API.get(
      `/api/team-automation/accounts/${encodeURIComponent(email)}`,
      withSkipErrorHandler(),
    ),
  );

export const createTeamAutomationAccount = async (payload) =>
  unwrapTeamAutomationResponse(
    await API.post(
      '/api/team-automation/accounts',
      payload,
      withSkipErrorHandler(),
    ),
  );

export const updateTeamAutomationAccount = async (email, payload) =>
  unwrapTeamAutomationResponse(
    await API.put(
      `/api/team-automation/accounts/${encodeURIComponent(email)}`,
      payload,
      withSkipErrorHandler(),
    ),
  );

export const deleteTeamAutomationAccount = async (email) =>
  unwrapTeamAutomationResponse(
    await API.delete(
      `/api/team-automation/accounts/${encodeURIComponent(email)}`,
      withSkipErrorHandler(),
    ),
  );

export const batchDeleteTeamAutomationAccounts = async (payload) =>
  unwrapTeamAutomationResponse(
    await API.post(
      '/api/team-automation/accounts/batch-delete',
      payload,
      withSkipErrorHandler(),
    ),
  );

export const batchUpdateTeamAutomationAccounts = async (payload) =>
  unwrapTeamAutomationResponse(
    await API.post(
      '/api/team-automation/accounts/batch-update',
      payload,
      withSkipErrorHandler(),
    ),
  );

export const importTeamAutomationAccountsCSV = async (formData) =>
  unwrapTeamAutomationResponse(
    await API.post(
      '/api/team-automation/accounts/import/csv',
      formData,
      withSkipErrorHandler(),
    ),
  );

export const importRegisteredTeamAutomationAccounts = async (payload) =>
  unwrapTeamAutomationResponse(
    await API.post(
      '/api/team-automation/accounts/import/registered',
      payload,
      withSkipErrorHandler(),
    ),
  );

export const importTeamAutomationRedeemCodes = async (payload) =>
  unwrapTeamAutomationResponse(
    await API.post(
      '/api/team-automation/accounts/import/redeem-codes',
      payload,
      withSkipErrorHandler(),
    ),
  );

export const buildTeamAutomationAccountsExportURL = (filters = {}) => {
  const params = new URLSearchParams();
  if (filters.status) {
    params.set('status', filters.status);
  }
  if (filters.search) {
    params.set('search', filters.search);
  }
  const query = params.toString();
  return `/api/team-automation/accounts/export/csv${query ? `?${query}` : ''}`;
};

export const startTeamAutomationRun = async (payload) =>
  unwrapTeamAutomationResponse(
    await API.post('/api/team-automation/run', payload, withSkipErrorHandler()),
  );

export const getTeamAutomationRunHistory = async (params = {}) =>
  unwrapTeamAutomationResponse(
    await API.get(
      '/api/team-automation/run/history',
      withSkipErrorHandler({ params }),
    ),
  );

export const batchDeleteTeamAutomationRuns = async (payload) =>
  unwrapTeamAutomationResponse(
    await API.post(
      '/api/team-automation/run/batch-delete',
      payload,
      withSkipErrorHandler(),
    ),
  );

export const getTeamAutomationRun = async (executionId) =>
  unwrapTeamAutomationResponse(
    await API.get(
      `/api/team-automation/run/${executionId}`,
      withSkipErrorHandler(),
    ),
  );

export const getTeamAutomationRunLogs = async (executionId, params = {}) =>
  unwrapTeamAutomationResponse(
    await API.get(
      `/api/team-automation/run/${executionId}/logs`,
      withSkipErrorHandler({ params }),
    ),
  );

export const stopTeamAutomationRun = async (executionId) =>
  unwrapTeamAutomationResponse(
    await API.post(
      `/api/team-automation/run/${executionId}/stop`,
      {},
      withSkipErrorHandler(),
    ),
  );
