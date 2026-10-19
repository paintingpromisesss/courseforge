import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import { useDock, type CenterTabsConfig } from './DockContext';
import { DockContextMenu } from './DockContextMenu';
import type { DockPanel } from './dock';

// --- icons ---------------------------------------------------------------

function TreeIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round">
      <line x1="1" y1="2" x2="11" y2="2" />
      <line x1="1" y1="6" x2="11" y2="6" />
      <line x1="1" y1="10" x2="11" y2="10" />
    </svg>
  );
}

function GitIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <line x1="6" y1="3" x2="6" y2="15" />
      <circle cx="18" cy="6" r="3" />
      <circle cx="6" cy="18" r="3" />
      <path d="M18 9a9 9 0 0 1-9 9" />
    </svg>
  );
}

function AIIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 2a4 4 0 0 1 4 4v2a4 4 0 0 1-8 0V6a4 4 0 0 1 4-4z" />
      <path d="M16 14H8a4 4 0 0 0-4 4v2h16v-2a4 4 0 0 0-4-4z" />
    </svg>
  );
}

// --- action bar buttons ---------------------------------------------------

/** Идея спеки: иконка отражает сторону закрепления маленькой точкой
 *  слева/справа. Плавающий AI точки не имеет. */
function PanelButton({ active, dirty, dotSide, title, onClick, onContextMenu, children }: {
  active: boolean;
  dirty?: boolean;
  dotSide: 'left' | 'right' | null;
  title: string;
  onClick: () => void;
  onContextMenu: (e: React.MouseEvent) => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      title={title}
      onClick={onClick}
      onContextMenu={onContextMenu}
      className={clsx(
        'relative flex items-center gap-1 px-2 h-7 rounded-md transition-colors active:scale-95 cursor-pointer',
        active
          ? 'bg-brand/15 text-brand'
          : dirty
            ? 'text-warn hover:text-tx-1 hover:bg-bg-4'
            : 'text-tx-3 hover:text-tx-1 hover:bg-bg-4',
      )}
    >
      {dotSide === 'left' && <SideDot />}
      <span className="relative shrink-0">
        {children}
        {dirty && (
          <span
            className={clsx(
              'absolute w-1.5 h-1.5 rounded-full bg-warn',
              dotSide === 'left' ? '-top-0.5 -right-1' : '-top-0.5 -left-1',
            )}
          />
        )}
      </span>
      {dotSide === 'right' && <SideDot />}
    </button>
  );
}

function SideDot() {
  return <span className="w-1 h-1 rounded-full bg-current opacity-50 shrink-0" />;
}

// --- center: theory/code segmented switch ---------------------------------

type Seg = { id: 'theory' | 'statement' | 'video' | 'submissions' | 'solution'; label: string };

function CenterSwitch({ active, segments, onSelect }: { active: string | null; segments: Seg[]; onSelect: (id: Seg['id']) => void }) {
  if (segments.length === 0) return <span />;
  return (
    <div className="flex items-center gap-0.5 rounded-md bg-bg-3/70 p-0.5" role="tablist">
      {segments.map((s) => {
        const isActive = active === s.id;
        return (
          <button
            key={s.id}
            type="button"
            role="tab"
            aria-selected={isActive}
            onClick={() => onSelect(s.id)}
            className={clsx(
              'px-2.5 h-6 rounded-[5px] text-xs font-medium transition-colors cursor-pointer whitespace-nowrap',
              isActive ? 'bg-bg-1 text-tx-1 shadow-xs' : 'text-tx-3 hover:text-tx-1',
            )}
          >
            {s.label}
          </button>
        );
      })}
    </div>
  );
}

// --- the bar ---------------------------------------------------------------

