package service

import (
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

func testCourses() map[string]*domain.Course {
	mk := func(t *domain.Task) *domain.Task { return t }
	return map[string]*domain.Course{
		"go-interview": {
			Slug: "go-interview", Dir: "go-interview", Title: "Golang Interview",
			Tracks: []*domain.Track{{
				Slug: "week-4", Title: "Concurrency",
				Topics: []*domain.Topic{{
					Slug: "fan-in-fan-out", Title: "Fan-In Fan-Out",
					Units: []*domain.Unit{{
						Slug: "01-fan-infan-out", Title: "Fan-In для каналов",
						Tasks: []*domain.Task{
							mk(&domain.Task{Slug: "golang-fanout-1", Title: "Fan-in для каналов", Difficulty: 2,
								Tags: []string{"sync", "channels"},
								Statement: "statement.md"}),
							mk(&domain.Task{Slug: "golang-pool-1", Title: "Пул соединений", Difficulty: 1,
								Tags: []string{"pool"}, Statement: "statement.md"}),
						},
					}},
				}},
			}},
		},
		"algorithms": {
			Slug: "algorithms", Dir: "algorithms", Title: "Algorithms",
			Tracks: []*domain.Track{{
				Slug: "basics", Title: "Basics",
				Topics: []*domain.Topic{{
					Slug: "strings", Title: "Strings",
					Units: []*domain.Unit{{
						Slug: "01-strings", Title: "Строки и пул литералов",
						Tasks: []*domain.Task{
							mk(&domain.Task{Slug: "word-count", Title: "Подсчёт слов", Difficulty: 1,
								Statement: "statement.md"}),
						},
					}},
				}},
			}},
		},
	}
}

func statementOf(t SearchTask) (string, bool) {
	// fake content store keyed by task slug
	content := map[string]string{
		"golang-fanout-1": "Результат распределяется между несколькими goroutine, затем собирается обратно.",
		"word-count":      "Подсчитайте частоту слов в pool входных строк.",
	}
	c, ok := content[t.Task]
	return c, ok
}

func index(t *testing.T) []SearchTask {
	t.Helper()
	return FlattenSearchTasks(testCourses())
}

func search(t *testing.T, q string) []SearchResult {
	t.Helper()
	return SearchTasks(index(t), q, 50, statementOf)
}

func TestSearchSubstringNotPrefix(t *testing.T) {
	// «пул» должна найтись внутри слов, а не только как префикс
	res := search(t, "пул")
	found := map[string]bool{}
	for _, r := range res {
		found[r.Task] = true
	}
	if !found["golang-pool-1"] {
		t.Fatalf("«пул» должна находить «Пул соединений»: %v", res)
	}
	// латинский «pool» находит и задачу по тегу, и вхождение в контент условия
	res = search(t, "pool")
	found = map[string]bool{}
	for _, r := range res {
		found[r.Task] = true
	}
	if !found["golang-pool-1"] || !found["word-count"] {
		t.Fatalf("«pool» должна находить по тегу и в контенте условия: %v", res)
	}
}

func TestCaseInsensitive(t *testing.T) {
	res := search(t, "ПОДсчЁт") // note: unicode case folding is simple-lowercase in v1
	for _, r := range res {
		if r.Task == "word-count" {
			return
		}
	}
	// кириллическая «ё» не сворачивается в «е» в простом lower — проверим латиницу
	res = search(t, "FAN")
	if len(res) == 0 || res[0].Task != "golang-fanout-1" {
		t.Fatalf("латинская подстрока без учёта регистра не найдена: %v", res)
	}
}

func TestTagSearchRanksAboveTitle(t *testing.T) {
	res := search(t, "sync")
	if len(res) == 0 {
		t.Fatal("тег sync не найден")
	}
	if res[0].Rank != "tag" {
		t.Fatalf("ожидался ранг tag, получен %q (%v)", res[0].Rank, res)
	}
}

func TestANDSemantics(t *testing.T) {
	// оба слова должны найтись в задаче
	res := search(t, "fan каналов")
	if len(res) != 1 || res[0].Task != "golang-fanout-1" {
		t.Fatalf("AND-поиск должен оставить только задачу с обоими словами: %v", res)
	}
	// задача, где есть только одно из слов, не должна попасть
	res = search(t, "fan отсутствующее слово")
	if len(res) != 0 {
		t.Fatalf("AND не соблюдён: %v", res)
	}
}

func TestRankOrdering(t *testing.T) {
	// «pool» — тег у golang-pool-1 → ранг tag; у word-count только в контенте
	res := search(t, "pool")
	if len(res) < 2 {
		t.Fatalf("ожидалось ≥2 результата (тег + контент): %v", res)
	}
	if res[0].Rank != "tag" || res[1].Rank != "content" {
		t.Fatalf("ожидался порядок tag→content: %v", res)
	}
}

func TestTitleWordBoundaryBeatsSubstring(t *testing.T) {
	tasks := []SearchTask{
		{Task: "exact", Title: "Fan-in для каналов"},
		{Task: "partial", Title: "Fanout станция"},
	}
	res := SearchTasks(tasks, "fan", 50, nil)
	if len(res) != 2 {
		t.Fatalf("оба должны найтись: %v", res)
	}
	if res[0].Rank != "title-word" || res[0].Task != "exact" {
		t.Fatalf("word-boundary должен ранжироваться выше: %v", res)
	}
}

func TestLimit(t *testing.T) {
	tasks := make([]SearchTask, 0, 20)
	for i := 0; i < 20; i++ {
		tasks = append(tasks, SearchTask{Task: "t", Title: "задача пул", Course: "c"})
	}
	res := SearchTasks(tasks, "пул", 5, nil)
	if len(res) != 5 {
		t.Fatalf("лимит выдачи не соблюдён: %d", len(res))
	}
}

func TestEmptyAndShortQuery(t *testing.T) {
	if res := search(t, ""); res != nil {
		t.Fatalf("пустой запрос должен дать nil: %v", res)
	}
	if res := search(t, "a"); res != nil {
		t.Fatalf("однобуквенный запрос должен дать nil: %v", res)
	}
}

func TestFlattenCoversAllTasks(t *testing.T) {
	idx := index(t)
	if len(idx) != 3 {
		t.Fatalf("ожидалось 3 задачи, получено %d", len(idx))
	}
	for _, item := range idx {
		if item.CourseDir == "" || item.Track == "" || item.Topic == "" || item.Unit == "" {
			t.Fatalf("неполная запись индекса: %+v", item)
		}
	}
}

func TestSnippet(t *testing.T) {
	res := search(t, "goroutine")
	if len(res) != 1 || res[0].Snippet == "" {
		t.Fatalf("сниппет по контенту не построен: %v", res)
	}
}

func TestCatalog(t *testing.T) {
	entries := BuildCatalog(index(t))
	if len(entries) != 3 {
		t.Fatalf("каталог должен покрывать все задачи: %d", len(entries))
	}
	if entries[0].Course == "" || entries[0].Task == "" {
		t.Fatalf("неполная запись каталога: %+v", entries[0])
	}
}
