import { useEffect, useRef } from 'react';
import clsx from 'clsx';

export interface DockMenuItem {
  id: string;
  label: string;
  checked?: boolean;
}

/**
 * ПКМ-меню кнопки action bar. Открывается над кнопкой (bottom anchor),
 * закрывается по клику вне, Esc или выбору пункта.
 */
export function DockContextMenu({ x, y, items, onPick, onClose }: {
  x: number;
  y: number; // distance from viewport top to the button's top edge
  items: DockMenuItem[];
  onPick: (id: string) => void;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        onClose();
      }
    };
    window.addEventListener('keydown', onKey, true);
    return () => window.removeEventListener('keydown', onKey, true);
  }, [onClose]);

  // Menu grows upward from the button; keep it inside the viewport.
  const left = Math.max(8, Math.min(x, window.innerWidth - 200));

  return (
    <div className="fixed inset-0 z-50" onMouseDown={onClose} onContextMenu={(e) => { e.preventDefault(); onClose(); }}>
      <div
        ref={ref}
        style={{ left, top: y, transform: 'translateY(-100%)' }}
        className="absolute py-1 min-w-[176px] rounded-lg border border-bdr bg-bg-3 shadow-xl select-none"
        onMouseDown={(e) => e.stopPropagation()}
      >
        {items.map((item, i) => (
          <div key={item.id}>
            {/* separator row */}
            {i > 0 && item.id.startsWith('-') && (
              <div className="my-1 border-t border-bdr-s" />
            )}
            {!item.id.startsWith('-') && (
              <button
                type="button"
                onClick={() => onPick(item.id)}
                className={clsx(
                  'w-full flex items-center gap-2 px-3 py-1.5 text-left text-xs transition-colors cursor-pointer',
                  item.checked ? 'text-tx-1' : 'text-tx-2 hover:bg-bg-4 hover:text-tx-1',
                )}
              >
                <span className={clsx('w-3 text-center shrink-0 text-[11px]', item.checked ? 'text-brand' : 'text-transparent')}>
                  ✓
                </span>
                <span className="truncate">{item.label}</span>
              </button>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
