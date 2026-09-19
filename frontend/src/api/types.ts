export interface CourseItem {
  slug: string;
  title: string;
  description: string;
  language: string;
  catalog_slug?: string;
  theory_count: number;
  task_count: number;
  done_count: number;
  theory_done_count: number;
}

export interface CatalogItem {
  slug: string;
  title: string;
  description: string;
  courses: CourseItem[];
}

export interface TaskItem {
  slug: string;
  title: string;
  difficulty?: number;
  tags?: string[];
  languages: string[];
  editorial_url?: string;
}

export interface UnitItem {
  slug: string;
  title: string;
  has_theory: boolean;
  video_url?: string;
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

export interface LangDriver {
  run_cmd: string[];
  test_cmd: string[];
  ext: string;
  test_ext: string;
}

export interface RunnerStatus {
  status: 'ok' | 'broken' | 'missing';
  binary: string;
  path?: string;
  version?: string;
  message?: string;
  platform: 'linux' | 'darwin' | 'windows' | string;
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

export interface CreateSubmissionReq {
  course_slug: string;
  task_slug: string;
  language: string;
  code: string;
}

export interface AIConfig {
  provider: 'openai' | 'anthropic';
  base_url: string;
  api_key: string;
  model: string;
  system_prompt: string;
  enabled: boolean;
  provider_keys?: Record<string, string>;
}

export interface AIMessage {
  role: 'user' | 'assistant' | 'system';
  content: string;
}

export interface AIModelItem {
  id: string;
  available: boolean;
}

export interface MCPConfig {
  enabled: boolean;
  transport: 'stdio' | 'sse';
  host: string;
  port: number;
  courses_dir: string;
  data_dir: string;
}

export interface MCPStatusResponse extends MCPConfig {
  binary_path: string;
  command?: string;
  args?: string[];
  sse_url?: string;
  platform: string;
  tools_count: number;
  available: boolean;
}

export interface MCPActiveTask {
  course_slug?: string;
  task_slug?: string;
  language?: string;
  updated_at?: string;
}



