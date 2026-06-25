package pagination

import "strconv"

const (
	defaultPage    = 1
	defaultPerPage = 10
	maxPerPage     = 100
)

type Params struct {
	Page    int
	PerPage int
}

type Meta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

func Parse(pageStr, perPageStr string) Params {
	page := defaultPage
	if n, err := strconv.Atoi(pageStr); err == nil && n > 0 {
		page = n
	}
	perPage := defaultPerPage
	if n, err := strconv.Atoi(perPageStr); err == nil && n > 0 {
		perPage = n
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	return Params{Page: page, PerPage: perPage}
}

func (p Params) Offset() int { return (p.Page - 1) * p.PerPage }

func (p Params) Limit() int { return p.PerPage }

func NewMeta(p Params, total int) Meta {
	return Meta{Page: p.Page, PerPage: p.PerPage, Total: total}
}
