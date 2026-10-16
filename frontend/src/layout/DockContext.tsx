import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import {
  aiTakesSide,
  initialDockState,
  loadAIMode,
  saveAIPosition,
  savePanelSide,
  setPanelSide as setPanelSidePure,
  togglePanel as togglePanelPure,
  type AIMode,
  type AIDockPosition,
  type CenterTab,
  type DockPanel,
  type DockSide,
  type DockState,
} from './dock';

// Event AIAssist listens to — cf_ai_mode changes made anywhere (action bar,
// AI panel header) funnel through it so every surface stays in sync.
export const AI_MODE_EVENT = 'cf_ai_mode_changed';

export interface CenterTabsConfig {
  unitHasTheory: boolean;
  hasTaskVideo: boolean;
  canOpenTask: boolean;
  onOpenTask: (view: CenterTab) => void;
}

interface DockContextValue {
  state: DockState;
  aiMode: AIMode;
  centerTab: CenterTab | null;
  diffFile: string | null;
  togglePanel: (panel: DockPanel) => void;
  setPanelSide: (panel: DockPanel, side: DockSide) => void;
  toggleAI: () => void;
  setAIPlacement: (placement: AIDockPosition) => void;
  setCenterTab: (tab: CenterTab | null) => void;
  openDiffFile: (path: string) => void;
  closeDiff: () => void;
}

const DockCtx = createContext<DockContextValue | null>(null);

export function useDock(): DockContextValue {
  const v = useContext(DockCtx);
  if (!v) throw new Error('useDock must be used inside <DockProvider>');
  return v;
}

export function DockProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<DockState>(initialDockState);
  const [aiMode, setAiModeState] = useState<AIMode>(loadAIMode);
  const [centerTab, setCenterTabState] = useState<CenterTab | null>(null);
  const [diffFile, setDiffFile] = useState<string | null>(null);

  // refs mirror state inside event callbacks (kept in sync via effects —
  // react-compiler forbids ref writes during render)
  const stateRef = useRef(state);
  const aiModeRef = useRef(aiMode);
  useEffect(() => {
    stateRef.current = state;
  }, [state]);
  useEffect(() => {
    aiModeRef.current = aiMode;
  }, [aiMode]);

  /** Single write path for cf_ai_mode: state + storage + broadcast. */
  const applyAIMode = useCallback((mode: AIMode) => {
    setAiModeState(mode);
    try {
      localStorage.setItem('cf_ai_mode', mode);
      if (mode !== 'closed') localStorage.setItem('cf_ai_last_mode', mode);
    } catch {
      /* ignore */
    }
    window.dispatchEvent(new CustomEvent<AIMode>(AI_MODE_EVENT, { detail: mode }));
  }, []);

  // AIAssist may change its own mode (header buttons, float toggle) — it
  // broadcasts the same event, so the action bar keeps reflecting it.
  useEffect(() => {
    const onModeChanged = (e: Event) => {
      const mode = (e as CustomEvent<AIMode>).detail;
      if (mode === 'docked' || mode === 'floating' || mode === 'closed') {
        setAiModeState(mode);
      }
    };
    window.addEventListener(AI_MODE_EVENT, onModeChanged);
    return () => window.removeEventListener(AI_MODE_EVENT, onModeChanged);
  }, []);

  /** ЛКМ по кнопке панели: открытие/закрытие/замещение. Если панель
   *  открывается на стороне, где закреплён docked AI — AI закрывается. */
  const togglePanel = useCallback(
    (panel: DockPanel) => {
      const prev = stateRef.current;
      const side = prev.position[panel];
      const opening = prev[side] !== panel;
      setState((t) => togglePanelPure(t, panel));
      if (opening && aiModeRef.current === 'docked' && prev.position.ai === side) {
        applyAIMode('closed');
      }
    },
    [applyAIMode],
  );

  /** ПКМ → Слева/Справа: смена закрепления, открытая панель переезжает.
   *  Переезд на сторону docked AI закрывает его слот. */
  const setPanelSide = useCallback(
    (panel: DockPanel, side: DockSide) => {
      const prev = stateRef.current;
      const oldSide = prev.position[panel];
      if (oldSide === side) return;
      savePanelSide(panel, side);
      setState((t) => setPanelSidePure(t, panel, side));
      const wasOpenOnOldSide = prev[oldSide] === panel;
      if (wasOpenOnOldSide && aiModeRef.current === 'docked' && prev.position.ai === side) {
        applyAIMode('closed');
      }
    },
    [applyAIMode],
  );

  /** ЛКМ по кнопке AI: toggle closed ↔ последний режим (cf_ai_last_mode). */
  const toggleAI = useCallback(() => {
    if (aiModeRef.current !== 'closed') {
      applyAIMode('closed');
      return;
    }
    let last: AIMode = 'docked';
    try {
      const saved = localStorage.getItem('cf_ai_last_mode');
      if (saved === 'floating') last = 'floating';
    } catch {
      /* ignore */
    }
    if (last === 'floating') {
      applyAIMode('floating');
      return;
    }
    const pos = stateRef.current.position.ai;
    const side: DockSide = pos === 'float' ? 'right' : pos;
    setState((t) => aiTakesSide(t, side));
    applyAIMode('docked');
  }, [applyAIMode]);

  // stable ref so the hotkey listener below does not rebind on aiMode changes
  const toggleAIRef = useRef(toggleAI);
  useEffect(() => {
    toggleAIRef.current = toggleAI;
  }, [toggleAI]);

  /** ПКМ по кнопке AI: слева / справа / плавающее окно. */
  const setAIPlacement = useCallback(
    (placement: AIDockPosition) => {
      saveAIPosition(placement);
      // позиция меняется и в state (в отличие от localStorage-only write),
      // иначе раскладка AI не сдвинется до перезагрузки
      setState((t) => {
        const next: DockState = { ...t, position: { ...t.position, ai: placement } };
        return placement === 'float' ? next : aiTakesSide(next, placement);
      });
      applyAIMode(placement === 'float' ? 'floating' : 'docked');
    },
    [applyAIMode],
  );

  const setCenterTab = useCallback((tab: CenterTab | null) => {
    setCenterTabState(tab);
  }, []);

  const openDiffFile = useCallback((path: string) => setDiffFile(path), []);
  const closeDiff = useCallback(() => setDiffFile(null), []);

  // Alt+1/2/3 — toggle дерева / git / AI, as in VS Code.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return;
      if (e.code === 'Digit1') {
        e.preventDefault();
        togglePanel('tree');
      } else if (e.code === 'Digit2') {
        e.preventDefault();
        togglePanel('git');
      } else if (e.code === 'Digit3') {
        e.preventDefault();
        toggleAIRef.current();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [togglePanel]);

  const value = useMemo<DockContextValue>(
    () => ({
      state,
      aiMode,
      centerTab,
      diffFile,
      togglePanel,
      setPanelSide,
      toggleAI,
      setAIPlacement,
      setCenterTab,
      openDiffFile,
      closeDiff,
    }),
    [state, aiMode, centerTab, diffFile, togglePanel, setPanelSide, toggleAI, setAIPlacement, setCenterTab, openDiffFile, closeDiff],
  );

  return <DockCtx.Provider value={value}>{children}</DockCtx.Provider>;
}
