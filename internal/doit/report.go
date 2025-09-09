package doit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// CreateReport - Create new report
func (c *Client) CreateReport(ctx context.Context, report Report) (*Report, error) {
	log.Println("CreateReport----------------")
	log.Println(report.Config.Filters)
	rb, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	log.Print("Report body----------------")
	log.Println(string(rb))

	urlRequestBase := fmt.Sprintf("%s/analytics/v1/reports", c.HostURL)
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

	reportResponse := Report{}
	err = json.Unmarshal(body, &reportResponse)
	if err != nil {
		log.Println("ERROR UNMARSHALL----------------")
		log.Println(err)
		return nil, err
	}
	log.Println("Report response----------------")
	log.Println(reportResponse)
	return &reportResponse, nil
}

// UpdateReport - Updates an report
func (c *Client) UpdateReport(ctx context.Context, reportID string, report Report) (*Report, error) {
	rb, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	log.Print("Report body----------------")
	log.Println(string(rb))
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/reports/%s", c.HostURL, reportID)
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

	reportResponse := Report{}
	err = json.Unmarshal(body, &reportResponse)
	if err != nil {
		return nil, err
	}
	log.Println("Report response----------------")
	log.Println(reportResponse)
	return &reportResponse, nil
}

func (c *Client) DeleteReport(ctx context.Context, reportID string) error {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/reports/%s", c.HostURL, reportID)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("DELETE", urlRequestContext, nil)
	if err != nil {
		return err
	}

	res, err := c.doRequest(ctx, req)
	log.Println(res)
	if err != nil {
		return err
	}

	return nil
}

// GetReport - Returns a specifc report
func (c *Client) GetReport(ctx context.Context, orderID string) (*Report, error) {

	urlRequestBase := fmt.Sprintf("%s/analytics/v1/reports/%s/config", c.HostURL, orderID)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("GET", urlRequestContext, nil)
	if err != nil {
		return nil, err
	}
	report := Report{}
	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	log.Println(string(body))

	err = json.Unmarshal(body, &report)
	if err != nil {
		return nil, err
	}
	return &report, nil
}
