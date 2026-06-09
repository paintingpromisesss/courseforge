import { useState, useRef } from 'react';
import { useParams, useNavigate, Outlet } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import clsx from 'clsx';
import { api } from '../api/client';
import { ProgressBar } from '../components/ui/ProgressBar';
import type { TrackItem, TopicItem, UnitItem, TaskItem } from '../api/types';

function countTasks(track: TrackItem): number {
  return track.topics.reduce((a, p) => a + p.units.reduce((b, u) => b + u.tasks.length, 0), 0);
}

function countTopicTasks(topic: TopicItem): number {
  return topic.units.reduce((a, u) => a + u.tasks.length, 0);
}

function doneInTrack(track: TrackItem, done: Record<string, boolean>): number {
  return track.topics.reduce((a, p) =>
    a + p.units.reduce((b, u) =>
      b + u.tasks.filter((t) => done[t.slug]).length, 0), 0);
}

function doneInTopic(topic: TopicItem, done: Record<string, boolean>): number {
  return topic.units.reduce((a, u) => a + u.tasks.filter((t) => done[t.slug]).length, 0);
}

interface SidebarProps {
  tracks: TrackItem[];
  done: Record<string, boolean>;
  activeTaskSlug?: string;
  activeUnitSlug?: string;
  onTask: (track: TrackItem, topic: TopicItem, unit: UnitItem, task: TaskItem) => void;
  onTheory: (track: TrackItem, topic: TopicItem, unit: UnitItem) => void;
}

