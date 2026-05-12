import React, { createContext, useContext, useState, useEffect, useCallback, useMemo } from 'react';
import axios from 'axios';
import { message } from 'antd';

interface Company {
  id: string;
  company_name: string;
  unified_social_credit_code?: string | null;
  legal_person?: string | null;
  legal_person_id_card?: string | null;
  address?: string | null;
}

interface CompanyContextType {
  currentCompanyId: string;
  setCurrentCompanyId: (id: string) => void;
  companies: Company[];
  loading: boolean;
  refreshCompanies: () => void;
}

const CompanyContext = createContext<CompanyContextType | undefined>(undefined);

export const CompanyProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [currentCompanyId, setCurrentCompanyIdState] = useState<string>(() => {
    const saved = localStorage.getItem('current_company_id');
    return (saved && saved !== 'undefined' && saved !== 'null') ? saved : 'c1';
  });
  const [companies, setCompanies] = useState<Company[]>([]);
  const [loading, setLoading] = useState(true);

  // 勿依赖 currentCompanyId：否则「纠正默认公司」会换新函数 → useEffect 再拉取 → 全树反复重渲染，子页面（如标书详情）会卡死
  const fetchCompanies = useCallback(async () => {
    try {
      const response = await axios.get('/api/companies');
      const data = response.data as Company[];
      setCompanies(data);

      setCurrentCompanyIdState((prev) => {
        if (data.length > 0 && !data.find((c) => c.id === prev)) {
          const firstId = data[0].id;
          localStorage.setItem('current_company_id', firstId);
          return firstId;
        }
        return prev;
      });
    } catch (err) {
      console.error('Failed to fetch companies', err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchCompanies();
  }, [fetchCompanies]);

  useEffect(() => {
    // Setup Axios global interceptor for company ID
    const interceptor = axios.interceptors.request.use(config => {
      if (currentCompanyId && currentCompanyId !== 'undefined' && currentCompanyId !== 'null') {
        config.headers['X-Company-Id'] = currentCompanyId;
      } else {
        config.headers['X-Company-Id'] = 'c1';
      }
      return config;
    });
    return () => axios.interceptors.request.eject(interceptor);
  }, [currentCompanyId]);

  const setCurrentCompanyId = useCallback((id: string) => {
    setCurrentCompanyIdState(id);
    localStorage.setItem('current_company_id', id);
    const comp = companies.find((c) => c.id === id);
    message.success(`已切换至：${comp?.company_name || '选定公司'}`);
  }, [companies]);

  const contextValue = useMemo(
    () => ({
      currentCompanyId,
      setCurrentCompanyId,
      companies,
      loading,
      refreshCompanies: fetchCompanies,
    }),
    [currentCompanyId, setCurrentCompanyId, companies, loading, fetchCompanies]
  );

  return (
    <CompanyContext.Provider value={contextValue}>
      {children}
    </CompanyContext.Provider>
  );
};

export const useCompany = () => {
  const context = useContext(CompanyContext);
  if (context === undefined) {
    throw new Error('useCompany must be used within a CompanyProvider');
  }
  return context;
};
