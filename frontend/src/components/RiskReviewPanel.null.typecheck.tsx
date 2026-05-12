import RiskReviewPanel from './RiskReviewPanel';

export const riskReviewPanelAcceptsNullRisks = (
    <RiskReviewPanel
        projectId="typecheck-project"
        risks={null}
        auditLoading={false}
        onAuditLoadingChange={() => {}}
        onRefreshRisks={() => {}}
        onConfirm={() => {}}
    />
);
