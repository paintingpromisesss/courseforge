import clsx from 'clsx';

interface Props {
  children: React.ReactNode;
  variant?: 'brand' | 'ok' | 'warn' | 'err' | 'neutral';
  className?: string;
}

export function Badge({ children, variant = 'neutral', className }: Props) {
  return (
    <span className={clsx(
      'inline-flex items-center px-2 py-0.5 rounded text-xs font-medium',
      variant === 'brand'   && 'bg-brand-subtle text-brand',
      variant === 'ok'      && 'bg-ok/15 text-ok',
      variant === 'warn'    && 'bg-warn/15 text-warn',
      variant === 'err'     && 'bg-err/15 text-err',
      variant === 'neutral' && 'bg-bg-4 text-tx-2',
      className,
    )}>
      {children}
    </span>
  );
}
