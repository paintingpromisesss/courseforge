---
name: next
description: Suggest what to study next in CourseForge based on progress. Use when the user asks what to do next, which task to pick, or wants a study plan.
---

Recommend the next task from real progress, not a guess.

1. Call `list_courses` to see courses, tracks, tasks and completion status.
2. Call `get_current_task` to see what is open now; finish it before suggesting a new one unless the user is stuck.
3. Pick 1-3 candidates: the first unfinished task in the course the user is working on, or a retry of a task with many failed submissions (`list_submissions`).
4. For each, give the task name, why it fits and a rough difficulty from `get_task_details`.
5. When the user chooses, call `set_active_task_context` so the UI and agent share the same task.
