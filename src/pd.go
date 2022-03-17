package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"text/template"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/go-ini/ini"
	"github.com/shurcooL/trayhost"
)

var pd *pagerduty.Client

var statuses = []string{"triggered", "acknowledged", "resolved"}
var teamIDs = []string{}
var userIDs = []string{}
var serviceIDs = []string{}

var includeFilters = []Filter{}
var excludeFilters = []Filter{}
var location = time.Local
var titleTemplate *template.Template = nil

func format(str string) (string, error) {
	date, _ := time.Parse(time.RFC3339, str)
	return date.In(location).Format("15:04"), nil
}

func pdInit(cfg *ini.File) {

	includeFilters = filterInit("include", cfg)
	excludeFilters = filterInit("exclude", cfg)
	pd = pagerduty.NewClient(cfg.Section("pagerduty").Key("token").String())
	timezone := cfg.Section("main").Key("timezone").String()
	if timezone != "" {
		var err error
		location, err = time.LoadLocation(timezone)
		if err != nil {
			log.Println("Error loading timezone information:", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()

	// get team ids
	teamList, err := pd.ListTeamsWithContext(ctx, pagerduty.ListTeamOptions{Query: ""})
	if err != nil {
		log.Println("Error: Cannot retrieve teams from Pagerduty API")
		os.Exit(1)
	}
	teamConf := make(map[string]bool)
	for _, v := range cfg.Section("pagerduty").Key("teams").Strings(",") {
		teamConf[v] = true
	}
	for _, t := range teamList.Teams {
		if teamConf[t.Name] {
			log.Printf("Found team %s with id %s", t.Name, t.ID)
			teamIDs = append(teamIDs, t.ID)
		}
	}

	// get use ids
	userList, err := pd.ListUsersWithContext(ctx, pagerduty.ListUsersOptions{Query: ""})
	if err != nil {
		log.Println("Error: Cannot retrieve user list from Pagerduty API")
		os.Exit(1)
	}
	userConf := make(map[string]bool)
	for _, v := range cfg.Section("pagerduty").Key("users").Strings(",") {
		userConf[v] = true
	}
	for _, u := range userList.Users {
		if userConf[u.Email] || userConf[u.Name] {
			log.Printf("Found user %s with id %s", u.Name, u.ID)
			userIDs = append(userIDs, u.ID)
		}
	}

	serviceConf := make(map[string]bool)
	for _, v := range cfg.Section("pagerduty").Key("services").Strings(",") {
		log.Printf("Service %s enabled in config", v)
		serviceConf[v] = true
	}

	ok := true
	opts := pagerduty.ListServiceOptions{
		Limit:  25,
		Offset: 0,
	}

	for k := range serviceConf {
		for ok {
			// get service ids that match given name
			opts.Query = k
			serviceList, err := pd.ListServicesWithContext(ctx, opts)
			if err != nil {
				log.Println("Error: Cannot retrieve service list from Pagerduty API")
				os.Exit(1)
			}
			for _, s := range serviceList.Services {
				log.Printf("Checking service %s", s.Name)
				if serviceConf[s.Name] {
					log.Printf("Found service %s with id %s", s.Name, s.ID)
					serviceIDs = append(serviceIDs, s.ID)
				}
			}
			ok = serviceList.More
			opts.Offset = opts.Offset + opts.Limit
		}
	}

	// setup and parse title template if any
	var fm = make(template.FuncMap)
	fm["format"] = format
	title := cfg.Section("pagerduty").Key("title").String()
	if title != "" {
		titleTemplate, err = template.New("title").Funcs(fm).Parse(title)
		if err != nil {
			log.Printf("Error parsing title template: %s", err)
			os.Exit(1)
		}
	}
}

func pdGetIncidents(cfg *ini.File) []pagerduty.Incident {

	lastdate := readTimestamp()
	incidents := make([]pagerduty.Incident, 0)

INCIDENTS:
	for _, i := range pdGetIncidentsSince(lastdate) {
		lastdate, _ = time.Parse(time.RFC3339, i.CreatedAt)
		log.Printf("Incident: %s", i.Summary)
		for _, team := range i.Teams {
			log.Printf("Team: %s", team.Summary)
		}
		log.Printf("Service: %s", i.Service.Summary)

		// check include filter
		if len(includeFilters) == 0 {
			goto EXCLUDES
		}
		for _, filter := range includeFilters {
			switch filter.property {
			case "service":
				if (filter.notmatch && (i.Service.Summary != filter.match)) || (!filter.notmatch && (i.Service.Summary == filter.match)) {
					if filter.filter.Find([]byte(i.Summary)) != nil {
						log.Printf("Included - service:%v, notmatch: %t, alert:<%s>", filter.match, filter.notmatch, i.Summary)
						goto EXCLUDES
					}
				}
			case "team":
				for _, t := range i.Teams {
					if (filter.notmatch && (t.Summary != filter.match)) || (!filter.notmatch && (t.Summary == filter.match)) {
						if filter.filter.Find([]byte(i.Summary)) != nil {
							log.Printf("Included - team:%v, notmatch: %t, alert:<%s>", filter.match, filter.notmatch, i.Summary)
							goto EXCLUDES
						}
					}
				}
			default:
				log.Printf("Include filter property <%s> not implemented yet.", filter.property)
			}
		}
		continue INCIDENTS

		// check exclude filter
	EXCLUDES:
		for _, filter := range excludeFilters {
			switch filter.property {
			case "service":
				if (filter.notmatch && i.Service.Summary != filter.match) || (!filter.notmatch && (i.Service.Summary == filter.match)) {
					if filter.filter.Find([]byte(i.Summary)) != nil {
						log.Printf("Excluded - service:%v, notmatch: %t, alert:<%s>", filter.match, filter.notmatch, i.Summary)
						continue INCIDENTS
					}
				}
			case "team":
				for _, t := range i.Teams {
					if (filter.notmatch && t.Summary != filter.match) || (!filter.notmatch && (t.Summary == filter.match)) {
						if filter.filter.Find([]byte(i.Summary)) != nil {
							log.Printf("Excluded - team:%v, notmatch: %t, alert:<%s>", filter.match, filter.notmatch, i.Summary)
							continue INCIDENTS
						}
					}
				}
			default:
				log.Printf("Exclude filter property <%s> not implemented yet.", filter.property)
			}
		}
		// Add incidents for notification
		incidents = append(incidents, i)
	}

	// write last timestamp for next run, add a second to make sure we don't get the same incident next time :-()
	writeTimestamp(lastdate.Add(time.Second))
	return incidents
}

func pdGetIncidentsSince(since time.Time) []pagerduty.Incident {
	incidents := make([]pagerduty.Incident, 0)

	opts := pagerduty.ListIncidentsOptions{
		Limit:      25,
		Offset:     0,
		Since:      since.Format(time.RFC3339),
		Statuses:   statuses,
		TeamIDs:    teamIDs,
		UserIDs:    userIDs,
		ServiceIDs: serviceIDs,
		SortBy:     "created_at:ASC",
		TimeZone:   "UTC",
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()

	ok := true
	for ok {
		log.Printf("API query since: %s, Limit: %v Offset: %v", since, opts.Limit, opts.Offset)
		resp, err := pd.ListIncidentsWithContext(ctx, opts)
		if err != nil {
			log.Println("Error: Cannot list incidents from Pagerduty API:", err)
			return incidents
		}
		ok = resp.More
		log.Printf("Got %d incidents", len(resp.Incidents))
		incidents = append(incidents, resp.Incidents...)
		opts.Offset = opts.Offset + opts.Limit
	}
	log.Printf("Returning %d incidents total.", len(incidents))
	return incidents
}

func pdNotify(i pagerduty.Incident) {
	date, _ := time.Parse(time.RFC3339, i.CreatedAt)
	reg := regexp.MustCompile(`\[#(\d+)\] (.+)`)

	title := fmt.Sprintf("Incident at %s (%s)", date.In(location).Format("15:04"), i.Status)
	if titleTemplate != nil {
		var tpl bytes.Buffer
		err := titleTemplate.Execute(&tpl, i)
		if err == nil {
			title = tpl.String()
		}
	}
	var message string
	match := reg.FindStringSubmatch(i.Summary)
	if match == nil {
		message = removeCharacters(i.Summary, "[]")
	} else {
		message = removeCharacters(match[2], "[]")
	}
	image := trayhost.Image{}

	if i.Urgency == "high" {
		image.Kind = "png"
		image.Bytes = getIcon("warning.png")
	}

	appNotify(title, message, i.HTMLURL, &image, 0)
}
