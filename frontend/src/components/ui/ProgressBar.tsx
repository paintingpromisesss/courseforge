import type { CourseItem } from '../../api/types';

interface Props { value: number; max: number; className?: string; }

// "· N/M ✓" suffix for a course card's meta line: tasks + theory combined.
export function CardDoneMark({ course }: { course: CourseItem }) {
  const done = course.done_count + course.theory_done_count;
  const max = course.task_count + course.theory_count;
  if (done === 0 || max === 0) return null;
  return <span className={done >= max ? 'text-ok' : 'text-brand'}> · {done}/{max} ✓</span>;
}

// Thin progress strip flush with the bottom edge of a card. Parent must be
// `relative overflow-hidden` (the card's own rounding clips the strip's
// corners — a border-radius on a 4px-tall strip gets clamped by CSS and
// wouldn't match the card's curve). Hidden until there's any progress to
// show; turns green when the course is fully done.
export function CardProgressStrip({ value, max }: { value: number; max: number }) {
  if (max === 0 || value === 0) return null;
  const pct = Math.round((value / max) * 100);
  return (
    <div className="absolute inset-x-0 bottom-0 h-1">
      <div
        className={`h-full transition-all duration-300 ${value >= max ? 'bg-ok' : 'bg-brand'}`}
        // px floor so the very first completed item is visible past the
        // card's clipped 16px corner
        style={{ width: `max(${pct}%, 28px)` }}
      />
    </div>
  );
}

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
