package crud

import (
	"chaos-go/internal/framework/httpx"
)

type ToggleOpts struct {
	Setter func(id int, status bool) (any, error)

	ErrRules []httpx.ErrRule
}
