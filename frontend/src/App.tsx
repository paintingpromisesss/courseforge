import { useState } from 'react';
import { Routes, Route, Navigate, Link, Outlet, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { CoursesPage } from './pages/CoursesPage';
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
  const { courseSlug } = useParams<{ courseSlug?: string }>();
  const [settingsOpen, setSettingsOpen] = useState(false);

  const { data: course } = useQuery({
    queryKey: ['course', courseSlug],
    queryFn: () => api.getCourse(courseSlug!),
    enabled: !!courseSlug,
  });

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-bg-1">
      <header className="flex items-center gap-3 px-4 h-11 border-b border-bdr bg-bg-2 shrink-0">
        <Link to="/" className="text-tx-3 hover:text-tx-1 text-sm transition-colors shrink-0">
          Курсы
        </Link>
        {course && (
          <>
            <span className="text-bdr">│</span>
            <span className="text-tx-2 text-sm truncate">{course.title}</span>
          </>
        )}
        <button
          onClick={() => setSettingsOpen(true)}
          className="ml-auto text-tx-3 hover:text-tx-1 transition-colors p-1 rounded hover:bg-bg-4"
        >
          <GearIcon />
        </button>
      </header>
      <div className="flex-1 overflow-hidden">
        <Outlet />
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
