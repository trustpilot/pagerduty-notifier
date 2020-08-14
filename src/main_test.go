package main

import (
	"testing"
)

func TestIncidents(t *testing.T) {

	logInit("stdout") // Initialize logger
	cfg := cfgInit()  // read configuration file
	pdInit(cfg)       // Initialize Pagerduty stuff

	for _, incident := range pdGetIncidents(cfg) {
		t.Log(incident)
	}

}
