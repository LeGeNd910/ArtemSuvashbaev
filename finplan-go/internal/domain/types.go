package domain

type Category struct {
	ID   int64
	Name string
	Mark string
}

type Expense struct {
	ID           int64
	Amount       float64
	Description  string
	Date         string
	CategoryID   int64
	CategoryName string
	CategoryMark string
}

type DashboardData struct {
	Expenses    []Expense
	Categories  []Category
	Total       float64
	Count       int
	SelectedCat string
	DateFrom    string
	DateTo      string
	Title       string
}

type FormData struct {
	Title      string
	Expense    Expense
	Categories []Category
	Error      string
	IsEdit     bool
}
