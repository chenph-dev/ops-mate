export default {
  status: {
    connecting: 'Connecting',
    reconnecting: 'Reconnecting ({{count}}/{{max}})',
    connected: 'Connected',
    disconnected: 'Not connected',
  },
  spinner: {
    reconnecting: 'Disconnected, reconnecting...',
    connecting: 'Connecting...',
  },
  menu: {
    copy: 'Copy',
    paste: 'Paste',
    selectAll: 'Select all',
    clear: 'Clear',
  },
  disconnectNotice: '[Session closed] Double-click host to reconnect',
  header: {
    title: 'Terminal',
    noHost: 'No host selected',
    sftp: 'Open SFTP browser',
    aiOpen: 'Collapse agent panel',
    aiClose: 'Open agent panel',
    refresh: 'Refresh connection',
    zoomIn: 'Zoom in (Ctrl++)',
    zoomOut: 'Zoom out (Ctrl+-)',
    search: 'Search (Ctrl+F)',
    clear: 'Clear',
    copy: 'Copy selection',
    disconnect: 'Disconnect',
  },
  statusbar: {
    fontSize: 'Font {{size}}',
  },
  search: {
    placeholder: 'Search terminal content...',
  },
  completion: {
    hint: '{{count}} items · Tab/Enter accept · ↑/↓ select · Esc close',
  },
};
