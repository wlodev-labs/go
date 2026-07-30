package pagination

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/wlodev-labs/go/ptr"
)

type Client struct {
	// DefaultLimit is the default number of items to return if no limit is specified.
	DefaultLimit int
	// MinLimit is the minimum number of items that can be requested.
	MinLimit int
	// MaxLimit is the maximum number of items that can be requested.
	MaxLimit int
}

func New(defaultLimit, minLimit, maxLimit int) *Client {
	return &Client{
		DefaultLimit: defaultLimit,
		MinLimit:     minLimit,
		MaxLimit:     maxLimit,
	}
}

type ParseParams struct {
	// Optional, if not provided, default limit will be used.
	Limit *int
	// Optional, if not provided, no offset will be applied.
	Until *time.Time
	// Optional, if not provided, no offset will be applied.
	Since *time.Time
}

type Parsed struct {
	Limit int
	Until *time.Time
	Since *time.Time
}

var ErrInvalidLimit = errors.New("invalid limit")

// Parse takes the request parameters passed by the user and returns a parsed
// pagination object that can be used to query the database. It validates the
// limit and returns an error if it's invalid.
func (c *Client) Parse(params ParseParams) (*Parsed, error) {
	limit := c.DefaultLimit
	if params.Limit != nil {
		if *params.Limit < c.MinLimit || *params.Limit > c.MaxLimit {
			return nil, ErrInvalidLimit
		}
		limit = *params.Limit
	}

	parsed := &Parsed{
		Limit: limit,
		Until: params.Until,
		Since: params.Since,
	}
	return parsed, nil
}

type Response struct {
	Count int
	Next  *time.Time
	Prev  *time.Time
}

// GetResponseData takes a slice of items, most presumably from a database
// query, and returns a Response object that contains the count of items and the
// next and previous timestamps for pagination.
func GetResponseData[T any](items []T, getTimeColumn func(item T) time.Time) *Response {
	response := &Response{
		Count: len(items),
	}

	if len(items) > 0 {
		response.Next = ptr.Time(getTimeColumn(items[len(items)-1]))
		response.Prev = ptr.Time(getTimeColumn(items[0]))
	}
	return response
}
