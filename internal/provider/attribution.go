package provider

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// CreateAttribution - Create new attribution
func (c *ClientTest) CreateAttribution(attribution Attribution) (*Attribution, error) {
	rb, err := json.Marshal(attribution)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/analytics/v1/attributions?customerContext=%s", c.HostURL, c.Auth.CustomerContext), strings.NewReader(string(rb)))
	log.Println("URL----------------")
	log.Println(req.URL)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
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

// UpdateAttribution - Updates an attribution
func (c *ClientTest) UpdateAttribution(attributionID string, attribution Attribution) (*Attribution, error) {
	rb, err := json.Marshal(attribution)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/analytics/v1/attributions/%s?customerContext=%s", c.HostURL, attributionID, c.Auth.CustomerContext), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}
	log.Println("Update URL----------------")
	log.Println(req.URL)
	body, err := c.doRequest(req)
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

func (c *ClientTest) DeleteAttribution(attributionID string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/analytics/v1/attributions/%s?customerContext=%s", c.HostURL, attributionID, c.Auth.CustomerContext), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req)
	if err != nil {
		return err
	}

	return nil
}

// GetAttribution - Returns a specifc attribution
func (c *ClientTest) GetAttribution(orderID string) (*Attribution, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/analytics/v1/attributions/%s?customerContext=%s", c.HostURL, orderID, c.Auth.CustomerContext), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
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
