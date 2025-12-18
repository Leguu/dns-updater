package main

import (
	"os/exec"
	"strings"
)

func GetIp() (string, error) {
	cmd := exec.Command("dig", "+short", "myip.opendns.com", "@resolver1.opendns.com", "-b", "192.168.1.2")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}
