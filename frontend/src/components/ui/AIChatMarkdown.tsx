import { useState, useCallback, isValidElement, memo, type ReactNode } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';

const REMARK_PLUGINS = [remarkGfm];
const REHYPE_PLUGINS = [rehypeHighlight];

interface Props {
  content: string;
}

const LANG_LABELS: Record<string, string> = {
  csharp: 'C#',
  cs: 'C#',
  cpp: 'C++',
  'c++': 'C++',
  cxx: 'C++',
  go: 'Go',
  golang: 'Go',
  java: 'Java',
  python: 'Python',
  py: 'Python',
  python3: 'Python',
  javascript: 'JavaScript',
  js: 'JavaScript',
  typescript: 'TypeScript',
  ts: 'TypeScript',
  rust: 'Rust',
  rs: 'Rust',
  sql: 'SQL',
  postgres: 'PostgreSQL',
  bash: 'Bash',
  sh: 'Shell',
  json: 'JSON',
  yaml: 'YAML',
  yml: 'YAML',
  html: 'HTML',
  css: 'CSS',
};

function langLabel(lang: string): string {
  if (!lang) return 'Code';
  return LANG_LABELS[lang.toLowerCase()] || lang;
}

function extractText(node: ReactNode): string {
  if (typeof node === 'string') return node;
  if (typeof node === 'number') return String(node);
  if (Array.isArray(node)) return node.map(extractText).join('');
  if (isValidElement(node) && node.props && (node.props as { children?: ReactNode }).children) {
    return extractText((node.props as { children?: ReactNode }).children);
  }
  return '';
}

function CodeBlock({ children }: { children: ReactNode }) {
  const [copied, setCopied] = useState(false);

  // Attempt to find language from code element's className
  let lang = '';
  if (isValidElement(children) && children.props) {
    const className = (children.props as { className?: string }).className || '';
    const match = /language-([a-zA-Z0-9_-]+)/.exec(className);
    if (match) {
      lang = match[1];
    }
  }

  const rawCode = extractText(children).replace(/\n$/, '');

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(rawCode);
      setCopied(true);
      setTimeout(() => setCopied(false), 1600);
    } catch {
      // clipboard access denied
    }
  }, [rawCode]);

  return (
    <div className="my-2.5 overflow-hidden rounded-xl border border-bdr/80 bg-[#121215] shadow-md">
      {/* Code Header Toolbar */}
      <div className="flex items-center justify-between px-3 py-1.5 border-b border-white/[0.08] bg-bg-3/70 text-[11px] font-mono text-tx-3 select-none">
        <span className="font-semibold text-tx-2 text-[11.5px]">{langLabel(lang)}</span>
        <button
          type="button"
          onClick={handleCopy}
          className="flex items-center gap-1.5 px-2 py-0.5 rounded-md hover:bg-white/10 text-tx-3 hover:text-tx-1 transition-colors text-[11px]"
          title="Скопировать код"
        >
          {copied ? (
            <>
              <svg
                width="12"
                height="12"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="text-ok"
              >
                <polyline points="20 6 9 17 4 12" />
              </svg>
              <span className="text-ok font-medium">Скопировано</span>
            </>
          ) : (
            <>
              <svg
                width="12"
                height="12"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
              </svg>
              <span>Копировать</span>
            </>
          )}
        </button>
      </div>

      {/* Code Body */}
      <pre className="p-3 overflow-x-auto text-[12.5px] leading-relaxed font-mono bg-transparent text-tx-1 m-0">
        {children}
      </pre>
    </div>
  );
}

const CHAT_MARKDOWN_COMPONENTS = {
  pre: ({ children }: { children?: ReactNode }) => <CodeBlock>{children}</CodeBlock>,
  a: ({ href, children }: { href?: string; children?: ReactNode }) => (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="text-brand hover:underline"
    >
      {children}
    </a>
  ),
};

export const AIChatMarkdown = memo(function AIChatMarkdown({ content }: Props) {
  return (
    <div className="chat-markdown text-[13px] leading-relaxed">
      <ReactMarkdown
        remarkPlugins={REMARK_PLUGINS}
        rehypePlugins={REHYPE_PLUGINS}
        components={CHAT_MARKDOWN_COMPONENTS}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
});
