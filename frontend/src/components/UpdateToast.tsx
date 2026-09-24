import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { motion, AnimatePresence } from 'framer-motion';
import { api } from '../api/client';
import { useSettings } from '../context/SettingsContext';

const DISMISS_KEY = 'cf:dismissed-update';

function ArrowUpCircleIcon() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10" />
      <polyline points="16 12 12 8 8 12" />
      <line x1="12" y1="16" x2="12" y2="8" />
    </svg>
  );
}

function CloseIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <line x1="18" y1="6" x2="6" y2="18" />
      <line x1="6" y1="6" x2="18" y2="18" />
    </svg>
  );
}

export function UpdateToast() {
  const { openSettings } = useSettings();
  const [manuallyDismissedVersion, setManuallyDismissedVersion] = useState<string | null>(null);

  const { data: verInfo } = useQuery({
    queryKey: ['version'],
    queryFn: api.getVersion,
    staleTime: 60_000,
  });

  const check = verInfo?.check;
  const updateAvailable = Boolean(check?.update_available && check?.latest_version);
  const latestVersion = check?.latest_version ?? '';

  let previouslyDismissed: string | null = null;
  try {
    previouslyDismissed = sessionStorage.getItem(DISMISS_KEY);
  } catch {
    // Ignore sessionStorage errors
  }

  const isDismissed =
    !updateAvailable ||
    !latestVersion ||
    manuallyDismissedVersion === latestVersion ||
    previouslyDismissed === latestVersion;

  const handleDismiss = () => {
    setManuallyDismissedVersion(latestVersion);
    try {
      sessionStorage.setItem(DISMISS_KEY, latestVersion);
    } catch {
      // Ignore sessionStorage errors
    }
  };

  const handleView = () => {
    handleDismiss();
    openSettings('about');
  };

  return (
    <AnimatePresence>
      {!isDismissed && (
        <motion.div
          role="status"
          aria-live="polite"
          className="fixed bottom-5 right-5 z-40 max-w-sm w-full p-4 rounded-2xl bg-bg-1 border border-brand/35 shadow-2xl text-tx-1 backdrop-blur-md"
          initial={{ opacity: 0, y: 24, scale: 0.95 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          exit={{ opacity: 0, y: 24, scale: 0.95 }}
          transition={{ type: 'spring', duration: 0.35, bounce: 0.15 }}
        >
          <div className="flex items-start gap-3">
            <div className="w-9 h-9 rounded-xl bg-brand/15 border border-brand/30 text-brand flex items-center justify-center shrink-0 mt-0.5">
              <ArrowUpCircleIcon />
            </div>

            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between gap-2">
                <span className="text-xs font-semibold text-tx-1">Доступно обновление</span>
                <button
                  type="button"
                  onClick={handleDismiss}
                  aria-label="Закрыть уведомление"
                  className="w-5 h-5 rounded flex items-center justify-center text-tx-3 hover:text-tx-1 transition-colors cursor-pointer"
                >
                  <CloseIcon />
                </button>
              </div>

              <p className="text-[11px] text-tx-2 mt-1 leading-snug">
                Вышла новая версия <span className="font-mono font-medium text-brand">{latestVersion}</span>.
                Вы можете обновить бинарный файл в один клик.
              </p>

              <div className="flex items-center gap-2 mt-3">
                <button
                  type="button"
                  onClick={handleView}
                  className="px-3 py-1.5 rounded-lg bg-brand hover:bg-brand-hover text-white text-xs font-medium transition-all shadow-sm cursor-pointer"
                >
                  Посмотреть
                </button>
                <button
                  type="button"
                  onClick={handleDismiss}
                  className="px-2.5 py-1.5 rounded-lg text-tx-3 hover:text-tx-1 text-xs transition-colors cursor-pointer"
                >
                  Позже
                </button>
              </div>
            </div>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
