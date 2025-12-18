#!/bin/sh

set -e

RED='\033[0;31m'
NC='\033[0m'

if [ -z "$CLOUDFLARE_API_TOKEN" ]; then
  printf "${RED}CLOUDFLARE_API_TOKEN is not set${NC}\n"
  exit 1
fi

if [ -z "$ZONE_ID" ]; then
  printf "${RED}ZONE_ID is not set${NC}\n"
  exit 1
fi

go build -o dns-updater \
  -ldflags "-X 'main.CloudFlareApiToken=$CLOUDFLARE_API_TOKEN' \
  -X 'main.ZoneId=$ZONE_ID' \
  -X 'main.MonitorNames=$MONITOR_NAMES'" \
  main.go