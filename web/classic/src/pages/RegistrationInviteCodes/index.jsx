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

import React, { useCallback, useEffect, useState } from 'react';
import {
  Button,
  Card,
  Form,
  Input,
  Modal,
  Pagination,
  Select,
  Space,
  Tag,
} from '@douyinfe/semi-ui';
import { IconCopy, IconDelete, IconEdit, IconPlus } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

import CardTable from '../../components/common/ui/CardTable';
import { API, renderQuota, showError, showSuccess } from '../../helpers';

const STATUS_ENABLED = 1;
const STATUS_DISABLED = 2;
const EMPTY_FORM = {
  code: '',
  group: 'default',
  expired_time: null,
  initial_quota: 0,
  max_registrations: 0,
};

function getDisplayStatus(record) {
  if (record.status === STATUS_DISABLED) return 'disabled';
  if (record.expired_time > 0 && record.expired_time < Date.now() / 1000) {
    return 'expired';
  }
  if (
    record.max_registrations > 0 &&
    record.registered_count >= record.max_registrations
  ) {
    return 'exhausted';
  }
  return 'enabled';
}

function toDate(value) {
  if (!value) return null;
  const date = value instanceof Date ? value : new Date(Number(value) * 1000);
  return Number.isNaN(date.getTime()) ? null : date;
}

