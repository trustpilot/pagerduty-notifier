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

	"github.com/caseymrm/menuet"
)

var pause = false
var pauseStopTime time.Time

// setup tray icon and menus
func menuItems() []menuet.MenuItem {

	items := []menuet.MenuItem{
		// {

		//     Text: "Startup on login",
		//     Clicked: func() {
		//         toggleStartup()
		//     },
		//     State: existsLaunchConf(),
		// },
		{
			Text: "Test",
			Clicked: func() {
				testNotification()
			},
			State: pause,
		},
		{
			Text: "Pause",
			Clicked: func() {
				togglePause()
			},
			State: pause,
		},
		{
			Text: "Info",
			Clicked: func() {
				openBrowser("https://github.com/trustpilot/pagerduty-notifier/blob/master/README.md")
			},
		},
		// {
		//     Text: "Quit",
		//     Clicked: func() {
		//         os.Exit(0)
		//     },
		// },
	}

	return items
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
	// iconData, err := ioutil.ReadFile("pd-bw.png")
	if err != nil {
		log.Fatalln(err)
	}

	menuet.App().Label = "com.trustpilot.pagerduty-notifier"
	menuet.App().Children = menuItems
	menuet.App().NotificationResponder = responseButton
	menuet.App().SetMenuState(&menuet.MenuState{
		// Title: "Pagerduty",
		Image: "pagerduty",
	})
}

func responseButton(id, response string) {
	log.Printf("id: %v", id)
	log.Printf("response: %s", response)

	// menuet.App().Alert(menuet.Alert{
	// 	MessageText:     "Alert",
	// 	InformativeText: response,
	// })
}

func testNotification() {
	notification := menuet.Notification{
		Title:                        "Incident 123",
		Subtitle: 					  "subtitle",
		Message:                      "P2, deadletter queue, something ..",
		ActionButton:                 "Acknowledge",
		// CloseButton:                  "Close",
		// ResponsePlaceholder:          "123",
		// RemoveFromNotificationCenter: false,
		Identifier: 				  "123",
	}

	// if url != "" {
	//     notification.Handler = func() { openBrowser(url) }
	// }

	// if image != nil {
	//     notification.Image = *image
	// }

	menuet.App().Notification(notification)

}

func togglePause() {
	if pause {
		appNotify("Pagerduty Notifier", "Unpausing notifications") //, "", nil, 10*time.Second)
		log.Println("Stop pause ...")

		pause = false
		menuet.App().MenuChanged()
		if clearOnUnpause {
			writeTimestamp(time.Now())
		}
	} else {
		msg := "Pausing notifications"
		if pauseTimeout > 0 {
			msg = fmt.Sprintf("%s for %d minutes", msg, pauseTimeout)
			pauseStopTime = time.Now().Add(time.Duration(pauseTimeout) * time.Minute)
		}
		appNotify("Pagerduty Notifier", msg) //, "", nil, 10*time.Second)
		log.Println("Start pause ...")

		pause = true
		menuet.App().MenuChanged()
	}
}
func toggleStartup() {
	if existsLaunchConf() {
		deleteLaunchConf()
		appNotify("Pagerduty Notifier", "Removed from Launch configuration") //, "", nil, 10*time.Second)
		menuet.App().MenuChanged()
	} else {
		writeLaunchConf()
		appNotify("Pagerduty Notifier", "Added to launch configuration") // , "", nil, 10*time.Second)
		menuet.App().MenuChanged()
	}
}

func appEnterLoop() {
	log.Print("Entering menuet RunApplication")
	menuet.App().RunApplication()
}

func appNotify(title string, message string) { //, url string, image *trayhost.Image, timeout time.Duration) {

	notification := menuet.Notification{
		Title:   title,
		Message: message,
	}

	// if url != "" {
	//     notification.Handler = func() { openBrowser(url) }
	// }

	// if image != nil {
	//     notification.Image = *image
	// }

	menuet.App().Notification(notification)
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
