import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Terminal as XTerm } from "@xterm/xterm";
import rawCommands from "@/data/commands.json";

export interface CommandTemplate {
  text: string;
  desc: string;
}

export interface CommandEntry {
  name: string;
  desc: string;
  templates?: CommandTemplate[];
}

interface CommandMatch {
  name: string;
  desc: string;
  text: string;
  type: "command" | "template";
}

export interface CompletionState {
  open: boolean;
  mode: "command" | "template";
  matches: CommandMatch[];
  selectedIndex: number;
  prefix: string;
  position: { x: number; y: number } | null;
}

const staticCommands: CommandEntry[] = rawCommands as CommandEntry[];

const MAX_MATCHES = 50;
const MIN_PREFIX_LEN = 1;

function buildCommandMap(
  staticEntries: CommandEntry[],
  remoteEntries: CommandEntry[],
): Map<string, CommandEntry> {
  const map = new Map<string, CommandEntry>();
  // 远程命令先写入，静态命令覆盖（保留模板和更完整描述）
  for (const entry of remoteEntries) {
    map.set(entry.name.toLowerCase(), entry);
  }
  for (const entry of staticEntries) {
    const key = entry.name.toLowerCase();
    const existing = map.get(key);
    map.set(key, {
      ...entry,
      desc: entry.desc || existing?.desc || "",
    });
  }
  return map;
}

function getCursorPixelPosition(xterm: XTerm): { x: number; y: number } | null {
  const element = xterm.element;
  if (!element) return null;

  const screen = element.querySelector(".xterm-screen") as HTMLElement | null;
  if (!screen) return null;

  const screenRect = screen.getBoundingClientRect();
  if (xterm.cols === 0 || xterm.rows === 0) return null;

  const cellWidth = screenRect.width / xterm.cols;
  const cellHeight = screenRect.height / xterm.rows;
  const { cursorX, cursorY } = xterm.buffer.active;

  return {
    x: screenRect.left + cursorX * cellWidth,
    y: screenRect.top + (cursorY + 1) * cellHeight,
  };
}

function isPrintable(charCode: number): boolean {
  return charCode >= 0x20 && charCode <= 0x7e;
}

interface UseCommandCompletionReturn {
  completion: CompletionState;
  handleInputData: (data: string) => void;
  handleKeyEvent: (e: KeyboardEvent) => boolean;
  onCursorMove: () => void;
  closeCompletion: () => void;
  acceptMatch: (match: CommandMatch) => void;
  loadRemoteCommands: (
    hostID: string,
    fetcher: (id: string) => Promise<CommandEntry[]>,
  ) => Promise<void>;
}

