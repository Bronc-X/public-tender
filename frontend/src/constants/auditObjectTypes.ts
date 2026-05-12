/** 与智能审核校验案台顶部「推荐入库」下拉选项一致 */
export const AUDIT_OBJECT_TYPE_OPTIONS = [
  { label: '通用文本', value: 'general' },
  { label: '人员档案', value: 'person' },
  { label: '资质证书', value: 'qualification' },
  { label: '劳动合同', value: 'laborcontract' },
  { label: '施工合同', value: 'performance' },
] as const;

export function labelForAuditObjectType(value: string | undefined | null): string {
  if (value == null || value === '') return '—';
  const found = AUDIT_OBJECT_TYPE_OPTIONS.find((o) => o.value === value);
  return found?.label ?? value;
}
