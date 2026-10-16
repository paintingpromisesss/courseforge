import clsx from 'clsx';
import { useResizable } from '../hooks/useResizable';
import { useDock, type CenterTabsConfig } from './DockContext';
import { ActionBar } from './ActionBar';
import { GitDiffSplit } from '../git/GitDiffSplit';
import type { DockPanel } from './dock';
import type { ReactNode } from 'react';

const DOCK_MIN_WIDTH = 200;
const DOCK_MAX_WIDTH = 650;

// tree keeps the historical width key so an existing layout carries over;
// git starts at a comfortable reading width.
const WIDTH_KEY: Record<DockPanel, string> = {
  tree: 'cf:sidebar-width',
  git: 'cf:dock-width:git',
};
const DEFAULT_WIDTH: Record<DockPanel, number> = {
  tree: 288,
  git: 320,
};

/** Обёртка дока: ширина + тонкая resize-ручка на внутреннем крае
 *  (двойной клик — сброс ширины). Содержимое решает сам панель. */
function DockZone({ side, panel, children }: { side: 'left' | 'right'; panel: DockPanel; children: ReactNode }) {
  const { width, isDragging, splitterProps } = useResizable({
    initialWidth: DEFAULT_WIDTH[panel],
    minWidth: DOCK_MIN_WIDTH,
    maxWidth: DOCK_MAX_WIDTH,
    // 'left' in the hook means "panel sits at the right edge, dragging its
    // left border"; a left-docked panel drags its right border instead.
    direction: side === 'left' ? 'right' : 'left',
    storageKey: WIDTH_KEY[panel],
  });

  const handle = (
    <div
      {...splitterProps}
      title="Потяните для изменения ширины (двойной клик — сброс)"
      className={clsx(
        'w-px shrink-0 bg-bdr cursor-col-resize relative z-20 transition-colors hover:bg-brand/60',
        isDragging && 'bg-brand shadow-[0_0_8px_rgba(124,58,237,0.5)]',
      )}
    >
      <div className={clsx('absolute inset-y-0 w-1.5 z-30', side === 'left' ? 'right-0' : 'left-0')} />
    </div>
  );

  return (
    <aside
      style={{ width }}
      className={clsx('shrink-0 h-full bg-bg-2 flex flex-row overflow-hidden', isDragging && 'select-none')}
    >
      {side === 'right' && handle}
      <div className="flex-1 min-w-0 h-full flex flex-col overflow-hidden">{children}</div>
      {side === 'left' && handle}
    </aside>
  );
}

/**
 * Каркас IDE-модели: [левый док | центр | правый док] + action bar снизу.
 * Доки открываются только когда их слот занят (см. DockContext), центральные
 * табы (теория/код) — основные и вечные, доки их только обрамляют.
 */
export function DockLayout({ tree, git, centerTabs, children }: {
  tree: ReactNode;
  git: ReactNode;
  centerTabs: CenterTabsConfig | null;
  children: ReactNode;
}) {
  const { state, diffFile, closeDiff } = useDock();

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <div className="flex flex-1 min-h-0 overflow-hidden">
        {state.left && (
          <DockZone side="left" panel={state.left}>
            {state.left === 'tree' ? tree : git}
          </DockZone>
        )}

        <div className="relative flex-1 min-w-0 h-full overflow-hidden">
          {children}
          {diffFile && (
            <div className="absolute inset-0 z-30 bg-bg-1">
              <GitDiffSplit path={diffFile} onClose={closeDiff} />
            </div>
          )}
        </div>

        {state.right && (
          <DockZone side="right" panel={state.right}>
            {state.right === 'tree' ? tree : git}
          </DockZone>
        )}
      </div>

      <ActionBar centerTabs={centerTabs} />
    </div>
  );
}

