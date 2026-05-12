import React from 'react';
import { BrowserRouter, Routes, Route, Navigate, useLocation } from 'react-router-dom';
import { ConfigProvider } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';

dayjs.locale('zh-cn');

const customLocale = {
    ...zhCN,
    Pagination: {
        ...zhCN.Pagination,
        items_per_page: '条/页',
        jump_to: '跳至',
        page: '页',
    }
};

import type { Location } from 'react-router-dom';
import MainLayout from './layout/MainLayout';
import LibraryLayout from './layout/LibraryLayout';
import BidPrepLayout from './layout/BidPrepLayout';
import Dashboard from './pages/Dashboard';
import PerformanceLibrary from './pages/PerformanceLibrary';
import PerformanceCreate from './pages/PerformanceCreate';
import PerformanceDetail from './pages/PerformanceDetail';
import PerformanceEdit from './pages/PerformanceEdit';
import FileCenter from './pages/FileCenter';
import SystemSettings from './pages/SystemSettings';
import PersonLibrary from './pages/PersonLibrary';
import PersonDetail from './pages/PersonDetail';

import QualificationLibrary from './pages/QualificationLibrary';
import QualificationDetail from './pages/QualificationDetail';
import QualificationEdit from './pages/QualificationEdit';
import HonorLibrary from './pages/HonorLibrary';
import HonorDetail from './pages/HonorDetail';
import HonorEdit from './pages/HonorEdit';
import FinancialReportLibrary from './pages/FinancialReportLibrary';
import FinancialReportFolderDetail from './pages/FinancialReportFolderDetail';
import OtherLibrary from './pages/OtherLibrary';
import OtherFolderDetail from './pages/OtherFolderDetail';
import AuditDetail from './pages/AuditDetail';
import AuditDetailDrawerRoute from './pages/AuditDetailDrawerRoute';
import IssueCenter from './pages/IssueCenter';
import ExportCenter from './pages/ExportCenter';
import BidProjectList from './pages/BidProjectList';
import BidProjectWorkbench from './pages/BidProjectWorkbench';
import TechBidProjectList from './pages/TechBidProjectList';
import TechBidProjectWorkbench from './pages/TechBidProjectWorkbench';
import TechBidPlaceholder from './pages/TechBidPlaceholder';
import TechKnowledgeHub from './pages/TechKnowledgeHub';
import TechHistoryLibrary from './pages/TechHistoryLibrary';
import TechHistoryProjectDetail from './pages/TechHistoryProjectDetail';
import { CompanyProvider } from './context/CompanyContext';
import 'antd/dist/reset.css';
import './App.css';

