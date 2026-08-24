export default {
  toolbar: {
    refresh: '刷新',
  },
  browser: {
    title: '索引',
    empty: '暂无索引',
    loadFailed: '加载索引失败：{{err}}',
  },
  query: {
    title: '查询',
    indexPlaceholder: '索引（左侧选择）',
    run: '执行',
    empty: '输入 DSL 查询后执行，命中文档显示在此',
    failed: '执行失败：{{err}}',
  },
  terminal: {
    title: '命令终端',
    placeholder: '输入 ES REST 路径，回车执行（如 _cat/indices）',
    empty: '执行命令后输出显示在此',
    failed: '执行失败：{{err}}',
  },
  info: {
    title: '集群信息',
    loadFailed: '加载集群信息失败：{{err}}',
  },
};
