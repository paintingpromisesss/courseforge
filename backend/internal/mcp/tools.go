package mcp

import (
	"context"
	"fmt"

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

	// 10. create_task
	s.mcpServer.AddTool(
		mcp.NewTool("create_task",
			mcp.WithDescription("Создать новую задачу внутри указанного юнита курса с шаблоном, тестами, эталонным решением и условием"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса")),
			mcp.WithString("unit_slug", mcp.Required(), mcp.Description("Слаг целевого юнита")),
			mcp.WithString("task_slug", mcp.Required(), mcp.Description("Слаг новой задачи (латиница, дефисы)")),
			mcp.WithString("title", mcp.Required(), mcp.Description("Название задачи")),
			mcp.WithString("statement", mcp.Required(), mcp.Description("Условие задачи в формате Markdown")),
			mcp.WithString("language", mcp.Required(), mcp.Description("Основной язык программирования (например: go, python)")),
			mcp.WithString("template_code", mcp.Required(), mcp.Description("Стартовый шаблон исходного кода для студента")),
			mcp.WithString("tests_code", mcp.Required(), mcp.Description("Код автотестов для проверки решения")),
			mcp.WithString("solution_code", mcp.Required(), mcp.Description("Эталонное авторское решение задачи")),
			mcp.WithNumber("difficulty", mcp.Description("Сложность задачи (от 1 до 5)")),
			mcp.WithArray("tags", mcp.WithStringItems(), mcp.Description("Теги задачи")),
			mcp.WithNumber("timeout_sec", mcp.Description("Таймаут выполнения в секундах")),
			mcp.WithNumber("memory_mb", mcp.Description("Лимит памяти в мегабайтах")),
			mcp.WithString("editorial_url", mcp.Description("Ссылка на разбор задачи")),
			mcp.WithString("video_url", mcp.Description("Ссылка на видео к задаче")),
		),
		s.handleCreateTask,
	)

	// 11. edit_task_statement
	s.mcpServer.AddTool(
		mcp.NewTool("edit_task_statement",
			mcp.WithDescription("Обновить условие (statement.md) существующей задачи курса"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса")),
			mcp.WithString("task_slug", mcp.Required(), mcp.Description("Слаг задачи")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Новый текст условия в формате Markdown")),
		),
		s.handleEditTaskStatement,
	)

	// 12. edit_task_code
	s.mcpServer.AddTool(
		mcp.NewTool("edit_task_code",
			mcp.WithDescription("Отредактировать файл кода задачи (шаблон, тесты или эталонное решение) для указанного языка"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса")),
			mcp.WithString("task_slug", mcp.Required(), mcp.Description("Слаг задачи")),
			mcp.WithString("language", mcp.Required(), mcp.Description("Язык программирования (например: go, python)")),
			mcp.WithString("file_type", mcp.Required(), mcp.Enum("template", "tests", "solution"), mcp.Description("Тип файла кода: 'template', 'tests' или 'solution'")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Новое содержимое файла кода")),
		),
		s.handleEditTaskCode,
	)

	// 13. update_task_metadata
	s.mcpServer.AddTool(
		mcp.NewTool("update_task_metadata",
			mcp.WithDescription("Обновить метаданные существующей задачи в task.yaml (название, сложность, теги, лимиты, ссылки на видео)"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса")),
			mcp.WithString("task_slug", mcp.Required(), mcp.Description("Слаг задачи")),
			mcp.WithString("title", mcp.Description("Новое название задачи")),
			mcp.WithNumber("difficulty", mcp.Description("Новая сложность (от 1 до 5)")),
			mcp.WithArray("tags", mcp.WithStringItems(), mcp.Description("Новый список тегов")),
			mcp.WithNumber("timeout_sec", mcp.Description("Таймаут выполнения в секундах")),
			mcp.WithNumber("memory_mb", mcp.Description("Лимит памяти в мегабайтах")),
			mcp.WithString("editorial_url", mcp.Description("Ссылка на разбор задачи")),
			mcp.WithString("video_url", mcp.Description("Ссылка на видео к задаче")),
		),
		s.handleUpdateTaskMetadata,
	)

	// 14. edit_unit_theory
	s.mcpServer.AddTool(
		mcp.NewTool("edit_unit_theory",
			mcp.WithDescription("Создать или обновить текст теории (theory.md) для юнита курса"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса")),
			mcp.WithString("unit_slug", mcp.Required(), mcp.Description("Слаг юнита")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Новый текст теории в формате Markdown")),
		),
		s.handleEditUnitTheory,
	)

	// 15. save_note
	s.mcpServer.AddTool(
		mcp.NewTool("save_note",
			mcp.WithDescription("Сохранить персональный конспект или заметку студента по юниту (перезапись или добавление)"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса")),
			mcp.WithString("unit_slug", mcp.Required(), mcp.Description("Слаг юнита")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Текст конспекта/заметки в формате Markdown")),
			mcp.WithString("mode", mcp.Required(), mcp.Enum("overwrite", "append"), mcp.Description("Режим записи: 'overwrite' (перезаписать) или 'append' (добавить в конец)")),
		),
		s.handleSaveNote,
	)

	// 16. get_note
	s.mcpServer.AddTool(
		mcp.NewTool("get_note",
			mcp.WithDescription("Получить персональный конспект студента по указанному юниту курса"),
			mcp.WithString("course_slug", mcp.Required(), mcp.Description("Слаг курса")),
			mcp.WithString("unit_slug", mcp.Required(), mcp.Description("Слаг юнита")),
		),
		s.handleGetNote,
	)
}

func (s *Server) handleGetCurrentTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, session := s.getDeps()
	if session == nil {
		return toolError("сессионный менеджер не настроен"), nil
	}
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

	active, err := session.GetActiveTask(ctx)
	if err != nil {
		return toolError("ошибка получения сессии: %v", err), nil
	}

	if active == nil {
		return toolJSON(map[string]any{
			"has_active_task": false,
			"message":         "Активная задача не выбрана. Используйте инструмент set_active_task_context.",
		})
	}

	details, err := provider.GetTaskDetails(ctx, active.CourseSlug, active.TaskSlug)
	if err != nil {
		return toolError("не удалось загрузить детали активной задачи: %v", err), nil
	}

	lang := active.Language
	if lang == "" && len(details.Languages) > 0 {
		lang = details.Languages[0]
	}

	var templateCode, testsCode string
	if lang != "" {
		_, templateCode, _ = provider.GetTaskTemplate(ctx, active.CourseSlug, active.TaskSlug, lang)
		_, testsCode, _ = provider.GetTaskTests(ctx, active.CourseSlug, active.TaskSlug, lang)
	}

	lastSub, _ := provider.GetLastSubmission(ctx, active.CourseSlug, active.TaskSlug)

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
	provider, session := s.getDeps()
	if session == nil {
		return toolError("сессионный менеджер не настроен"), nil
	}
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
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

	details, err := provider.GetTaskDetails(ctx, courseSlug, taskSlug)
	if err != nil {
		return toolError("задача не найдена: %v", err), nil
	}

	active, err := session.SetActiveTask(ctx, courseSlug, taskSlug, language)
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
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

	courses, err := provider.ListCourses(ctx)
	if err != nil {
		return toolError("ошибка загрузки курсов: %v", err), nil
	}
	return toolJSON(map[string]any{
		"courses": courses,
	})
}

func (s *Server) handleGetTaskDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	taskSlug, err := request.RequireString("task_slug")
	if err != nil {
		return toolError("task_slug обязателен"), nil
	}

	details, err := provider.GetTaskDetails(ctx, courseSlug, taskSlug)
	if err != nil {
		return toolError("ошибка: %v", err), nil
	}
	return toolJSON(details)
}

func (s *Server) handleGetTaskTemplate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

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

	filename, code, err := provider.GetTaskTemplate(ctx, courseSlug, taskSlug, language)
	if err != nil {
		return toolError("ошибка получения шаблона: %v", err), nil
	}

	return toolJSON(map[string]string{
		"filename": filename,
		"code":     code,
	})
}

func (s *Server) handleGetTaskSolution(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

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

	filename, code, err := provider.GetTaskSolution(ctx, courseSlug, taskSlug, language)
	if err != nil {
		return toolError("ошибка получения решения: %v", err), nil
	}

	return toolJSON(map[string]string{
		"filename": filename,
		"code":     code,
	})
}

func (s *Server) handleGetTaskTests(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

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

	filename, code, err := provider.GetTaskTests(ctx, courseSlug, taskSlug, language)
	if err != nil {
		return toolError("ошибка получения тестов: %v", err), nil
	}

	return toolJSON(map[string]string{
		"filename": filename,
		"code":     code,
	})
}

func (s *Server) handleListSubmissions(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	taskSlug, err := request.RequireString("task_slug")
	if err != nil {
		return toolError("task_slug обязателен"), nil
	}

	limit := int(request.GetFloat("limit", 10))

	subs, err := provider.ListSubmissions(ctx, courseSlug, taskSlug, limit)
	if err != nil {
		return toolError("ошибка получения сабмитов: %v", err), nil
	}

	return toolJSON(map[string]any{
		"submissions": subs,
	})
}

func (s *Server) handleRunSolution(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

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

	result, err := provider.RunSolution(ctx, RunSolutionRequest{
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

func (s *Server) handleCreateTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	unitSlug, err := request.RequireString("unit_slug")
	if err != nil {
		return toolError("unit_slug обязателен"), nil
	}
	taskSlug, err := request.RequireString("task_slug")
	if err != nil {
		return toolError("task_slug обязателен"), nil
	}
	title, err := request.RequireString("title")
	if err != nil {
		return toolError("title обязателен"), nil
	}
	statement, err := request.RequireString("statement")
	if err != nil {
		return toolError("statement обязателен"), nil
	}
	language, err := request.RequireString("language")
	if err != nil {
		return toolError("language обязателен"), nil
	}
	templateCode, err := request.RequireString("template_code")
	if err != nil {
		return toolError("template_code обязателен"), nil
	}
	testsCode, err := request.RequireString("tests_code")
	if err != nil {
		return toolError("tests_code обязателен"), nil
	}
	solutionCode, err := request.RequireString("solution_code")
	if err != nil {
		return toolError("solution_code обязателен"), nil
	}

	var tags []string
	args := request.GetArguments()
	if rawTags, ok := args["tags"].([]any); ok {
		for _, t := range rawTags {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	req := CreateTaskRequest{
		CourseSlug:   courseSlug,
		UnitSlug:     unitSlug,
		TaskSlug:     taskSlug,
		Title:        title,
		Statement:    statement,
		Language:     language,
		TemplateCode: templateCode,
		TestsCode:    testsCode,
		SolutionCode: solutionCode,
		Difficulty:   int(request.GetFloat("difficulty", 0)),
		Tags:         tags,
		TimeoutSec:   int(request.GetFloat("timeout_sec", 0)),
		MemoryMB:     int(request.GetFloat("memory_mb", 0)),
		EditorialURL: request.GetString("editorial_url", ""),
		VideoURL:     request.GetString("video_url", ""),
	}

	details, err := provider.CreateTask(ctx, req)
	if err != nil {
		return toolError("ошибка создания задачи: %v", err), nil
	}

	return toolJSON(details)
}

func (s *Server) handleEditTaskStatement(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	taskSlug, err := request.RequireString("task_slug")
	if err != nil {
		return toolError("task_slug обязателен"), nil
	}
	content, err := request.RequireString("content")
	if err != nil {
		return toolError("content обязателен"), nil
	}

	if err := provider.EditTaskStatement(ctx, courseSlug, taskSlug, content); err != nil {
		return toolError("ошибка обновления условия: %v", err), nil
	}

	return toolJSON(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Условие задачи %s успешно обновлено", taskSlug),
	})
}

func (s *Server) handleEditTaskCode(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

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
	fileType, err := request.RequireString("file_type")
	if err != nil {
		return toolError("file_type обязателен"), nil
	}
	content, err := request.RequireString("content")
	if err != nil {
		return toolError("content обязателен"), nil
	}

	filename, err := provider.EditTaskCode(ctx, courseSlug, taskSlug, language, fileType, content)
	if err != nil {
		return toolError("ошибка обновления кода задачи: %v", err), nil
	}

	return toolJSON(map[string]any{
		"success":   true,
		"file_type": fileType,
		"filename":  filename,
		"language":  language,
		"message":   fmt.Sprintf("Файл %s (%s) для задачи %s успешно обновлен", filename, fileType, taskSlug),
	})
}

func (s *Server) handleUpdateTaskMetadata(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	taskSlug, err := request.RequireString("task_slug")
	if err != nil {
		return toolError("task_slug обязателен"), nil
	}

	req := UpdateTaskMetadataRequest{
		CourseSlug: courseSlug,
		TaskSlug:   taskSlug,
	}

	args := request.GetArguments()
	if _, ok := args["title"]; ok {
		t := request.GetString("title", "")
		req.Title = &t
	}
	if _, ok := args["difficulty"]; ok {
		d := int(request.GetFloat("difficulty", 0))
		req.Difficulty = &d
	}
	if rawTags, ok := args["tags"].([]any); ok {
		var tags []string
		for _, t := range rawTags {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
		req.Tags = &tags
	}
	if _, ok := args["timeout_sec"]; ok {
		sec := int(request.GetFloat("timeout_sec", 0))
		req.TimeoutSec = &sec
	}
	if _, ok := args["memory_mb"]; ok {
		mb := int(request.GetFloat("memory_mb", 0))
		req.MemoryMB = &mb
	}
	if _, ok := args["editorial_url"]; ok {
		u := request.GetString("editorial_url", "")
		req.EditorialURL = &u
	}
	if _, ok := args["video_url"]; ok {
		u := request.GetString("video_url", "")
		req.VideoURL = &u
	}

	details, err := provider.UpdateTaskMetadata(ctx, req)
	if err != nil {
		return toolError("ошибка обновления метаданных: %v", err), nil
	}

	return toolJSON(details)
}

func (s *Server) handleEditUnitTheory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	unitSlug, err := request.RequireString("unit_slug")
	if err != nil {
		return toolError("unit_slug обязателен"), nil
	}
	content, err := request.RequireString("content")
	if err != nil {
		return toolError("content обязателен"), nil
	}

	if err := provider.EditUnitTheory(ctx, courseSlug, unitSlug, content); err != nil {
		return toolError("ошибка обновления теории юнита: %v", err), nil
	}

	return toolJSON(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Теория для юнита %s успешно сохранена", unitSlug),
	})
}

func (s *Server) handleSaveNote(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	unitSlug, err := request.RequireString("unit_slug")
	if err != nil {
		return toolError("unit_slug обязателен"), nil
	}
	content, err := request.RequireString("content")
	if err != nil {
		return toolError("content обязателен"), nil
	}
	mode, err := request.RequireString("mode")
	if err != nil {
		return toolError("mode обязателен ('overwrite' или 'append')"), nil
	}

	if err := provider.SaveNote(ctx, courseSlug, unitSlug, content, mode); err != nil {
		return toolError("ошибка сохранения конспекта: %v", err), nil
	}

	return toolJSON(map[string]any{
		"success": true,
		"mode":    mode,
		"message": fmt.Sprintf("Конспект по юниту %s успешно сохранен (%s)", unitSlug, mode),
	})
}

func (s *Server) handleGetNote(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return toolError("провайдер CourseForge не инициализирован"), nil
	}

	courseSlug, err := request.RequireString("course_slug")
	if err != nil {
		return toolError("course_slug обязателен"), nil
	}
	unitSlug, err := request.RequireString("unit_slug")
	if err != nil {
		return toolError("unit_slug обязателен"), nil
	}

	content, hasNote, err := provider.GetNote(ctx, courseSlug, unitSlug)
	if err != nil {
		return toolError("ошибка чтения конспекта: %v", err), nil
	}

	return toolJSON(map[string]any{
		"has_note":    hasNote,
		"course_slug": courseSlug,
		"unit_slug":   unitSlug,
		"content":     content,
	})
}
