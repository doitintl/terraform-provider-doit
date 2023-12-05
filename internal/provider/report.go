package provider

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// CreateReport - Create new report
func (c *ClientTest) CreateReport(report Report) (*Report, error) {
	log.Println("CreateReport----------------")
	log.Println(report.Config.Filters)
	rb, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	log.Print("Report body----------------")
	log.Println(string(rb))

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/analytics/v1/reports/?customerContext=%s", c.HostURL, c.Auth.CustomerContext), strings.NewReader(string(rb)))
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
func (c *ClientTest) UpdateReport(reportID string, report Report) (*Report, error) {
	rb, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	log.Print("Report body----------------")
	log.Println(string(rb))
	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/analytics/v1/reports/%s/?customerContext=%s", c.HostURL, reportID, c.Auth.CustomerContext), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}
	log.Println("Update URL----------------")
	log.Println(req.URL)
	body, err := c.doRequest(req)
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

func (c *ClientTest) DeleteReport(reportID string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/analytics/v1/reports/%s/?customerContext=%s", c.HostURL, reportID, c.Auth.CustomerContext), nil)
	if err != nil {
		return err
	}

	res, err := c.doRequest(req)
	log.Println(res)
	if err != nil {
		return err
	}

	return nil
}

// GetReport - Returns a specifc report
func (c *ClientTest) GetReport(orderID string) (*Report, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/analytics/v1/reports/%s/config?customerContext=%s", c.HostURL, orderID, c.Auth.CustomerContext), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	log.Println("Body----------------")
	log.Println(string(body))
	report := Report{}
	err = json.Unmarshal(body, &report)
	if err != nil {
		return nil, err
	}

	return &report, nil
}
