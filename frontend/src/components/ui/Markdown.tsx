import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useTheme } from '../../context/ThemeContext';

interface Props {
  content: string;
  assetBase?: string;
}

export function Markdown({ content, assetBase }: Props) {
  const { theme } = useTheme();

  return (
    <div className="markdown-body max-w-none">
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
            return <img src={resolved} alt={alt ?? ''} style={style} />;
          },
        } : undefined}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
