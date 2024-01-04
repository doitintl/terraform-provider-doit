package provider

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type AuthResponseTest struct {
	DoiTAPITOken    string `json:"doiTAPITOken"`
	CustomerContext string `json:"customerContext"`
}

// AuthStruct -
type AuthStructTest struct {
	DoiTAPITOken    string `json:"doiTAPITOken"`
	CustomerContext string `json:"customerContext"`
}

// Client
type ClientTest struct {
	HostURL     string
	HTTPClient  *http.Client
	Auth        AuthStructTest
	Ratelimiter *rate.Limiter
}

// NewClient -
func NewClientTest(host, doiTAPIClient, customerContext *string, rl *rate.Limiter) (*ClientTest, error) {
	
	c := ClientTest{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		// Default DoiT URL
		HostURL: HostURL,
		Auth: AuthStructTest{
			DoiTAPITOken:    *doiTAPIClient,
			CustomerContext: *customerContext,
		},
		Ratelimiter: rl,
	}

	if host != nil {
		c.HostURL = *host
	}

	_, err := c.SignIn()
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func (c *ClientTest) SignIn() (*AuthResponseTest, error) {
	if c.Auth.DoiTAPITOken == "" {
		return nil, fmt.Errorf("define Doit API Token")
	}
	//rb, err := json.Marshal(c.Auth)
	//if err != nil {
	//	return nil, err
	//}

	url_ccontext := "/?customerContext=" + c.Auth.CustomerContext
	req, err := http.NewRequest("GET", c.HostURL+url_ccontext, nil)
	if err != nil {
		return nil, err
	}

	_, err = c.doRequest(req)
	if err != nil {
		return nil, err
	}

	ar := AuthResponseTest{
		DoiTAPITOken:    c.Auth.DoiTAPITOken,
		CustomerContext: c.Auth.CustomerContext,
	}
	//err = json.Unmarshal(body, &ar)
	if err != nil {
		return nil, err
	}

	return &ar, nil
}

func (c *ClientTest) doRequest(req *http.Request) ([]byte, error) {
	//req.Header.Set("Authorization", c.Token)
	ctx := context.Background()
	err := c.Ratelimiter.Wait(ctx) // This is a blocking call. Honors the rate limit
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Auth.DoiTAPITOken)
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("status: %d, body: %s", res.StatusCode, body)
	}

	return body, err
}

func (c *ClientTest) createAttribution(req *http.Request) ([]byte, error) {
	req.Header.Set("Authorization", c.Auth.DoiTAPITOken)

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d, body: %s", res.StatusCode, body)
	}

	return body, err
}