export function useCommandCompletion(
  xtermRef: React.RefObject<XTerm | null>,
  sendData: (data: string) => void,
): UseCommandCompletionReturn {
  const sendDataRef = useRef(sendData);

  const remoteCommandsRef = useRef<CommandEntry[]>([]);
  const commandsMapRef = useRef<Map<string, CommandEntry>>(
    buildCommandMap(staticCommands, []),
  );
  const inputBufferRef = useRef("");

  const [completion, setCompletion] = useState<CompletionState>({
    open: false,
    mode: "command",
    matches: [],
    selectedIndex: 0,
    prefix: "",
    position: null,
  });

  const closeCompletion = useCallback(() => {
    setCompletion((prev) =>
      prev.open
        ? { ...prev, open: false, matches: [], selectedIndex: 0 }
        : prev,
    );
  }, []);

  const computeMatches = useCallback(
    (buffer: string) => {
      const trimmed = buffer.trimStart();
      if (
        trimmed.length < MIN_PREFIX_LEN ||
        /[|;&<>()`$]/.test(trimmed)
      ) {
        setCompletion((prev) =>
          prev.open
            ? { ...prev, open: false, matches: [], selectedIndex: 0 }
            : prev,
        );
        return;
      }

      const xterm = xtermRef.current;
      const position = xterm ? getCursorPixelPosition(xterm) : null;

      const spaceIndex = trimmed.search(/\s/);
      if (spaceIndex === -1) {
        // 命令模式
        const prefix = trimmed.toLowerCase();
        const matches: CommandMatch[] = [];
        for (const entry of commandsMapRef.current.values()) {
          if (entry.name.toLowerCase().startsWith(prefix)) {
            matches.push({
              name: entry.name,
              desc: entry.desc,
              text: entry.name,
              type: "command",
            });
          }
          if (matches.length >= MAX_MATCHES) break;
        }
        matches.sort((a, b) => a.name.localeCompare(b.name));
        setCompletion({
          open: matches.length > 0,
          mode: "command",
          matches,
          selectedIndex: 0,
          prefix: trimmed,
          position,
        });
        return;
      }

      // 参数模板模式
      const firstWord = trimmed.slice(0, spaceIndex);
      const rest = trimmed.slice(spaceIndex + 1);
      const entry = commandsMapRef.current.get(firstWord.toLowerCase());
      const templates = entry?.templates;
      if (!templates || templates.length === 0 || /\s/.test(rest)) {
        setCompletion((prev) =>
          prev.open
            ? { ...prev, open: false, matches: [], selectedIndex: 0 }
            : prev,
        );
        return;
      }

      const prefix = trimmed.toLowerCase();
      const matches: CommandMatch[] = [];
      for (const template of templates) {
        if (template.text.toLowerCase().startsWith(prefix)) {
          matches.push({
            name: template.text,
            desc: template.desc,
            text: template.text,
            type: "template",
          });
        }
        if (matches.length >= MAX_MATCHES) break;
      }
      matches.sort((a, b) => a.name.localeCompare(b.name));
      setCompletion({
        open: matches.length > 0,
        mode: "template",
        matches,
        selectedIndex: 0,
        prefix: trimmed,
        position,
      });
    },
    [xtermRef],
  );

  const acceptMatch = useCallback((match: CommandMatch) => {
    const current = inputBufferRef.current;
    const trimmed = current.trimStart();
    if (match.text !== trimmed) {
      // 退格删除已输入前缀，再写入完整匹配文本
      const backspaces = "\x7f".repeat(trimmed.length);
      sendDataRef.current(backspaces + match.text);
      inputBufferRef.current = match.text;
    }
    setCompletion((prev) =>
      prev.open
        ? { ...prev, open: false, matches: [], selectedIndex: 0 }
        : prev,
    );
  }, []);

  const acceptSelected = useCallback(() => {
    setCompletion((prev) => {
      if (!prev.open || prev.matches.length === 0) return prev;
      acceptMatch(prev.matches[prev.selectedIndex]);
      return { ...prev, open: false, matches: [], selectedIndex: 0 };
    });
  }, [acceptMatch]);

  const handleInputData = useCallback(
    (data: string) => {
      let buffer = inputBufferRef.current;

      for (const ch of data) {
        const code = ch.charCodeAt(0);
        if (code === 0x0d || code === 0x0a) {
          // Enter / 换行：命令已执行，重置
          buffer = "";
        } else if (code === 0x7f) {
          // Backspace
          buffer = buffer.slice(0, -1);
        } else if (code === 0x03 || code === 0x15) {
          // Ctrl+C / Ctrl+U
          buffer = "";
        } else if (code === 0x09) {
          // Tab：不加入缓冲区，交给 key handler 处理
          continue;
        } else if (code === 0x1b) {
          // Escape 或控制序列（方向键等）：关闭补全并清空缓冲区
          buffer = "";
        } else if (isPrintable(code)) {
          buffer += ch;
        }
      }

      inputBufferRef.current = buffer;

      if (inputBufferRef.current.trimStart().length < MIN_PREFIX_LEN) {
        closeCompletion();
        return;
      }
      computeMatches(inputBufferRef.current);
    },
    [closeCompletion, computeMatches],
  );

  const handleKeyEvent = useCallback(
    (e: KeyboardEvent): boolean => {
      // xterm 的 attachCustomKeyEventHandler 对 keydown/keypress/keyup 都会回调。
      // 若不区分事件类型，keyup 的 ArrowDown/ArrowUp 会再次触发选中移动，
      // 导致按一次方向键选中项跳两行。这里仅处理 keydown。
      if (e.type !== "keydown") return true;
      if (!completion.open) return true;

      if (e.key === "Tab" || e.key === "Enter") {
        e.preventDefault();
        acceptSelected();
        return false;
      }
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setCompletion((prev) => ({
          ...prev,
          selectedIndex:
            prev.matches.length === 0
              ? 0
              : (prev.selectedIndex + 1) % prev.matches.length,
        }));
        return false;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setCompletion((prev) => ({
          ...prev,
          selectedIndex:
            prev.matches.length === 0
              ? 0
              : (prev.selectedIndex - 1 + prev.matches.length) %
                prev.matches.length,
        }));
        return false;
      }
      if (e.key === "Escape") {
        closeCompletion();
        return false;
      }
      return true;
    },
    [completion.open, acceptSelected, closeCompletion],
  );

  const onCursorMove = useCallback(() => {
    const xterm = xtermRef.current;
    if (!xterm || !completion.open) return;
    const position = getCursorPixelPosition(xterm);
    if (position) {
      setCompletion((prev) => ({ ...prev, position }));
    }
  }, [completion.open, xtermRef]);

  const loadRemoteCommands = useCallback(
    async (
      hostID: string,
      fetcher: (id: string) => Promise<CommandEntry[]>,
    ): Promise<void> => {
      try {
        const entries = await fetcher(hostID);
        remoteCommandsRef.current = entries;
        commandsMapRef.current = buildCommandMap(staticCommands, entries);
      } catch {
        // 远程命令抓取失败不影响静态补全
      }
    },
    [],
  );

  // 暴露稳定回调，内部委托给 ref
  const handleInputDataRef = useRef(handleInputData);
  const handleKeyEventRef = useRef(handleKeyEvent);
  const onCursorMoveRef = useRef(onCursorMove);
  const closeCompletionRef = useRef(closeCompletion);
  const acceptMatchRef = useRef(acceptMatch);

  useEffect(() => {
    sendDataRef.current = sendData;
    handleInputDataRef.current = handleInputData;
    handleKeyEventRef.current = handleKeyEvent;
    onCursorMoveRef.current = onCursorMove;
    closeCompletionRef.current = closeCompletion;
    acceptMatchRef.current = acceptMatch;
  });

  const handleInputDataStable = useCallback(
    (data: string) => handleInputDataRef.current(data),
    [],
  );
  const handleKeyEventStable = useCallback(
    (e: KeyboardEvent) => handleKeyEventRef.current(e),
    [],
  );
  const onCursorMoveStable = useCallback(
    () => onCursorMoveRef.current(),
    [],
  );
  const closeCompletionStable = useCallback(
    () => closeCompletionRef.current(),
    [],
  );
  const acceptMatchStable = useCallback(
    (match: CommandMatch) => acceptMatchRef.current(match),
    [],
  );

  return useMemo(
    () => ({
      completion,
      handleInputData: handleInputDataStable,
      handleKeyEvent: handleKeyEventStable,
      onCursorMove: onCursorMoveStable,
      closeCompletion: closeCompletionStable,
      acceptMatch: acceptMatchStable,
      loadRemoteCommands,
    }),
    [
      completion,
      handleInputDataStable,
      handleKeyEventStable,
      onCursorMoveStable,
      closeCompletionStable,
      acceptMatchStable,
      loadRemoteCommands,
    ],
  );
}
