package sequence

import (
	"context"
	"fmt"

	"github.com/go-resty/resty/v2"
)

type Client interface {
	List(ctx context.Context, sc *SeriesCollection) error
	Create(ctx context.Context, s *Series) error
	CreateEntry(ctx context.Context, seriesID string, e *Entry) error
}

type client struct {
	api *resty.Client
}

func New(host, token string) Client {
	c := new(client)
	c.api = resty.New().
		SetHostURL(host).
		SetAuthToken(token)
	return c
}

func (c *client) List(ctx context.Context, sc *SeriesCollection) error {
	e := new(ErrorResponse)
	res, err := c.api.R().
		SetResult(sc).
		SetError(e).
		SetContext(ctx).
		Get("/sequence/v1/series")
	if err != nil {
		return err
	}
	if res.IsError() {
		return fmt.Errorf("%d: %v", res.StatusCode(), e.Message)
	}
	return nil
}

func (c *client) Create(ctx context.Context, s *Series) error {
	e := new(ErrorResponse)
	res, err := c.api.R().
		SetBody(s).
		SetError(e).
		SetResult(s).
		SetContext(ctx).
		Post("/sequence/v1/series")
	if err != nil {
		return err
	}
	if res.IsError() {
		return fmt.Errorf("%d: %v", res.StatusCode(), e.Message)
	}
	return nil
}

func (c *client) CreateEntry(ctx context.Context, seriesID string, se *Entry) error {
	e := new(ErrorResponse)
	res, err := c.api.R().
		SetBody(se).
		SetError(e).
		SetResult(se).
		SetContext(ctx).
		Post("/sequence/v1/series/" + seriesID + "/entries")
	if err != nil {
		return err
	}
	if res.IsError() {
		return fmt.Errorf("%d: %v", res.StatusCode(), e.Message)
	}
	return nil
}
