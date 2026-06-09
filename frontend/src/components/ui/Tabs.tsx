import clsx from 'clsx';

interface Tab { id: string; label: React.ReactNode; }

interface Props {
  tabs: Tab[];
  active: string;
  onChange: (id: string) => void;
  className?: string;
}

export function Tabs({ tabs, active, onChange, className }: Props) {
  return (
    <div className={clsx('flex h-11 shrink-0 border-b border-bdr', className)}>
      {tabs.map((tab) => (
        <button
          key={tab.id}
          onClick={() => onChange(tab.id)}
          className={clsx(
            'px-4 h-full text-sm font-medium border-b-2 -mb-px transition-colors',
            active === tab.id
              ? 'border-brand text-tx-1'
              : 'border-transparent text-tx-2 hover:text-tx-1',
          )}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}
