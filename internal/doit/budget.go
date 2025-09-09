package doit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// CreateBudget - Create new budget
func (c *Client) CreateBudget(ctx context.Context, budget Budget) (*Budget, error) {
	rb, err := json.Marshal(budget)
	if err != nil {
		return nil, err
	}
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/budgets", c.HostURL)
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

	budgetResponse := Budget{}
	err = json.Unmarshal(body, &budgetResponse)
	if err != nil {
		log.Println("ERROR UNMARSHALL----------------")
		log.Println(err)
		return nil, err
	}
	log.Println("Budget response----------------")
	log.Println(budgetResponse)
	return &budgetResponse, nil
}

// UpdateBudget - Updates an budget
func (c *Client) UpdateBudget(ctx context.Context, budgetID string, budget Budget) (*Budget, error) {
	rb, err := json.Marshal(budget)
	if err != nil {
		return nil, err
	}
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/budgets/%s", c.HostURL, budgetID)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("PATCH", urlRequestContext, strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}
	log.Println("Update BODY----------------")
	log.Println(string(rb))
	log.Println("Update URL----------------")
	log.Println(req.URL)
	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	log.Println("Update BODY----------------")
	log.Println(string(body))
	budgetResponse := Budget{}
	err = json.Unmarshal(body, &budgetResponse)
	if err != nil {
		return nil, err
	}
	log.Println("Budget response----------------")
	log.Println(budgetResponse)
	return &budgetResponse, nil
}

func (c *Client) DeleteBudget(ctx context.Context, budgetID string) error {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/budgets/%s", c.HostURL, budgetID)
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

// GetBudget - Returns a specifc budget
func (c *Client) GetBudget(ctx context.Context, orderID string) (*Budget, error) {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/budgets/%s", c.HostURL, orderID)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("GET", urlRequestContext, nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	budget := Budget{}
	err = json.Unmarshal(body, &budget)
	if err != nil {
		return nil, err
	}

	return &budget, nil
}
