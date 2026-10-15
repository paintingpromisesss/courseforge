import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { SyncHistoryItem } from '../api/types';
import { formatSyncTime, rollbackQuestion } from '../lib/syncHelpers';

// CloudSettingsSection: cloud-vault sync setup, triggers, status, history and
// rollback. Rendered inside the 'github' settings tab.
export function CloudSettingsSection() {
  const qc = useQueryClient();
  const [remoteUrl, setRemoteUrl] = useState('');
  const [branch, setBranch] = useState('');
  const [interval, setIntervalMin] = useState('');
  const [notice, setNotice] = useState<{ kind: 'ok' | 'err'; text: string } | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [pendingRollback, setPendingRollback] = useState<string | null>(null);

  const { data: cfg } = useQuery({ queryKey: ['sync-config'], queryFn: api.syncConfig });
  const { data: status } = useQuery({
    queryKey: ['sync-status'],
    queryFn: api.syncStatus,
    refetchInterval: (query) => (query.state.data?.syncing ? 3000 : false),
  });
  const { data: history } = useQuery({
    queryKey: ['sync-history'],
    queryFn: () => api.syncHistory(50),
  });

  const refresh = () => {
    qc.invalidateQueries({ queryKey: ['sync-config'] });
    qc.invalidateQueries({ queryKey: ['sync-status'] });
    qc.invalidateQueries({ queryKey: ['sync-history'] });
  };

  const saveMut = useMutation({
    mutationFn: (body: Parameters<typeof api.syncSaveConfig>[0]) => api.syncSaveConfig(body),
    onSuccess: () => {
      setNotice({ kind: 'ok', text: 'Настройки сохранены' });
      refresh();
    },
    onError: (err) => setNotice({ kind: 'err', text: err instanceof Error ? err.message : String(err) }),
  });

  const pushMut = useMutation({
    mutationFn: api.syncPush,
    onSuccess: () => {
      setNotice({ kind: 'ok', text: 'Снапшот отправлен в облако' });
      refresh();
    },
    onError: (err) => setNotice({ kind: 'err', text: err instanceof Error ? err.message : String(err) }),
  });

  const pullMut = useMutation({
    mutationFn: api.syncPull,
    onSuccess: () => {
      setNotice({ kind: 'ok', text: 'Данные загружены из облака' });
      refresh();
      qc.invalidateQueries({ queryKey: ['courses'] });
      qc.invalidateQueries({ queryKey: ['catalogs'] });
    },
    onError: (err) => setNotice({ kind: 'err', text: err instanceof Error ? err.message : String(err) }),
  });

  const rollbackMut = useMutation({
    mutationFn: (commit: string) => api.syncRollback(commit),
    onSuccess: () => {
      setPendingRollback(null);
      setNotice({ kind: 'ok', text: 'Откат выполнен' });
      refresh();
      qc.invalidateQueries({ queryKey: ['courses'] });
    },
    onError: (err) => setNotice({ kind: 'err', text: err instanceof Error ? err.message : String(err) }),
  });

  const restoreMut = useMutation({
    mutationFn: api.syncRestoreImports,
    onSuccess: (res) => {
      setNotice({
        kind: 'ok',
        text: res.restored.length > 0
          ? `Восстановлено курсов: ${res.restored.length}`
          : 'Нет курсов для восстановления',
      });
      refresh();
      qc.invalidateQueries({ queryKey: ['courses'] });
    },
    onError: (err) => setNotice({ kind: 'err', text: err instanceof Error ? err.message : String(err) }),
  });

  const configured = status?.configured ?? false;
  const triggers = cfg?.triggers;

  const saveSetup = () => {
    saveMut.mutate({
      remote_url: remoteUrl.trim() || cfg?.remote_url,
      branch: branch.trim() || cfg?.branch,
      enabled: true,
      triggers: {
        on_progress: triggers?.on_progress ?? false,
        interval_min: interval.trim() ? Number(interval) : (triggers?.interval_min ?? 0),
        on_startup_pull: triggers?.on_startup_pull ?? false,
      },
    });
    setRemoteUrl('');
    setBranch('');
  };

  const toggleTrigger = (key: 'on_progress' | 'on_startup_pull') => {
    if (!triggers) return;
    saveMut.mutate({
      triggers: {
        on_progress: key === 'on_progress' ? !triggers.on_progress : triggers.on_progress,
        interval_min: triggers.interval_min,
        on_startup_pull: key === 'on_startup_pull' ? !triggers.on_startup_pull : triggers.on_startup_pull,
      },
    });
  };

  const saveInterval = () => {
    if (!triggers) return;
    saveMut.mutate({
      triggers: {
        on_progress: triggers.on_progress,
        interval_min: interval.trim() ? Number(interval) : 0,
        on_startup_pull: triggers.on_startup_pull,
      },
    });
  };

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h3 className="text-sm font-semibold text-tx-1">Облако</h3>
        <p className="text-xs text-tx-3 mt-0.5">
          Личный репозиторий-«сейф»: все курсы и прогресс отправляются снапшотом в ваш GitHub, с историей и откатом.
        </p>
      </div>

      {notice && (
        <div className={
          notice.kind === 'ok'
            ? 'rounded-xl border border-ok/30 bg-ok/10 p-3 text-xs font-medium text-ok'
            : 'rounded-xl border border-err/30 bg-err/10 p-3 text-xs font-medium text-err'
        }>
          {notice.text}
        </div>
      )}

      {/* 1. Настройка */}
      <div className="rounded-xl border border-bdr bg-bg-2/50 p-4 space-y-4">
        <div className="text-xs font-semibold text-tx-2">Репозиторий-хранилище</div>
        <input
          type="text"
          value={remoteUrl}
          onChange={(e) => setRemoteUrl(e.target.value)}
          placeholder={cfg?.remote_url || 'https://github.com/me/courseforge-vault'}
          className="w-full bg-bg-3 border border-bdr rounded-lg px-3 py-2 text-xs text-tx-1 placeholder:text-tx-3/50 focus:border-brand focus:ring-1 focus:ring-brand outline-none font-mono"
        />
        <div className="flex gap-2">
          <input
            type="text"
            value={branch}
            onChange={(e) => setBranch(e.target.value)}
            placeholder={cfg?.branch || 'main'}
            className="w-32 bg-bg-3 border border-bdr rounded-lg px-3 py-2 text-xs text-tx-1 placeholder:text-tx-3/50 focus:border-brand focus:ring-1 focus:ring-brand outline-none font-mono"
          />
          <button
            type="button"
            disabled={saveMut.isPending || (!remoteUrl.trim() && !branch.trim() && !cfg?.enabled)}
            onClick={saveSetup}
            className="px-5 py-2 bg-brand hover:bg-brand-hover disabled:opacity-50 disabled:cursor-not-allowed text-white text-xs font-semibold rounded-xl transition-all cursor-pointer"
          >
            {saveMut.isPending ? 'Сохранение...' : 'Включить синхронизацию'}
          </button>
        </div>
        {configured && cfg?.enabled && (
          <button
            type="button"
            onClick={() => saveMut.mutate({ enabled: false })}
            className="px-5 py-2 rounded-xl border border-err/30 bg-err/10 hover:bg-err/20 text-err text-xs font-medium transition-colors cursor-pointer"
          >
            Выключить
          </button>
        )}
      </div>

      {/* 3. Триггеры */}
      {triggers && (
        <div className="rounded-xl border border-bdr bg-bg-2/50 p-4 space-y-3">
          <div className="text-xs font-semibold text-tx-2">Автоматическая синхронизация</div>
          <label className="flex items-center gap-2 text-xs text-tx-2 cursor-pointer">
            <input
              type="checkbox"
              checked={triggers.on_progress}
              onChange={() => toggleTrigger('on_progress')}
              className="accent-brand"
            />
            После каждого решённого задания (с задержкой 30 с)
          </label>
          <label className="flex items-center gap-2 text-xs text-tx-2 cursor-pointer">
            <input
              type="checkbox"
              checked={triggers.on_startup_pull}
              onChange={() => toggleTrigger('on_startup_pull')}
              className="accent-brand"
            />
            Загружать данные из облака при запуске
          </label>
          <div className="flex items-center gap-2">
            <input
              type="number"
              min={0}
              value={interval}
              onChange={(e) => setIntervalMin(e.target.value)}
              placeholder={String(triggers.interval_min || 0)}
              className="w-24 bg-bg-3 border border-bdr rounded-lg px-3 py-2 text-xs text-tx-1 focus:border-brand outline-none"
            />
            <span className="text-xs text-tx-3">мин — периодический push (0 = выкл)</span>
            <button
              type="button"
              disabled={!interval.trim() || saveMut.isPending}
              onClick={saveInterval}
              className="px-3 py-1.5 rounded-lg border border-bdr bg-bg-3 hover:bg-bg-4 text-tx-2 text-xs transition-colors cursor-pointer disabled:opacity-40"
            >
              ОК
            </button>
          </div>
        </div>
      )}

      {/* 4. Статус */}
      {configured && (
        <div className="rounded-xl border border-bdr bg-bg-2/50 p-4 space-y-3">
          <div className="flex items-center justify-between">
            <div className="text-xs font-semibold text-tx-2">Состояние</div>
            {status?.syncing && <span className="text-[11px] text-brand">синхронизация...</span>}
          </div>
          <div className="text-xs text-tx-3">
            Последняя синхронизация: {formatSyncTime(status?.last_sync || cfg?.last_sync)}
            {status?.branch && <> · ветка <span className="font-mono">{status.branch}</span></>}
            {status?.commit && <> · <span className="font-mono">{status.commit.slice(0, 7)}</span></>}
          </div>
          <div className="flex gap-2">
            <button
              type="button"
              disabled={pushMut.isPending || (status?.syncing ?? false)}
              onClick={() => pushMut.mutate()}
              className="px-5 py-2 bg-brand hover:bg-brand-hover disabled:opacity-50 text-white text-xs font-semibold rounded-xl transition-all cursor-pointer"
            >
              {pushMut.isPending ? 'Отправка...' : 'Синхронизировать'}
            </button>
            <button
              type="button"
              disabled={pullMut.isPending || (status?.syncing ?? false)}
              onClick={() => pullMut.mutate()}
              className="px-5 py-2 rounded-xl border border-bdr bg-bg-3 hover:bg-bg-4 text-tx-2 text-xs font-medium transition-colors cursor-pointer disabled:opacity-40"
            >
              {pullMut.isPending ? 'Загрузка...' : 'Скачать'}
            </button>
          </div>
          {(status?.pending_imports?.length ?? 0) > 0 && (
            <div className="space-y-2">
              <div className="text-xs text-tx-3">
                Импортированные курсы из облака отсутствуют локально: {status?.pending_imports.join(', ')}
              </div>
              <button
                type="button"
                disabled={restoreMut.isPending}
                onClick={() => restoreMut.mutate()}
                className="px-5 py-2 rounded-xl border border-bdr bg-bg-3 hover:bg-bg-4 text-tx-2 text-xs font-medium transition-colors cursor-pointer"
              >
                Восстановить импортированные курсы
              </button>
            </div>
          )}
        </div>
      )}

      {/* 5. История */}
      {configured && (history?.length ?? 0) > 0 && (
        <div className="rounded-xl border border-bdr bg-bg-2/50 p-4 space-y-2">
          <div className="text-xs font-semibold text-tx-2">История</div>
          <div className="divide-y divide-bdr/50">
            {(history ?? []).map((h: SyncHistoryItem) => (
              <div key={h.commit} className="py-2 space-y-1">
                <button
                  type="button"
                  onClick={() => setExpanded(expanded === h.commit ? null : h.commit)}
                  className="w-full flex items-center justify-between text-left text-xs text-tx-2 hover:text-tx-1 cursor-pointer"
                >
                  <span className="font-mono text-tx-3">{h.commit.slice(0, 7)}</span>
                  <span className="truncate ml-2 flex-1">{h.subject}</span>
                  <span className="text-tx-3 whitespace-nowrap ml-2">{formatSyncTime(h.time)}</span>
                </button>
                {expanded === h.commit && (
                  <div className="space-y-2 pl-2">
                    <HistoryFiles commit={h.commit} />
                    <button
                      type="button"
                      disabled={rollbackMut.isPending}
                      onClick={() => setPendingRollback(h.commit)}
                      className="px-3 py-1.5 rounded-lg border border-err/30 bg-err/10 hover:bg-err/20 text-err text-xs font-medium transition-colors cursor-pointer"
                    >
                      Откатиться сюда
                    </button>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Rollback confirm — in-page, no window.confirm */}
      {pendingRollback && (
        <div className="rounded-xl border border-err/30 bg-err/10 p-4 space-y-3">
          <div className="text-xs text-tx-2">{rollbackQuestion(pendingRollback)}</div>
          <div className="flex gap-2">
            <button
              type="button"
              disabled={rollbackMut.isPending}
              onClick={() => rollbackMut.mutate(pendingRollback)}
              className="px-4 py-2 bg-err hover:bg-err/80 disabled:opacity-50 text-white text-xs font-semibold rounded-xl cursor-pointer"
            >
              {rollbackMut.isPending ? 'Откат...' : 'Откатить'}
            </button>
            <button
              type="button"
              onClick={() => setPendingRollback(null)}
              className="px-4 py-2 rounded-xl border border-bdr bg-bg-3 hover:bg-bg-4 text-tx-2 text-xs font-medium cursor-pointer"
            >
              Отмена
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

// HistoryFiles lazily loads the file list of an expanded commit.
function HistoryFiles({ commit }: { commit: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ['sync-commit-files', commit],
    queryFn: () => api.syncCommitFiles(commit),
  });
  if (isLoading) return <div className="text-[11px] text-tx-3">загрузка...</div>;
  return (
    <div className="text-[11px] text-tx-3 font-mono space-y-0.5 max-h-40 overflow-y-auto">
      {(data?.files ?? []).map((f) => <div key={f}>{f}</div>)}
    </div>
  );
}
