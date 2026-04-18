package bzzoiro

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	http    *resty.Client
	baseURL string
	token   string
}

func New(baseURL, token string) *Client {
	r := resty.New().
		SetHeader("Authorization", "Token "+token).
		SetTimeout(15 * time.Second)
	return &Client{http: r, baseURL: baseURL, token: token}
}

// ProxyPlayerPhoto fetches the player photo from the bzzoiro image API and
// writes it directly to w. Returns an error if the image cannot be fetched.
func (c *Client) ProxyPlayerPhoto(w io.Writer, headerSetter func(string), apiID uint) error {
	url := fmt.Sprintf("%s/img/player/%d/?token=%s", c.baseURL, apiID, c.token)
	resp, err := http.Get(url) //nolint:noctx
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

func fetchAll[Raw any](c *Client, path string, params map[string]string) ([]Raw, error) {
	var all []Raw
	page := 1
	for {
		var resp paginated[Raw]
		url := c.baseURL + path
		req := c.http.R().SetResult(&resp)
		for k, v := range params {
			req = req.SetQueryParam(k, v)
		}
		req = req.SetQueryParam("page", fmt.Sprintf("%d", page))

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
		page++
	}
	return all, nil
}
