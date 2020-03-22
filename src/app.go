package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/shurcooL/trayhost"
)

// setup tray icon and menus
var menuItems = []trayhost.MenuItem{
	{
		Title: "Startup on login",
		Handler: func() {
			toggleStartup()
		},
	},
	{
		Title: "Pause",
		Handler: func() {
			appNotify("Pagerduty Notifier", "This is not implemeted yet.", "", 30*time.Second)
		},
	},
	{
		Title: "Info",
		Handler: func() {
			openBrowser("https://github.com/trustpilot/pagerduty-notifier/blob/master/README.md")
		},
	},
	trayhost.SeparatorMenuItem(),
	{
		Title:   "Quit",
		Handler: trayhost.Exit,
	},
}

func appInit() {

	// On macOS, when you run an app bundle, the working directory of the executed process
	// is the root directory (/), not the app bundle's Contents/Resources directory.
	// Change directory to Resources so that we can load resources from there.
	ep, err := os.Executable()
	if err != nil {
		log.Fatalln("os.Executable:", err)
	}
	err = os.Chdir(filepath.Join(filepath.Dir(ep), "..", "Resources"))
	if err != nil {
		log.Fatalln("os.Chdir:", err)
	}

	// Load tray icon.
	iconData, err := ioutil.ReadFile("pd-bw.png")
	if err != nil {
		log.Fatalln(err)
	}

	trayhost.Initialize("Pagerduty Notifier", iconData, menuItems)
}

func toggleStartup() {
	if existsLaunchConf() {
		deleteLaunchConf()
		appNotify("Pagerduty Notifier", "Removed from Launch configuration", "", 10*time.Second)
	} else {
		writeLaunchConf()
		appNotify("Pagerduty Notifier", "Added to launch configuration", "", 10*time.Second)
	}
}

func appEnterLoop() {
	log.Print("Entering trayhost loop")
	trayhost.EnterLoop()
}

func appNotify(title string, message string, url string, timeout time.Duration) {

	notification := trayhost.Notification{
		Title:   title,
		Body:    message,
		Timeout: timeout,
	}

	if url != "" {
		notification.Handler = func() { openBrowser(url) }
	}

	notification.Display()
}

func removeCharacters(input string, characters string) string {
	filter := func(r rune) rune {
		if strings.IndexRune(characters, r) < 0 {
			return r
		}
		return -1
	}
	return strings.Map(filter, input)
}

func getIcon(s string) []byte {
	b, err := ioutil.ReadFile(s)
	if err != nil {
		fmt.Print(err)
	}
	return b
}

func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}
}
