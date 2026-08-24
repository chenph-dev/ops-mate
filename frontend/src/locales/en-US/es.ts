export default {
  toolbar: {
    refresh: 'Refresh',
  },
  browser: {
    title: 'Indices',
    empty: 'No indices',
    loadFailed: 'Failed to load indices: {{err}}',
  },
  query: {
    title: 'Query',
    indexPlaceholder: 'Index (select on the left)',
    run: 'Run',
    empty: 'Enter a DSL query and run; matching documents appear here',
    failed: 'Execution failed: {{err}}',
  },
  terminal: {
    title: 'Command Terminal',
    placeholder: 'Enter an ES REST path, Enter to run (e.g. _cat/indices)',
    empty: 'Command output appears here',
    failed: 'Execution failed: {{err}}',
  },
  info: {
    title: 'Cluster Info',
    loadFailed: 'Failed to load cluster info: {{err}}',
  },
};
