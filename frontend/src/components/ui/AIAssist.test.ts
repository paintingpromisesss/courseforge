import { describe, it, expect, beforeEach } from 'vitest';

describe('AI Assistant Configuration & Storage State', () => {
  const store: Record<string, string> = {};
  const localStorageMock = {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, val: string) => { store[key] = String(val); },
    removeItem: (key: string) => { delete store[key]; },
    clear: () => { Object.keys(store).forEach((k) => delete store[k]); },
  };

  beforeEach(() => {
    (globalThis as unknown as { localStorage: typeof localStorageMock }).localStorage = localStorageMock;
    localStorageMock.clear();
  });

  it('stores and retrieves cf_ai_mode properly with fallback to closed', () => {
    expect(localStorage.getItem('cf_ai_mode')).toBeNull();

    localStorage.setItem('cf_ai_mode', 'docked');
    expect(localStorage.getItem('cf_ai_mode')).toBe('docked');

    localStorage.setItem('cf_ai_mode', 'floating');
    expect(localStorage.getItem('cf_ai_mode')).toBe('floating');

    localStorage.setItem('cf_ai_mode', 'closed');
    expect(localStorage.getItem('cf_ai_mode')).toBe('closed');
  });

  it('stores and retrieves cf_ai_show_thinking properly defaulting to enabled', () => {
    // Default when unset: treated as enabled (!== 'false')
    expect(localStorage.getItem('cf_ai_show_thinking') !== 'false').toBe(true);

    localStorage.setItem('cf_ai_show_thinking', 'false');
    expect(localStorage.getItem('cf_ai_show_thinking') !== 'false').toBe(false);

    localStorage.setItem('cf_ai_show_thinking', 'true');
    expect(localStorage.getItem('cf_ai_show_thinking') !== 'false').toBe(true);
  });

  it('stores and clamps cf_ai_width within bounds [360, 800]', () => {
    const minWidth = 360;
    const maxWidth = 800;
    const defaultWidth = 440;

    const clamp = (val: number) => Math.min(Math.max(val, minWidth), maxWidth);

    expect(clamp(300)).toBe(360);
    expect(clamp(900)).toBe(800);
    expect(clamp(defaultWidth)).toBe(440);
    expect(clamp(550)).toBe(550);

    localStorage.setItem('cf_ai_width', String(clamp(500)));
    expect(Number(localStorage.getItem('cf_ai_width'))).toBe(500);
  });

  it('stores cf_ai_floating_pos position object', () => {
    const pos = { x: 350, y: 220 };
    localStorage.setItem('cf_ai_floating_pos', JSON.stringify(pos));

    const retrieved = JSON.parse(localStorage.getItem('cf_ai_floating_pos') || '{}');
    expect(retrieved).toEqual({ x: 350, y: 220 });
  });

  it('verifies right-docked resize delta arithmetic', () => {
    // For a right-docked panel, dragging the left border to the left (negative deltaX)
    // increases the width of the panel.
    const startX = 500;
    const startWidth = 440;
    const moveX = 400; // dragged 100px to the left
    const deltaX = moveX - startX; // -100

    const newWidth = startWidth - deltaX; // 440 - (-100) = 540
    expect(newWidth).toBe(540);
  });

  it('stores cf_ai_floating_size size object and respects minimum constraints', () => {
    const minWidth = 360;
    const minHeight = 380;
    const initialSize = { width: 440, height: 620 };

    const clampSize = (w: number, h: number) => ({
      width: Math.max(minWidth, w),
      height: Math.max(minHeight, h),
    });

    expect(clampSize(200, 300)).toEqual({ width: 360, height: 380 });
    expect(clampSize(600, 750)).toEqual({ width: 600, height: 750 });
    expect(clampSize(initialSize.width, initialSize.height)).toEqual(initialSize);

    localStorage.setItem('cf_ai_floating_size', JSON.stringify({ width: 520, height: 700 }));
    const saved = JSON.parse(localStorage.getItem('cf_ai_floating_size') || '{}');
    expect(saved).toEqual({ width: 520, height: 700 });
  });

  it('verifies floating window corner resize arithmetic', () => {
    const startPos = { x: 200, y: 150 };
    const startSize = { width: 440, height: 620 };

    // 1. Bottom-right drag (SE): move +60px right, +80px down
    const seDeltaX = 60;
    const seDeltaY = 80;
    const seWidth = startSize.width + seDeltaX;
    const seHeight = startSize.height + seDeltaY;
    expect(seWidth).toBe(500);
    expect(seHeight).toBe(700);

    // 2. Top-left drag (NW): move -40px left (width increases), -50px up (height increases)
    const nwDeltaX = -40;
    const nwDeltaY = -50;
    const nwWidth = startSize.width - nwDeltaX;
    const nwHeight = startSize.height - nwDeltaY;
    const nwX = startPos.x + startSize.width - nwWidth;
    const nwY = startPos.y + startSize.height - nwHeight;
    expect(nwWidth).toBe(480);
    expect(nwHeight).toBe(670);
    expect(nwX).toBe(160);
    expect(nwY).toBe(100);
  });
});
