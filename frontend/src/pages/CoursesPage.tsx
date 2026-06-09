import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { motion } from 'framer-motion';
import { api } from '../api/client';
import { Badge } from '../components/ui/Badge';

export function CoursesPage() {
  const { data: courses, isLoading, error } = useQuery({
    queryKey: ['courses'],
    queryFn: api.listCourses,
  });

  if (isLoading) return <div className="p-8 text-tx-3">Загрузка...</div>;
  if (error) return <div className="p-8 text-err">Ошибка загрузки курсов</div>;

  return (
    <div className="overflow-auto h-full">
      <div className="max-w-5xl mx-auto px-6 py-12">
        <h1 className="text-2xl font-semibold text-tx-1 mb-8">Курсы</h1>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {courses?.map((course, i) => (
            <motion.div
              key={course.slug}
              initial={{ opacity: 0, y: 16 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: i * 0.05, duration: 0.25 }}
            >
              <Link
                to={`/courses/${course.slug}`}
                className="block bg-bg-2 border border-bdr rounded-xl p-5 hover:border-bdr-e hover:shadow-md hover:-translate-y-px transition-all group"
              >
                <div className="flex items-start justify-between gap-2 mb-2">
                  <h2 className="text-tx-1 font-medium text-sm leading-snug">
                    {course.title}
                  </h2>
                  <Badge variant="brand">{course.language}</Badge>
                </div>
                {course.description && (
                  <p className="text-tx-3 text-xs line-clamp-2">{course.description}</p>
                )}
              </Link>
            </motion.div>
          ))}
        </div>
      </div>
    </div>
  );
}
