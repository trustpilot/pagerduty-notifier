package main

import (
	"log"
	"testing"
)

func TestIncidents(t *testing.T) {
	logInit("stdout")     // Initialize logger
	cfg, err := cfgInit() // read configuration file
	if err != nil {
		log.Printf("error: %v", err)
		t.Skipf(
			"%s requires a valid configuration file to test listing of real incidents",
			t.Name(),
		)
	}
	pdInit(cfg) // Initialize Pagerduty stuff

	for _, incident := range pdGetIncidents(cfg) {
		t.Log(incident)
	}
}
