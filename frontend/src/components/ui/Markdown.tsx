import { createContext, useContext, useMemo, useState, type ReactNode } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import clsx from 'clsx';
import { useTheme } from '../../context/ThemeContext';

interface Props {
  content: string;
  assetBase?: string;
}

function embedSrc(href: string): string | null {
  const yt = href.match(/(?:youtube\.com\/watch\?v=|youtu\.be\/)([a-zA-Z0-9_-]{11})/);
  if (yt) return `https://www.youtube.com/embed/${yt[1]}`;
  const ia = href.match(/archive\.org\/(?:details|embed)\/([^/?#]+)/);
  if (ia) return `https://archive.org/embed/${ia[1]}`;
  return null;
}

function isVideoFile(href: string): boolean {
  return /\.(mp4|webm|ogg)(\?|#|$)/i.test(href);
}

export function VideoEmbed({ href }: { href: string }) {
  if (isVideoFile(href)) {
    return (
      <video
        src={href}
        controls
        style={{ width: '100%', aspectRatio: '16 / 9', margin: '1rem 0', background: '#000' }}
      />
    );
  }
  const src = embedSrc(href);
  if (!src) return <a href={href}>{href}</a>;
  return (
    <div style={{ position: 'relative', paddingBottom: '56.25%', height: 0, margin: '1rem 0' }}>
      <iframe
        src={src}
        style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', border: 0 }}
        allowFullScreen
      />
    </div>
  );
}

// ---- multi-language code snippets -------------------------------------------
// A `<!-- code-snippets -->` marker followed by consecutive fenced code blocks
// is rendered as one tabbed widget (one tab per language) with a copy button,
// instead of a stack of separate code blocks.

const SNIPPET_MARKER = '<!-- code-snippets -->';
// Authors may bracket the group with a closing marker; it must never leak as text.
const SNIPPET_CLOSE = /<!--\s*\/code-snippets\s*-->/g;
// Same marker, non-global: used to locate the group's end so fence consumption
// never overruns into an unrelated code block that follows the group.
const SNIPPET_CLOSE_ONE = /<!--\s*\/code-snippets\s*-->/;

// ---- accordion --------------------------------------------------------------
// `<!-- accordion "Title" -->` ... `<!-- /accordion -->` renders as a collapsible
// panel; the body keeps full markdown formatting (rendered recursively).
const ACCORDION_OPEN = /<!--\s*accordion\s+"([^"]*)"\s*-->/;
const ACCORDION_CLOSE = /<!--\s*\/accordion\s*-->/;

// Strip the common leading indent so authors can indent the body without it
// turning into a markdown code block.
function dedent(s: string): string {
  const lines = s.replace(/^\r?\n/, '').replace(/\s+$/, '').split('\n');
  const indents = lines.filter((l) => l.trim()).map((l) => l.match(/^[ \t]*/)![0].length);
  const min = indents.length ? Math.min(...indents) : 0;
  return lines.map((l) => l.slice(min)).join('\n');
}

const LANG_LABELS: Record<string, string> = {
  csharp: 'C#', cs: 'C#',
  cpp: 'C++', 'c++': 'C++', cxx: 'C++',
  go: 'Go', golang: 'Go',
  java: 'Java',
  python: 'Python', py: 'Python', python3: 'Python', python2: 'Python',
  javascript: 'JavaScript', js: 'JavaScript',
  typescript: 'TypeScript', ts: 'TypeScript',
  rust: 'Rust', rs: 'Rust',
};

function langLabel(lang: string): string {
  return LANG_LABELS[lang.toLowerCase()] || lang || 'Code';
}

// Runner keys (e.g. "python3") aren't always highlight.js language names.
// Map the version-suffixed ones to the names hljs registers, else highlighting is off.
const HLJS_ALIAS: Record<string, string> = {
  python3: 'python', python2: 'python', py3: 'python',
};
function fenceLang(lang: string): string {
  return HLJS_ALIAS[lang.toLowerCase()] || lang;
}

interface Snippet {
  lang: string;
  code: string;
}

type Part =
  | { kind: 'md'; text: string }
  | { kind: 'snippets'; items: Snippet[] }
  | { kind: 'accordion'; title: string; body: string };

// Consume one fenced block at the start of `s`, allowing leading blank lines.
const LEADING_FENCE = /^\s*```([^\n`]*)\r?\n([\s\S]*?)\r?\n[ \t]*```[ \t]*/;

function pushMd(parts: Part[], text: string) {
  const cleaned = text.replace(SNIPPET_CLOSE, '');
  if (cleaned.trim()) parts.push({ kind: 'md', text: cleaned });
}

function parseParts(content: string): Part[] {
  const parts: Part[] = [];
  let rest = content;

  while (rest) {
    const snipAt = rest.indexOf(SNIPPET_MARKER);
    const accM = ACCORDION_OPEN.exec(rest);
    const accAt = accM ? accM.index : -1;

    if (snipAt === -1 && accAt === -1) {
      pushMd(parts, rest);
      break;
    }

    // Accordion comes first: emit it and continue past its closing marker.
    if (accAt !== -1 && (snipAt === -1 || accAt < snipAt)) {
      pushMd(parts, rest.slice(0, accAt));
      const after = rest.slice(accAt + accM![0].length);
      const close = ACCORDION_CLOSE.exec(after);
      const body = close ? after.slice(0, close.index) : after;
      parts.push({ kind: 'accordion', title: accM![1], body: dedent(body) });
      rest = close ? after.slice(close.index + close[0].length) : '';
      continue;
    }

    const at = snipAt;
    pushMd(parts, rest.slice(0, at));

    const after = rest.slice(at + SNIPPET_MARKER.length);
    const close = SNIPPET_CLOSE_ONE.exec(after);
    // The group spans up to its closing marker; without one, fall back to the
    // run of fences that immediately follow the opening marker.
    let region = close ? after.slice(0, close.index) : after;

    const items: Snippet[] = [];
    let m: RegExpMatchArray | null;
    while ((m = region.match(LEADING_FENCE))) {
      items.push({ lang: m[1].trim(), code: m[2] });
      region = region.slice(m[0].length);
    }
    if (items.length > 0) parts.push({ kind: 'snippets', items });

    rest = close ? after.slice(close.index + close[0].length) : region;
  }
  return parts;
}

function CopyIcon({ copied }: { copied: boolean }) {
  if (copied) {
    return (
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
        <polyline points="20 6 9 17 4 12" />
      </svg>
    );
  }
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
    </svg>
  );
}

// Shared across every CodeTabs in one Markdown render (one theory), so picking a
// language in any widget switches all widgets that offer it.
const SnippetLangContext = createContext<{
  lang: string | null;
  setLang: (lang: string) => void;
}>({ lang: null, setLang: () => {} });

function CodeTabs({ items }: { items: Snippet[] }) {
  const shared = useContext(SnippetLangContext);
  const [localIdx, setLocalIdx] = useState(0);
  const [copied, setCopied] = useState(false);

  // Follow the shared language when this widget offers it; otherwise stay put.
  const sharedIdx = shared.lang
    ? items.findIndex((s) => langLabel(s.lang) === shared.lang)
    : -1;
  const active = sharedIdx >= 0 ? sharedIdx : Math.min(localIdx, items.length - 1);
  const cur = items[active];

  const select = (i: number) => {
    setLocalIdx(i);
    shared.setLang(langLabel(items[i].lang));
  };

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(cur.code);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard unavailable */
    }
  };

  return (
    <div className="code-tabs my-5 overflow-hidden rounded-lg border border-bdr bg-bg-2">
      <div className="flex items-center gap-1 border-b border-bdr px-2 py-2">
        {items.length > 1 ? (
          items.map((s, i) => (
            <button
              key={i}
              type="button"
              onClick={() => select(i)}
              className={clsx(
                'rounded-full border px-3 py-1 text-xs font-medium transition-colors',
                i === active
                  ? 'border-brand bg-brand text-white'
                  : 'border-bdr text-tx-2 hover:bg-bg-4 hover:text-tx-1',
              )}
            >
              {langLabel(s.lang)}
            </button>
          ))
        ) : (
          // single language: a static label, no selector
          <span className="px-2 py-1 text-xs font-medium text-tx-3">{langLabel(cur.lang)}</span>
        )}
        <button
          type="button"
          onClick={copy}
          title="Скопировать"
          className="ml-auto flex items-center rounded-md p-1.5 text-tx-3 transition-colors hover:bg-bg-4 hover:text-tx-1"
        >
          <CopyIcon copied={copied} />
        </button>
      </div>
      <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeHighlight]}>
        {`\`\`\`${fenceLang(cur.lang)}\n${cur.code}\n\`\`\``}
      </ReactMarkdown>
    </div>
  );
}

