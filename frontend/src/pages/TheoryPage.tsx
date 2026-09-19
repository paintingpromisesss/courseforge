import { useCallback, useLayoutEffect, useMemo, useState } from 'react';
import { useParams, useOutletContext } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';

import { api } from '../api/client';
import { Markdown, VideoEmbed } from '../components/ui/Markdown';
import type { CoursePageContext } from './CoursePage';

export function TheoryPage() {
  const { courseSlug, trackSlug, topicSlug, unitSlug } = useParams<{
    courseSlug: string; trackSlug: string; topicSlug: string; unitSlug: string;
  }>();
  const qc = useQueryClient();
  const { mainRef, course } = useOutletContext<CoursePageContext>();
  const [markingDone, setMarkingDone] = useState(false);

  const { data: fetchedCourse } = useQuery({
    queryKey: ['course', courseSlug],
    queryFn: () => api.getCourse(courseSlug!),
    enabled: !course && !!courseSlug,
  });
  const currentCourse = course ?? fetchedCourse;

  const unit = useMemo(() => {
    if (!currentCourse) return null;
    return currentCourse.tracks
      .flatMap((t) => t.topics)
      .flatMap((p) => p.units)
      .find((u) => u.slug === unitSlug);
  }, [currentCourse, unitSlug]);

  const shouldFetchTheory = unit ? unit.has_theory : true;

  const { data, isLoading, error } = useQuery({
    queryKey: ['theory', courseSlug, trackSlug, topicSlug, unitSlug],
    queryFn: () => api.getTheory(courseSlug!, trackSlug!, topicSlug!, unitSlug!),
    enabled: !!(courseSlug && trackSlug && topicSlug && unitSlug) && shouldFetchTheory,
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
      // course cards show done counts
      qc.invalidateQueries({ queryKey: ['courses'] });
      qc.invalidateQueries({ queryKey: ['catalogs'] });
    } finally {
      setMarkingDone(false);
    }
  }, [courseSlug, unitSlug, theoryDone, qc]);

  useLayoutEffect(() => {
    if (mainRef.current) mainRef.current.scrollTop = 0;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [unitSlug]);

  if (isLoading || (!unit && !error)) return <div className="p-8 text-tx-3 text-sm">Загрузка...</div>;
  if (error && !unit?.video_url) return <div className="p-8 text-err text-sm">Ошибка загрузки теории</div>;

  const BASE = import.meta.env.VITE_API_URL ?? '/api';
  const assetBase = `${BASE}/courses/${courseSlug}/tracks/${trackSlug}/topics/${topicSlug}/units/${unitSlug}`;

  const hasContent = !!data?.trim();
  const hasVideo = !!unit?.video_url;
  const contentHasUnitVideo = !!(unit?.video_url && data?.includes(unit.video_url));
  const showTopVideo = hasVideo && !contentHasUnitVideo;
  const showUnitTitle = unit?.title && (!hasContent || !data?.trim().startsWith('#'));

  return (
    <div className="max-w-3xl mx-auto px-8 py-8">
      {showUnitTitle && (
        <h1 className="text-2xl font-bold text-tx-1 mb-6">{unit.title}</h1>
      )}
      {showTopVideo && <VideoEmbed href={unit.video_url!} />}
      {hasContent && <Markdown content={data!} assetBase={assetBase} />}
      {!hasContent && !hasVideo && !isLoading && (
        <div className="text-tx-3 text-sm italic py-4">В этой теме пока нет материалов.</div>
      )}
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
