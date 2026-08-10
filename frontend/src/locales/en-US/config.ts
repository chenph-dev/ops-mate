export default {
  title: 'LLM Model Config',
  subtitle: 'Configure the AI provider. Changes take effect immediately.',
  provider: 'LLM Model Provider',
  providerRequired: 'Select an LLM model provider',
  providerPlaceholder: 'Select an LLM model provider',
  providerOpenAI: 'OpenAI compatible',
  providerClaude: 'Anthropic',
  baseURLPlaceholder:
    'Full URL (with /v1), e.g. https://api.deepseek.com/v1; leave empty for default',
  apiKeyPlaceholder: 'Enter API key (leave empty for local services)',
  model: 'Model',
  modelRequired: 'Enter a model name',
  modelPlaceholder: 'e.g. claude-sonnet-5 / gpt-4o / deepseek-chat',
  save: 'Save config',
  reset: 'Reset',
  saveSuccess: 'Config saved',
  saveFailed: 'Save failed: {{err}}',
};
