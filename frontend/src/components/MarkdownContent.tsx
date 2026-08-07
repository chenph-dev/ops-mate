import { useState, type ReactNode } from 'react';
import ReactMarkdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { ClipboardSetText } from '@wailsjs/runtime/runtime';

const MONO = '"Cascadia Code", "Fira Code", "Consolas", monospace';

/** 从 React 元素递归提取纯文本（用于代码块复制）。 */
function getText(node: ReactNode): string {
  if (typeof node === 'string' || typeof node === 'number') return String(node);
  if (Array.isArray(node)) return node.map(getText).join('');
  if (node && typeof node === 'object' && 'props' in node) {
    const p = (node as { props?: { children?: ReactNode } }).props;
    return getText(p?.children);
  }
  return '';
}

/** 代码块：等宽深色背景 + 右上角复制按钮。 */
function CodeBlock({ children }: { children: ReactNode }): React.JSX.Element {
  const [copied, setCopied] = useState(false);
  const handleCopy = (): void => {
    const text = getText(children).replace(/\n$/, '');
    void ClipboardSetText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };
  return (
    <div style={{ position: 'relative', margin: '4px 0' }}>
      <pre
        style={{
          fontFamily: MONO,
          fontSize: 11,
          lineHeight: 1.5,
          background: 'rgba(0,0,0,0.3)',
          border: '1px solid var(--antd-color-border-secondary)',
          borderRadius: 4,
          padding: '6px 8px',
          overflow: 'auto',
          margin: 0,
          color: 'var(--antd-color-text)',
        }}
      >
        {children}
      </pre>
      <button
        onClick={() => void handleCopy()}
        style={{
          position: 'absolute',
          top: 4,
          right: 4,
          fontSize: 10,
          padding: '1px 6px',
          background: 'var(--antd-color-bg-elevated)',
          border: '1px solid var(--antd-color-border)',
          borderRadius: 4,
          color: 'var(--antd-color-text-secondary)',
          cursor: 'pointer',
          opacity: 0.8,
        }}
      >
        {copied ? '已复制' : '复制'}
      </button>
    </div>
  );
}

const components: Components = {
  pre({ children }) {
    return <CodeBlock>{children}</CodeBlock>;
  },
  code({ node, className, children, ...props }) {
    // 块级代码：带 language 前缀，或跨行（fenced 无语言标注时）
    const isBlock = Boolean(
      (className && /language-[\w-]+/.test(className)) ||
      (node?.position && node.position.start.line !== node.position.end.line),
    );
    if (isBlock) {
      return (
        <code
          className={className}
          style={{ fontFamily: MONO, fontSize: 11, whiteSpace: 'pre' }}
          {...props}
        >
          {children}
        </code>
      );
    }
    // 行内代码
    return (
      <code
        style={{
          fontFamily: MONO,
          fontSize: 11,
          background: 'rgba(0,0,0,0.25)',
          padding: '1px 4px',
          borderRadius: 3,
          color: 'var(--antd-color-text)',
        }}
        {...props}
      >
        {children}
      </code>
    );
  },
  p({ children }) {
    return (
      <p
        style={{
          margin: '0 0 4px',
          fontSize: 12,
          lineHeight: 1.5,
          color: 'var(--antd-color-text)',
        }}
      >
        {children}
      </p>
    );
  },
  ul({ children }) {
    return (
      <ul
        style={{
          margin: '2px 0',
          paddingLeft: 18,
          fontSize: 12,
          lineHeight: 1.6,
          color: 'var(--antd-color-text)',
        }}
      >
        {children}
      </ul>
    );
  },
  ol({ children }) {
    return (
      <ol
        style={{
          margin: '2px 0',
          paddingLeft: 18,
          fontSize: 12,
          lineHeight: 1.6,
          color: 'var(--antd-color-text)',
        }}
      >
        {children}
      </ol>
    );
  },
  li({ children }) {
    return <li style={{ marginBottom: 2 }}>{children}</li>;
  },
  h1({ children }) {
    return <h1 style={headingStyle(16)}>{children}</h1>;
  },
  h2({ children }) {
    return <h2 style={headingStyle(14)}>{children}</h2>;
  },
  h3({ children }) {
    return <h3 style={headingStyle(13)}>{children}</h3>;
  },
  h4({ children }) {
    return <h4 style={headingStyle(12)}>{children}</h4>;
  },
  a({ href, children }) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noreferrer"
        style={{ color: 'var(--antd-color-primary)' }}
      >
        {children}
      </a>
    );
  },
  blockquote({ children }) {
    return (
      <blockquote
        style={{
          margin: '4px 0',
          paddingLeft: 8,
          borderLeft: '3px solid var(--antd-color-border-secondary)',
          color: 'var(--antd-color-text-secondary)',
        }}
      >
        {children}
      </blockquote>
    );
  },
  table({ children }) {
    return (
      <div style={{ overflow: 'auto', margin: '4px 0' }}>
        <table
          style={{
            borderCollapse: 'collapse',
            fontSize: 11,
            lineHeight: 1.5,
            width: '100%',
          }}
        >
          {children}
        </table>
      </div>
    );
  },
  th({ children }) {
    return (
      <th
        style={{
          border: '1px solid var(--antd-color-border-secondary)',
          padding: '3px 8px',
          fontWeight: 600,
          textAlign: 'left',
          background: 'rgba(0,0,0,0.2)',
        }}
      >
        {children}
      </th>
    );
  },
  td({ children }) {
    return (
      <td
        style={{
          border: '1px solid var(--antd-color-border-secondary)',
          padding: '3px 8px',
        }}
      >
        {children}
      </td>
    );
  },
  hr() {
    return (
      <hr
        style={{
          border: 'none',
          borderTop: '1px solid var(--antd-color-border-secondary)',
          margin: '6px 0',
        }}
      />
    );
  },
};

function headingStyle(fontSize: number): React.CSSProperties {
  return {
    fontSize,
    fontWeight: 600,
    margin: '6px 0 4px',
    color: 'var(--antd-color-text)',
  };
}

/** 模型输出 Markdown 渲染：支持表格/代码块/列表，代码块带复制按钮。 */
export default function MarkdownContent({
  content,
}: {
  content: string;
}): React.JSX.Element {
  return (
    <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
      {content}
    </ReactMarkdown>
  );
}
