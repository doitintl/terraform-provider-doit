package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// CreateAttribution - Create new attribution.
func (c *Client) CreateAttribution(ctx context.Context, attribution Attribution) (*Attribution, error) {
	rb, err := json.Marshal(attribution)
	if err != nil {
		return nil, err
	}
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/attributions", c.HostURL)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("POST", urlRequestContext, strings.NewReader(string(rb)))
	log.Println("URL----------------")
	log.Println(req.URL)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		log.Println("ERROR REQUEST----------------")
		log.Println(err)
		return nil, err
	}

	attributionResponse := Attribution{}
	err = json.Unmarshal(body, &attributionResponse)
	if err != nil {
		log.Println("ERROR UNMARSHALL----------------")
		log.Println(err)
		return nil, err
	}
	log.Println("Attribution response----------------")
	log.Println(attributionResponse)
	return &attributionResponse, nil
}

// UpdateAttribution - Updates an attribution.
func (c *Client) UpdateAttribution(ctx context.Context, attributionID string, attribution Attribution) (*Attribution, error) {
	rb, err := json.Marshal(attribution)
	if err != nil {
		return nil, err
	}
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/attributions/%s", c.HostURL, attributionID)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("PATCH", urlRequestContext, strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}
	log.Println("Update URL----------------")
	log.Println(req.URL)
	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	attributionResponse := Attribution{}
	err = json.Unmarshal(body, &attributionResponse)
	if err != nil {
		return nil, err
	}
	log.Println("Attribution response----------------")
	log.Println(attributionResponse)
	return &attributionResponse, nil
}

func (c *Client) DeleteAttribution(ctx context.Context, attributionID string) error {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/attributions/%s", c.HostURL, attributionID)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)

	req, err := http.NewRequest("DELETE", urlRequestContext, nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

// GetAttribution - Returns a specifc attribution.
func (c *Client) GetAttribution(ctx context.Context, orderID string) (*Attribution, error) {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/attributions/%s", c.HostURL, orderID)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("GET", urlRequestContext, nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	attribution := Attribution{}
	err = json.Unmarshal(body, &attribution)
	if err != nil {
		return nil, err
	}

	return &attribution, nil
}
