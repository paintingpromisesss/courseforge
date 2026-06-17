import { useCallback, useLayoutEffect, useState } from 'react';
import { useParams, useOutletContext } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';

import { api } from '../api/client';
import { Markdown } from '../components/ui/Markdown';
import type { CoursePageContext } from './CoursePage';

export function TheoryPage() {
  const { courseSlug, trackSlug, topicSlug, unitSlug } = useParams<{
    courseSlug: string; trackSlug: string; topicSlug: string; unitSlug: string;
  }>();
  const qc = useQueryClient();
  const { mainRef } = useOutletContext<CoursePageContext>();
  const [markingDone, setMarkingDone] = useState(false);

  const { data, isLoading, error } = useQuery({
    queryKey: ['theory', courseSlug, trackSlug, topicSlug, unitSlug],
    queryFn: () => api.getTheory(courseSlug!, trackSlug!, topicSlug!, unitSlug!),
    enabled: !!(courseSlug && trackSlug && topicSlug && unitSlug),
  });

  const { data: progress } = useQuery({
    queryKey: ['progress', courseSlug],
    queryFn: () => api.getProgress(courseSlug!),
    enabled: !!courseSlug,
  });

  const theoryDone = !!(unitSlug && progress?.completed_tasks?.[unitSlug]);

  const markTheoryDone = useCallback(async () => {
    if (!courseSlug || !unitSlug || theoryDone) return;
    setMarkingDone(true);
    try {
      await api.markDone(courseSlug, unitSlug, true);
      await qc.invalidateQueries({ queryKey: ['progress', courseSlug] });
    } finally {
      setMarkingDone(false);
    }
  }, [courseSlug, unitSlug, theoryDone, qc]);

  useLayoutEffect(() => {
    if (mainRef.current) mainRef.current.scrollTop = 0;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [unitSlug]);

  if (isLoading) return <div className="p-8 text-tx-3 text-sm">Загрузка...</div>;
  if (error) return <div className="p-8 text-err text-sm">Ошибка загрузки теории</div>;

  const BASE = import.meta.env.VITE_API_URL ?? '/api';
  const assetBase = `${BASE}/courses/${courseSlug}/tracks/${trackSlug}/topics/${topicSlug}/units/${unitSlug}`;

  return (
    <div className="max-w-3xl mx-auto px-8 py-8">
      <Markdown content={data ?? ''} assetBase={assetBase} />
      <div className="mt-8 flex justify-end">
        <button
          type="button"
          onClick={() => void markTheoryDone().catch(() => {})}
          disabled={theoryDone || markingDone}
          className={clsx(
            'rounded px-3 py-1.5 text-sm transition-colors',
            theoryDone
              ? 'bg-bg-4 text-ok'
              : 'bg-brand text-white hover:bg-brand-hover disabled:opacity-70',
          )}
        >
          {theoryDone ? 'Тема пройдена' : markingDone ? 'Сохраняю...' : 'Отметить тему пройденной'}
        </button>
      </div>
    </div>
  );
}
