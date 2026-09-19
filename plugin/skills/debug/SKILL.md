---
name: debug
description: Diagnose why a CourseForge solution fails its tests. Use when the user says tests fail, gets a compile error, timeout or wrong answer, or asks to run their solution.
---

Find the cause from real output, then guide the student to the fix.

1. Get the task: `get_current_task`, then `get_task_tests`.
2. Run the student's code with `run_solution` (pass `course_slug`, `task_slug`, `language`, `code`). Prefer the code the student shows; do not invent a fix and run that.
3. If unclear which attempt is meant, check `list_submissions` and compare the last failing run with the current code.
4. Classify the failure: compile error, wrong answer, runtime error, timeout or memory limit. Quote the relevant lines of the output.
5. Explain what the failing test expects versus what the code did, and name the likely cause. Give one hint, not the full fix.
6. After the student changes the code, rerun `run_solution` to confirm.
