export default {
  title: '运维技能',
  subtitle:
    '按 Anthropic Agent Skills 规范打包（SKILL.md + scripts/），ZIP 上传后 Agent 可按需加载指南并在目标主机远程执行脚本。',
  upload: '上传技能 ZIP',
  empty: '暂无技能，点击上方按钮上传 ZIP。',
  name: '名称',
  description: '描述',
  enabled: '启用',
  delete: '删除',
  deleteConfirm: '删除技能「{{name}}」？文件与配置将一并移除。',
  uploadSuccess: '技能 {{name}} 已安装',
  uploadFailed: '安装失败：{{err}}',
  toggleFailed: '更新失败：{{err}}',
  deleteFailed: '删除失败：{{err}}',
  loadFailed: '加载技能列表失败：{{err}}',
};
