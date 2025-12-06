#!/bin/sh

set -e

./build.sh

sudo cp dns-updater /usr/local/bin/dns-updater

sudo cp dns-updater.service /etc/systemd/system/dns-updater.service

sudo systemctl daemon-reload

sudo systemctl enable dns-updater

sudo systemctl start dns-updater