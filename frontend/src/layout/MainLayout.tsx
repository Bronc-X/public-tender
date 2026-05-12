import React, { useState } from 'react';
import { Layout, Menu, Button, theme, Space, Select, Divider } from 'antd';
import { useCompany } from '../context/CompanyContext';
import {
  DesktopOutlined,
  SettingOutlined,
  UserOutlined,
  ProjectOutlined,
  SolutionOutlined,
  MenuUnfoldOutlined,
  MenuFoldOutlined,
  HistoryOutlined,
  AuditOutlined,
  FileTextOutlined,
  SecurityScanOutlined,
  TrophyOutlined,
  DatabaseOutlined,
  FolderOpenOutlined,
  AccountBookOutlined,
  BankOutlined,
  WarningOutlined,
  FileOutlined
} from '@ant-design/icons';
import { useNavigate, useLocation, Outlet } from 'react-router-dom';

const { Header, Content, Sider } = Layout;

const MainLayout: React.FC = () => {
  const [collapsed, setCollapsed] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const { currentCompanyId, setCurrentCompanyId, companies, loading } = useCompany();

  const {
    token: { colorBgContainer, borderRadiusLG },
  } = theme.useToken();

  const handleCompanyChange = (newCompanyId: string) => {
    setCurrentCompanyId(newCompanyId);
    
    // If we are currently on a detail page, redirect to dashboard to avoid "fetch failed" errors
    // since the current detail ID likely doesn't exist in the new company.
    const detailPagePatterns = [
      /^\/library\/[^\/]+\/.+/,      // e.g. /library/persons/c1-P123
      /^\/bid-projects\/.+/,         // e.g. /bid-projects/123
      /^\/tech-bid-projects\/.+/,    // e.g. /tech-bid-projects/123
      /^\/audits\/.+/,               // e.g. /audits/123
      /^\/tech-library\/history\/.+/ // e.g. /tech-library/history/123
    ];

    if (detailPagePatterns.some(pattern => pattern.test(location.pathname))) {
      navigate('/dashboard');
    }
  };

  const menuItems = [
    {
      key: '/dashboard',
      icon: <DesktopOutlined />,
      label: '首页工作台',
    },
    {
      key: '/file-center/repository',
      icon: <FolderOpenOutlined />,
      label: '文件库',
    },
    {
      key: '/library',
      icon: <DatabaseOutlined />,
      label: '资料库',
    },
    {
      key: '/bid-projects',
      icon: <FileTextOutlined />,
      label: '标书制作',
    },
    {
      key: '/issues',
      icon: <WarningOutlined />,
      label: '异常中心',
    },
    {
      key: '/settings',
      icon: <SettingOutlined />,
      label: '系统设置',
    },
  ];

  const handleMenuClick = ({ key }: { key: string }) => {
    navigate(key);
  };

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        trigger={null}
        collapsible
        collapsed={collapsed}
        width={160}
        theme="light"
        className="ant-layout-sider"
      >
        <div className="logo-container">
          {!collapsed ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <svg viewBox="0 0 1239 1024" xmlns="http://www.w3.org/2000/svg" width="26" height="26" style={{ display: 'block', flexShrink: 0 }}>
                <path d="M0 854.518932m50.095726 0l1055.863773 0q50.095726 0 50.095726 50.095727l0 0.481689q0 50.095726-50.095726 50.095727l-1055.863773 0q-50.095726 0-50.095726-50.095727l0-0.481689q0-50.095726 50.095726-50.095727Z" fill="#80C6FF" />
                <path d="M218.687113 0.001445h337.182774a93.929487 93.929487 0 0 1 94.411177 93.929487v861.261143H124.757626V93.930932A93.929487 93.929487 0 0 1 218.687113 0.001445zM729.759861 240.846284h205.681492a96.337935 96.337935 0 0 1 96.337935 96.337935v568.393819h-302.019427V240.846284z" fill="#80C6FF" />
                <path d="M505.774161 300.575804H275.526495a24.566174 24.566174 0 0 1-24.084484-24.084484 24.084484 24.084484 0 0 1 24.084484-24.084484h230.247666a24.084484 24.084484 0 0 1 24.084484 24.084484 24.084484 24.084484 0 0 1-24.084484 24.084484zM505.774161 425.81512H275.526495a24.084484 24.084484 0 0 1-24.084484-24.084484 24.566174 24.566174 0 0 1 24.084484-24.084484h230.247666a24.084484 24.084484 0 0 1 24.084484 24.084484 24.084484 24.084484 0 0 1-24.084484 24.084484zM505.774161 551.536125H275.526495a24.084484 24.084484 0 0 1-24.084484-24.084484 24.566174 24.566174 0 0 1 24.084484-24.084483h230.247666a24.084484 24.084484 0 0 1 24.084484 24.084483 24.084484 24.084484 0 0 1-24.084484 24.084484z" fill="#D2EFFF" />
              </svg>
              <span>AI工作台</span>
            </div>
          ) : (
            <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', width: '100%' }}>
              <svg viewBox="0 0 1239 1024" xmlns="http://www.w3.org/2000/svg" width="26" height="26" style={{ display: 'block' }}>
                <path d="M0 854.518932m50.095726 0l1055.863773 0q50.095726 0 50.095726 50.095727l0 0.481689q0 50.095726-50.095726 50.095727l-1055.863773 0q-50.095726 0-50.095726-50.095727l0-0.481689q0-50.095726 50.095726-50.095727Z" fill="#80C6FF" />
                <path d="M218.687113 0.001445h337.182774a93.929487 93.929487 0 0 1 94.411177 93.929487v861.261143H124.757626V93.930932A93.929487 93.929487 0 0 1 218.687113 0.001445zM729.759861 240.846284h205.681492a96.337935 96.337935 0 0 1 96.337935 96.337935v568.393819h-302.019427V240.846284z" fill="#80C6FF" />
                <path d="M505.774161 300.575804H275.526495a24.566174 24.566174 0 0 1-24.084484-24.084484 24.084484 24.084484 0 0 1 24.084484-24.084484h230.247666a24.084484 24.084484 0 0 1 24.084484 24.084484 24.084484 24.084484 0 0 1-24.084484 24.084484zM505.774161 425.81512H275.526495a24.084484 24.084484 0 0 1-24.084484-24.084484 24.566174 24.566174 0 0 1 24.084484-24.084484h230.247666a24.084484 24.084484 0 0 1 24.084484 24.084484 24.084484 24.084484 0 0 1-24.084484 24.084484zM505.774161 551.536125H275.526495a24.084484 24.084484 0 0 1-24.084484-24.084484 24.566174 24.566174 0 0 1 24.084484-24.084483h230.247666a24.084484 24.084484 0 0 1 24.084484 24.084483 24.084484 24.084484 0 0 1-24.084484 24.084484z" fill="#D2EFFF" />
              </svg>
            </div>
          )}
        </div>
        <Menu
          theme="light"
          mode="inline"
          selectedKeys={[
            location.pathname.startsWith('/bid-projects') ? '/bid-projects' :
              location.pathname.startsWith('/tech-bid-projects') ? '/bid-projects' :
                location.pathname.startsWith('/library') ? '/library' :
                  location.pathname.startsWith('/tech-library/history') ? '/library' :
                    location.pathname.startsWith('/tech-library/knowledge') ? '/library' :
                              location.pathname.startsWith('/tech-library') ? location.pathname :
                                location.pathname.startsWith('/file-center') ? location.pathname :
                                  location.pathname.startsWith('/audits') ? '/file-center/audit' :
                                    location.pathname
          ]}
          defaultOpenKeys={['library', 'bid-prep', 'file-center-group']}
          items={menuItems}
          onClick={handleMenuClick}
        />
      </Sider>
      <Layout>
        <Header className="ant-layout-header">
          <Button
            type="text"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed(!collapsed)}
            style={{ fontSize: '16px', width: 64, height: 64 }}
          />
          <Space size="large">
            <div className="flex items-center gap-2">
              <span className="text-gray-400 text-xs">当前公司：</span>
              <Select
                value={currentCompanyId}
                onChange={handleCompanyChange}
                loading={loading}
                style={{ width: 480, flexShrink: 0 }}
                popupMatchSelectWidth={false}
                variant="filled"
                options={companies.map(c => ({ value: c.id, label: c.company_name }))}
                popupRender={(menu) => (
                  <>
                    {menu}
                    <Divider style={{ margin: '8px 0' }} />
                    <Button
                      type="link"
                      size="small"
                      block
                      icon={<BankOutlined />}
                      onMouseDown={(e) => e.preventDefault()}
                      onClick={() => navigate('/settings?tab=company')}
                    >
                      管理多公司
                    </Button>
                  </>
                )}
                className="font-medium"
              />
            </div>
          </Space>
        </Header>
        <Content className="content-container">
          <div
            key={currentCompanyId}
            style={{
              padding: 24,
              minHeight: 360,
              background: colorBgContainer,
            }}
          >
            <Outlet />
          </div>
        </Content>
      </Layout>
    </Layout>
  );
};

export default MainLayout;
