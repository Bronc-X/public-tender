import React, { useState, useEffect } from 'react';
import { Card, Table, Button, Space, Typography, Tag, Modal, Upload, Tabs, message, Empty, Alert, Tooltip, Spin } from 'antd';
import { LinkOutlined, CheckCircleOutlined, PlusOutlined, ExportOutlined, FolderFilled, LeftOutlined } from '@ant-design/icons';
import axios from 'axios';
import { useCompany } from '../context/CompanyContext';
import LibraryDetailModal from './LibraryDetailModal';

const { Text } = Typography;
const { TabPane } = Tabs;

interface ResourceCombinationPanelProps {
  project_id: string;
  sourceData: any; // The latestCompanyAdaptation data
  savedBindings?: Record<string, any>;
  onReadyToNext?: (payload: any) => void;
  onOpenGenericRules?: () => void;
  onRefresh?: () => void;
}

const CATEGORY_LABELS: Record<string, string> = {
  qualification: '资质要求',
  person: '人员要求',
  project_performance: '业绩要求',
  financial: '财务要求',
  other_requirements: '其他要求',
  scoring_item: '加分项'
};

interface RuleItem {
  id: string;
  category: string;
  requirement_text: string;
  status?: string;
  matched_record_id?: string;
  matched_record_name?: string;
  matched_details?: {
    matched_value?: string;
  };
}

