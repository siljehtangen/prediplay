package bzzoiro

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// photoClient is a plain HTTP client used only for image proxying.
// Separate from the resty client so it can have its own timeout without
// affecting API request settings.
var photoClient = &http.Client{Timeout: 10 * time.Second}

type Client struct {
	http    *resty.Client
	baseURL string
	token   string
}

func New(baseURL, token string) *Client {
	r := resty.New().
		SetHeader("Authorization", "Token "+token).
		SetTimeout(15 * time.Second)
	return &Client{http: r, baseURL: strings.TrimRight(baseURL, "/"), token: token}
}

func (c *Client) ProxyPlayerPhoto(w io.Writer, headerSetter func(string), apiID uint) error {
	url := fmt.Sprintf("%s/img/player/%d/?token=%s", c.baseURL, apiID, c.token)
	resp, err := photoClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("photo not found: status %d", resp.StatusCode)
	}
	headerSetter(resp.Header.Get("Content-Type"))
	_, err = io.Copy(w, resp.Body)
	return err
}

const maxPages = 50

func fetchAll[Raw any](c *Client, path string, params map[string]string) ([]Raw, error) {
	var all []Raw
	url := c.baseURL + path
	for page := 1; ; page++ {
		var resp paginated[Raw]
		req := c.http.R().SetResult(&resp)
		for k, v := range params {
			req = req.SetQueryParam(k, v)
		}
		req = req.SetQueryParam("page", strconv.Itoa(page))

		r, err := req.Get(url)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		if r.IsError() {
			return nil, fmt.Errorf("API error %d: %s", r.StatusCode(), r.String())
		}
		all = append(all, resp.Results...)
		if len(all) >= resp.Count || len(resp.Results) == 0 {
			break
		}
		if page >= maxPages {
			slog.Warn("fetchAll page limit reached, stopping early", "limit", maxPages, "path", path)
			break
		}
	}
	return all, nil
}
