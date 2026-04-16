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

import { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, showWarning } from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { useTableCompactMode } from '../common/useTableCompactMode';

export const useUsersData = () => {
  const { t } = useTranslation();
  const [compactMode, setCompactMode] = useTableCompactMode('users');
  const normalizeStatusFilter = (value) => {
    const next = String(value || 'all').trim().toLowerCase();
    if (['enabled', 'disabled', 'deleted', 'all'].includes(next)) {
      return next;
    }
    return 'all';
  };
  const normalizeRoleFilter = (value) => {
    const next = String(value || 'all').trim().toLowerCase();
    if (['common', 'admin', 'root', 'all'].includes(next)) {
      return next;
    }
    return 'all';
  };

  // State management
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [searching, setSearching] = useState(false);
  const [groupOptions, setGroupOptions] = useState([]);
  const [userCount, setUserCount] = useState(0);
  const [selectedKeys, setSelectedKeys] = useState([]);
  const [statusFilter, setStatusFilter] = useState(
    normalizeStatusFilter(localStorage.getItem('users-status-filter') || 'all'),
  );
  const [roleFilter, setRoleFilter] = useState(
    normalizeRoleFilter(localStorage.getItem('users-role-filter') || 'all'),
  );

  // Modal states
  const [showAddUser, setShowAddUser] = useState(false);
  const [showEditUser, setShowEditUser] = useState(false);
  const [editingUser, setEditingUser] = useState({
    id: undefined,
  });

  // Form initial values
  const formInitValues = {
    searchKeyword: '',
    searchGroup: '',
  };

  // Form API reference
  const [formApi, setFormApi] = useState(null);

  // Get form values helper function
  const getFormValues = () => {
    const formValues = formApi ? formApi.getValues() : {};
    return {
      searchKeyword: formValues.searchKeyword || '',
      searchGroup: formValues.searchGroup || '',
    };
  };

  // Set user format with key field
  const setUserFormat = (users) => {
    for (let i = 0; i < users.length; i++) {
      users[i].key = users[i].id;
    }
    setUsers(users);
  };

  const rowSelection = {
    getCheckboxProps: (record) => ({
      disabled: record.DeletedAt !== null,
      name: record.username,
    }),
    selectedRowKeys: selectedKeys.map((user) => user.id),
    onChange: (selectedRowKeys, selectedRows) => {
      setSelectedKeys(selectedRows);
    },
  };

  const buildFilterQuery = (
    status = statusFilter,
    role = roleFilter,
  ) => {
    const params = [];
    const normalizedStatus = normalizeStatusFilter(status);
    const normalizedRole = normalizeRoleFilter(role);
    if (normalizedStatus !== 'all') {
      params.push(`status=${encodeURIComponent(normalizedStatus)}`);
    }
    if (normalizedRole !== 'all') {
      params.push(`role=${encodeURIComponent(normalizedRole)}`);
    }
    return params.length > 0 ? `&${params.join('&')}` : '';
  };

  // Load users data
  const loadUsers = async (
    startIdx = 1,
    pageSizeValue = pageSize,
    status = statusFilter,
    role = roleFilter,
  ) => {
    setLoading(true);
    const res = await API.get(
      `/api/user/?p=${startIdx}&page_size=${pageSizeValue}${buildFilterQuery(status, role)}`,
    );
    const { success, message, data } = res.data;
    if (success) {
      const newPageData = data.items;
      setActivePage(data.page);
      setUserCount(data.total);
      setUserFormat(newPageData);
    } else {
      showError(message);
    }
    setLoading(false);
  };

  // Search users with keyword and group
  const searchUsers = async (
    startIdx,
    pageSizeValue,
    searchKeyword = null,
    searchGroup = null,
    status = statusFilter,
    role = roleFilter,
  ) => {
    // If no parameters passed, get values from form
    if (searchKeyword === null || searchGroup === null) {
      const formValues = getFormValues();
      searchKeyword = formValues.searchKeyword;
      searchGroup = formValues.searchGroup;
    }

    if (searchKeyword === '' && searchGroup === '') {
      // If keyword is blank, load files instead
      await loadUsers(startIdx, pageSizeValue, status, role);
      return;
    }
    setSearching(true);
    const res = await API.get(
      `/api/user/search?keyword=${encodeURIComponent(searchKeyword)}&group=${encodeURIComponent(searchGroup)}&p=${startIdx}&page_size=${pageSizeValue}${buildFilterQuery(status, role)}`,
    );
    const { success, message, data } = res.data;
    if (success) {
      const newPageData = data.items;
      setActivePage(data.page);
      setUserCount(data.total);
      setUserFormat(newPageData);
    } else {
      showError(message);
    }
    setSearching(false);
  };

  // Manage user operations (promote, demote, enable, disable, delete)
  const manageUser = async (userId, action, record) => {
    // Trigger loading state to force table re-render
    setLoading(true);

    const res = await API.post('/api/user/manage', {
      id: userId,
      action,
    });

    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      const user = res.data.data;

      // Create a new array and new object to ensure React detects changes
      const newUsers = users.map((u) => {
        if (u.id === userId) {
          if (action === 'delete') {
            return { ...u, DeletedAt: new Date() };
          }
          return { ...u, status: user.status, role: user.role };
        }
        return u;
      });

      setUsers(newUsers);
    } else {
      showError(message);
    }

    setLoading(false);
  };

  const resetUserPasskey = async (user) => {
    if (!user) {
      return;
    }
    try {
      const res = await API.delete(`/api/user/${user.id}/reset_passkey`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('Passkey 已重置'));
      } else {
        showError(message || t('操作失败，请重试'));
      }
    } catch (error) {
      showError(t('操作失败，请重试'));
    }
  };

  const resetUserTwoFA = async (user) => {
    if (!user) {
      return;
    }
    try {
      const res = await API.delete(`/api/user/${user.id}/2fa`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('二步验证已重置'));
      } else {
        showError(message || t('操作失败，请重试'));
      }
    } catch (error) {
      showError(t('操作失败，请重试'));
    }
  };

  // Handle page change
  const handlePageChange = (page) => {
    setActivePage(page);
    const { searchKeyword, searchGroup } = getFormValues();
    if (searchKeyword === '' && searchGroup === '') {
      loadUsers(page, pageSize, statusFilter, roleFilter).then();
    } else {
      searchUsers(
        page,
        pageSize,
        searchKeyword,
        searchGroup,
        statusFilter,
        roleFilter,
      ).then();
    }
  };

  // Handle page size change
  const handlePageSizeChange = async (size) => {
    localStorage.setItem('page-size', size + '');
    setPageSize(size);
    setActivePage(1);
    const { searchKeyword, searchGroup } = getFormValues();
    try {
      if (searchKeyword === '' && searchGroup === '') {
        await loadUsers(1, size, statusFilter, roleFilter);
      } else {
        await searchUsers(1, size, searchKeyword, searchGroup, statusFilter, roleFilter);
      }
    } catch (reason) {
      showError(reason);
    }
  };

  // Handle table row styling for disabled/deleted users
  const handleRow = (record, index) => {
    if (record.DeletedAt !== null || record.status !== 1) {
      return {
        style: {
          background: 'var(--semi-color-disabled-border)',
        },
      };
    } else {
      return {};
    }
  };

  // Refresh data
  const refresh = async (page = activePage) => {
    const { searchKeyword, searchGroup } = getFormValues();
    if (searchKeyword === '' && searchGroup === '') {
      await loadUsers(page, pageSize, statusFilter, roleFilter);
    } else {
      await searchUsers(
        page,
        pageSize,
        searchKeyword,
        searchGroup,
        statusFilter,
        roleFilter,
      );
    }
    setSelectedKeys([]);
  };

  const getBatchManageUserActionLabel = (action) => {
    switch (action) {
      case 'enable':
        return t('启用');
      case 'disable':
        return t('禁用');
      case 'promote':
        return t('提升');
      case 'demote':
        return t('降级');
      case 'delete':
        return t('注销');
      default:
        return t('处理');
    }
  };

  const batchManageUsers = async (action) => {
    if (selectedKeys.length === 0) {
      showError(t('请至少选择一个用户！'));
      return;
    }
    if (!['enable', 'disable', 'promote', 'demote', 'delete'].includes(action)) {
      showError(t('不支持的批量操作'));
      return;
    }

    setLoading(true);
    try {
      const ids = selectedKeys.map((user) => user.id);
      const res = await API.post('/api/user/manage/batch', { ids, action });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('批量处理用户失败'));
        return;
      }

      const updatedCount = Number(data?.updated_count) || 0;
      const failedCount = Number(data?.failed_count) || 0;
      const actionLabel = getBatchManageUserActionLabel(action);

      if (failedCount > 0) {
        showWarning(
          t('批量{{action}}完成，成功 {{updatedCount}} 个，失败 {{failedCount}} 个', {
            action: actionLabel,
            updatedCount,
            failedCount,
          }),
        );
      } else {
        showSuccess(
          t('已批量{{action}} {{count}} 个用户', {
            action: actionLabel,
            count: updatedCount,
          }),
        );
      }

      await refresh();
    } catch (error) {
      showError(error?.message || t('批量处理用户失败'));
    } finally {
      setLoading(false);
    }
  };

  const handleStatusFilterChange = async (value) => {
    const nextStatus = normalizeStatusFilter(value);
    localStorage.setItem('users-status-filter', nextStatus);
    setStatusFilter(nextStatus);
    setActivePage(1);
    const { searchKeyword, searchGroup } = getFormValues();
    if (searchKeyword === '' && searchGroup === '') {
      await loadUsers(1, pageSize, nextStatus, roleFilter);
    } else {
      await searchUsers(1, pageSize, searchKeyword, searchGroup, nextStatus, roleFilter);
    }
  };

  const handleRoleFilterChange = async (value) => {
    const nextRole = normalizeRoleFilter(value);
    localStorage.setItem('users-role-filter', nextRole);
    setRoleFilter(nextRole);
    setActivePage(1);
    const { searchKeyword, searchGroup } = getFormValues();
    if (searchKeyword === '' && searchGroup === '') {
      await loadUsers(1, pageSize, statusFilter, nextRole);
    } else {
      await searchUsers(1, pageSize, searchKeyword, searchGroup, statusFilter, nextRole);
    }
  };

  const resetAllFilters = async () => {
    formApi?.reset?.();
    localStorage.setItem('users-status-filter', 'all');
    localStorage.setItem('users-role-filter', 'all');
    setStatusFilter('all');
    setRoleFilter('all');
    setActivePage(1);
    await loadUsers(1, pageSize, 'all', 'all');
  };

  // Fetch groups data
  const fetchGroups = async () => {
    try {
      let res = await API.get(`/api/group/`);
      if (res === undefined) {
        return;
      }
      setGroupOptions(
        res.data.data.map((group) => ({
          label: group,
          value: group,
        })),
      );
    } catch (error) {
      showError(error.message);
    }
  };

  // Modal control functions
  const closeAddUser = () => {
    setShowAddUser(false);
  };

  const closeEditUser = () => {
    setShowEditUser(false);
    setEditingUser({
      id: undefined,
    });
  };

  // Initialize data on component mount
  useEffect(() => {
    loadUsers(1, pageSize, statusFilter, roleFilter)
      .then()
      .catch((reason) => {
        showError(reason);
      });
    fetchGroups().then();
  }, []);

  return {
    // Data state
    users,
    loading,
    activePage,
    pageSize,
    userCount,
    searching,
    groupOptions,
    selectedKeys,
    statusFilter,
    roleFilter,

    // Modal state
    showAddUser,
    showEditUser,
    editingUser,
    setShowAddUser,
    setShowEditUser,
    setEditingUser,

    // Form state
    formInitValues,
    formApi,
    setFormApi,

    // UI state
    compactMode,
    setCompactMode,

    // Actions
    loadUsers,
    searchUsers,
    manageUser,
    batchManageUsers,
    resetUserPasskey,
    resetUserTwoFA,
    handleStatusFilterChange,
    handleRoleFilterChange,
    handlePageChange,
    handlePageSizeChange,
    handleRow,
    rowSelection,
    refresh,
    resetAllFilters,
    closeAddUser,
    closeEditUser,
    getFormValues,

    // Translation
    t,
  };
};
