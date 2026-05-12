import React, { useEffect, useState } from 'react';
import { Modal, Descriptions, Spin, Result, Tag, Table, Space, Typography, Divider, Image } from 'antd';
import { FilePdfOutlined } from '@ant-design/icons';
import axios from 'axios';
import { useCompany } from '../context/CompanyContext';
import dayjs from 'dayjs';

const { Title, Text } = Typography;

interface LibraryDetailModalProps {
  visible: boolean;
  onCancel: () => void;
  targetType?: string;
  targetId?: string;
}

const LibraryDetailModal: React.FC<LibraryDetailModalProps> = ({ visible, onCancel, targetType, targetId }) => {
  const { currentCompanyId } = useCompany();
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<any | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!visible || !targetType || !targetId) return;

    const fetchData = async () => {
      setLoading(true);
      setError(null);
      setData(null);
      try {
        let endpoint = '';
        if (targetType === 'person') endpoint = `/api/persons/${targetId}`;
        else if (targetType === 'qualification') endpoint = `/api/qualifications/${targetId}`;
        else if (targetType === 'performance') endpoint = `/api/performances/${targetId}`;
        else if (targetType === 'file') endpoint = `/api/files/${targetId}`;
        else if (targetType === 'financial') endpoint = `/api/financial-reports/folders/${targetId}/files`;
        else if (targetType.startsWith('knowledge') || targetType === 'laborcontract') {
          // For knowledge, targetId might be a marker like "file:laborcontract:..." or a specific ID
          // Let's assume we handle both
          if (targetId.startsWith('knowledge:')) {
             const actualId = targetId.split(':').pop();
             endpoint = `/api/tech-bid/knowledge/${actualId}`;
          } else {
             endpoint = `/api/tech-bid/knowledge/${targetId}`;
          }
        }

        if (endpoint) {
          if (targetType === 'file' && targetId.includes(',')) {
             // Handle multiple files
             const ids = targetId.split(',');
             const resps = await Promise.all(ids.map(id => axios.get(`/api/files/${id}`, { headers: { 'x-company-id': currentCompanyId } })));
             setData(resps.map(r => r.data));
          } else {
            const resp = await axios.get(endpoint, {
              headers: { 'x-company-id': currentCompanyId }
            });
            setData(resp.data);
          }
        }
      } catch (err: any) {
        console.error('Fetch detail failed:', err);
        setError(err.response?.data?.error || '获取详情失败');
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [visible, targetType, targetId, currentCompanyId]);

  const renderContent = () => {
    if (loading) return <div style={{ padding: '40px 0', textAlign: 'center' }}><Spin tip="加载中..." /></div>;
    if (error) return <Result status="error" title="详情加载失败" subTitle={error} />;
    if (!data) return <Result status="info" title="暂无详情数据" />;


    if (targetType === 'person') {
      return (
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          <Descriptions  bordered column={2}>
            <Descriptions.Item label="姓名">{data.name}</Descriptions.Item>
            <Descriptions.Item label="身份证号">{data.id_card_no || '-'}</Descriptions.Item>
            <Descriptions.Item label="在职状态">
              {data.on_job_status === 'active' ? (
                <Tag color="green">在职</Tag>
              ) : data.on_job_status === 'resigned' ? (
                <Tag color="red">离职</Tag>
              ) : (
                <Text type="secondary">--</Text>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="录入时间">{dayjs(data.created_at).format('YYYY-MM-DD')}</Descriptions.Item>
          </Descriptions>
          {data.certificates && data.certificates.length > 0 && (
            <Table
              title={() => <strong>资质证书</strong>}
              dataSource={data.certificates}
              pagination={false}
              size="small"
              rowKey="id"
              columns={[
                { title: '证书名称', dataIndex: 'qualification_name' },
                { title: '编号', dataIndex: 'certificate_no' },
                { title: '有效期至', dataIndex: 'valid_to' },
              ]}
            />
          )}
        </Space>
      );
    }

    if (targetType === 'qualification') {
      return (
        <Descriptions  bordered column={1}>
          <Descriptions.Item label="资质名称">{data.qualification_name}</Descriptions.Item>
          <Descriptions.Item label="证书编号">{data.certificate_no || '-'}</Descriptions.Item>
          <Descriptions.Item label="等级/类别">{data.qualification_level || '-'}</Descriptions.Item>
          <Descriptions.Item label="发证机关">{data.issuing_authority || '-'}</Descriptions.Item>
          <Descriptions.Item label="有效期">{data.valid_from} ~ {data.valid_to}</Descriptions.Item>
        </Descriptions>
      );
    }

    if (targetType === 'performance') {
      return (
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          <Descriptions bordered column={2}>
            <Descriptions.Item label="项目名称" span={2}>{data.project_name}</Descriptions.Item>
            <Descriptions.Item label="中标金额">{data.bid_amount_value || data.amount_value || '-'}</Descriptions.Item>
            <Descriptions.Item label="项目经理">{data.project_manager_name || '-'}</Descriptions.Item>
            <Descriptions.Item label="技术负责人">{data.technical_leader_name || '-'}</Descriptions.Item>
            <Descriptions.Item label="安全负责人">{data.safety_leader_name || '-'}</Descriptions.Item>
            <Descriptions.Item label="项目地点" span={2}>{data.project_location || '-'}</Descriptions.Item>
            <Descriptions.Item label="建设单位" span={2}>{data.owner_org || '-'}</Descriptions.Item>
            <Descriptions.Item label="中标日期">{data.winning_date || '-'}</Descriptions.Item>
            <Descriptions.Item label="竣工日期">{data.completion_date || '-'}</Descriptions.Item>
            <Descriptions.Item label="建设规模" span={2}>{data.scale_desc || '-'}</Descriptions.Item>
          </Descriptions>
          
          {data.proofs && data.proofs.length > 0 && (
            <Table
              title={() => <strong>业绩归档佐证文件</strong>}
              dataSource={data.proofs}
              pagination={false}
              size="small"
              rowKey="id"
              columns={[
                { title: '文件类型', dataIndex: 'proof_type' },
                { title: '文件名', dataIndex: 'file_name' },
                { title: '上传时间', render: (_: unknown, r: any) => r.created_at ? dayjs(r.created_at).format('YYYY-MM-DD') : '-' },
              ]}
            />
          )}
        </Space>
      );
    }

    
    if (targetType === 'file') {
      const files = Array.isArray(data) ? data : [data];
      return (
        <Space direction="vertical" style={{ width: '100%' }}>
          {files.map((fileData: any) => (
            <Descriptions key={fileData.id || fileData.filename} bordered column={1}>
               <Descriptions.Item label="文件名">{fileData.original_name || fileData.filename}</Descriptions.Item>
               <Descriptions.Item label="上传时间">{fileData.created_at ? dayjs(fileData.created_at).format('YYYY-MM-DD HH:mm') : '-'}</Descriptions.Item>
            </Descriptions>
          ))}
        </Space>
      );
    }

    if (targetType === 'financial') {
      // Data is an array of files in this folder
      const files = Array.isArray(data) ? data : [];
      return (
        <Space direction="vertical" style={{ width: '100%' }}>
          {files.length === 0 ? (
            <Result status="info" title="该财报文件夹内暂无扫描件" />
          ) : (
            <Image.PreviewGroup>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 16 }}>
                {files.map((file: any) => (
                  <div key={file.id} style={{ textAlign: 'center', width: 120 }}>
                    <Image
                      src={`/api/file-binary/${file.id}`}
                      alt={file.file_name}
                      style={{ width: 120, height: 160, objectFit: 'contain', border: '1px solid #d9d9d9', borderRadius: 4 }}
                    />
                    <div style={{ marginTop: 8, fontSize: 12, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={file.file_name}>
                      {file.file_name}
                    </div>
                  </div>
                ))}
              </div>
            </Image.PreviewGroup>
          )}
        </Space>
      );
    }
if (targetType && (targetType === 'laborcontract' || targetType.startsWith('knowledge'))) {
      // If it's a knowledge item, show the content. 
      // If there are multiple items for the same file, this modal currently only shows one.
      // But knowledge items are usually self-contained titles/contents.
      return (
        <div style={{ padding: 16, background: '#f5f5f5', borderRadius: 8 }}>
          <Title level={4}>{data.item_name || data.title}</Title>
          <Divider />
          <div style={{ whiteSpace: 'pre-wrap' }}>{data.item_content || data.content}</div>
        </div>
      );
    }

    return <pre>{JSON.stringify(data, null, 2)}</pre>;
  };

  return (
    <Modal
      title={`${targetType === 'person' ? '人员' : targetType === 'qualification' ? '资质' : targetType === 'performance' ? '业绩' : targetType === 'financial' ? '财报' : '归档'}详情`}
      open={visible}
      onCancel={onCancel}
      footer={null}
      width={700}
      centered
    >
      {data && (() => {
         const firstData = Array.isArray(data) ? data[0] : data;
         const hasPic = firstData?.stored_path || (firstData?.certificates && firstData.certificates[0]?.stored_path) || (firstData?.proofs && firstData.proofs[0]?.file_asset_id) || (targetType === 'file' && firstData?.id);
         if (!hasPic) return null;
         
         if (targetType === 'file' && Array.isArray(data)) {
           return (
             <div style={{ marginBottom: 20, textAlign: 'center', background: '#f0f2f5', padding: 10, borderRadius: 8 }}>
               <Image.PreviewGroup>
                 <Space wrap>
                   {data.map((item: any) => (
                     item.ext?.includes('.pdf') ? (
                       <div key={item.id} style={{ width: 120, height: 160, overflow: 'hidden', border: '1px solid #d9d9d9', background: '#fff' }}>
                         <FilePdfOutlined style={{ fontSize: 40, color: 'red', marginTop: 40 }} />
                         <div style={{ fontSize: 12, marginTop: 10 }}>PDF 文件</div>
                       </div>
                     ) : (
                       <Image key={item.id} src={`/api/file-binary/${item.id}`} style={{ width: 120, height: 160, objectFit: 'contain', background: '#fff', border: '1px solid #d9d9d9' }} />
                     )
                   ))}
                 </Space>
               </Image.PreviewGroup>
             </div>
           );
         }

         let ext = firstData.ext;
         let fileId = firstData.file_asset_id || firstData.id;
         
         if (!firstData.stored_path && firstData.certificates && firstData.certificates[0]?.stored_path) {
            ext = firstData.certificates[0].ext;
            fileId = firstData.certificates[0].file_asset_id || firstData.certificates[0].id;
         } else if (!firstData.stored_path && firstData.proofs && firstData.proofs[0]?.file_asset_id) {
            ext = firstData.proofs[0].ext;
            fileId = firstData.proofs[0].file_asset_id;
         }
         return (
           <div style={{ marginBottom: 20, textAlign: 'center', background: '#f0f2f5', padding: 10, borderRadius: 8 }}>
             {ext?.includes('.pdf') ? (
                <iframe src={`/api/file-binary/${fileId}`} width="100%" height="400" style={{border: 'none'}} />
             ) : (
                <Image src={`/api/file-binary/${fileId}`} style={{ maxWidth: '100%', maxHeight: 400, objectFit: 'contain' }} />
             )}
           </div>
         );
      })()}
      {renderContent()}
    </Modal>
  );
};

export default LibraryDetailModal;
