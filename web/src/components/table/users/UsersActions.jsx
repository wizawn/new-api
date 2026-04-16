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

import React from 'react';
import { Button, Dropdown, Modal } from '@douyinfe/semi-ui';
import { showError } from '../../../helpers';

const UsersActions = ({
  selectedKeys,
  setShowAddUser,
  batchManageUsers,
  refresh,
  t,
}) => {
  // Add new user
  const handleAddUser = () => {
    setShowAddUser(true);
  };

  const selectedCount = selectedKeys.length;

  const handleBatchManage = (action, title, content) => {
    if (selectedCount === 0) {
      showError(t('请至少选择一个用户！'));
      return;
    }
    Modal.confirm({
      title,
      content,
      onOk: () => batchManageUsers(action),
    });
  };

  const batchManageMenu = [
    {
      node: 'item',
      name: t('启用所选用户'),
      onClick: () =>
        handleBatchManage(
          'enable',
          t('批量启用用户'),
          t('确定要启用所选用户吗？'),
        ),
    },
    {
      node: 'item',
      name: t('禁用所选用户'),
      onClick: () =>
        handleBatchManage(
          'disable',
          t('批量禁用用户'),
          t('确定要禁用所选用户吗？'),
        ),
    },
    {
      node: 'item',
      name: t('提升所选用户'),
      onClick: () =>
        handleBatchManage(
          'promote',
          t('批量提升用户'),
          t('确定要将所选用户提升为管理员吗？'),
        ),
    },
    {
      node: 'item',
      name: t('降级所选用户'),
      onClick: () =>
        handleBatchManage(
          'demote',
          t('批量降级用户'),
          t('确定要将所选用户降级为普通用户吗？'),
        ),
    },
    {
      node: 'item',
      type: 'danger',
      name: t('注销所选用户'),
      onClick: () =>
        handleBatchManage(
          'delete',
          t('批量注销用户'),
          t('确定要注销所选用户吗？此操作不可逆。'),
        ),
    },
  ];

  return (
    <div className='flex gap-2 w-full md:w-auto order-2 md:order-1'>
      <Button
        type='primary'
        className='w-full md:w-auto'
        onClick={handleAddUser}
        size='small'
      >
        {t('添加用户')}
      </Button>
      <Dropdown
        menu={batchManageMenu}
        trigger='click'
        position='bottomLeft'
      >
        <Button
          type='secondary'
          className='w-full md:w-auto'
          size='small'
          disabled={selectedCount === 0}
        >
          {t('批量管理')} {selectedCount > 0 ? `(${selectedCount})` : ''}
        </Button>
      </Dropdown>
      <Button
        type='tertiary'
        className='w-full md:w-auto'
        onClick={() => refresh?.()}
        size='small'
      >
        {t('刷新')}
      </Button>
    </div>
  );
};

export default UsersActions;
