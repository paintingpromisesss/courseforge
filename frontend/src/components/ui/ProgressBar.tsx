interface Props { value: number; max: number; className?: string; }

export function ProgressBar({ value, max, className }: Props) {
  const pct = max === 0 ? 0 : Math.round((value / max) * 100);
  return (
    <div className={`h-1 rounded-full bg-bg-4 overflow-hidden ${className ?? ''}`}>
      <div
        className="h-full rounded-full bg-brand transition-all duration-300"
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}