function Accordion({ title, body, assetBase }: { title: string; body: string; assetBase?: string }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="accordion my-4 overflow-hidden rounded-lg border border-bdr bg-bg-2">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        className="flex w-full items-center justify-between gap-2 px-4 py-3 text-left text-sm font-medium text-tx-1 transition-colors hover:bg-bg-4"
      >
        <span>{title}</span>
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className={clsx('shrink-0 text-tx-3 transition-transform duration-300', open && 'rotate-180')}
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>
      <div
        className="grid transition-[grid-template-rows] duration-300 ease-out"
        style={{ gridTemplateRows: open ? '1fr' : '0fr' }}
      >
        <div className="overflow-hidden">
          <div className="border-t border-bdr px-4 py-3">
            <Markdown content={body} assetBase={assetBase} />
          </div>
        </div>
      </div>
    </div>
  );
}

export function Markdown({ content, assetBase }: Props) {
  const { theme } = useTheme();
  const parts = useMemo(() => parseParts(content), [content]);
  const [snippetLang, setSnippetLang] = useState<string | null>(null);
  const snippetCtx = useMemo(
    () => ({ lang: snippetLang, setLang: setSnippetLang }),
    [snippetLang],
  );

  const components = {
    img: ({ src, alt }: { src?: string; alt?: string }) => {
      const resolved = assetBase && src?.startsWith('assets/')
        ? `${assetBase}/assets/${src.slice('assets/'.length)}`
        : src;
      const style = theme === 'dark'
        ? { filter: 'invert(1) hue-rotate(180deg)' }
        : undefined;
      return <img src={resolved} alt={alt ?? ''} style={style} />;
    },
    a: ({ href, children }: { href?: string; children?: ReactNode }) => {
      if (href && (isVideoFile(href) || embedSrc(href))) {
        return <VideoEmbed href={href} />;
      }
      return <a href={href}>{children}</a>;
    },
  };

  return (
    <SnippetLangContext.Provider value={snippetCtx}>
      <div className="markdown-body max-w-none">
        {parts.map((part, i) =>
          part.kind === 'snippets' ? (
            <CodeTabs key={i} items={part.items} />
          ) : part.kind === 'accordion' ? (
            <Accordion key={i} title={part.title} body={part.body} assetBase={assetBase} />
          ) : (
            <ReactMarkdown
              key={i}
              remarkPlugins={[remarkGfm]}
              rehypePlugins={[rehypeHighlight]}
              components={components}
            >
              {part.text}
            </ReactMarkdown>
          ),
        )}
      </div>
    </SnippetLangContext.Provider>
  );
}
