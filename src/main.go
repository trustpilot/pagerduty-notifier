package main

import (
	"runtime"
	"time"
)

func main() {

	// We need to lock the go threads to avoid NSInternalInconsistencyException from 'NSWindow drag regions should only be invalidated on the Main Thread!'
	// The exception comes from "github.com/shurcooL/trayhost" which uses cgo to underlying Mac OS cocoa libraries. Maybe because the darwin part
	// is approx 3 year old, written for an older OS version ?
	runtime.LockOSThread()

	appInit()        // setup application tray menu, icons etc
	logInit()        // Initialize logger
	cfg := cfgInit() // read configuration file
	pdInit(cfg)      // Initialize Pagerduty stuff

	interval, err := cfg.Section("pagerduty").Key("interval").Int()
	if err != nil {
		interval = 30
	}

	go func() {
		for {
			for _, incident := range pdGetIncidents(cfg) {
				pdNotify(incident)
			}
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}()

	appEnterLoop()
}
