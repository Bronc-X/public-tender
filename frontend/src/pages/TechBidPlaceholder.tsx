import React from 'react';
import { Result, Button } from 'antd';
import { useNavigate } from 'react-router-dom';
import { ExperimentOutlined } from '@ant-design/icons';

const TechBidPlaceholder: React.FC = () => {
  const navigate = useNavigate();
  return (
    <div style={{ padding: '100px 0' }}>
      <Result
        icon={<ExperimentOutlined style={{ color: '#1890ff', fontSize: 72 }} />}
        title="技术标 AI 智能生成引擎"
        subTitle="我们正在通过深度学习与施工方案库进行语义对齐，技术标自动生成模块即将上线，敬请期待！"
        extra={
          <Button type="primary" size="large" onClick={() => navigate('/dashboard')}>
            返回首页工作台
          </Button>
        }
      />
    </div>
  );
};

export default TechBidPlaceholder;
