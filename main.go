package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func GetIp() (string, error) {
	cmd := exec.Command("dig", "+short", "myip.opendns.com", "@resolver1.opendns.com", "-b", "192.168.1.2")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

type ARecordSettings struct {
	IPV4Only bool `json:"ipv4_only,omitempty"`
	IPV6Only bool `json:"ipv6_only,omitempty"`
}

type ARecord struct {
	Name string `json:"name"`
	TTL  int    `json:"ttl"`
	Type string `json:"type"`

	Comment  string           `json:"comment,omitempty"`
	Content  string           `json:"content,omitempty"`
	Proxied  bool             `json:"proxied,omitempty"`
	Settings *ARecordSettings `json:"settings,omitempty"`
	Tags     []string         `json:"tags,omitempty"`
}

type ARecordResponse struct {
	ID         string          `json:"id"`
	CreatedOn  time.Time       `json:"created_on"`
	Meta       json.RawMessage `json:"meta"`
	ModifiedOn time.Time       `json:"modified_on"`
	Proxiable  bool            `json:"proxiable"`

	CommentModifiedOn *time.Time `json:"comment_modified_on,omitempty"`
	TagsModifiedOn    *time.Time `json:"tags_modified_on,omitempty"`

	ARecord
}

type CloudflareClient struct {
	ZoneId   string
	ApiToken string
}

func (c *CloudflareClient) GetCloudflareARecords(ctx context.Context) ([]ARecordResponse, error) {
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
		Result  []ARecordResponse `json:"result,omitempty"`
		Success bool              `json:"success,omitempty"`
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

func (c *CloudflareClient) UpdateCloudflareIP(ctx context.Context, dnsRecordId string, current ARecord) error {
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

var CloudFlareApiToken string

var ZoneId string

func main() {
	if os.Getenv("DEBUG") == "true" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	if CloudFlareApiToken == "" {
		CloudFlareApiToken = os.Getenv("CLOUDFLARE_API_TOKEN")
	}

	if ZoneId == "" {
		ZoneId = os.Getenv("ZONE_ID")
	}

	if CloudFlareApiToken == "" {
		slog.Error("CLOUDFLARE_API_TOKEN is not set")
		os.Exit(1)
	}

	if ZoneId == "" {
		slog.Error("ZONE_ID is not set")
		os.Exit(1)
	}

	cloudflareClient := CloudflareClient{
		ZoneId:   ZoneId,
		ApiToken: CloudFlareApiToken,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	records, err := cloudflareClient.GetCloudflareARecords(ctx)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to get A records: %s", err))
		os.Exit(1)
	}

	if len(records) == 0 {
		slog.Error("No A records found")
		os.Exit(1)
	}

	slog.Info(fmt.Sprintf("Starting to monitor IP address for zone %s", ZoneId))

	checkAndUpdate := func() {
		ip, err := GetIp()
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to get IP: %s", err))
			os.Exit(1)
		}

		records, err := cloudflareClient.GetCloudflareARecords(ctx)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to get A records: %s", err))
			os.Exit(1)
		}

		for _, record := range records {
			if record.Content == ip {
				slog.Debug(fmt.Sprintf("IP hasn't changed for `%s`, no update needed: %s", record.Name, ip))
				continue
			}

			if record.Type != "A" {
				continue
			}

			slog.Info(fmt.Sprintf("IP changed for `%s` from %s to %s, updating DNS record", record.Name, record.Content, ip))

			record.Content = ip

			err = cloudflareClient.UpdateCloudflareIP(ctx, record.ID, record.ARecord)
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to update A record: %s", err))
				os.Exit(1)
			}
		}
	}

	checkAndUpdate()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Shutting down...")
			return

		case <-time.After(30 * time.Minute):
			checkAndUpdate()
		}
	}
}
