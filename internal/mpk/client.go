package mpk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
	_ "time/tzdata"

	"github.com/stscoundrel/orochi/internal/domain"
)

const defaultEndpoint = "https://api.contio.fi/mpk/api/v1/rss"
const maxResponseBytes int64 = 10 << 20

var helsinkiLocation = loadHelsinkiLocation()

func loadHelsinkiLocation() *time.Location {
	location, err := time.LoadLocation("Europe/Helsinki")
	if err != nil {
		panic(err)
	}
	return location
}

type Client struct {
	Endpoint string
	HTTP     *http.Client
}

func NewClient() *Client {
	return &Client{
		Endpoint: defaultEndpoint,
		HTTP:     &http.Client{Timeout: 25 * time.Second},
	}
}

func (c *Client) Fetch(ctx context.Context, now time.Time) ([]domain.Event, error) {
	feedURL, err := buildURL(c.Endpoint, now)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch RSS: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feed returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read RSS: %w", err)
	}
	if int64(len(data)) > maxResponseBytes {
		return nil, fmt.Errorf("feed exceeds %d bytes", maxResponseBytes)
	}
	return Parse(bytes.NewReader(data))
}

func buildURL(endpoint string, now time.Time) (string, error) {
	feedURL, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("feed URL: %w", err)
	}

	query := feedURL.Query()
	query.Set("startDate", now.In(helsinkiLocation).Format("02-01-2006"))
	query.Set("Location", "Helsinki")
	query.Set("Specialfield", "Ammunta")
	feedURL.RawQuery = query.Encode()

	return feedURL.String(), nil
}
