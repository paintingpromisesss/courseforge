import clsx from 'clsx';

interface DifficultyBadgeProps {
  difficulty?: number;
  size?: 'sm' | 'md';
  active?: boolean;
  className?: string;
}

export function getDifficultyConfig(diff: number) {
  switch (diff) {
    case 1:
      return {
        label: 'Легкая',
        shortLabel: 'Легк.',
        colorClass: 'text-ok bg-ok/10 border-ok/25',
        dotClass: 'bg-ok',
      };
    case 2:
      return {
        label: 'Средняя',
        shortLabel: 'Средн.',
        colorClass: 'text-warn bg-warn/10 border-warn/25',
        dotClass: 'bg-warn',
      };
    case 3:
      return {
        label: 'Сложная',
        shortLabel: 'Сложн.',
        colorClass: 'text-err bg-err/10 border-err/25',
        dotClass: 'bg-err',
      };
    default:
      return {
        label: `Сложность ${diff}`,
        shortLabel: `${diff}`,
        colorClass: 'text-tx-2 bg-bg-4 border-bdr',
        dotClass: 'bg-tx-3',
      };
  }
}

export function DifficultyBadge({
  difficulty,
  size = 'md',
  active = false,
  className,
}: DifficultyBadgeProps) {
  if (!difficulty) return null;
  const cfg = getDifficultyConfig(difficulty);

  if (size === 'sm') {
    return (
      <span
        title={`Сложность: ${cfg.label}`}
        className={clsx(
          'text-[10px] leading-tight px-1.5 py-0.5 rounded font-medium shrink-0 transition-colors border select-none',
          active
            ? 'bg-white/20 text-white border-white/30'
            : cfg.colorClass,
          className,
        )}
      >
        {cfg.shortLabel}
      </span>
    );
  }

  return (
    <span
      className={clsx(
        'inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium border select-none',
        cfg.colorClass,
        className,
      )}
    >
      <span className={clsx('w-1.5 h-1.5 rounded-full shrink-0', cfg.dotClass)} />
      <span>{cfg.label}</span>
    </span>
  );
}
