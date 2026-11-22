package webapp

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"finplan/internal/domain"
	"finplan/internal/storage"
)

type App struct {
	store *storage.Store
	pages map[string]*template.Template
}

func New(store *storage.Store) (*App, error) {
	funcs := template.FuncMap{
		"money": func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) },
	}
	pageNames := []string{"dashboard.html", "form.html", "about.html"}
	pages := make(map[string]*template.Template, len(pageNames))
	for _, page := range pageNames {
		parsed, err := template.New("layout.html").Funcs(funcs).ParseFiles(
			"web/templates/layout.html", filepath.Join("web/templates", page),
		)
		if err != nil {
			return nil, err
		}
		pages[page] = parsed
	}
	return &App{store: store, pages: pages}, nil
}

func (a *App) render(w http.ResponseWriter, page string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, ok := a.pages[page]
	if !ok {
		http.Error(w, "Страница не найдена", http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("ошибка HTML-шаблона %s: %v", page, err)
		http.Error(w, "Не удалось сформировать страницу", http.StatusInternalServerError)
	}
}

func (a *App) Dashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	var categoryID int64
	categoryText := q.Get("category")
	if categoryText != "" {
		parsed, err := strconv.ParseInt(categoryText, 10, 64)
		if err != nil || parsed < 1 {
			http.Error(w, "Некорректный номер категории", http.StatusBadRequest)
			return
		}
		categoryID = parsed
	}
	from, to := q.Get("from"), q.Get("to")
	if from != "" {
		if _, err := time.Parse("2006-01-02", from); err != nil {
			http.Error(w, "Некорректная дата «от»", 400)
			return
		}
	}
	if to != "" {
		if _, err := time.Parse("2006-01-02", to); err != nil {
			http.Error(w, "Некорректная дата «до»", 400)
			return
		}
	}
	if from != "" && to != "" && from > to {
		http.Error(w, "Начальная дата позже конечной", 400)
		return
	}

	expenses, total, err := a.store.List(categoryID, from, to)
	if err != nil {
		http.Error(w, "Не удалось получить список расходов", 500)
		return
	}
	categories, err := a.store.Categories()
	if err != nil {
		http.Error(w, "Не удалось получить категории", 500)
		return
	}

	a.render(w, "dashboard.html", domain.DashboardData{
		Title: "Обзор расходов", Expenses: expenses, Categories: categories,
		Total: total, Count: len(expenses), SelectedCat: categoryText, DateFrom: from, DateTo: to,
	})
}

func (a *App) NewExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", 405)
		return
	}
	cats, err := a.store.Categories()
	if err != nil {
		http.Error(w, "Не удалось загрузить категории", 500)
		return
	}
	a.render(w, "form.html", domain.FormData{Title: "Новая трата", Categories: cats,
		Expense: domain.Expense{Date: time.Now().Format("2006-01-02")}})
}

func readExpense(r *http.Request, cats *storage.Store) (domain.Expense, string) {
	if err := r.ParseForm(); err != nil {
		return domain.Expense{}, "Не удалось прочитать форму"
	}
	amount, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(r.FormValue("amount")), ",", "."), 64)
	description := strings.TrimSpace(r.FormValue("description"))
	date := r.FormValue("date")
	categoryID, catErr := strconv.ParseInt(r.FormValue("category_id"), 10, 64)
	e := domain.Expense{Amount: amount, Description: description, Date: date, CategoryID: categoryID}
	if err != nil || amount <= 0 {
		return e, "Укажи сумму больше нуля"
	}
	if description == "" {
		return e, "Добавь короткое описание траты"
	}
	if len([]rune(description)) > 100 {
		return e, "Описание должно быть не длиннее 100 символов"
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return e, "Выбери корректную дату"
	}
	if catErr != nil || categoryID < 1 || !cats.CategoryExists(categoryID) {
		return e, "Выбери категорию из списка"
	}
	return e, ""
}

func (a *App) CreateExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", 405)
		return
	}
	e, message := readExpense(r, a.store)
	if message != "" {
		cats, _ := a.store.Categories()
		a.render(w, "form.html", domain.FormData{Title: "Новая трата", Expense: e, Categories: cats, Error: message})
		return
	}
	if err := a.store.Insert(e); err != nil {
		http.Error(w, "Не удалось сохранить трату", 500)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) EditExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", 405)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	e, err := a.store.ByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	cats, err := a.store.Categories()
	if err != nil {
		http.Error(w, "Не удалось загрузить категории", 500)
		return
	}
	a.render(w, "form.html", domain.FormData{Title: "Изменить трату", Expense: e, Categories: cats, IsEdit: true})
}

func (a *App) UpdateExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", 405)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	e, message := readExpense(r, a.store)
	e.ID = id
	if message != "" {
		cats, _ := a.store.Categories()
		a.render(w, "form.html", domain.FormData{Title: "Изменить трату", Expense: e, Categories: cats, Error: message, IsEdit: true})
		return
	}
	if err := a.store.Save(e); err != nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", 405)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	if err := a.store.Remove(id); err != nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) About(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", 405)
		return
	}
	a.render(w, "about.html", map[string]string{"Title": "О проекте"})
}

func (a *App) Ping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", 405)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("pong"))
}

func (a *App) RequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
