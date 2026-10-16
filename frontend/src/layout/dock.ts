// Pure dock-model core: panel ids, slot semantics and localStorage helpers.
// Kept free of React so the reducer stays unit-testable (see dock.test.ts).
//
// The model (see docs/superpowers/specs/2026-09-28-dock-actionbar-ui-design.md):
//   - one dock slot per side (`left` / `right`); a slot holds at most one panel;
//   - tree and git are slot panels — opening one on an occupied side replaces
//     the occupant (the old panel closes);
//   - AI is special: 'left'/'right' positions still obey slots when docked, but
//     'float' hovers above the content without taking a slot.

export type DockPanel = 'tree' | 'git';
export type DockSide = 'left' | 'right';
export type AIDockPosition = DockSide | 'float';
export type AIMode = 'docked' | 'floating' | 'closed';
export type CenterTab = 'theory' | 'statement' | 'video' | 'submissions' | 'solution';

export interface DockState {
  // where every panel prefers to live (persistent)
  position: { tree: DockSide; git: DockSide; ai: AIDockPosition };
  // what occupies each slot right now (session-only, never persisted)
  left: DockPanel | null;
  right: DockPanel | null;
}

export const emptyDockState: DockState = {
  position: { tree: 'left', git: 'left', ai: 'right' },
  left: null,
  right: null,
};

// --- slot semantics (the behavior table from the spec) ---

/** ЛКМ по кнопке: открытая панель закрывается, иначе открывается на своей
 *  стороне, замещая текущего occupant слота. */
export function togglePanel(state: DockState, panel: DockPanel): DockState {
  const side = state.position[panel];
  if (state[side] === panel) {
    return { ...state, [side]: null };
  }
  return { ...state, [side]: panel };
}

/** ПКМ → Dock Left / Dock Right: смена закрепления; открытая панель
 *  переехать в новый слот (замещая occupant, если тот занят). */
export function setPanelSide(state: DockState, panel: DockPanel, side: DockSide): DockState {
  if (state.position[panel] === side) return state;
  const next: DockState = {
    ...state,
    position: { ...state.position, [panel]: side },
  };
  const other: DockSide = side === 'left' ? 'right' : 'left';
  if (next[other] === panel) {
    // the panel was open on the other side — move it, replacing the occupant
    next[other] = null;
    next[side] = panel;
  }
  return next;
}

/** AI docks to a side and thus takes that slot away from tree/git. */
export function aiTakesSide(state: DockState, side: DockSide): DockState {
  return { ...state, [side]: null };
}

// --- localStorage persistence (positions only; slots are session-only) ---

function positionKey(panel: DockPanel | 'ai'): string {
  return `cf:dock:${panel}`;
}

function readStored(key: string, allowed: string[], fallback: string): string {
  try {
    const v = localStorage.getItem(key);
    if (v && allowed.includes(v)) return v;
  } catch {
    /* private mode etc. — fall through */
  }
  return fallback;
}

export function loadPanelSide(panel: DockPanel): DockSide {
  return readStored(positionKey(panel), ['left', 'right'], 'left') as DockSide;
}

export function loadAIPosition(): AIDockPosition {
  return readStored(positionKey('ai'), ['left', 'right', 'float'], 'right') as AIDockPosition;
}

export function loadAIMode(): AIMode {
  return readStored('cf_ai_mode', ['docked', 'floating', 'closed'], 'closed') as AIMode;
}

export function savePanelSide(panel: DockPanel, side: DockSide): void {
  try {
    localStorage.setItem(positionKey(panel), side);
  } catch {
    /* ignore */
  }
}

export function saveAIPosition(position: AIDockPosition): void {
  try {
    localStorage.setItem(positionKey('ai'), position);
  } catch {
    /* ignore */
  }
}

/** Initial state assembled from persisted positions with empty slots. */
export function initialDockState(): DockState {
  return {
    position: {
      tree: loadPanelSide('tree'),
      git: loadPanelSide('git'),
      ai: loadAIPosition(),
    },
    left: null,
    right: null,
  };
}