function Sidebar({ tracks, done, activeTaskSlug, activeUnitSlug, onTask, onTheory }: SidebarProps) {
  const [openTracks, setOpenTracks] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(tracks.map((t) => [t.slug, true]))
  );
  const [openTopics, setOpenTopics] = useState<Record<string, boolean>>({});
  const [openUnits, setOpenUnits] = useState<Record<string, boolean>>({});

  const toggle = (map: Record<string, boolean>, set: (v: Record<string, boolean>) => void, key: string) =>
    set({ ...map, [key]: !map[key] });

  return (
    <nav className="w-64 shrink-0 border-r border-bdr bg-bg-2 h-full overflow-y-auto">
      <div className="p-3 space-y-1">
        {tracks.map((track) => {
          const total = countTasks(track);
          const d = doneInTrack(track, done);
          return (
            <div key={track.slug}>
              <button
                onClick={() => toggle(openTracks, setOpenTracks, track.slug)}
                className="w-full flex items-center gap-2 px-2 py-1.5 rounded hover:bg-bg-4 transition-colors text-left group"
              >
                <span className={clsx('text-tx-3 text-xs transition-transform', openTracks[track.slug] && 'rotate-90')}>▶</span>
                <span className="flex-1 text-sm font-medium text-tx-1 truncate">{track.title}</span>
                {total > 0 && <span className="text-xs text-tx-3 shrink-0">{d}/{total}</span>}
              </button>
              {openTracks[track.slug] && (
                <div className="ml-3 mt-0.5 space-y-0.5">
                  {track.topics.map((topic) => {
                    const td = doneInTopic(topic, done);
                    const tt = countTopicTasks(topic);
                    return (
                      <div key={topic.slug}>
                        <button
                          onClick={() => toggle(openTopics, setOpenTopics, topic.slug)}
                          className="w-full flex items-center gap-2 px-2 py-1 rounded hover:bg-bg-4 transition-colors text-left"
                        >
                          <span className={clsx('text-tx-3 text-xs transition-transform', openTopics[topic.slug] && 'rotate-90')}>▶</span>
                          <span className="flex-1 text-xs text-tx-2 truncate">{topic.title}</span>
                          {tt > 0 && <span className="text-xs text-tx-3 shrink-0">{td}/{tt}</span>}
                        </button>
                        {openTopics[topic.slug] && (
                          <div className="ml-3 mt-0.5 space-y-0.5">
                            {topic.units.map((unit) => {
                              const theoryOnly = unit.has_theory && unit.tasks.length === 0;
                              if (theoryOnly) {
                                return (
                                  <button
                                    key={unit.slug}
                                    onClick={() => onTheory(track, topic, unit)}
                                    className={clsx(
                                      'w-full flex items-center gap-2 px-2 py-1 rounded text-left transition-colors text-xs',
                                      activeUnitSlug === unit.slug
                                        ? 'bg-brand-subtle text-brand'
                                        : 'text-tx-2 hover:bg-bg-4 hover:text-tx-1',
                                    )}
                                  >
                                    <span className={clsx('text-xs', done[unit.slug] ? 'text-ok' : 'text-tx-3')}>
                                      {done[unit.slug] ? '✓' : '·'}
                                    </span>
                                    <span className="truncate">{unit.title}</span>
                                  </button>
                                );
                              }
                              return (
                                <div key={unit.slug}>
                                  <button
                                    onClick={() => toggle(openUnits, setOpenUnits, unit.slug)}
                                    className="w-full flex items-center gap-2 px-2 py-1 rounded hover:bg-bg-4 transition-colors text-left"
                                  >
                                    <span className={clsx('text-tx-3 text-xs transition-transform', openUnits[unit.slug] && 'rotate-90')}>▶</span>
                                    <span className="flex-1 text-xs text-tx-3 truncate">{unit.title}</span>
                                  </button>
                                  {openUnits[unit.slug] && (
                                    <div className="ml-3 mt-0.5 space-y-0.5">
                                      {unit.tasks.map((task) => (
                                        <button
                                          key={task.slug}
                                          onClick={() => onTask(track, topic, unit, task)}
                                          className={clsx(
                                            'w-full flex items-center gap-2 px-2 py-1 rounded text-left transition-colors text-xs',
                                            activeTaskSlug === task.slug
                                              ? 'bg-brand-subtle text-brand'
                                              : 'text-tx-2 hover:bg-bg-4 hover:text-tx-1',
                                          )}
                                        >
                                          <span className={clsx('text-xs', done[task.slug] ? 'text-ok' : 'text-tx-3')}>
                                            {done[task.slug] ? '✓' : '·'}
                                          </span>
                                          <span className="truncate">{task.title}</span>
                                        </button>
                                      ))}
                                    </div>
                                  )}
                                </div>
                              );
                            })}
                          </div>
                        )}
                      </div>
                    );
                  })}
                  {total > 0 && <ProgressBar value={d} max={total} className="mx-2 mt-1 mb-2" />}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </nav>
  );
}

export interface CoursePageContext {
  mainRef: React.RefObject<HTMLElement | null>;
}

export function CoursePage() {
  const { courseSlug } = useParams<{ courseSlug: string }>();
  const navigate = useNavigate();
  const mainRef = useRef<HTMLElement>(null);

  const { data: course, isLoading } = useQuery({
    queryKey: ['course', courseSlug],
    queryFn: () => api.getCourse(courseSlug!),
    enabled: !!courseSlug,
  });

  const { data: progress } = useQuery({
    queryKey: ['progress', courseSlug],
    queryFn: () => api.getProgress(courseSlug!),
    enabled: !!courseSlug,
  });

  const done = progress?.completed_tasks ?? {};
  const { taskSlug, unitSlug } = useParams<{ taskSlug?: string; unitSlug?: string }>();

  if (isLoading) return <div className="p-8 text-tx-3">Загрузка...</div>;
  if (!course) return <div className="p-8 text-err">Курс не найден</div>;

  const handleTask = (track: TrackItem, topic: TopicItem, unit: UnitItem, task: TaskItem) => {
    navigate(
      `/courses/${courseSlug}/tracks/${track.slug}/topics/${topic.slug}/units/${unit.slug}/tasks/${task.slug}`,
    );
  };

  const handleTheory = (track: TrackItem, topic: TopicItem, unit: UnitItem) => {
    navigate(
      `/courses/${courseSlug}/tracks/${track.slug}/topics/${topic.slug}/units/${unit.slug}/theory`,
    );
  };

  return (
    <div className="flex h-full overflow-hidden">
      <Sidebar
        tracks={course.tracks}
        done={done}
        activeTaskSlug={taskSlug}
        activeUnitSlug={unitSlug}
        onTask={handleTask}
        onTheory={handleTheory}
      />
      <main ref={mainRef} className="flex-1 overflow-auto">
        <Outlet context={{ mainRef } satisfies CoursePageContext} />
      </main>
    </div>
  );
}
