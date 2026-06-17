import { useState } from 'react';
import { Routes, Route, Navigate, Link, useOutlet, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { motion, AnimatePresence } from 'framer-motion';
import clsx from 'clsx';
import { CoursesPage } from './pages/CoursesPage';
import { CatalogPage } from './pages/CatalogPage';
import { CoursePage } from './pages/CoursePage';
import { TaskPage } from './pages/TaskPage';
import { TheoryPage } from './pages/TheoryPage';
import { SettingsPanel } from './components/SettingsPanel';
import { api } from './api/client';

function GearIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06-.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  );
}

function AppLayout() {
  const { courseSlug, catalogSlug } = useParams<{ courseSlug?: string; catalogSlug?: string }>();
  const [settingsOpen, setSettingsOpen] = useState(false);

  const { data: course } = useQuery({
    queryKey: ['course', courseSlug],
    queryFn: () => api.getCourse(courseSlug!),
    enabled: !!courseSlug,
  });

  const { data: catalogs } = useQuery({
    queryKey: ['catalogs'],
    queryFn: api.listCatalogs,
    enabled: !!catalogSlug || !!courseSlug,
  });
  const catalog = catalogSlug ? catalogs?.find(c => c.slug === catalogSlug) : undefined;
  // a course page knows its parent catalog by membership, not from the course payload
  const parentCatalog = courseSlug ? catalogs?.find(c => c.courses.some(x => x.slug === courseSlug)) : undefined;

  const outlet = useOutlet();
  const routeKey = courseSlug ? `course:${courseSlug}` : catalogSlug ? `catalog:${catalogSlug}` : 'home';

  const crumbs: { label: string; to: string }[] = [{ label: 'Главная', to: '/' }];
  if (catalog) crumbs.push({ label: catalog.title, to: `/catalogs/${catalog.slug}` });
  if (course) {
    if (parentCatalog) crumbs.push({ label: parentCatalog.title, to: `/catalogs/${parentCatalog.slug}` });
    crumbs.push({ label: course.title, to: `/courses/${courseSlug}` });
  }

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-bg-1">
      <header className="flex items-center gap-2 px-4 h-11 border-b border-bdr bg-bg-2 shrink-0">
        {crumbs.map((c, i) => {
          const last = i === crumbs.length - 1;
          return (
            <span key={c.to} className="flex items-center gap-2 min-w-0">
              {i > 0 && <span className="text-bdr shrink-0">›</span>}
              <Link
                to={c.to}
                className={clsx(
                  'text-sm transition-colors truncate',
                  last ? 'text-tx-1' : 'text-tx-3 hover:text-tx-1',
                )}
              >
                {c.label}
              </Link>
            </span>
          );
        })}
        <button
          onClick={() => setSettingsOpen(true)}
          className="ml-auto text-tx-3 hover:text-tx-1 transition-colors p-1 rounded hover:bg-bg-4"
        >
          <GearIcon />
        </button>
      </header>
      <div className="flex-1 overflow-hidden">
        {/* Coarse key: stable across task navigation inside a course, so entering/
            leaving a course animates but selecting tasks within it doesn't.
            useOutlet captures the route element so the exiting copy is frozen. */}
        <AnimatePresence mode="wait">
          <motion.div key={routeKey} className="h-full">
            {outlet}
          </motion.div>
        </AnimatePresence>
      </div>
      <SettingsPanel open={settingsOpen} onClose={() => setSettingsOpen(false)} />
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex items-center justify-center h-full text-tx-3 text-sm">
      Выбери задачу в боковом меню
    </div>
  );
}

export default function App() {
  return (
    <Routes>
      <Route element={<AppLayout />}>
        <Route path="/" element={<CoursesPage />} />
        <Route path="/catalogs/:catalogSlug" element={<CatalogPage />} />
        <Route path="/courses/:courseSlug" element={<CoursePage />}>
          <Route
            path="tracks/:trackSlug/topics/:topicSlug/units/:unitSlug/tasks/:taskSlug"
            element={<TaskPage />}
          />
          <Route
            path="tracks/:trackSlug/topics/:topicSlug/units/:unitSlug/theory"
            element={<TheoryPage />}
          />
          <Route index element={<EmptyState />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
