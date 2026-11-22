package storage

import (
	"database/sql"
	"fmt"
	"strings"

	"finplan/internal/domain"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(filename string) (*Store, error) {
	db, err := sql.Open("sqlite", filename)
	if err != nil {
		return nil, fmt.Errorf("открытие SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("проверка SQLite: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Prepare() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		mark TEXT NOT NULL DEFAULT '•'
	)`)
	if err != nil {
		return fmt.Errorf("создание таблицы категорий: %w", err)
	}
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS expenses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		amount REAL NOT NULL CHECK(amount > 0),
		description TEXT NOT NULL,
		spent_on TEXT NOT NULL,
		category_id INTEGER NOT NULL,
		FOREIGN KEY(category_id) REFERENCES categories(id)
	)`)
	if err != nil {
		return fmt.Errorf("создание таблицы расходов: %w", err)
	}

	defaults := []domain.Category{
		{Name: "Продукты", Mark: "🥑"},
		{Name: "Транспорт", Mark: "🚇"},
		{Name: "Дом и счета", Mark: "🏠"},
		{Name: "Учёба", Mark: "📚"},
		{Name: "Досуг", Mark: "🎮"},
		{Name: "Покупки", Mark: "🛍️"},
		{Name: "Другое", Mark: "✦"},
	}
	for _, c := range defaults {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO categories(name, mark) VALUES(?, ?)`, c.Name, c.Mark); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Categories() ([]domain.Category, error) {
	rows, err := s.db.Query(`SELECT id, name, mark FROM categories ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Mark); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (s *Store) CategoryExists(id int64) bool {
	var found int
	err := s.db.QueryRow(`SELECT 1 FROM categories WHERE id = ?`, id).Scan(&found)
	return err == nil
}

func (s *Store) List(categoryID int64, from, to string) ([]domain.Expense, float64, error) {
	query := `SELECT e.id, e.amount, e.description, e.spent_on, e.category_id, c.name, c.mark
		FROM expenses e JOIN categories c ON c.id = e.category_id`
	var where []string
	var args []any
	if categoryID > 0 {
		where = append(where, "e.category_id = ?")
		args = append(args, categoryID)
	}
	if from != "" {
		where = append(where, "e.spent_on >= ?")
		args = append(args, from)
	}
	if to != "" {
		where = append(where, "e.spent_on <= ?")
		args = append(args, to)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += ` ORDER BY e.spent_on DESC, e.id DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []domain.Expense
	var total float64
	for rows.Next() {
		var e domain.Expense
		if err := rows.Scan(&e.ID, &e.Amount, &e.Description, &e.Date, &e.CategoryID, &e.CategoryName, &e.CategoryMark); err != nil {
			return nil, 0, err
		}
		total += e.Amount
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (s *Store) ByID(id int64) (domain.Expense, error) {
	var e domain.Expense
	err := s.db.QueryRow(`SELECT e.id, e.amount, e.description, e.spent_on, e.category_id, c.name, c.mark
		FROM expenses e JOIN categories c ON c.id=e.category_id WHERE e.id=?`, id).
		Scan(&e.ID, &e.Amount, &e.Description, &e.Date, &e.CategoryID, &e.CategoryName, &e.CategoryMark)
	return e, err
}

func (s *Store) Insert(e domain.Expense) error {
	_, err := s.db.Exec(`INSERT INTO expenses(amount, description, spent_on, category_id) VALUES(?,?,?,?)`,
		e.Amount, e.Description, e.Date, e.CategoryID)
	return err
}

func (s *Store) Save(e domain.Expense) error {
	res, err := s.db.Exec(`UPDATE expenses SET amount=?, description=?, spent_on=?, category_id=? WHERE id=?`,
		e.Amount, e.Description, e.Date, e.CategoryID, e.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) Remove(id int64) error {
	res, err := s.db.Exec(`DELETE FROM expenses WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return sql.ErrNoRows
	}
	return err
}