export function ActionBar({ centerTabs }: { centerTabs: CenterTabsConfig | null }) {
  const { courseSlug, trackSlug, topicSlug, unitSlug, taskSlug } = useParams<{
    courseSlug: string;
    trackSlug?: string;
    topicSlug?: string;
    unitSlug?: string;
    taskSlug?: string;
  }>();
  const {
    state,
    aiMode,
    centerTab,
    togglePanel,
    setPanelSide,
    toggleAI,
    setAIPlacement,
    setCenterTab,
  } = useDock();

  // git button exists only for git-managed courses (404 = not, like GitTreeToggle)
  const { data: gitStatus } = useQuery({
    queryKey: ['gitStatus', courseSlug],
    queryFn: () => api.gitStatus(courseSlug!),
    enabled: !!courseSlug,
    retry: false,
  });

  const [menu, setMenu] = useState<{ kind: DockPanel | 'ai'; x: number; y: number } | null>(null);

  const openMenu = (kind: DockPanel | 'ai') => (e: React.MouseEvent) => {
    e.preventDefault();
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    setMenu({ kind, x: r.left, y: r.top });
  };

  const onMenuPick = (id: string) => {
    if (!menu) return;
    if (menu.kind === 'ai') {
      if (id === 'left' || id === 'right' || id === 'float') setAIPlacement(id);
    } else if (id === 'left' || id === 'right') {
      setPanelSide(menu.kind, id);
    }
    setMenu(null);
  };

  // --- center segments -----------------------------------------------------
  const onTask = !!taskSlug;
  const onTheoryRoute = !onTask && !!(courseSlug && trackSlug && topicSlug && unitSlug);

  let activeSeg: string | null = null;
  const segments: Seg[] = [];
  if (onTask && centerTabs) {
    activeSeg = centerTab ?? 'statement';
    if (centerTabs.unitHasTheory) {
      segments.push({ id: 'theory', label: 'Теория' });
    }
    segments.push({ id: 'statement', label: 'Задача' });
    segments.push({ id: 'submissions', label: 'Посылки' });
    if (centerTabs.hasTaskVideo) segments.push({ id: 'video', label: 'Видео' });
    segments.push({ id: 'solution', label: 'Решение' });
  } else if (onTheoryRoute) {
    activeSeg = 'theory';
    segments.push({ id: 'theory', label: 'Теория' });
    if (centerTabs?.canOpenTask) segments.push({ id: 'statement', label: 'Задача' });
  }

  const selectSeg = (id: Seg['id']) => {
    if (onTask) {
      setCenterTab(id);
    } else if (onTheoryRoute && centerTabs) {
      centerTabs.onOpenTask(id);
    }
  };

  // --- AI button placement dot ----------------------------------------------
  const aiDot: 'left' | 'right' | null =
    state.position.ai === 'float' || aiMode === 'floating' ? null : state.position.ai;

  return (
    <div className="relative flex items-center gap-2 px-2 h-9 shrink-0 border-t border-bdr bg-bg-2 select-none">
      {/* left zone: content panels */}
      <div className="flex items-center gap-1 shrink-0">
        <PanelButton
          active={state.left === 'tree' || state.right === 'tree'}
          dotSide={state.position.tree}
          title="Содержание курса (Alt+1)"
          onClick={() => togglePanel('tree')}
          onContextMenu={openMenu('tree')}
        >
          <TreeIcon />
        </PanelButton>
        {gitStatus && (
          <PanelButton
            active={state.left === 'git' || state.right === 'git'}
            dirty={gitStatus.dirty}
            dotSide={state.position.git}
            title={gitStatus.dirty ? 'Git: есть локальные изменения (Alt+2)' : 'Git: ветки, история и обновления (Alt+2)'}
            onClick={() => togglePanel('git')}
            onContextMenu={openMenu('git')}
          >
            <GitIcon />
          </PanelButton>
        )}
      </div>

      {/* center zone: theory/code switch (unit/task pages only) */}
      <div className="flex-1 flex justify-center min-w-0">
        <CenterSwitch active={activeSeg} segments={segments} onSelect={selectSeg} />
      </div>

      {/* right zone: AI */}
      <div className="flex items-center gap-1 shrink-0">
        {taskSlug && (
          <PanelButton
            active={aiMode !== 'closed'}
            dotSide={aiDot}
            title="AI-ассистент (Alt+3)"
            onClick={toggleAI}
            onContextMenu={openMenu('ai')}
          >
            <AIIcon />
          </PanelButton>
        )}
      </div>

      {menu && (
        <DockContextMenu
          x={menu.x}
          y={menu.y}
          items={
            menu.kind === 'ai'
              ? [
                  // как у дерева/гита: галочка — позиция закрепления, она видна
                  // и когда панель скрыта; плавающему(window открыт) — своя
                  { id: 'left', label: 'Закрепить слева', checked: state.position.ai === 'left' && aiMode !== 'floating' },
                  { id: 'right', label: 'Закрепить справа', checked: state.position.ai === 'right' && aiMode !== 'floating' },
                  { id: '-sep', label: '' },
                  { id: 'float', label: 'Плавающее окно', checked: aiMode === 'floating' || state.position.ai === 'float' },
                ]
              : [
                  { id: 'left', label: 'Закрепить слева', checked: state.position[menu.kind as DockPanel] === 'left' },
                  { id: 'right', label: 'Закрепить справа', checked: state.position[menu.kind as DockPanel] === 'right' },
                ]
          }
          onPick={onMenuPick}
          onClose={() => setMenu(null)}
        />
      )}
    </div>
  );
}
