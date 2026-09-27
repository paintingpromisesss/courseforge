import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';

export function GitHubSettingsSection() {
  const qc = useQueryClient();
  const [token, setToken] = useState('');
  const [username, setUsername] = useState('');
  const [showToken, setShowToken] = useState(false);
  const [notice, setNotice] = useState<{ kind: 'ok' | 'err'; text: string } | null>(null);

  const { data: auth } = useQuery({ queryKey: ['git-auth'], queryFn: api.gitAuth });

  const refreshAuth = () => qc.invalidateQueries({ queryKey: ['git-auth'] });

  const saveMut = useMutation({
    mutationFn: (body: { token?: string; username?: string }) => api.gitSaveAuth(body),
    onSuccess: () => {
      setToken('');
      setUsername('');
      setNotice({ kind: 'ok', text: 'Токен сохранён' });
      refreshAuth();
    },
    onError: (err) => setNotice({ kind: 'err', text: err instanceof Error ? err.message : String(err) }),
  });

  const testMut = useMutation({
    mutationFn: api.gitTestAuth,
    onSuccess: (res) => {
      setNotice(res.ok
        ? { kind: 'ok', text: `Токен работает${res.login ? ` — GitHub: ${res.login}` : ''}` }
        : { kind: 'err', text: res.error || 'Токен не прошёл проверку' });
    },
    onError: (err) => setNotice({ kind: 'err', text: err instanceof Error ? err.message : String(err) }),
  });

  const configured = auth?.configured ?? false;

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h3 className="text-sm font-semibold text-tx-1">GitHub</h3>
        <p className="text-xs text-tx-3 mt-0.5">
          Токен доступа (PAT) нужен для импорта курсов из приватных репозиториев. Для публичных — не обязателен.
        </p>
      </div>

      {notice && (
        <div
          className={
            notice.kind === 'ok'
              ? 'rounded-xl border border-ok/30 bg-ok/10 p-3 text-xs font-medium text-ok'
              : 'rounded-xl border border-err/30 bg-err/10 p-3 text-xs font-medium text-err'
          }
        >
          {notice.text}
        </div>
      )}

      <div className="rounded-xl border border-bdr bg-bg-2/50 p-4 space-y-4">
        <div className="space-y-1.5">
          <div className="flex items-center justify-between">
            <label className="text-xs font-medium text-tx-2">Personal Access Token</label>
            {configured && auth && (
              <span className="text-[11px] text-ok font-medium">
                Токен сохранён: {auth.token_masked || auth.username}
              </span>
            )}
          </div>
          <div className="relative flex items-center">
            <input
              type={showToken ? 'text' : 'password'}
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder={configured ? '•••••••••••• (сохранён — введите новый для замены)' : 'ghp_...'}
              autoComplete="off"
              className="w-full bg-bg-3 border border-bdr rounded-lg pl-3 pr-9 py-2 text-xs text-tx-1 placeholder:text-tx-3/50 focus:border-brand focus:ring-1 focus:ring-brand outline-none font-mono transition-colors"
            />
            <button
              type="button"
              onClick={() => setShowToken((v) => !v)}
              className="absolute right-2 text-tx-3 hover:text-tx-1 p-1 rounded transition-colors cursor-pointer"
              title={showToken ? 'Скрыть токен' : 'Показать токен'}
            >
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                <circle cx="12" cy="12" r="3" />
              </svg>
            </button>
          </div>
          <p className="text-[11px] text-tx-3">
            Создайте токен на GitHub → Settings → Developer settings → Personal access tokens. Достаточно публичного доступа (public_repo); для приватных репозиториев нужен scope <code className="font-mono">repo</code>.
          </p>
        </div>

        <div className="space-y-1.5">
          <label className="text-xs font-medium text-tx-2">Имя пользователя (необязательно)</label>
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder={auth?.username || 'например, octocat'}
            className="w-full bg-bg-3 border border-bdr rounded-lg px-3 py-2 text-xs text-tx-1 placeholder:text-tx-3/50 focus:border-brand focus:ring-1 focus:ring-brand outline-none font-mono transition-colors"
          />
        </div>

        <div className="flex flex-wrap items-center gap-2 pt-1">
          <button
            type="button"
            disabled={!token.trim() || saveMut.isPending}
            onClick={() => saveMut.mutate({ token: token.trim(), username: username.trim() || undefined })}
            className="px-5 py-2 bg-brand hover:bg-brand-hover disabled:opacity-50 disabled:cursor-not-allowed text-white text-xs font-semibold rounded-xl transition-all shadow-md shadow-brand/20 active:scale-[0.98] cursor-pointer"
          >
            {saveMut.isPending ? 'Сохранение...' : 'Сохранить'}
          </button>
          <button
            type="button"
            disabled={!configured || testMut.isPending}
            onClick={() => testMut.mutate()}
            title={configured ? 'Проверить токен через GitHub API' : 'Сначала сохраните токен'}
            className="px-5 py-2 rounded-xl border border-bdr bg-bg-3 hover:bg-bg-4 text-tx-2 hover:text-tx-1 text-xs font-medium disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer"
          >
            {testMut.isPending ? 'Проверка...' : 'Проверить'}
          </button>
          {configured && (
            <button
              type="button"
              disabled={saveMut.isPending}
              onClick={() => saveMut.mutate({ token: '' })}
              className="px-5 py-2 rounded-xl border border-err/30 bg-err/10 hover:bg-err/20 text-err text-xs font-medium disabled:opacity-40 transition-colors cursor-pointer"
            >
              Удалить токен
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
