import { describe, it, expect, beforeEach } from 'vitest';
import {
  emptyDockState,
  togglePanel,
  setPanelSide,
  aiTakesSide,
  initialDockState,
  savePanelSide,
} from './dock';
import type { DockState } from './dock';

// Behavior table from the spec:
//   - ЛКМ по кнопке панели, чья сторона свободна → панель открывается на своей стороне;
//   - ЛКМ по кнопке другой панели на той же стороне → текущий док закрывается, открывается новая панель;
//   - ЛКМ по кнопке открытой панели → док закрывается;
//   - ПКМ → Dock Left / Right, открытая панель переезжает.
describe('dock slots', () => {
  it('opens a panel on its own empty side', () => {
    const s = togglePanel(emptyDockState, 'tree');
    expect(s.left).toBe('tree');
    expect(s.right).toBeNull();
  });

  it('closing again clears the slot', () => {
    let s = togglePanel(emptyDockState, 'tree');
    s = togglePanel(s, 'tree');
    expect(s.left).toBeNull();
  });

  it('replaces the occupant when both panels share a side', () => {
    // tree and git are both docked left; tree is open
    const base: DockState = {
      ...emptyDockState,
      position: { ...emptyDockState.position, git: 'left' },
      left: 'tree',
    };
    const s = togglePanel(base, 'git');
    expect(s.left).toBe('git'); // tree closed, git opened — same slot
    expect(s.right).toBeNull();
  });

  it('reopening the replaced panel replaces back', () => {
    const base: DockState = {
      ...emptyDockState,
      left: 'git', // git occupies left
    };
    const s = togglePanel(base, 'tree');
    expect(s.left).toBe('tree');
  });

  it('a right-docked panel never touches the left slot', () => {
    const base: DockState = {
      ...emptyDockState,
      position: { tree: 'left', git: 'right', ai: 'right' },
      left: 'tree',
    };
    let s = togglePanel(base, 'git');
    expect(s.right).toBe('git');
    expect(s.left).toBe('tree'); // coexists — different sides
    s = togglePanel(s, 'git');
    expect(s.right).toBeNull();
  });
});

describe('setPanelSide', () => {
  it('changes the docked side of a closed panel', () => {
    const s = setPanelSide(emptyDockState, 'git', 'right');
    expect(s.position.git).toBe('right');
    expect(s.left).toBeNull();
    expect(s.right).toBeNull();
  });

  it('moves an open panel across sides, replacing the occupant', () => {
    const base: DockState = {
      ...emptyDockState,
      position: { tree: 'left', git: 'right', ai: 'right' },
      left: 'tree',
      right: 'git',
    };
    const s = setPanelSide(base, 'tree', 'right');
    expect(s.position.tree).toBe('right');
    expect(s.left).toBeNull(); // tree left the left slot
    expect(s.right).toBe('tree'); // git was replaced
  });

  it('is a no-op when the side does not change', () => {
    const base: DockState = { ...emptyDockState, left: 'tree' };
    expect(setPanelSide(base, 'tree', 'left')).toBe(base);
  });

  it('keeps the panel closed when it was closed', () => {
    const base: DockState = {
      ...emptyDockState,
      position: { tree: 'left', git: 'left', ai: 'right' },
      left: 'git',
    };
    const s = setPanelSide(base, 'tree', 'right');
    expect(s.left).toBe('git'); // untouched
    expect(s.right).toBeNull(); // tree stays closed
  });
});

describe('aiTakesSide', () => {
  it('frees the side the AI docks to', () => {
    const base: DockState = { ...emptyDockState, left: 'tree' };
    expect(aiTakesSide(base, 'left')).toEqual({ ...base, left: null });
  });
});

describe('persistence', () => {
  beforeEach(() => {
    const store: Record<string, string> = {};
    (globalThis as unknown as { localStorage: Storage }).localStorage = {
      getItem: (k) => store[k] ?? null,
      setItem: (k, v) => { store[k] = String(v); },
      removeItem: (k) => { delete store[k]; },
      clear: () => { Object.keys(store).forEach((k) => delete store[k]); },
      key: () => null,
      length: 0,
    };
  });

  it('initialDockState reads persisted sides and opens the tree by default', () => {
    savePanelSide('git', 'right');
    const s = initialDockState();
    expect(s.position.tree).toBe('left');
    expect(s.position.git).toBe('right');
    // при входе в курс содержание открыто по умолчанию
    expect(s.left).toBe('tree');
    expect(s.right).toBeNull();
  });

  it('initialDockState ignores corrupted values', () => {
    localStorage.setItem('cf:dock:tree', 'banana');
    const s = initialDockState();
    expect(s.position.tree).toBe('left');
  });
});
