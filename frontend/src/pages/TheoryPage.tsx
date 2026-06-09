import { useCallback, useEffect, useLayoutEffect } from 'react';
import { useParams, useOutletContext } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import { Markdown } from '../components/ui/Markdown';
import { useReadOnScroll } from '../hooks/useReadOnScroll';
import type { CoursePageContext } from './CoursePage';

export function TheoryPage() {
  const { courseSlug, trackSlug, topicSlug, unitSlug } = useParams<{
    courseSlug: string; trackSlug: string; topicSlug: string; unitSlug: string;
  }>();
  const qc = useQueryClient();
  const { mainRef } = useOutletContext<CoursePageContext>();

  const { data, isLoading, error } = useQuery({
    queryKey: ['theory', courseSlug, trackSlug, topicSlug, unitSlug],
    queryFn: () => api.getTheory(courseSlug!, trackSlug!, topicSlug!, unitSlug!),
    enabled: !!(courseSlug && trackSlug && topicSlug && unitSlug),
  });

  const handleRead = useCallback(() => {
    if (!courseSlug || !unitSlug) return;
    api.markDone(courseSlug, unitSlug, true)
      .then(() => qc.invalidateQueries({ queryKey: ['progress', courseSlug] }))
      .catch(() => {});
  }, [courseSlug, unitSlug, qc]);

  const { reset } = useReadOnScroll(handleRead, mainRef, true);

  // Scroll to top before paint when unit changes
  useLayoutEffect(() => {
    if (mainRef.current) mainRef.current.scrollTop = 0;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [unitSlug]);

  // Reset read flag when unit changes
  useEffect(() => {
    reset();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [unitSlug]);

  // Re-check when content loads (catches short theories)
  useEffect(() => {
    if (data) reset();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data]);

  if (isLoading) return <div className="p-8 text-tx-3 text-sm">Загрузка...</div>;
  if (error) return <div className="p-8 text-err text-sm">Ошибка загрузки теории</div>;

  const BASE = import.meta.env.VITE_API_URL ?? 'http://localhost:8080/api';
  const assetBase = `${BASE}/courses/${courseSlug}/tracks/${trackSlug}/topics/${topicSlug}/units/${unitSlug}`;

  return (
    <div className="max-w-3xl mx-auto px-8 py-8">
      <Markdown content={data ?? ''} assetBase={assetBase} />
    </div>
  );
}
