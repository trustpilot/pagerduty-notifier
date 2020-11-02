package main

import (
	"log"
	"runtime"
	"time"
)

var pauseTimeout int
var clearOnUnpause bool

func main() {

	// We need to lock the go threads to avoid NSInternalInconsistencyException from 'NSWindow drag regions should only be invalidated on the Main Thread!'
	// The exception comes from "github.com/shurcooL/trayhost" which uses cgo to underlying Mac OS cocoa libraries. Maybe because the darwin part
	// is approx 3 year old, written for an older OS version ?
	runtime.LockOSThread()

	logInit("syslog") // Initialize logger
	appInit()         // setup application tray menu, icons etc
	cfg := cfgInit()  // read configuration file
	pdInit(cfg)       // Initialize Pagerduty stuff

	interval, err := cfg.Section("pagerduty").Key("interval").Int()
	if err != nil {
		interval = 30
	}

	pauseTimeout, err = cfg.Section("main").Key("pause.timeout").Int()
	if err != nil { pauseTimeout = 0 }
	clearOnUnpause, err = cfg.Section("main").Key("clear.on.unpause").Bool()
	if err != nil {clearOnUnpause = true}

	go func() {
		for {
			if pause {
				if (pauseTimeout > 0) {
					if time.Now().After(pauseStopTime) {
						log.Println("Pause timeout ...")
						togglePause()
					}
				}
			} else {
				for _, incident := range pdGetIncidents(cfg) {
					pdNotify(incident)
				}
			}
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}()

	appEnterLoop()
}
