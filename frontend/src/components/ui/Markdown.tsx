import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import clsx from 'clsx';
import { useTheme } from '../../context/ThemeContext';

interface Props {
  content: string;
  assetBase?: string;
}

export function Markdown({ content, assetBase }: Props) {
  const { theme } = useTheme();

  return (
    <div className={clsx(
      'prose prose-sm max-w-none',
      theme === 'dark' && 'prose-invert',
      'prose-headings:text-tx-1 prose-p:text-tx-2 prose-li:text-tx-2',
      'prose-code:bg-bg-4 prose-code:px-1 prose-code:rounded prose-code:text-tx-1',
      'prose-pre:bg-bg-4 prose-pre:border prose-pre:border-bdr',
    )}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={assetBase ? {
          img: ({ src, alt }) => {
            const resolved = src?.startsWith('assets/')
              ? `${assetBase}/assets/${src.slice('assets/'.length)}`
              : src;
            const style = theme === 'dark'
              ? { filter: 'invert(1) hue-rotate(180deg)' }
              : undefined;
            return <img src={resolved} alt={alt ?? ''} className="max-w-full rounded" style={style} />;
          },
        } : undefined}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