export const ResourceCombinationPanel: React.FC<ResourceCombinationPanelProps> = ({ project_id, sourceData, savedBindings, onReadyToNext, onRefresh }) => {
  const { currentCompanyId } = useCompany();
  const [bindings, setBindings] = useState<Record<string, any>>({});
  const [activeModalItem, setActiveModalItem] = useState<{ id: string, category: string, reqText: string } | null>(null);
  
  // Library Data
  const [libData, setLibData] = useState<any[]>([]);
  const [libLoading, setLibLoading] = useState(false);
  const [detailModalState, setDetailModalState] = useState<{ targetType: string, targetId: string } | null>(null);
  const [localIgnored, setLocalIgnored] = useState<string[]>([]);

  // Financial specific states
  const [financialSelectedFolderId, setFinancialSelectedFolderId] = useState<string | null>(null);
  const [financialFolderFiles, setFinancialFolderFiles] = useState<any[]>([]);
  const [fetchingFolderFiles, setFetchingFolderFiles] = useState(false);
  const [selectedFinancialFileIds, setSelectedFinancialFileIds] = useState<React.Key[]>([]);

  // Parse and Flatten Rules
  const rules: RuleItem[] = Array.isArray(sourceData?.results) ? sourceData.results : [];

  // Group Rules for UI
  const groupedRules = rules.reduce<Record<string, RuleItem[]>>((acc, rule) => {
    // Exclude ones explicitly ignored (generic rules that don't need evidence)
    if (rule.status === 'ignored' || localIgnored.includes(rule.id)) return acc;
    if (!acc[rule.category]) acc[rule.category] = [];
    acc[rule.category].push(rule);
    return acc;
  }, {});

  // Pre-fill initial bindings with successful mappings from AI (Step 3) if any
  useEffect(() => {
    if (savedBindings && Object.keys(savedBindings).length > 0) {
       setBindings(savedBindings);
       return;
    }
    const initialBindings: Record<string, any> = {};
    rules.forEach((rule) => {
      // If the AI matched something successfully, we pre-fill it!
      if (rule.status === 'passed' || rule.status === 'success') {
         if (rule.matched_record_name || rule.matched_details) {
             initialBindings[rule.id] = {
               target_type: rule.matched_record_id ? 'db_record' : 'auto_matched',
               record_id: rule.matched_record_id,
               record_name: rule.matched_record_name || rule.matched_details?.matched_value || 'AI自动匹配项'
             };
         }
      }
    });
    setBindings(initialBindings);
  }, [sourceData, savedBindings]);

  // Handle Bind Modal Open
  const handleOpenBindModal = (rule: any) => {
    setActiveModalItem({ id: rule.id, category: rule.category, reqText: rule.requirement_text });
    setFinancialSelectedFolderId(null);
    setFinancialFolderFiles([]);
    setSelectedFinancialFileIds([]);
    fetchLibraryData(rule.category);
  };

  const fetchLibraryData = async (category: string) => {
    setLibLoading(true);
    try {
      let endpoint = '';
      if (category === 'qualification') endpoint = '/qualifications?company_id=' + currentCompanyId;
      else if (category === 'person') endpoint = '/persons?company_id=' + currentCompanyId;
      else if (category === 'project_performance') endpoint = '/performances?company_id=' + currentCompanyId;
      else if (category === 'financial') endpoint = '/financial-reports/folders';
      else if (category === 'other_requirements') endpoint = '/others/folders';
      else endpoint = '/knowledge-library?content_type=other&company_id=' + currentCompanyId; 
      
      const res = await axios.get('/api' + endpoint, { headers: { 'x-company-id': currentCompanyId || '' } });
      let data = res.data || [];
      // Filter out personnel qualifications from enterprise qualification binding
      if (category === 'qualification') {
        data = data.filter((item: any) => item.owner_type === 'company' || !item.owner_type);
      }
      setLibData(data);
    } catch (e) {
      console.error(e);
      message.error("无法加载资料库数据");
    } finally {
      setLibLoading(false);
    }
  };

  const handleSelectLibItem = (item: any) => {
    if (!activeModalItem) return;
    
    // Different tables have different name fields
    const itemName = item.qualification_name || item.type_name || item.cert_type || item.name || item.project_name || item.honor_name || item.file_name || item.folder_name || '未命名资源';
    
    const newBindings = {
      ...bindings,
      [activeModalItem.id]: {
        target_type: (activeModalItem.category === 'financial' || activeModalItem.category === 'other_requirements') ? 'file' : 'db_record',
        record_id: item.id,
        record_name: itemName
      }
    };
    setBindings(newBindings);
    setActiveModalItem(null);
    axios.post(`/api/bid-projects/${project_id}/resource-combination`, { bindings: newBindings }, { headers: { 'x-company-id': currentCompanyId || '' } })
      .then(() => message.success("绑定成功并已自动保存"))
      .catch((e) => message.error("自动保存失败：" + (e.response?.data?.message || e.message)));

  };

  const handleConfirmFinancialSelection = () => {
    if (!activeModalItem) return;
    const selectedFiles = financialFolderFiles.filter(f => selectedFinancialFileIds.includes(f.id));
    if (selectedFiles.length === 0) {
       message.warning('请至少选择一个选项');
       return;
    }

    const itemName = selectedFiles.map(f => f.file_name).join(', ');
    const recordIds = selectedFiles.map(f => f.id).join(',');

    const newBindings = {
      ...bindings,
      [activeModalItem.id]: {
        target_type: 'file',
        record_id: recordIds,
        record_name: itemName
      }
    };
    setBindings(newBindings);
    setActiveModalItem(null);
    axios.post(`/api/bid-projects/${project_id}/resource-combination`, { bindings: newBindings }, { headers: { 'x-company-id': currentCompanyId || '' } })
      .then(() => message.success("绑定成功并已自动保存"))
      .catch((e) => message.error("自动保存失败：" + (e.response?.data?.message || e.message)));
  };

  const handleFileUpload = (info: any) => {
    if (info.file.status === 'done') {
      const responseArray = info.file.response;
      let assetId = '';
      if (Array.isArray(responseArray) && responseArray.length > 0) {
        assetId = responseArray[0].id;
      } else if (responseArray?.id) {
        assetId = responseArray.id;
      } else if (responseArray?.file_id) {
        assetId = responseArray.file_id;
      }

      if (activeModalItem && assetId) {
        const newBindings = {
          ...bindings,
          [activeModalItem.id]: {
            target_type: 'file',
            record_id: assetId,
            record_name: info.file.name
          }
        };
        setBindings(newBindings);
        setActiveModalItem(null);
        axios.post(`/api/bid-projects/${project_id}/resource-combination`, { bindings: newBindings }, { headers: { 'x-company-id': currentCompanyId || '' } })
          .then(() => message.success("上传并关联成功（已自动保存）!"))
          .catch((e) => message.error("自动保存失败：" + (e.response?.data?.message || e.message)));
      } else {
        message.error("上传接口返回结构异常，无法绑定");
      }
    } else if (info.file.status === 'error') {
      message.error(`${info.file.name} 文件上传失败`);
    }
  };


  const handleMoveToGeneric = async (ruleId: string, event: React.MouseEvent, reqText: string) => {
    // 1. DOM Animation
    const btnRect = (event.currentTarget as HTMLElement).getBoundingClientRect();
    const targetEl = document.getElementById('generic-commitments-float-btn');
    
    if (targetEl) {
      const targetRect = targetEl.getBoundingClientRect();
      const flyingEl = document.createElement('div');
      flyingEl.innerText = reqText;
      flyingEl.style.position = 'fixed';
      flyingEl.style.left = `${btnRect.left - 200}px`;
      flyingEl.style.top = `${btnRect.top}px`;
      flyingEl.style.maxWidth = '300px';
      flyingEl.style.padding = '4px 12px';
      flyingEl.style.background = '#eef2ff';
      flyingEl.style.border = '1px solid #c7d2fe';
      flyingEl.style.color = '#4f46e5';
      flyingEl.style.borderRadius = '20px';
      flyingEl.style.fontSize = '12px';
      flyingEl.style.whiteSpace = 'nowrap';
      flyingEl.style.overflow = 'hidden';
      flyingEl.style.textOverflow = 'ellipsis';
      flyingEl.style.zIndex = '9999';
      flyingEl.style.transition = 'all 1.0s cubic-bezier(0.25, 0.1, 0.25, 1)';
      flyingEl.style.pointerEvents = 'none';
      flyingEl.style.boxShadow = '0 4px 12px rgba(0,0,0,0.1)';
      document.body.appendChild(flyingEl);

      // Trigger animation
      setTimeout(() => {
        flyingEl.style.left = `${targetRect.left + 20}px`;
        flyingEl.style.top = `${targetRect.top + 20}px`;
        flyingEl.style.transform = 'scale(0.1)';
        flyingEl.style.opacity = '0';
      }, 10);

      // Cleanup DOM
      setTimeout(() => {
        if (document.body.contains(flyingEl)) {
          document.body.removeChild(flyingEl);
        }
      }, 1000);
    }

    // 2. Optimistic UI update
    setLocalIgnored(prev => [...prev, ruleId]);

    // 3. API request
    try {
      const updatedData = { ...sourceData };
      if (!updatedData.results) return;
      
      updatedData.results = updatedData.results.map((r: any) => 
        r.id === ruleId ? { ...r, status: 'ignored' } : r
      );
      
      await axios.put(`/api/bid-projects/${project_id}/company-adaptation`, updatedData, {
         headers: { 'x-company-id': currentCompanyId || '' }
      });
      // message.success('已移入池中');
      if (onRefresh) onRefresh();
    } catch (error) {
      setLocalIgnored(prev => prev.filter(id => id !== ruleId));
      message.error('移动失败，请重试');
    }
  };

  // Give the bindings to parent whenever they change (or manually)
  useEffect(() => {
    if (onReadyToNext) {
      onReadyToNext(bindings);
    }
  }, [bindings]);

  return (
    <div style={{ padding: '0 24px' }}>


      {Object.entries(groupedRules).map(([category, items], idx) => (
        <Card title={CATEGORY_LABELS[category] || category} key={idx} style={{ marginTop: 20 }} headStyle={{ background: '#f8fafc' }} size="small">
          <Table 
            dataSource={items} 
            rowKey="id" 
            pagination={false}
            size="small"
            tableLayout="fixed"
            columns={[
              { title: '指标要求', dataIndex: 'requirement_text', width: '60%' },
              { title: '当前确权资源', key: 'bound_state', width: '25%', render: (_, record) => {
                const boundInfo = bindings[record.id];
                if (boundInfo) {
                  return (
                     <div style={{ display: 'flex', width: '100%', overflow: 'hidden' }}>
                       <Tag 
                          color="blue" 
                          icon={<CheckCircleOutlined />} 
                          style={{ 
                            cursor: 'pointer', 
                            padding: '2px 8px', 
                            fontSize: 13,
                            maxWidth: '100%',
                            display: 'inline-flex',
                            alignItems: 'center'
                          }}
                          title={boundInfo.record_name}
                          onClick={() => {
                             if (boundInfo.target_type === 'auto_matched') {
                                 message.error('系统提示：当前确权为极简文本匹配旧记录产生，无底层ID！请返回上一步点击【重新对标分析】或在本行点击【重新挂靠】将其升格为实体记录。');
                             } else {
                                 let tgtType = record.category;
                                 if (tgtType === 'project_performance') tgtType = 'performance';
                                 if (boundInfo.target_type === 'file') tgtType = 'file';
                                 if (!boundInfo.record_id) return message.warning('未能获取记录ID');
                                 setDetailModalState({ targetType: tgtType, targetId: boundInfo.record_id });
                             }
                          }}
                       >
                          <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                            {boundInfo.record_name}
                          </span>
                       </Tag>
                     </div>
                  );
                }
                return <Text type="danger">未确权</Text>;
              }},
              { title: '操作', key: 'action', width: '15%', align: 'right', render: (_, record) => {
                 const isBound = !!bindings[record.id];
                 return (
                   <Space size="small">
                     <Tooltip title={isBound ? '重新挂靠' : '立即挂靠'}>
                       <Button type={isBound ? "default" : "primary"} size="small" icon={<LinkOutlined />} onClick={() => handleOpenBindModal(record)} />
                     </Tooltip>
                     <Tooltip title="移入独立承诺池（不对标）">
                       <Button type="text" size="small" style={{ color: '#94a3b8' }} icon={<ExportOutlined />} onClick={(e) => handleMoveToGeneric(record.id, e, record.requirement_text)} />
                     </Tooltip>
                   </Space>
                 );
              }}
            ]}
          />
        </Card>
      ))}

      <Modal
        title={`挂靠资源 - ${CATEGORY_LABELS[activeModalItem?.category || '']}`}
        open={!!activeModalItem}
        onCancel={() => setActiveModalItem(null)}
        footer={null}
        width={800}
      >
        <Text style={{ display: 'block', marginBottom: 16 }}>
          <strong>当前需响应要求：</strong> {activeModalItem?.reqText}
        </Text>

        <Tabs defaultActiveKey="library">
          <TabPane tab="从企业资料库选择" key="library">
             {libLoading ? <Text>加载中...</Text> : (
               (activeModalItem?.category === 'financial' || activeModalItem?.category === 'other_requirements') ? (
                 financialSelectedFolderId ? (
                   <div>
                     <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
                       <Button type="link" icon={<LeftOutlined />} onClick={() => setFinancialSelectedFolderId(null)} style={{ paddingLeft: 0 }}>返回文件夹列表</Button>
                       <Button type="primary" onClick={handleConfirmFinancialSelection} disabled={selectedFinancialFileIds.length === 0}>确认关联选中项</Button>
                     </div>
                     {fetchingFolderFiles ? <div style={{ textAlign: 'center' }}><Spin /></div> : (
                        financialFolderFiles.length === 0 ? <Empty description="该文件夹内暂无记录" /> : (
                          <Table
                            dataSource={financialFolderFiles}
                            rowKey="id"
                            size="small"
                            pagination={false}
                            rowSelection={{
                              selectedRowKeys: selectedFinancialFileIds,
                              onChange: (keys) => setSelectedFinancialFileIds(keys)
                            }}
                            onRow={(record) => ({
                              onClick: () => {
                                const index = selectedFinancialFileIds.indexOf(record.id);
                                if (index > -1) {
                                  setSelectedFinancialFileIds(selectedFinancialFileIds.filter(id => id !== record.id));
                                } else {
                                  setSelectedFinancialFileIds([...selectedFinancialFileIds, record.id]);
                                }
                              },
                              style: { cursor: 'pointer' }
                            })}
                            columns={[
                              { title: '文件名称', dataIndex: 'file_name' },
                              { title: '上传时间', render: (_, r) => new Date(r.created_at).toLocaleDateString() }
                            ]}
                          />
                        )
                     )}
                   </div>
                 ) : (
                   libData.length === 0 ? <Empty description="暂无文件夹" /> : (
                     <div style={{ display: 'flex', flexWrap: 'wrap', gap: 24, padding: 12 }}>
                       {libData.map((folder: any) => (
                         <div 
                           key={folder.id} 
                           style={{ width: 100, display: 'flex', flexDirection: 'column', alignItems: 'center', cursor: 'pointer' }}
                           onClick={async () => {
                             setFinancialSelectedFolderId(folder.id);
                             setSelectedFinancialFileIds([]);
                             setFetchingFolderFiles(true);
                             try {
                               const basePath = activeModalItem?.category === 'other_requirements' ? '/api/others' : '/api/financial-reports';
                               const resp = await axios.get(`${basePath}/folders/${folder.id}/files`);
                               setFinancialFolderFiles(resp.data || []);
                             } catch (err) {
                               message.error('加载列表失败');
                             } finally {
                               setFetchingFolderFiles(false);
                             }
                           }}
                         >
                           <FolderFilled style={{ fontSize: 70, color: '#fbbf24', filter: 'drop-shadow(0 2px 4px rgba(0,0,0,0.1))' }} />
                           <Text style={{ marginTop: 8, fontSize: 13 }} ellipsis={{ tooltip: folder.folder_name }}>{folder.folder_name}</Text>
                         </div>
                       ))}
                     </div>
                   )
                 )
               ) : (
                 libData.length === 0 ? (
                   <Empty description="您的资料库中没有检测到对应分类的档案。建议您前往资料库完善结构化信息，或者直接使用右侧的【快捷补充】功能直接上传证明片区。" />
                 ) : (
                   <Table 
                     dataSource={libData} 
                     rowKey="id"
                     size="small"
                     pagination={{ pageSize: 5 }}
                     columns={[
                       { title: '资源名称/证书类别', render: (_, r) => r.qualification_name || r.type_name || r.cert_type || r.name || r.project_name || r.honor_name || r.file_name || r.folder_name || '未命名资源' },
                       { title: '级别/补充说明', render: (_, r) => r.qualification_level || r.qualification_type || r.qualification_certificate_no || r.role_type || r.specialty || r.contract_amount || r.level || r.role || r.amount || '-' },
                       { title: '操作', key: 'action', align: 'right', render: (_, r) => (
                         <Button size="small" type="primary" onClick={() => handleSelectLibItem(r)}>关联此项</Button>
                       )}
                     ]}
                   />
                 )
               )
             )}
          </TabPane>
          <TabPane tab="没有档案？快捷补充证明" key="upload">
             <div style={{ padding: '30px 20px', textAlign: 'center' }}>
               <Alert message="实战技巧：有时招标文件需要一些奇葩证明（比如本市房产租赁合同），我们不必要大费周章地把它填成数据库规范档案。直接在这里像发微信图片一样传扫描件，瞬间响应要求拿分！" type="success" style={{ marginBottom: 20, textAlign: 'left' }} />
               <Upload.Dragger
                  name="file"
                  multiple={false}
                  action="/api/files/upload"
                  headers={{ 'x-company-id': currentCompanyId || '' }}
                  data={{ source_module: 'tender', content_type: 'other' }}
                  onChange={handleFileUpload}
                  showUploadList={false}
               >
                 <p className="ant-upload-drag-icon"><PlusOutlined style={{ fontSize: 40, color: '#1677ff' }} /></p>
                 <p className="ant-upload-text">点击或将证件扫描件拖拽到这里上传</p>
                 <p className="ant-upload-hint">支持 PDF, JPG, PNG 等图像格式。上传完成后该文件将直接入库并锚定当下指标！</p>
               </Upload.Dragger>
             </div>
          </TabPane>
        </Tabs>
      </Modal>
      
      <LibraryDetailModal 
        visible={!!detailModalState}
        targetType={detailModalState?.targetType}
        targetId={detailModalState?.targetId}
        onCancel={() => setDetailModalState(null)}
      />
    </div>
  );
};
