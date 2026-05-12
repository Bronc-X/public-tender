import React from 'react';
import { Drawer, Space } from 'antd';
import { FileSearchOutlined as AuditOutlined } from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import AuditDetail from './AuditDetail';

/**
 * Renders audit UI in a drawer when navigated to /audits/:id with location.state.background
 * (modal route pattern — URL updates while file center stays underneath).
 */
const AuditDetailDrawerRoute: React.FC = () => {
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();

  const handleClose = () => {
    navigate(-1);
  };

  return (
    <Drawer
      title={
        <Space>
          <AuditOutlined /> 智能审核校验案台
        </Space>
      }
      open={Boolean(id)}
      width="90%"
      onClose={handleClose}
      destroyOnClose
      styles={{ body: { padding: 0 } }}
    >
      {id ? (
        <AuditDetail
          auditId={id}
          isDrawer
          onAuditSuccess={() => {
            navigate(-1);
          }}
        />
      ) : null}
    </Drawer>
  );
};

export default AuditDetailDrawerRoute;
