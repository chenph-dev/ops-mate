export default {
  title: 'Ops Skills',
  subtitle:
    'Packaged per Anthropic Agent Skills spec (SKILL.md + scripts/). Upload a ZIP; the Agent can load the guide on demand and run scripts remotely on the target host.',
  upload: 'Upload Skill ZIP',
  empty: 'No skills yet. Click the button above to upload a ZIP.',
  name: 'Name',
  description: 'Description',
  enabled: 'Enabled',
  delete: 'Delete',
  deleteConfirm: 'Delete skill "{{name}}"? Files and config will be removed.',
  uploadSuccess: 'Skill {{name}} installed',
  uploadFailed: 'Install failed: {{err}}',
  toggleFailed: 'Update failed: {{err}}',
  deleteFailed: 'Delete failed: {{err}}',
  loadFailed: 'Failed to load skills: {{err}}',
};
