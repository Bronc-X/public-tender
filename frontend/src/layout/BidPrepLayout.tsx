import React, { useEffect, useState } from 'react';
import { Tabs } from 'antd';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

const BidPrepLayout: React.FC = () => {
  const location = useLocation();
  const navigate = useNavigate();
  const [activeKey, setActiveKey] = useState('/bid-projects');

  const tabItems = [
    { key: '/bid-projects', label: '商务标制作' },
    { key: '/tech-bid-projects', label: '技术标制作' }
  ];

  useEffect(() => {
    const match = tabItems.find(item => location.pathname.startsWith(item.key));
    if (match) {
      setActiveKey(match.key);
    }
  }, [location.pathname]);

  const handleTabChange = (key: string) => {
    navigate(key);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* 采用负边距，使其与外部 MainLayout 的 padding 完美对齐无缝连接 */}
      <div style={{ background: '#fff', padding: '0 24px', margin: '-24px -24px 24px -24px' }}>
        <Tabs 
          activeKey={activeKey} 
          onChange={handleTabChange} 
          items={tabItems} 
          style={{ marginBottom: -1 }} 
        />
      </div>
      
      {/* 渲染具体的制作列表或详情工作台 */}
      <div style={{ flex: 1 }}>
        <Outlet />
      </div>
    </div>
  );
};

export default BidPrepLayout;
