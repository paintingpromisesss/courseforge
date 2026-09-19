---
name: review
description: Review the student's code for a CourseForge task without running or fixing it for them. Use when the user asks "review my solution", "is this good", or wants feedback on style, complexity or edge cases.
---

Give a code review, not a rewrite.

1. Get the task: `get_current_task`, then `get_task_details` and `get_task_tests`.
2. Read the student's code from the conversation or open file. Do not call `get_task_solution`.
3. Comment on: correctness against the statement, edge cases the tests cover or miss, complexity, naming and idiomatic use of the language.
4. Prioritize: list the 1-3 most important issues first, minor style points last.
5. Point at lines and explain why; suggest the direction of a fix, not the fixed code, unless asked.
6. If you are unsure the code passes, offer `run_solution` instead of guessing.
