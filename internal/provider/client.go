package provider

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"golang.org/x/time/rate"
)

type AuthResponse struct {
	DoiTAPITOken    string `json:"doiTAPITOken"`
	CustomerContext string `json:"customerContext"`
}

type Auth struct {
	DoiTAPITOken    string `json:"doiTAPITOken"`
	CustomerContext string `json:"customerContext"`
}

type Client struct {
	HostURL     string
	HTTPClient  *http.Client
	Auth        Auth
	Ratelimiter *rate.Limiter
}

func NewClient(ctx context.Context, host, doiTAPIClient, customerContext *string, rl *rate.Limiter) (*Client, error) {

	c := Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		// Default DoiT URL
		HostURL: HostURL,
		Auth: Auth{
			DoiTAPITOken:    *doiTAPIClient,
			CustomerContext: *customerContext,
		},
		Ratelimiter: rl,
	}

	if host != nil {
		c.HostURL = *host
	}
	_, err := c.SignIn(ctx)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func (c *Client) SignIn(ctx context.Context) (*AuthResponse, error) {
	if c.Auth.DoiTAPITOken == "" {
		return nil, fmt.Errorf("define Doit API Token")
	}

	urlCcontext := "/auth/v1/validate?customerContext=" + c.Auth.CustomerContext
	req, err := http.NewRequest("GET", c.HostURL+urlCcontext, nil)
	if err != nil {
		return nil, err
	}

	_, err = c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	ar := AuthResponse{
		DoiTAPITOken:    c.Auth.DoiTAPITOken,
		CustomerContext: c.Auth.CustomerContext,
	}

	return &ar, nil
}

func (c *Client) doRequest(ctx context.Context, req *http.Request) ([]byte, error) {
	err := c.Ratelimiter.Wait(ctx) // This is a blocking call. Honors the rate limit
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Auth.DoiTAPITOken)

	res := &http.Response{}
	operation := func() (*http.Response, error) {
		res, err := c.HTTPClient.Do(req)
		return res, err
	}

	retryable := func() error {
		var errRetry error
		res, errRetry = operation()
		if res == nil {
			log.Println("no response")
			log.Println(errRetry)
			return fmt.Errorf("no response")
		}
		if res.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("rate limit exceeded")
		}
		err = errRetry
		return nil
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 5 * time.Minute
	errRetryOutside := backoff.Retry(retryable, b)
	if errRetryOutside != nil {
		return nil, errRetryOutside
	}

	// in case the error is different to rate limit
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("status: %d, body: %s", res.StatusCode, body)
	}

	return body, err
}

func addContextToURL(context, url string) (urlContext string) {
	urlContext = url
	if len(strings.TrimSpace(context)) != 0 {
		urlContext = fmt.Sprintf(url+"?customerContext=%s", context)
	}
	return urlContext
}
