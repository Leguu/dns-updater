package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type RecordSettings struct {
	IPV4Only bool `json:"ipv4_only,omitempty"`
	IPV6Only bool `json:"ipv6_only,omitempty"`
}

type Record struct {
	Name string `json:"name"`
	TTL  int    `json:"ttl"`
	Type string `json:"type"`

	Comment  string          `json:"comment,omitempty"`
	Content  string          `json:"content,omitempty"`
	Proxied  bool            `json:"proxied,omitempty"`
	Settings *RecordSettings `json:"settings,omitempty"`
	Tags     []string        `json:"tags,omitempty"`
}

type RecordResponse struct {
	ID         string          `json:"id"`
	CreatedOn  time.Time       `json:"created_on"`
	Meta       json.RawMessage `json:"meta"`
	ModifiedOn time.Time       `json:"modified_on"`
	Proxiable  bool            `json:"proxiable"`

	CommentModifiedOn *time.Time `json:"comment_modified_on,omitempty"`
	TagsModifiedOn    *time.Time `json:"tags_modified_on,omitempty"`

	Record
}

type CloudflareClient struct {
	ZoneId   string
	ApiToken string
}

func (c *CloudflareClient) GetCloudflareRecords(ctx context.Context) ([]RecordResponse, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", c.ZoneId)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.ApiToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Result  []RecordResponse `json:"result,omitempty"`
		Success bool             `json:"success,omitempty"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors,omitempty"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("failed to decode A records: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("failed to get A records: %s", result.Errors)
	}

	return result.Result, nil
}

func (c *CloudflareClient) UpdateCloudflareIP(ctx context.Context, dnsRecordId string, current Record) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", c.ZoneId, dnsRecordId)

	json, err := json.Marshal(current)
	if err != nil {
		return fmt.Errorf("failed to marshal A record: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(json))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.ApiToken))

	_, err = http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update A record: %w", err)
	}

	return nil
}
