import React, { useEffect, useState } from 'react';
import { Tabs } from 'antd';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

const LibraryLayout: React.FC = () => {
  const location = useLocation();
  const navigate = useNavigate();
  const [activeKey, setActiveKey] = useState('/library/qualifications');

  const tabItems = [
    { key: '/library/qualifications', label: '资质库' },
    { key: '/library/performances', label: '业绩库' },
    { key: '/library/persons', label: '人员库' },
    { key: '/library/honors', label: '荣誉库' },
    { key: '/library/financial-reports', label: '财报库' },
    { key: '/library/others', label: '其他库' },
    { key: '/tech-library/history', label: '标书库' },
    { key: '/tech-library/knowledge', label: '知识库' }
  ];

  useEffect(() => {
    // 特殊情况处理：知识库存在不同子Type的默认重定向
    if (location.pathname.startsWith('/tech-library/knowledge')) {
       setActiveKey('/tech-library/knowledge');
       return;
    }
    // 普通精确匹配和嵌套路由匹配
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
      
      {/* 渲染子库的具体内容 */}
      <div style={{ flex: 1 }}>
        <Outlet />
      </div>
    </div>
  );
};

export default LibraryLayout;
