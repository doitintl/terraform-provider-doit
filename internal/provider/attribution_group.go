package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// CreateAttributionGroup - Create new attributionGroup
func (c *Client) CreateAttributionGroup(ctx context.Context, attributionGroup AttributionGroup) (*AttributionGroup, error) {
	log.Println("CreateAttributionGroup")
	log.Println(attributionGroup)
	rb, err := json.Marshal(attributionGroup)
	if err != nil {
		return nil, err
	}
	log.Println(strings.NewReader(string(rb)))

	urlRequestBase := fmt.Sprintf("%s/analytics/v1/attributiongroups", c.HostURL)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)

	req, err := http.NewRequest("POST", urlRequestContext, strings.NewReader(string(rb)))
	log.Println("URL:")
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

	attributionGroupResponse := AttributionGroup{}
	err = json.Unmarshal(body, &attributionGroupResponse)
	if err != nil {
		return nil, err
	}
	log.Println("AttributionGroup response:")
	log.Println(attributionGroupResponse)
	return &attributionGroupResponse, nil
}

// UpdateAttributionGroup - Updates an attributionGroup
func (c *Client) UpdateAttributionGroup(ctx context.Context, attributionGroupID string, attributionGroup AttributionGroup) (*AttributionGroup, error) {
	rb, err := json.Marshal(attributionGroup)
	if err != nil {
		return nil, err
	}
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/attributiongroups/%s", c.HostURL, attributionGroupID)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("PATCH", urlRequestContext, strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}
	log.Println("Update UR:")
	log.Println(req.URL)
	body, err := c.doRequest(ctx, req)
	log.Println("body:")
	log.Println(string(body))
	if err != nil {
		return nil, err
	}

	return &attributionGroup, nil
}

func (c *Client) DeleteAttributionGroup(ctx context.Context, attributionGroupID string) error {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/attributiongroups/%s", c.HostURL, attributionGroupID)
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

// GetAttributionGroup - Returns a specifc attribution
func (c *Client) GetAttributionGroup(ctx context.Context, attributionGroupID string) (*AttributionGroup, error) {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/attributiongroups/%s", c.HostURL, attributionGroupID)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("GET", urlRequestContext, nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	log.Println("AttributionGroup body----------------")
	log.Println(string(body))
	attributionGroup := AttributionGroup{}
	attributionGroupGet := AttributionGroupGet{}
	err = json.Unmarshal(body, &attributionGroupGet)
	if err != nil {
		return nil, err
	}

	// code that copy attributeGroupGet in attributionGroup
	attributionGroup.Id = attributionGroupGet.Id
	attributionGroup.Name = attributionGroupGet.Name
	attributionGroup.Description = attributionGroupGet.Description
	// code to intialise attributionGroup.Attribution as empty array
	attributionGroup.Attributions = []string{}
	// code that iterate attributionGroupGet.Attribution
	for _, attribution := range attributionGroupGet.Attributions {
		attributionGroup.Attributions = append(attributionGroup.Attributions, attribution.Id)
	}
	return &attributionGroup, nil

}
