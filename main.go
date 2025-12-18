package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var CloudFlareApiToken string

var ZoneId string

var MonitorNames string

func isIncluded(name string) bool {
	for monitorName := range strings.SplitSeq(MonitorNames, ",") {
		if strings.EqualFold(monitorName, name) {
			return true
		}
	}
	return false
}

var printRecords = flag.Bool("print-records", false, "print the records to the console")

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

	if MonitorNames == "" {
		MonitorNames = os.Getenv("MONITOR_NAMES")
	}

	if CloudFlareApiToken == "" {
		slog.Error("CLOUDFLARE_API_TOKEN is not set")
		os.Exit(1)
	}

	if ZoneId == "" {
		slog.Error("ZONE_ID is not set")
		os.Exit(1)
	}

	if MonitorNames == "" {
		slog.Error("MONITOR_NAMES is not set")
		os.Exit(1)
	}

	cloudflareClient := CloudflareClient{
		ZoneId:   ZoneId,
		ApiToken: CloudFlareApiToken,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	flag.Parse()

	if *printRecords {
		records, err := cloudflareClient.GetCloudflareRecords(ctx)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to get A records: %s", err))
			os.Exit(1)
		}

		for _, record := range records {
			fmt.Printf("%s: %s\n", record.Name, record.Content)
		}
		os.Exit(0)
	}

	records, err := cloudflareClient.GetCloudflareRecords(ctx)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to get A records: %s", err))
		os.Exit(1)
	}

	if len(records) == 0 {
		slog.Error("No A records found")
		os.Exit(1)
	}

	if len(MonitorNames) > 0 {
		slog.Info(fmt.Sprintf("Monitoring names: %s", MonitorNames))
	}

	slog.Info(fmt.Sprintf("Starting to monitor IP address for zone %s", ZoneId))

	checkAndUpdate := func() {
		ip, err := GetIp()
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to get IP: %s", err))
			os.Exit(1)
		}

		records, err := cloudflareClient.GetCloudflareRecords(ctx)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to get A records: %s", err))
			os.Exit(1)
		}

		for _, record := range records {
			if !isIncluded(record.Name) {
				continue
			}

			if record.Content == ip {
				slog.Debug(fmt.Sprintf("IP hasn't changed for `%s`, no update needed: %s", record.Name, ip))
				continue
			}

			if record.Type != "A" {
				continue
			}

			slog.Info(fmt.Sprintf("IP changed for `%s` from %s to %s, updating DNS record", record.Name, record.Content, ip))

			record.Content = ip

			err = cloudflareClient.UpdateCloudflareIP(ctx, record.ID, record.Record)
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
