import { useState, useCallback, useRef, useEffect } from 'react';

interface UseResizableOptions {
  initialWidth: number;
  minWidth: number;
  maxWidth: number | (() => number);
  direction?: 'left' | 'right'; // 'left' means dragging left increases width (right-docked panel)
  storageKey?: string;
  debounceMs?: number;
}

export function useResizable({
  initialWidth,
  minWidth,
  maxWidth,
  direction = 'left',
  storageKey,
  debounceMs = 200,
}: UseResizableOptions) {
  const getMaxWidth = useCallback(() => {
    return typeof maxWidth === 'function' ? maxWidth() : maxWidth;
  }, [maxWidth]);

  const [width, setWidth] = useState<number>(() => {
    const currentMax = typeof maxWidth === 'function' ? maxWidth() : maxWidth;
    if (storageKey && typeof window !== 'undefined') {
      try {
        const saved = localStorage.getItem(storageKey);
        if (saved) {
          const parsed = Number(saved);
          if (!isNaN(parsed) && parsed >= minWidth && parsed <= currentMax) {
            return parsed;
          }
        }
      } catch {
        /* ignore */
      }
    }
    return Math.min(initialWidth, currentMax);
  });

  const [isDragging, setIsDragging] = useState(false);
  const widthRef = useRef(width);

  useEffect(() => {
    widthRef.current = width;
  }, [width]);

  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const saveWidth = useCallback(
    (w: number, immediate = false) => {
      if (!storageKey || typeof window === 'undefined') return;
      if (saveTimerRef.current) {
        clearTimeout(saveTimerRef.current);
        saveTimerRef.current = null;
      }
      if (immediate) {
        try {
          localStorage.setItem(storageKey, String(w));
        } catch {
          /* ignore */
        }
      } else {
        saveTimerRef.current = setTimeout(() => {
          try {
            localStorage.setItem(storageKey, String(w));
          } catch {
            /* ignore */
          }
        }, debounceMs);
      }
    },
    [storageKey, debounceMs],
  );

  useEffect(() => {
    return () => {
      if (saveTimerRef.current) {
        clearTimeout(saveTimerRef.current);
      }
    };
  }, []);

  // Auto-clamp if max width shrinks below current width
  useEffect(() => {
    const currentMax = getMaxWidth();
    if (width > currentMax && currentMax >= minWidth) {
      setWidth(currentMax);
      saveWidth(currentMax, true);
    }
  }, [getMaxWidth, width, minWidth, saveWidth]);

  const handlePointerDown = useCallback(
    (e: React.PointerEvent) => {
      e.preventDefault();
      setIsDragging(true);
      const startX = e.clientX;
      const startWidth = widthRef.current;

      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none';

      const onPointerMove = (moveEvent: PointerEvent) => {
        const delta = moveEvent.clientX - startX;
        // If dragging from left edge of right-docked panel, moving left (delta < 0) increases width
        const rawNewWidth = direction === 'left' ? startWidth - delta : startWidth + delta;
        const currentMax = getMaxWidth();
        const clamped = Math.min(Math.max(rawNewWidth, minWidth), currentMax);
        setWidth(clamped);
        saveWidth(clamped, false);
      };

      const onPointerUp = () => {
        setIsDragging(false);
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
        saveWidth(widthRef.current, true);
        window.removeEventListener('pointermove', onPointerMove);
        window.removeEventListener('pointerup', onPointerUp);
        window.removeEventListener('pointercancel', onPointerUp);
      };

      window.addEventListener('pointermove', onPointerMove);
      window.addEventListener('pointerup', onPointerUp);
      window.addEventListener('pointercancel', onPointerUp);
    },
    [direction, minWidth, getMaxWidth, saveWidth],
  );

  const resetWidth = useCallback(() => {
    const currentMax = getMaxWidth();
    const target = Math.min(Math.max(initialWidth, minWidth), currentMax);
    setWidth(target);
    saveWidth(target, true);
  }, [initialWidth, minWidth, getMaxWidth, saveWidth]);

  return {
    width,
    setWidth,
    isDragging,
    resetWidth,
    splitterProps: {
      onPointerDown: handlePointerDown,
      onDoubleClick: resetWidth,
    },
  };
}
