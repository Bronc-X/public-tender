import React from 'react';
import { Result } from 'antd';

const PagePlaceholder: React.FC<{ title: string }> = ({ title }) => (
  <Result
    status="info"
    title={`${title} 模块开发中`}
    subTitle="该功能将在下个版本中可用。"
  />
);

export const Library = () => <PagePlaceholder title="资料库" />;
export const Imports = () => <PagePlaceholder title="导入中心" />;
export const Audits = () => <PagePlaceholder title="审核台" />;
export const Issues = () => <PagePlaceholder title="异常中心" />;
export const Exports = () => <PagePlaceholder title="导出中心" />;
export const Settings = () => <PagePlaceholder title="系统设置" />;
