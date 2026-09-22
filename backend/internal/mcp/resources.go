package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) registerResources() {
	// 1. Static active task resource
	s.mcpServer.AddResource(
		mcp.NewResource(
			"courseforge://active-task",
			"Active Task Context",
			mcp.WithResourceDescription("Текущая активная задача студента в сессии"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleReadActiveTaskResource,
	)

	// 2. Static courses list resource
	s.mcpServer.AddResource(
		mcp.NewResource(
			"courseforge://courses",
			"Courses Catalog",
			mcp.WithResourceDescription("Список всех доступных курсов и прогресс их прохождения"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleReadCoursesResource,
	)

	// 3. Dynamic resource template for task statement
	s.mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"courseforge://courses/{course_slug}/tasks/{task_slug}/statement",
			"Task Statement",
			mcp.WithTemplateDescription("Текст условия задачи в формате Markdown"),
			mcp.WithTemplateMIMEType("text/markdown"),
		),
		s.handleReadTaskStatementResource,
	)

	// 4. Dynamic resource template for task template
	s.mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"courseforge://courses/{course_slug}/tasks/{task_slug}/template/{language}",
			"Task Code Template",
			mcp.WithTemplateDescription("Шаблон кода задачи для указанного языка"),
			mcp.WithTemplateMIMEType("text/plain"),
		),
		s.handleReadTaskTemplateResource,
	)

	// 5. Dynamic resource template for task solution
	s.mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"courseforge://courses/{course_slug}/tasks/{task_slug}/solution/{language}",
			"Task Solution",
			mcp.WithTemplateDescription("Эталонное авторское решение задачи"),
			mcp.WithTemplateMIMEType("text/plain"),
		),
		s.handleReadTaskSolutionResource,
	)
}

func (s *Server) handleReadActiveTaskResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	_, session := s.getDeps()
	if session == nil {
		return nil, fmt.Errorf("сессионный менеджер не настроен")
	}

	active, err := session.GetActiveTask(ctx)
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(map[string]any{
		"active_task": active,
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleReadCoursesResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return nil, fmt.Errorf("провайдер CourseForge не инициализирован")
	}

	courses, err := provider.ListCourses(ctx)
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(courses, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleReadTaskStatementResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return nil, fmt.Errorf("провайдер CourseForge не инициализирован")
	}

	// URI shape: courseforge://courses/{course_slug}/tasks/{task_slug}/statement
	parts := strings.Split(strings.TrimPrefix(request.Params.URI, "courseforge://courses/"), "/")
	if len(parts) < 4 || parts[1] != "tasks" || parts[3] != "statement" {
		return nil, fmt.Errorf("invalid statement URI: %s", request.Params.URI)
	}

	courseSlug := parts[0]
	taskSlug := parts[2]

	statement, err := provider.GetTaskStatement(ctx, courseSlug, taskSlug)
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/markdown",
			Text:     statement,
		},
	}, nil
}

func (s *Server) handleReadTaskTemplateResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return nil, fmt.Errorf("провайдер CourseForge не инициализирован")
	}

	// URI shape: courseforge://courses/{course_slug}/tasks/{task_slug}/template/{language}
	parts := strings.Split(strings.TrimPrefix(request.Params.URI, "courseforge://courses/"), "/")
	if len(parts) < 5 || parts[1] != "tasks" || parts[3] != "template" {
		return nil, fmt.Errorf("invalid template URI: %s", request.Params.URI)
	}

	courseSlug := parts[0]
	taskSlug := parts[2]
	language := parts[4]

	_, code, err := provider.GetTaskTemplate(ctx, courseSlug, taskSlug, language)
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/plain",
			Text:     code,
		},
	}, nil
}

func (s *Server) handleReadTaskSolutionResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	provider, _ := s.getDeps()
	if provider == nil {
		return nil, fmt.Errorf("провайдер CourseForge не инициализирован")
	}

	// URI shape: courseforge://courses/{course_slug}/tasks/{task_slug}/solution/{language}
	parts := strings.Split(strings.TrimPrefix(request.Params.URI, "courseforge://courses/"), "/")
	if len(parts) < 5 || parts[1] != "tasks" || parts[3] != "solution" {
		return nil, fmt.Errorf("invalid solution URI: %s", request.Params.URI)
	}

	courseSlug := parts[0]
	taskSlug := parts[2]
	language := parts[4]

	_, code, err := provider.GetTaskSolution(ctx, courseSlug, taskSlug, language)
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/plain",
			Text:     code,
		},
	}, nil
}