/** Supports opening /audits/:id as overlay while keeping file-center URL in state.background */
const AppRoutes: React.FC = () => {
    const location = useLocation();
    const background = (location.state as { background?: Location } | null | undefined)?.background;

    return (
        <>
            <Routes location={background ?? location}>
                {/* 无 path 的 layout：避免 RR7 下 path="/" 仅匹配根路径导致子路由 Outlet 空白 */}
                <Route element={<MainLayout />}>
                        <Route index element={<Navigate to="/dashboard" replace />} />
                        <Route path="dashboard" element={<Dashboard />} />
                        
                        {/* Library Routes Wrapped with LibraryLayout */}
                        <Route path="library" element={<Navigate to="/library/qualifications" replace />} />
                        <Route element={<LibraryLayout />}>
                            <Route path="library/performances" element={<PerformanceLibrary />} />
                            <Route path="library/performances/create" element={<PerformanceCreate />} />
                            <Route path="library/performances/:id" element={<PerformanceDetail />} />
                            <Route path="library/performances/:id/edit" element={<PerformanceEdit />} />
                            <Route path="library/persons" element={<PersonLibrary />} />

                            <Route path="library/persons/:id" element={<PersonDetail />} />
                            <Route path="library/qualifications" element={<QualificationLibrary />} />
                            <Route path="library/qualifications/:id/edit" element={<QualificationEdit />} />
                            <Route path="library/qualifications/:id" element={<QualificationDetail />} />
                            <Route path="library/honors" element={<HonorLibrary />} />
                            <Route path="library/honors/:id/edit" element={<HonorEdit />} />
                            <Route path="library/honors/:id" element={<HonorDetail />} />
                            <Route path="library/financial-reports" element={<FinancialReportLibrary />} />
                            <Route path="library/financial-reports/:folderId" element={<FinancialReportFolderDetail />} />
                            <Route path="library/others" element={<OtherLibrary />} />
                            <Route path="library/others/:folderId" element={<OtherFolderDetail />} />
                            
                            {/* Tech Knowledge Repository mapped to Library Layout */}
                            <Route path="tech-library/history/:projectId" element={<TechHistoryProjectDetail />} />
                            <Route path="tech-library/history" element={<TechHistoryLibrary />} />
                            <Route path="tech-library/knowledge" element={<Navigate to="/tech-library/knowledge/method" replace />} />
                            <Route path="tech-library/knowledge/:type" element={<TechKnowledgeHub />} />
                            <Route path="tech-library/methods" element={<Navigate to="/tech-library/knowledge/method" replace />} />
                            <Route path="tech-library/equipments" element={<Navigate to="/tech-library/knowledge/equipment" replace />} />
                            <Route path="tech-library/systems" element={<Navigate to="/tech-library/knowledge/system" replace />} />
                            <Route path="tech-library/performance" element={<Navigate to="/tech-library/knowledge/performance" replace />} />
                            <Route path="tech-library/risks" element={<Navigate to="/tech-library/knowledge/risks" replace />} />
                            <Route path="tech-library/regions" element={<Navigate to="/tech-library/knowledge/regions" replace />} />
                            <Route path="tech-library/subcontractors" element={<Navigate to="/tech-library/knowledge/subcontractors" replace />} />
                            <Route path="tech-library/costs" element={<Navigate to="/tech-library/knowledge/costs" replace />} />
                            <Route path="tech-library/parses" element={<Navigate to="/tech-library/knowledge/method" replace />} />
                        </Route>

                        {/* Functional Routes */}
                        <Route path="file-center" element={<FileCenter />} />
                        <Route path="file-center/repository" element={<FileCenter independentTab="repository" />} />
                        <Route path="file-center/import" element={<FileCenter independentTab="import" />} />
                        <Route path="imports" element={<Navigate to="/file-center/import" replace />} />
                        <Route path="file-center/audit" element={<FileCenter independentTab="audit" />} />
                        <Route path="audits" element={<Navigate to="/file-center/audit" replace />} />
                        <Route path="audits/:id" element={<AuditDetail />} />
                        <Route path="issues" element={<IssueCenter />} />
                        <Route path="exports" element={<ExportCenter />} />
                        
                        {/* Bid Preparation Wrapped with BidPrepLayout */}
                        <Route path="bid-prep" element={<Navigate to="/bid-projects" replace />} />
                        <Route element={<BidPrepLayout />}>
                            <Route path="bid-projects" element={<BidProjectList />} />
                            <Route path="bid-projects/:id" element={<BidProjectWorkbench />} />
                            <Route path="tech-bid-projects" element={<TechBidProjectList />} />
                            <Route path="tech-bid-projects/:id" element={<TechBidProjectWorkbench />} />
                            <Route path="tech-bid" element={<TechBidPlaceholder />} />
                        </Route>
                        
                        {/* Bid Preparation */}
                        <Route path="tech-library/styles" element={<Navigate to="/tech-library/knowledge/method" replace />} />
                        
                        {/* System */}
                        <Route path="settings" element={<SystemSettings />} />
                    </Route>
                </Routes>
            {background != null && (
                <Routes>
                    <Route path="/audits/:id" element={<AuditDetailDrawerRoute />} />
                </Routes>
            )}
        </>
    );
};

const App: React.FC = () => {
    return (
        <ConfigProvider locale={customLocale}>
            <CompanyProvider>
                <BrowserRouter>
                    <AppRoutes />
                </BrowserRouter>
            </CompanyProvider>
        </ConfigProvider>
    );
};

export default App;
