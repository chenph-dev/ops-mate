import { useState, useCallback, useRef } from 'react';
import { ExecuteCommand } from '@wailsjs/go/handler/HostsHandler';

export interface TerminalEntry {
  id: string;
  type: 'input' | 'output' | 'error' | 'system';
  text: string;
}

export function useTerminal(hostId: string | null) {
  const [entries, setEntries] = useState<TerminalEntry[]>([]);
  const [executing, setExecuting] = useState(false);
  const idCounter = useRef(0);

  const addEntry = useCallback((type: TerminalEntry['type'], text: string) => {
    idCounter.current += 1;
    const entry: TerminalEntry = { id: `term-${idCounter.current}`, type, text };
    setEntries((prev) => [...prev, entry]);
    return entry.id;
  }, []);

  const clearEntries = useCallback(() => {
    setEntries([]);
  }, []);

  const runCommand = useCallback(async (command: string) => {
    if (!hostId || executing) return;
    const cmd = command.trim();
    if (!cmd) return;

    addEntry('input', `$ ${cmd}`);
    setExecuting(true);
    try {
      const output = await ExecuteCommand(hostId, cmd);
      if (output.trim()) {
        addEntry('output', output.trimEnd());
      }
    } catch (err) {
      addEntry('error', `错误: ${err}`);
    } finally {
      setExecuting(false);
    }
  }, [hostId, executing, addEntry]);

  return { entries, executing, runCommand, clearEntries };
}
