import { useState, useCallback, useRef, useEffect } from 'react';

export type ResizeDirection =
  | 'top'
  | 'bottom'
  | 'left'
  | 'right'
  | 'top-left'
  | 'top-right'
  | 'bottom-left'
  | 'bottom-right';

export interface Position {
  x: number;
  y: number;
}

export interface Size {
  width: number;
  height: number;
}

export interface UseDraggableOptions {
  storageKey?: string;
  storageKeySize?: string;
  defaultOffset?: { right: number; bottom: number };
  width?: number;
  height?: number;
  minWidth?: number;
  minHeight?: number;
  debounceMs?: number;
}

const DEFAULT_WIDTH = 440;
const DEFAULT_HEIGHT = 620;
const DEFAULT_MIN_WIDTH = 360;
const DEFAULT_MIN_HEIGHT = 380;

export function useDraggable({
  storageKey,
  storageKeySize,
  defaultOffset = { right: 24, bottom: 24 },
  width: initialWidth = DEFAULT_WIDTH,
  height: initialHeight = DEFAULT_HEIGHT,
  minWidth = DEFAULT_MIN_WIDTH,
  minHeight = DEFAULT_MIN_HEIGHT,
  debounceMs = 200,
}: UseDraggableOptions) {
  // 1. SIZE STATE
  const [size, setSize] = useState<Size>(() => {
    if (storageKeySize && typeof window !== 'undefined') {
      try {
        const saved = localStorage.getItem(storageKeySize);
        if (saved) {
          const parsed = JSON.parse(saved);
          if (
            typeof parsed?.width === 'number' &&
            typeof parsed?.height === 'number' &&
            !isNaN(parsed.width) &&
            !isNaN(parsed.height) &&
            parsed.width >= minWidth &&
            parsed.height >= minHeight
          ) {
            const maxW = Math.max(minWidth, window.innerWidth - 24);
            const maxH = Math.max(minHeight, window.innerHeight - 24);
            return {
              width: Math.min(Math.round(parsed.width), maxW),
              height: Math.min(Math.round(parsed.height), maxH),
            };
          }
        }
      } catch {
        /* ignore */
      }
    }
    return { width: initialWidth, height: initialHeight };
  });

  const sizeRef = useRef(size);
  useEffect(() => {
    sizeRef.current = size;
  }, [size]);

  const saveSizeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const saveSize = useCallback(
    (sz: Size, immediate = false) => {
      if (!storageKeySize || typeof window === 'undefined') return;
      if (saveSizeTimerRef.current) {
        clearTimeout(saveSizeTimerRef.current);
        saveSizeTimerRef.current = null;
      }
      if (immediate) {
        try {
          localStorage.setItem(storageKeySize, JSON.stringify(sz));
        } catch {
          /* ignore */
        }
      } else {
        saveSizeTimerRef.current = setTimeout(() => {
          try {
            localStorage.setItem(storageKeySize, JSON.stringify(sz));
          } catch {
            /* ignore */
          }
        }, debounceMs);
      }
    },
    [storageKeySize, debounceMs],
  );

  // 2. POSITION STATE
  const getDefaultPos = useCallback(
    (currentW = sizeRef.current.width, currentH = sizeRef.current.height): Position => {
      if (typeof window === 'undefined') return { x: 100, y: 100 };
      const x = Math.max(12, window.innerWidth - currentW - defaultOffset.right);
      const y = Math.max(12, window.innerHeight - currentH - defaultOffset.bottom);
      return { x, y };
    },
    [defaultOffset.right, defaultOffset.bottom],
  );

  const [position, setPosition] = useState<Position>(() => {
    if (storageKey && typeof window !== 'undefined') {
      try {
        const saved = localStorage.getItem(storageKey);
        if (saved) {
          const parsed = JSON.parse(saved);
          if (
            typeof parsed?.x === 'number' &&
            typeof parsed?.y === 'number' &&
            !isNaN(parsed.x) &&
            !isNaN(parsed.y) &&
            (parsed.x > 50 || parsed.y > 50)
          ) {
            const maxX = Math.max(12, window.innerWidth - size.width - 12);
            const maxY = Math.max(12, window.innerHeight - size.height - 12);
            return {
              x: Math.min(Math.max(12, Math.round(parsed.x)), maxX),
              y: Math.min(Math.max(12, Math.round(parsed.y)), maxY),
            };
          }
        }
      } catch {
        /* ignore */
      }
    }
    return getDefaultPos(size.width, size.height);
  });

  const [isDragging, setIsDragging] = useState(false);
  const [isResizing, setIsResizing] = useState(false);
  const posRef = useRef(position);

  useEffect(() => {
    posRef.current = position;
  }, [position]);

  const savePosTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const savePosition = useCallback(
    (pos: Position, immediate = false) => {
      if (!storageKey || typeof window === 'undefined') return;
      if (savePosTimerRef.current) {
        clearTimeout(savePosTimerRef.current);
        savePosTimerRef.current = null;
      }
      if (immediate) {
        try {
          localStorage.setItem(storageKey, JSON.stringify(pos));
        } catch {
          /* ignore */
        }
      } else {
        savePosTimerRef.current = setTimeout(() => {
          try {
            localStorage.setItem(storageKey, JSON.stringify(pos));
          } catch {
            /* ignore */
          }
        }, debounceMs);
      }
    },
    [storageKey, debounceMs],
  );

  // Clamp on window resize
  useEffect(() => {
    const handleResize = () => {
      const maxW = Math.max(minWidth, window.innerWidth - 24);
      const maxH = Math.max(minHeight, window.innerHeight - 24);

      let curW = sizeRef.current.width;
      let curH = sizeRef.current.height;
      if (curW > maxW || curH > maxH) {
        curW = Math.min(curW, maxW);
        curH = Math.min(curH, maxH);
        const nextSize = { width: curW, height: curH };
        setSize(nextSize);
        saveSize(nextSize, true);
      }

      setPosition((curr) => {
        const maxX = Math.max(12, window.innerWidth - curW - 12);
        const maxY = Math.max(12, window.innerHeight - curH - 12);
        const clampedX = Math.min(Math.max(12, curr.x), maxX);
        const clampedY = Math.min(Math.max(12, curr.y), maxY);
        if (clampedX !== curr.x || clampedY !== curr.y) {
          const next = { x: clampedX, y: clampedY };
          savePosition(next, true);
          return next;
        }
        return curr;
      });
    };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [minWidth, minHeight, saveSize, savePosition]);

  // 3. DRAGGING HANDLER (Header bar)
  const handlePointerDown = useCallback(
    (e: React.PointerEvent) => {
      const target = e.target as HTMLElement | null;
      if (target?.closest('button, input, textarea, a, [role="button"], select')) {
        return;
      }

      e.preventDefault();
      setIsDragging(true);
      const startPointerX = e.clientX;
      const startPointerY = e.clientY;
      const startPosX = posRef.current.x;
      const startPosY = posRef.current.y;

      document.body.style.userSelect = 'none';
      document.body.style.cursor = 'grabbing';

      const onPointerMove = (moveEvent: PointerEvent) => {
        const deltaX = moveEvent.clientX - startPointerX;
        const deltaY = moveEvent.clientY - startPointerY;

        const maxX = Math.max(12, window.innerWidth - sizeRef.current.width - 12);
        const maxY = Math.max(12, window.innerHeight - sizeRef.current.height - 12);

        const newX = Math.min(Math.max(12, startPosX + deltaX), maxX);
        const newY = Math.min(Math.max(12, startPosY + deltaY), maxY);

        const next = { x: Math.round(newX), y: Math.round(newY) };
        setPosition(next);
        savePosition(next, false);
      };

      const onPointerUp = () => {
        setIsDragging(false);
        document.body.style.userSelect = '';
        document.body.style.cursor = '';
        savePosition(posRef.current, true);
        window.removeEventListener('pointermove', onPointerMove);
        window.removeEventListener('pointerup', onPointerUp);
        window.removeEventListener('pointercancel', onPointerUp);
      };

      window.addEventListener('pointermove', onPointerMove);
      window.addEventListener('pointerup', onPointerUp);
      window.addEventListener('pointercancel', onPointerUp);
    },
    [savePosition],
  );

  // 4. RESIZING HANDLER (Corners and Edges)
  const handleResizePointerDown = useCallback(
    (direction: ResizeDirection) => (e: React.PointerEvent) => {
      e.preventDefault();
      e.stopPropagation();

      setIsResizing(true);
      const startPointerX = e.clientX;
      const startPointerY = e.clientY;
      const startPosX = posRef.current.x;
      const startPosY = posRef.current.y;
      const startWidth = sizeRef.current.width;
      const startHeight = sizeRef.current.height;

      const cursorMap: Record<ResizeDirection, string> = {
        top: 'ns-resize',
        bottom: 'ns-resize',
        left: 'ew-resize',
        right: 'ew-resize',
        'top-left': 'nwse-resize',
        'top-right': 'nesw-resize',
        'bottom-left': 'nesw-resize',
        'bottom-right': 'nwse-resize',
      };

      document.body.style.userSelect = 'none';
      document.body.style.cursor = cursorMap[direction];

      const onPointerMove = (moveEvent: PointerEvent) => {
        const deltaX = moveEvent.clientX - startPointerX;
        const deltaY = moveEvent.clientY - startPointerY;

        let newWidth = startWidth;
        let newHeight = startHeight;
        let newX = startPosX;
        let newY = startPosY;

        const maxViewportW = Math.max(minWidth, window.innerWidth - 12);
        const maxViewportH = Math.max(minHeight, window.innerHeight - 12);

        // Horizontal resizing
        if (direction.includes('right')) {
          const maxAllowedW = maxViewportW - startPosX;
          newWidth = Math.max(minWidth, Math.min(startWidth + deltaX, maxAllowedW));
        } else if (direction.includes('left')) {
          const maxAllowedW = startPosX + startWidth - 12;
          newWidth = Math.max(minWidth, Math.min(startWidth - deltaX, maxAllowedW));
          newX = startPosX + startWidth - newWidth;
        }

        // Vertical resizing
        if (direction.includes('bottom')) {
          const maxAllowedH = maxViewportH - startPosY;
          newHeight = Math.max(minHeight, Math.min(startHeight + deltaY, maxAllowedH));
        } else if (direction.includes('top')) {
          const maxAllowedH = startPosY + startHeight - 12;
          newHeight = Math.max(minHeight, Math.min(startHeight - deltaY, maxAllowedH));
          newY = startPosY + startHeight - newHeight;
        }

        const nextSize = { width: Math.round(newWidth), height: Math.round(newHeight) };
        const nextPos = { x: Math.round(newX), y: Math.round(newY) };

        setSize(nextSize);
        setPosition(nextPos);
        saveSize(nextSize, false);
        savePosition(nextPos, false);
      };

      const onPointerUp = () => {
        setIsResizing(false);
        document.body.style.userSelect = '';
        document.body.style.cursor = '';
        saveSize(sizeRef.current, true);
        savePosition(posRef.current, true);
        window.removeEventListener('pointermove', onPointerMove);
        window.removeEventListener('pointerup', onPointerUp);
        window.removeEventListener('pointercancel', onPointerUp);
      };

      window.addEventListener('pointermove', onPointerMove);
      window.addEventListener('pointerup', onPointerUp);
      window.addEventListener('pointercancel', onPointerUp);
    },
    [minWidth, minHeight, saveSize, savePosition],
  );

  const resetPosition = useCallback(() => {
    const def = getDefaultPos();
    setPosition(def);
    savePosition(def, true);
  }, [getDefaultPos, savePosition]);

  const resetSize = useCallback(() => {
    const def = { width: initialWidth, height: initialHeight };
    setSize(def);
    saveSize(def, true);
  }, [initialWidth, initialHeight, saveSize]);

  return {
    position,
    setPosition,
    size,
    setSize,
    isDragging,
    isResizing,
    resetPosition,
    resetSize,
    dragHandleProps: {
      onPointerDown: handlePointerDown,
      style: { cursor: isDragging ? 'grabbing' : 'grab' },
    },
    getResizeHandleProps: (direction: ResizeDirection) => ({
      onPointerDown: handleResizePointerDown(direction),
    }),
  };
}
