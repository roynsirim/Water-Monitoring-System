// Package pagination provides HTTP pagination utilities.
package pagination

import (
	"net/http"
	"strconv"
)

// Params contains parsed pagination parameters.
type Params struct {
	Page     int
	PageSize int
	Offset   int
}

// Defaults contains default pagination values.
type Defaults struct {
	Page     int
	PageSize int
	MaxSize  int
}

// DefaultParams returns standard pagination defaults.
func DefaultParams() Defaults {
	return Defaults{
		Page:     1,
		PageSize: 20,
		MaxSize:  100,
	}
}

// ParseFromRequest extracts pagination from query params.
func ParseFromRequest(r *http.Request, defaults Defaults) Params {
	page := parseIntQuery(r, "page", defaults.Page)
	pageSize := parseIntQuery(r, "page_size", defaults.PageSize)

	// Also support "limit" and "offset" style
	if limit := parseIntQuery(r, "limit", 0); limit > 0 {
		pageSize = limit
	}

	// Normalize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaults.PageSize
	}
	if pageSize > defaults.MaxSize {
		pageSize = defaults.MaxSize
	}

	offset := (page - 1) * pageSize

	return Params{
		Page:     page,
		PageSize: pageSize,
		Offset:   offset,
	}
}

// Result contains paginated response metadata.
type Result[T any] struct {
	Data       []T  `json:"data"`
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	TotalCount int  `json:"total_count"`
	TotalPages int  `json:"total_pages"`
	HasMore    bool `json:"has_more"`
}

// NewResult creates a paginated result from a slice.
func NewResult[T any](items []T, totalCount int, params Params) Result[T] {
	if items == nil {
		items = []T{}
	}
	totalPages := (totalCount + params.PageSize - 1) / params.PageSize
	if totalPages < 1 {
		totalPages = 1
	}
	return Result[T]{
		Data:       items,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalCount: totalCount,
		TotalPages: totalPages,
		HasMore:    params.Page < totalPages,
	}
}

// Paginate applies pagination to a slice.
func Paginate[T any](items []T, params Params) []T {
	total := len(items)
	start := params.Offset
	end := start + params.PageSize

	if start > total {
		return []T{}
	}
	if end > total {
		end = total
	}
	return items[start:end]
}

// PaginateWithTotal applies pagination and returns items with total count.
func PaginateWithTotal[T any](items []T, params Params) ([]T, int) {
	total := len(items)
	paginated := Paginate(items, params)
	return paginated, total
}

func parseIntQuery(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

// ─── Link Headers (RFC 5988) ──────────────────────────────────────────────────

// SetLinkHeader adds pagination Link header to response.
func SetLinkHeader(w http.ResponseWriter, baseURL string, page, totalPages int) {
	if totalPages <= 1 {
		return
	}

	var links []string
	if page > 1 {
		links = append(links, buildLink(baseURL, 1, "first"))
		links = append(links, buildLink(baseURL, page-1, "prev"))
	}
	if page < totalPages {
		links = append(links, buildLink(baseURL, page+1, "next"))
		links = append(links, buildLink(baseURL, totalPages, "last"))
	}

	if len(links) > 0 {
		w.Header().Set("Link", joinLinks(links))
	}
}

func buildLink(base string, page int, rel string) string {
	sep := "?"
	if containsQuery(base) {
		sep = "&"
	}
	return "<" + base + sep + "page=" + strconv.Itoa(page) + ">; rel=\"" + rel + "\""
}

func containsQuery(url string) bool {
	for _, c := range url {
		if c == '?' {
			return true
		}
	}
	return false
}

func joinLinks(links []string) string {
	result := ""
	for i, l := range links {
		if i > 0 {
			result += ", "
		}
		result += l
	}
	return result
}
