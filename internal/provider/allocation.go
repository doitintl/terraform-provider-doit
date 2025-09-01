package provider

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"terraform-provider-doit/internal/provider/models"
)

// CreateAllocation - Create new allocation
func (c *ClientTest) CreateAllocation(allocation models.SingleAllocation) (*models.SingleAllocation, error) {
	rb, err := json.Marshal(allocation)
	if err != nil {
		return nil, err
	}
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations", c.HostURL)
	req, err := http.NewRequest("POST", urlRequestBase, strings.NewReader(string(rb)))
	log.Println("URL----------------")
	log.Println(req.URL)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		log.Println("ERROR REQUEST----------------")
		log.Println(err)
		log.Println(string(rb))
		return nil, err
	}

	allocationResponse := models.SingleAllocation{}
	err = json.Unmarshal(body, &allocationResponse)
	if err != nil {
		log.Println("ERROR UNMARSHALL----------------")
		log.Println(err)
		return nil, err
	}
	log.Println("Allocation response----------------")
	log.Println(allocationResponse)
	return &allocationResponse, nil
}

// UpdateAllocation - Updates an allocation
func (c *ClientTest) UpdateAllocation(allocationID string, allocation models.SingleAllocation) (*models.SingleAllocation, error) {
	rb, err := json.Marshal(allocation)
	if err != nil {
		return nil, err
	}
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations/%s", c.HostURL, allocationID)
	req, err := http.NewRequest("PATCH", urlRequestBase, strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}
	log.Println("Update URL----------------")
	log.Println(req.URL)
	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	allocationResponse := models.SingleAllocation{}
	err = json.Unmarshal(body, &allocationResponse)
	if err != nil {
		return nil, err
	}
	log.Println("Allocation response----------------")
	log.Println(allocationResponse)
	return &allocationResponse, nil
}

func (c *ClientTest) DeleteAllocation(allocationID string) error {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations/%s", c.HostURL, allocationID)

	req, err := http.NewRequest("DELETE", urlRequestBase, nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req)
	if err != nil {
		return err
	}

	return nil
}

// GetAllocation - Returns a specifc allocation
func (c *ClientTest) GetAllocation(orderID string) (*models.SingleAllocation, error) {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations/%s", c.HostURL, orderID)
	req, err := http.NewRequest("GET", urlRequestBase, nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	allocation := models.SingleAllocation{}
	err = json.Unmarshal(body, &allocation)
	if err != nil {
		return nil, err
	}

	return &allocation, nil
}
