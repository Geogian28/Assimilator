package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// FailureReport must perfectly match the JSON structure Tormon expects
type FailureReport struct {
	Hostname    string `json:"hostname"`
	PackageName string `json:"package_name"`
	Message     string `json:"message"`
}

// ReportFailureToTormon fires an alert to your dashboard
func reportToTormon(packageName, errorMessage string) {
	report := FailureReport{
		Hostname:    appConfig.Hostname,
		PackageName: packageName,
		Message:     errorMessage,
	}

	payload, err := json.Marshal(report)
	if err != nil {
		Error("failed to marshal Tormon report: ", err)
		return
	}

	// Use a timeout so a downed monitoring server doesn't break your deployments
	client := &http.Client{Timeout: 5 * time.Second}

	url := appConfig.TormonAddress
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url + "/api/report"
	} else {
		url = url + "/api/report"
	}

	// Trace("Tormon URL: ", url)
	// fmt.Println("appConfig.TormonAddress=", appConfig.TormonAddress)
	// url := appConfig.TormonAddress + "/api/report"
	// Trace("Tormon URL: ", url)

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		Error("failed to reach Tormon API: ", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		Debug("Reported %s failure to Tormon.\n", packageName)
	} else {
		Error("tormon returned unexpected status: ", resp.StatusCode)
	}
}
