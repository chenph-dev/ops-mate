export default {
  title: 'LLM模型配置',
  subtitle: '配置 AI 提供商。更改后立即生效。',
  provider: 'LLM模型供应商',
  providerRequired: '请选择 LLM 模型供应商',
  providerPlaceholder: '选择 LLM 模型供应商',
  providerOpenAI: 'OpenAI 兼容',
  providerClaude: 'Anthropic',
  baseURLPlaceholder:
    '完整地址（含 /v1），如 https://api.deepseek.com/v1；留空用默认',
  apiKeyPlaceholder: '输入 API Key（本地服务可留空）',
  model: '模型',
  modelRequired: '请输入模型名称',
  modelPlaceholder: '如 claude-sonnet-5 / gpt-4o / deepseek-chat',
  save: '保存配置',
  reset: '重置',
  saveSuccess: '配置已保存',
  saveFailed: '保存失败: {{err}}',
};
