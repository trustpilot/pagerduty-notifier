package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"log/syslog"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/go-ini/ini"
)

// config files
var timestampFile = ".pagerduty.ts"
var launchconf = "com.trustpilot.pagerduty-notifier.plist"

type Filter struct {
	property string
	match    string
	notmatch bool
	filter   *regexp.Regexp
}

func filterInit(filtertype string, cfg *ini.File) []Filter {
	var list []Filter
	for _, key := range cfg.Section(filtertype).KeyStrings() {
		filter, err := regexp.Compile(cfg.Section(filtertype).Key(key).String())
		if err != nil {
			log.Printf("Error compiling regular expression <%s> : %s", cfg.Section(filtertype).Key(key).String(), err)
			filter = nil
		}
		s := strings.Split(key, ".")
		property, match := s[0], s[1]
		notmatch := strings.HasPrefix(match, "!")
		if notmatch {
			match = strings.Replace(match, "!", "", 1)
		}
		list = append(list, Filter{property: property, match: match, notmatch: notmatch, filter: filter})
	}
	return list
}

func logInit(out string) {
	switch out {
	case "syslog":
		// setup logger to use syslog
		logwriter, e := syslog.New(syslog.LOG_NOTICE, "pagerdutynotifier")
		if e == nil {
			log.SetOutput(logwriter)
		}
	case "stdout":
		log.SetOutput(os.Stdout)
	}
}

func cfgInit() (*ini.File, error) {

	var configFile string

	// find the HOME/.pagerduty.ini file
	switch runtime.GOOS {
	case "darwin", "linux":
		configFile = fmt.Sprintf("%s/.pagerduty.ini", os.Getenv("HOME"))
		timestampFile = fmt.Sprintf("%s/.pagerduty.ts", os.Getenv("HOME"))
	case "windows":
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		configFile = fmt.Sprintf("%s/.pagerduty.ini", home)
		timestampFile = fmt.Sprintf("%s/.pagerduty.ts", home)
	default:
	}

	// test if we can read the config file, if it doesn't exist we create one from template and notify about it.
	_, err := ini.Load(configFile)
	if err != nil {
		switch runtime.GOOS {
		case "linux":
			appNotify(configFile, "No ini file found, click here to see how.", "https://github.com/trustpilot/pagerduty-notifier", nil, 0)
			return nil, fmt.Errorf("no ini file found at %s", configFile)
		case "windows":
			appNotify(configFile, "No ini file found, click here to see how.", "https://github.com/trustpilot/pagerduty-notifier", nil, 0)
			return nil, fmt.Errorf("no ini file found at %s", configFile)
		case "darwin":
			input, err := ioutil.ReadFile("template.ini")
			if err != nil {
				return nil, fmt.Errorf("no ini file found at %s and template.ini could not be loaded either", configFile)
			}
			err = ioutil.WriteFile(configFile, input, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to create %s from template.ini: %w", configFile, err)
			}

			appNotify(
				"HOME/.pagerduty.ini", "Created default config file, please edit and add valid PagerDuty token!",
				"https://github.com/trustpilot/pagerduty-notifier", nil, 0)
			os.Exit(0)

		default:
			return nil, fmt.Errorf("unsupported platform %q", runtime.GOOS)
		}
	}

	// init pagerduty api
	cfg, err := ini.Load(configFile)
	if err != nil {
		appNotify(
			configFile, fmt.Sprintf("error reading config file %s: %v", configFile, err),
			"https://github.com/trustpilot/pagerduty-notifier", nil, 0)
		return nil, fmt.Errorf("error reading config file %s: %w", configFile, err)
	}
	return cfg, nil
}

func readTimestamp() time.Time {
	var lastdate time.Time
	// read the (optional) last timestamp
	timestamp, err := ioutil.ReadFile(timestampFile)
	if err == nil {
		lastdate, err = time.Parse(time.RFC3339, string(timestamp))
		if err != nil {
			log.Printf("Error parsing timestamp file <%s>, returning 12 hours ago", timestampFile)
			return time.Now().Add(time.Duration(-12) * time.Hour)
		}
		return lastdate
	}
	log.Printf("Error reading timestamp file <%s>, returning 12 hours ago", timestampFile)
	return time.Now().Add(time.Duration(-12) * time.Hour)
}

func writeTimestamp(timestamp time.Time) {

	err := ioutil.WriteFile(timestampFile, []byte(timestamp.Format(time.RFC3339)), 0644)
	if err != nil {
		log.Println("Error writing timestamp to file.")
	}
}

func writeLaunchConf() error {
	src := launchconf
	dst := fmt.Sprintf("%s/Library/LaunchAgents/%s", os.Getenv("HOME"), launchconf)

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}

func existsLaunchConf() bool {
	dst := fmt.Sprintf("%s/Library/LaunchAgents/%s", os.Getenv("HOME"), launchconf)
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return false
	}
	return true
}

func deleteLaunchConf() error {
	dst := fmt.Sprintf("%s/Library/LaunchAgents/%s", os.Getenv("HOME"), launchconf)
	return os.Remove(dst)
}
