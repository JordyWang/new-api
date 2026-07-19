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

import React, { useEffect, useState } from 'react';
import { Banner, Button, Card, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';

const emptyConfig = {
  code: '',
  group: 'default',
  expired_time: 0,
  initial_quota: 0,
  max_registrations: 0,
  registered_count: 0,
};

const MAX_INT32 = 2147483647;

function toDate(value) {
  if (!value) return null;
  const date = value instanceof Date ? value : new Date(Number(value) * 1000);
  return Number.isNaN(date.getTime()) ? null : date;
}

export default function SettingsRegistrationInviteCode() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState(emptyConfig);

  const loadConfig = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/registration/invite-code');
      if (res.data.success) {
        setInputs({ ...emptyConfig, ...res.data.data });
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(t('加载失败，请重试'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadConfig();
  }, []);

  const handleChange = (field, value) => {
    setInputs((current) => ({ ...current, [field]: value }));
  };

  const handleSave = async () => {
    setLoading(true);
    try {
      const expiredDate = toDate(inputs.expired_time);
      const res = await API.put('/api/registration/invite-code', {
        code: String(inputs.code || '').trim(),
        group: String(inputs.group || '').trim(),
        expired_time: expiredDate
          ? Math.floor(expiredDate.getTime() / 1000)
          : 0,
        initial_quota: Number(inputs.initial_quota) || 0,
        max_registrations: Number(inputs.max_registrations) || 0,
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      setInputs({ ...emptyConfig, ...res.data.data });
      showSuccess(t('保存成功'));
    } catch (error) {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  };

  const remaining =
    inputs.max_registrations > 0
      ? Math.max(inputs.max_registrations - inputs.registered_count, 0)
      : null;

  return (
    <Spin spinning={loading}>
      <Card style={{ marginTop: '10px' }}>
        <Form values={inputs} style={{ marginBottom: 15 }}>
          <Form.Section text={t('邀请码')}>
            {!inputs.code && (
              <Banner
                type='warning'
                description={t('邀请码未配置时，所有新用户注册都会被拒绝。')}
                closeIcon={null}
                className='!rounded-lg mb-3'
              />
            )}
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Input
                  field='code'
                  label={t('邀请码')}
                  placeholder={t('邀请码')}
                  onChange={(value) => handleChange('code', value)}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Input
                  field='group'
                  label={t('用户分组')}
                  placeholder='default'
                  onChange={(value) => handleChange('group', value)}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field='initial_quota'
                  label={t('新用户初始额度')}
                  min={0}
                  max={MAX_INT32}
                  step={1}
                  precision={0}
                  onChange={(value) => handleChange('initial_quota', value)}
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.DatePicker
                  field='expired_time'
                  label={t('过期时间')}
                  value={toDate(inputs.expired_time)}
                  onChange={(value) =>
                    handleChange('expired_time', value || null)
                  }
                  showClear
                  type='dateTime'
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field='max_registrations'
                  label={t('最大注册人数')}
                  min={0}
                  max={MAX_INT32}
                  step={1}
                  precision={0}
                  extraText={t('填写 0 表示不限制')}
                  onChange={(value) => handleChange('max_registrations', value)}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Input
                  field='registered_count'
                  label={t('已注册人数')}
                  value={String(inputs.registered_count || 0)}
                  disabled
                />
                <div className='text-tertiary text-xs mt-1'>
                  {t('剩余')}: {remaining === null ? t('无限制') : remaining}
                </div>
              </Col>
            </Row>
            <Row>
              <Col>
                <Button size='default' onClick={handleSave} loading={loading}>
                  {t('保存设置')}
                </Button>
              </Col>
            </Row>
          </Form.Section>
        </Form>
      </Card>
    </Spin>
  );
}