export default function RegistrationInviteCodes() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState([]);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState('');
  const [modalVisible, setModalVisible] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [formValues, setFormValues] = useState(EMPTY_FORM);

  const loadCodes = useCallback(
    async (targetPage = page, targetPageSize = pageSize) => {
      setLoading(true);
      try {
        const response = await API.get('/api/registration/invite-codes', {
          params: {
            p: targetPage,
            page_size: targetPageSize,
            keyword: keyword.trim(),
            status,
          },
        });
        if (!response.data.success) {
          showError(response.data.message);
          return;
        }
        setItems(response.data.data?.items || []);
        setTotal(response.data.data?.total || 0);
      } catch (error) {
        showError(t('加载邀请码失败'));
      } finally {
        setLoading(false);
      }
    },
    [keyword, page, pageSize, status, t],
  );

  useEffect(() => {
    loadCodes();
  }, [page, pageSize]);

  const openCreate = () => {
    setEditingId(null);
    setFormValues(EMPTY_FORM);
    setModalVisible(true);
  };

  const openEdit = (record) => {
    setEditingId(record.id);
    setFormValues({
      code: record.code,
      group: record.group,
      expired_time: toDate(record.expired_time),
      initial_quota: record.initial_quota,
      max_registrations: record.max_registrations,
    });
    setModalVisible(true);
  };

  const saveCode = async () => {
    const code = String(formValues.code || '').trim();
    const group = String(formValues.group || '').trim();
    if (!code || !group) {
      showError(t('请填写邀请码和用户分组'));
      return;
    }
    const expiredDate = toDate(formValues.expired_time);
    const payload = {
      code,
      group,
      expired_time: expiredDate ? Math.floor(expiredDate.getTime() / 1000) : 0,
      initial_quota: Number(formValues.initial_quota) || 0,
      max_registrations: Number(formValues.max_registrations) || 0,
    };
    setLoading(true);
    try {
      const response = editingId
        ? await API.put(`/api/registration/invite-codes/${editingId}`, payload)
        : await API.post('/api/registration/invite-codes', payload);
      if (!response.data.success) {
        showError(response.data.message);
        return;
      }
      showSuccess(t(editingId ? '邀请码更新成功' : '邀请码创建成功'));
      setModalVisible(false);
      await loadCodes();
    } catch (error) {
      showError(t('保存邀请码失败'));
    } finally {
      setLoading(false);
    }
  };

  const toggleStatus = async (record) => {
    const nextStatus =
      record.status === STATUS_ENABLED ? STATUS_DISABLED : STATUS_ENABLED;
    try {
      const response = await API.patch(
        `/api/registration/invite-codes/${record.id}/status`,
        { status: nextStatus },
      );
      if (!response.data.success) {
        showError(response.data.message);
        return;
      }
      showSuccess(
        t(nextStatus === STATUS_ENABLED ? '邀请码已启用' : '邀请码已禁用'),
      );
      await loadCodes();
    } catch (error) {
      showError(t('更新邀请码状态失败'));
    }
  };

  const deleteCode = (record) => {
    Modal.confirm({
      title: t('删除邀请码'),
      content: t('确定删除邀请码 {{code}}？此操作无法撤销。', {
        code: record.code,
      }),
      okType: 'danger',
      onOk: async () => {
        const response = await API.delete(
          `/api/registration/invite-codes/${record.id}`,
        );
        if (!response.data.success) {
          showError(response.data.message);
          return;
        }
        showSuccess(t('邀请码删除成功'));
        await loadCodes();
      },
    });
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 70 },
    {
      title: t('邀请码'),
      dataIndex: 'code',
      render: (value) => (
        <Space>
          <span className='font-mono'>{value}</span>
          <Button
            theme='borderless'
            size='small'
            icon={<IconCopy />}
            onClick={() => navigator.clipboard.writeText(value)}
          />
        </Space>
      ),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (_, record) => {
        const displayStatus = getDisplayStatus(record);
        const labels = {
          enabled: t('已启用'),
          disabled: t('已禁用'),
          expired: t('已过期'),
          exhausted: t('已用完'),
        };
        const colors = {
          enabled: 'green',
          disabled: 'grey',
          expired: 'orange',
          exhausted: 'red',
        };
        return <Tag color={colors[displayStatus]}>{labels[displayStatus]}</Tag>;
      },
    },
    { title: t('用户分组'), dataIndex: 'group' },
    {
      title: t('新用户初始额度'),
      dataIndex: 'initial_quota',
      render: (value) => renderQuota(value),
    },
    {
      title: t('已注册人数'),
      dataIndex: 'registered_count',
      render: (value, record) =>
        `${value} / ${record.max_registrations || t('无限制')}`,
    },
    {
      title: t('过期时间'),
      dataIndex: 'expired_time',
      render: (value) =>
        value ? new Date(value * 1000).toLocaleString() : t('永不过期'),
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      fixed: 'right',
      render: (_, record) => (
        <Space>
          <Button
            theme='borderless'
            icon={<IconEdit />}
            onClick={() => openEdit(record)}
          >
            {t('编辑')}
          </Button>
          <Button theme='borderless' onClick={() => toggleStatus(record)}>
            {t(record.status === STATUS_ENABLED ? '禁用' : '启用')}
          </Button>
          <Button
            theme='borderless'
            type='danger'
            icon={<IconDelete />}
            onClick={() => deleteCode(record)}
          >
            {t('删除')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <Card className='!rounded-2xl'>
        <div className='mb-4 flex flex-wrap items-center justify-between gap-2'>
          <Space wrap>
            <Input
              value={keyword}
              onChange={setKeyword}
              placeholder={t('搜索邀请码、分组或 ID')}
              showClear
            />
            <Select
              value={status}
              onChange={setStatus}
              style={{ width: 130 }}
              optionList={[
                { label: t('全部状态'), value: '' },
                { label: t('已启用'), value: 'enabled' },
                { label: t('已禁用'), value: 'disabled' },
                { label: t('已过期'), value: 'expired' },
                { label: t('已用完'), value: 'exhausted' },
              ]}
            />
            <Button
              onClick={() => {
                setPage(1);
                loadCodes(1, pageSize);
              }}
            >
              {t('搜索')}
            </Button>
          </Space>
          <Button type='primary' icon={<IconPlus />} onClick={openCreate}>
            {t('创建邀请码')}
          </Button>
        </div>
        <CardTable
          rowKey='id'
          columns={columns}
          dataSource={items}
          loading={loading}
          pagination={false}
          scroll={{ x: 'max-content' }}
        />
        <div className='mt-4 flex justify-end'>
          <Pagination
            currentPage={page}
            pageSize={pageSize}
            total={total}
            showSizeChanger
            pageSizeOpts={[10, 20, 50, 100]}
            onPageChange={setPage}
            onPageSizeChange={(size) => {
              setPage(1);
              setPageSize(size);
            }}
          />
        </div>
      </Card>

      <Modal
        title={t(editingId ? '编辑邀请码' : '创建邀请码')}
        visible={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={saveCode}
        confirmLoading={loading}
        closeOnEsc={!loading}
      >
        <Form values={formValues}>
          <Form.Input
            field='code'
            label={t('邀请码')}
            value={formValues.code}
            onChange={(value) =>
              setFormValues((current) => ({ ...current, code: value }))
            }
          />
          <Form.Input
            field='group'
            label={t('用户分组')}
            value={formValues.group}
            onChange={(value) =>
              setFormValues((current) => ({ ...current, group: value }))
            }
          />
          <Form.InputNumber
            field='initial_quota'
            label={t('新用户初始额度')}
            min={0}
            value={formValues.initial_quota}
            onChange={(value) =>
              setFormValues((current) => ({
                ...current,
                initial_quota: value,
              }))
            }
          />
          <Form.DatePicker
            field='expired_time'
            label={t('过期时间')}
            type='dateTime'
            showClear
            value={formValues.expired_time}
            onChange={(value) =>
              setFormValues((current) => ({
                ...current,
                expired_time: value || null,
              }))
            }
          />
          <Form.InputNumber
            field='max_registrations'
            label={t('最大注册人数')}
            min={0}
            extraText={t('填写 0 表示不限制')}
            value={formValues.max_registrations}
            onChange={(value) =>
              setFormValues((current) => ({
                ...current,
                max_registrations: value,
              }))
            }
          />
        </Form>
      </Modal>
    </div>
  );
}
