import { useRef, useCallback, useEffect, useState } from 'react';

export function useReadOnScroll(
  onRead: () => void,
  scrollRoot: React.RefObject<HTMLElement | null>,
  active: boolean,
) {
  const called = useRef(false);
  const [tick, setTick] = useState(0);

  const reset = useCallback(() => {
    called.current = false;
    setTick((t) => t + 1);
  }, []);

  useEffect(() => {
    if (!active || !scrollRoot.current) return;
    const el = scrollRoot.current;
    called.current = false;

    const check = () => {
      if (called.current) return;
      const atBottom = el.scrollHeight <= el.clientHeight + 10
        || el.scrollTop + el.clientHeight >= el.scrollHeight - 50;
      if (atBottom) {
        called.current = true;
        onRead();
      }
    };

    check();
    el.addEventListener('scroll', check, { passive: true });
    return () => el.removeEventListener('scroll', check);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, onRead, tick]);

  return { reset };
}
