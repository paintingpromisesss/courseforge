import clsx from 'clsx';

interface TagBadgeProps {
  tag: string;
  size?: 'sm' | 'md';
  onClick?: () => void;
  selected?: boolean;
  removable?: boolean;
  onRemove?: () => void;
  className?: string;
}

export function TagBadge({
  tag,
  size = 'md',
  onClick,
  selected = false,
  removable = false,
  onRemove,
  className,
}: TagBadgeProps) {
  const isClickable = !!onClick;

  return (
    <span
      onClick={onClick}
      className={clsx(
        'inline-flex items-center gap-1 font-medium transition-colors select-none',
        size === 'sm'
          ? 'text-[11px] px-2 py-0.5 rounded-md'
          : 'text-xs px-2.5 py-1 rounded-md',
        selected
          ? 'bg-brand/15 text-brand border border-brand/35 shadow-xs'
          : 'bg-bg-3 text-tx-2 hover:bg-bg-4 hover:text-tx-1 border border-bdr-s',
        isClickable && 'cursor-pointer',
        className,
      )}
    >
      <span className="text-tx-3 text-[10px]">#</span>
      <span>{tag}</span>
      {removable && onRemove && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
          className="ml-0.5 text-tx-3 hover:text-tx-1 rounded-full p-0.5 transition-colors"
          aria-label={`Удалить тег ${tag}`}
        >
          <svg width="10" height="10" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="2">
            <line x1="2" y1="2" x2="10" y2="10" />
            <line x1="10" y1="2" x2="2" y2="10" />
          </svg>
        </button>
      )}
    </span>
  );
}
