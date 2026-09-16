package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) registerTools() {
	// 1. get_current_task
	s.mcpServer.AddTool(
		mcp.NewTool("get_current_task",
			mcp.WithDescription("Получить метаданные и контекст текущей активной задачи студента (условие, шаблон, тесты, статус последней попытки)"),
		),
		s.handleGetCurrentTask,
	)

	// 2. set_active_task_context
	s.mcpServer.AddTool(
		mcp.NewTool("set_active_task_context",
			mcp.WithDescription("Установить активную задачу в контексте сессии студента"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса (например: 'go-interview')")),
			mcp.WithString("task_slug", mcp.Required(), mcp.Description("Слаг задачи")),
			mcp.WithString("language", mcp.Description("Опциональный язык по умолчанию (например: 'go', 'python')")),
		),
		s.handleSetActiveTaskContext,
	)

	// 3. list_courses
	s.mcpServer.AddTool(
		mcp.NewTool("list_courses",
			mcp.WithDescription("Получить список всех курсов и прогресс их прохождения"),
		),
		s.handleListCourses,
	)

	// 4. get_task_details
	s.mcpServer.AddTool(
		mcp.NewTool("get_task_details",
			mcp.WithDescription("Получить подробные сведения о задаче: условие (statement), доступные языки, лимиты времени/памяти"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса")),
			mcp.WithString("task_slug", mcp.Required(), mcp.Description("Слаг задачи")),
		),
		s.handleGetTaskDetails,
	)

	// 5. get_task_template
	s.mcpServer.AddTool(
		mcp.NewTool("get_task_template",
			mcp.WithDescription("Получить стартовый шаблон исходного кода для задачи на указанном языке"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса")),
			mcp.WithString("task_slug", mcp.Required(), mcp.Description("Слаг задачи")),
			mcp.WithString("language", mcp.Required(), mcp.Description("Язык программирования (например: go, python)")),
		),
		s.handleGetTaskTemplate,
	)

	// 6. get_task_solution
	s.mcpServer.AddTool(
		mcp.NewTool("get_task_solution",
			mcp.WithDescription("Получить авторское эталонное решение задачи на указанном языке"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса")),
			mcp.WithString("task_slug", mcp.Required(), mcp.Description("Слаг задачи")),
			mcp.WithString("language", mcp.Required(), mcp.Description("Язык программирования")),
		),
		s.handleGetTaskSolution,
	)

	// 7. get_task_tests
	s.mcpServer.AddTool(
		mcp.NewTool("get_task_tests",
			mcp.WithDescription("Получить код юнит-тестов для задачи на указанном языке"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса")),
			mcp.WithString("task_slug", mcp.Required(), mcp.Description("Слаг задачи")),
			mcp.WithString("language", mcp.Required(), mcp.Description("Язык программирования")),
		),
		s.handleGetTaskTests,
	)

	// 8. list_submissions
	s.mcpServer.AddTool(
		mcp.NewTool("list_submissions",
			mcp.WithDescription("Получить историю попыток (сабмитов) студента по задаче из базы данных"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса")),
			mcp.WithString("task_slug", mcp.Required(), mcp.Description("Слаг задачи")),
			mcp.WithNumber("limit", mcp.Description("Максимальное количество возвращаемых попыток (по умолчанию 10)")),
		),
		s.handleListSubmissions,
	)

	// 9. run_solution
	s.mcpServer.AddTool(
		mcp.NewTool("run_solution",
			mcp.WithDescription("Запустить решение задачи в песочнице CourseForge против тестов и вернуть результат выполнения"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса")),
			mcp.WithString("task_slug", mcp.Required(), mcp.Description("Слаг задачи")),
			mcp.WithString("language", mcp.Required(), mcp.Description("Язык программирования")),
			mcp.WithString("code", mcp.Required(), mcp.Description("Исходный код решения для проверки")),
			mcp.WithBoolean("save_submission", mcp.Description("Сохранять ли результат в историю сабмитов (по умолчанию true)")),
		),
		s.handleRunSolution,
	)
}

func (s *Server) handleGetCurrentTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.session == nil {
		return toolError("сессионный менеджер не настроен"), nil
	}

	active, err := s.session.GetActiveTask(ctx)
	if err != nil {
		return toolError("ошибка получения сессии: %v", err), nil
	}

	if active == nil {
		return toolJSON(map[string]any{
			"has_active_task": false,
			"message":         "Активная задача не выбрана. Используйте инструмент set_active_task_context.",
		})
	}

	details, err := s.provider.GetTaskDetails(ctx, active.CourseSlug, active.TaskSlug)
	if err != nil {
		return toolError("не удалось загрузить детали активной задачи: %v", err), nil
	}

	lang := active.Language
	if lang == "" && len(details.Languages) > 0 {
		lang = details.Languages[0]
	}

	var templateCode, testsCode string
	if lang != "" {
		_, templateCode, _ = s.provider.GetTaskTemplate(ctx, active.CourseSlug, active.TaskSlug, lang)
		_, testsCode, _ = s.provider.GetTaskTests(ctx, active.CourseSlug, active.TaskSlug, lang)
	}

	lastSub, _ := s.provider.GetLastSubmission(ctx, active.CourseSlug, active.TaskSlug)

	resp := map[string]any{
		"has_active_task": true,
		"course_slug":     active.CourseSlug,
		"task_slug":       active.TaskSlug,
		"language":        lang,
		"title":           details.Title,
		"statement":       details.Statement,
		"editorial_url":   details.EditorialURL,
		"languages":       details.Languages,
		"limits":          details.Limits,
		"is_completed":    details.IsCompleted,
		"template":        templateCode,
		"tests":           testsCode,
		"last_submission": lastSub,
	}

	return toolJSON(resp)
}

func (s *Server) handleSetActiveTaskContext(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.session == nil {
		return toolError("сессионный менеджер не настроен"), nil
	}

	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	taskSlug, err := request.RequireString("task_slug")
	if err != nil {
		return toolError("task_slug обязателен"), nil
	}
	language := request.GetString("language", "")

	details, err := s.provider.GetTaskDetails(ctx, courseSlug, taskSlug)
	if err != nil {
		return toolError("задача не найдена: %v", err), nil
	}

	active, err := s.session.SetActiveTask(ctx, courseSlug, taskSlug, language)
	if err != nil {
		return toolError("не удалось сохранить активную задачу: %v", err), nil
	}

	return toolJSON(map[string]any{
		"success": true,
		"active_task": map[string]any{
			"course_slug": active.CourseSlug,
			"task_slug":   active.TaskSlug,
			"title":       details.Title,
			"language":    active.Language,
		},
	})
}

func (s *Server) handleListCourses(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	courses, err := s.provider.ListCourses(ctx)
	if err != nil {
		return toolError("ошибка загрузки курсов: %v", err), nil
	}
	return toolJSON(map[string]any{
		"courses": courses,
	})
}

func (s *Server) handleGetTaskDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	taskSlug, err := request.RequireString("task_slug")
	if err != nil {
		return toolError("task_slug обязателен"), nil
	}

	details, err := s.provider.GetTaskDetails(ctx, courseSlug, taskSlug)
	if err != nil {
		return toolError("ошибка: %v", err), nil
	}
	return toolJSON(details)
}

func (s *Server) handleGetTaskTemplate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	taskSlug, err := request.RequireString("task_slug")
	if err != nil {
		return toolError("task_slug обязателен"), nil
	}
	language, err := request.RequireString("language")
	if err != nil {
		return toolError("language обязателен"), nil
	}

	filename, code, err := s.provider.GetTaskTemplate(ctx, courseSlug, taskSlug, language)
	if err != nil {
		return toolError("ошибка получения шаблона: %v", err), nil
	}

	return toolJSON(map[string]string{
		"filename": filename,
		"code":     code,
	})
}

func (s *Server) handleGetTaskSolution(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	taskSlug, err := request.RequireString("task_slug")
	if err != nil {
		return toolError("task_slug обязателен"), nil
	}
	language, err := request.RequireString("language")
	if err != nil {
		return toolError("language обязателен"), nil
	}

	filename, code, err := s.provider.GetTaskSolution(ctx, courseSlug, taskSlug, language)
	if err != nil {
		return toolError("ошибка получения решения: %v", err), nil
	}

	return toolJSON(map[string]string{
		"filename": filename,
		"code":     code,
	})
}

func (s *Server) handleGetTaskTests(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	taskSlug, err := request.RequireString("task_slug")
	if err != nil {
		return toolError("task_slug обязателен"), nil
	}
	language, err := request.RequireString("language")
	if err != nil {
		return toolError("language обязателен"), nil
	}

	filename, code, err := s.provider.GetTaskTests(ctx, courseSlug, taskSlug, language)
	if err != nil {
		return toolError("ошибка получения тестов: %v", err), nil
	}

	return toolJSON(map[string]string{
		"filename": filename,
		"code":     code,
	})
}

func (s *Server) handleListSubmissions(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	taskSlug, err := request.RequireString("task_slug")
	if err != nil {
		return toolError("task_slug обязателен"), nil
	}

	limit := int(request.GetFloat("limit", 10))

	subs, err := s.provider.ListSubmissions(ctx, courseSlug, taskSlug, limit)
	if err != nil {
		return toolError("ошибка получения сабмитов: %v", err), nil
	}

	return toolJSON(map[string]any{
		"submissions": subs,
	})
}

func (s *Server) handleRunSolution(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	taskSlug, err := request.RequireString("task_slug")
	if err != nil {
		return toolError("task_slug обязателен"), nil
	}
	language, err := request.RequireString("language")
	if err != nil {
		return toolError("language обязателен"), nil
	}
	code, err := request.RequireString("code")
	if err != nil {
		return toolError("code обязателен"), nil
	}
	saveSubmission := request.GetBool("save_submission", true)

	result, err := s.provider.RunSolution(ctx, RunSolutionRequest{
		CourseSlug:     courseSlug,
		TaskSlug:       taskSlug,
		Language:       language,
		Code:           code,
		SaveSubmission: saveSubmission,
	})
	if err != nil {
		return toolError("ошибка выполнения: %v", err), nil
	}

	return toolJSON(result)
}
