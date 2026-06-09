export interface CourseItem {
  slug: string;
  title: string;
  description: string;
  language: string;
}

export interface TaskItem {
  slug: string;
  title: string;
  languages: string[];
}

export interface UnitItem {
  slug: string;
  title: string;
  has_theory: boolean;
  tasks: TaskItem[];
}

export interface TopicItem {
  slug: string;
  title: string;
  description: string;
  units: UnitItem[];
}

export interface TrackItem {
  slug: string;
  title: string;
  description: string;
  topics: TopicItem[];
}

export interface CourseDetail extends CourseItem {
  tracks: TrackItem[];
}

export interface Progress {
  course_slug: string;
  completed_tasks: Record<string, boolean>;
}

export interface RunResp {
  stdout: string;
  stderr: string;
  exit_code: number;
  duration_ms: number;
  timed_out: boolean;
}

export interface LangDriver {
  run_cmd: string[];
  test_cmd: string[];
  ext: string;
  test_ext: string;
}

export interface InstallReq {
  lang: string;
  pkg?: string;
  url?: string;
  bin_path: string;
  run_cmd: string[];
  test_cmd: string[];
  ext: string;
  test_ext: string;
}

export interface InstallStatus {
  status: 'downloading' | 'extracting' | 'installing' | 'done' | 'error';
  progress: number;
  message?: string;
}

export interface Submission {
  id: number;
  course_slug: string;
  task_slug: string;
  language: string;
  code: string;
  stdout: string;
  stderr: string;
  exit_code: number;
  passed_tests: number;
  total_tests: number;
  duration_ms: number;
  timed_out: boolean;
  created_at: string;
}
