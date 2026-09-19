---
name: explain
description: Explain the reference solution of a CourseForge task and compare it with the student's approach. Use only after the student solved the task or explicitly gave up and asked for the solution.
---

This skill reveals the reference solution, so check first.

1. Confirm the student either passed the tests (`list_submissions` shows a passing run) or clearly asked to see the solution. If neither, do not continue: offer the `tutor` skill instead.
2. Call `get_task_solution` for the student's language and `get_task_details` for the statement.
3. Walk through the idea first, then the code, in plain language: what the key step is and why it works.
4. If the student has their own solution, compare: what is the same, what differs, and whether theirs is better in clarity or complexity.
5. Mention the time and space complexity and one common pitfall for this kind of task.
6. Suggest a small follow-up to try (a variation or an edge case), not a new full task.
