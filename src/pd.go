package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
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

var includes = []Filter{}
var excludes = []Filter{}
var location = time.Local

func pdInit(cfg *ini.File) {

	includes = filterInit("include", cfg)
	excludes = filterInit("exclude", cfg)
	pd = pagerduty.NewClient(cfg.Section("pagerduty").Key("token").String())
	timezone := cfg.Section("pagerduty").Key("timezone").String()
	if timezone != "" {
		var err error
		location, err = time.LoadLocation(timezone)
		if err != nil {
			log.Println("Error loading timezone information:", err)
		}
	}

	// get team ids
	teamList, err := pd.ListTeams(pagerduty.ListTeamOptions{Query: ""})
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
	userList, err := pd.ListUsers(pagerduty.ListUsersOptions{Query: ""})
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

	// get service ids
	serviceList, err := pd.ListServices(pagerduty.ListServiceOptions{Query: ""})
	if err != nil {
		log.Println("Error: Cannot retrieve service list from Pagerduty API")
		os.Exit(1)
	}
	serviceConf := make(map[string]bool)
	for _, v := range cfg.Section("pagerduty").Key("services").Strings(",") {
		serviceConf[v] = true
	}
	for _, s := range serviceList.Services {
		if serviceConf[s.Name] {
			log.Printf("Found service %s with id %s", s.Name, s.ID)
			serviceIDs = append(serviceIDs, s.ID)
		}
	}
}

func pdGetIncidents(cfg *ini.File) []pagerduty.Incident {

	lastdate := readTimestamp()
	incidents := make([]pagerduty.Incident, 0)

INCIDENTS:
	for _, i := range pdGetIncidentsSince(lastdate) {
		lastdate, _ = time.Parse(time.RFC3339, i.CreatedAt)
		log.Printf("Incident: %s", i.APIObject.Summary)
		for _, team := range i.Teams {
			log.Printf("Team: %s", team.Summary)
		}
		log.Printf("Service: %s", i.Service.Summary)

		// check include filter
		if len(includes) == 0 {
			goto EXCLUDES
		}
		for _, filter := range includes {
			switch filter.property {
			case "service":
				if (filter.notmatch && i.Service.Summary != filter.match) || (!filter.notmatch && (i.Service.Summary == filter.match)) {
					if filter.filter.Find([]byte(i.APIObject.Summary)) != nil {
						log.Printf("Included - service:%v, notmatch: %t, alert:<%s>", filter.match, filter.notmatch, i.APIObject.Summary)
						goto EXCLUDES
					}
				}
			case "team":
				for _, t := range i.Teams {
					if (filter.notmatch && t.Summary != filter.match) || (!filter.notmatch && (t.Summary == filter.match)) {
						if filter.filter.Find([]byte(i.APIObject.Summary)) != nil {
							log.Printf("Included - team:%v, notmatch: %t, alert:<%s>", filter.match, filter.notmatch, i.APIObject.Summary)
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
		for _, filter := range excludes {
			switch filter.property {
			case "service":
				if (filter.notmatch && i.Service.Summary != filter.match) || (!filter.notmatch && (i.Service.Summary == filter.match)) {
					if filter.filter.Find([]byte(i.APIObject.Summary)) != nil {
						log.Printf("Excluded - service:%v, notmatch: %t, alert:<%s>", filter.match, filter.notmatch, i.APIObject.Summary)
						continue INCIDENTS
					}
				}
			case "team":
				for _, t := range i.Teams {
					if (filter.notmatch && t.Summary != filter.match) || (!filter.notmatch && (t.Summary == filter.match)) {
						if filter.filter.Find([]byte(i.APIObject.Summary)) != nil {
							log.Printf("Excluded - team:%v, notmatch: %t, alert:<%s>", filter.match, filter.notmatch, i.APIObject.Summary)
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
	resp := &pagerduty.ListIncidentsResponse{}

	opts := pagerduty.ListIncidentsOptions{
		Since:      since.Format(time.RFC3339),
		Statuses:   statuses,
		TeamIDs:    teamIDs,
		UserIDs:    userIDs,
		ServiceIDs: serviceIDs,
		SortBy:     "created_at:ASC",
		TimeZone:   "UTC",
		APIListObject: pagerduty.APIListObject{
			Limit:  25,
			Offset: 0,
		},
	}

	for ok := true; ok; ok = resp.APIListObject.More {
		log.Printf("API query since: %s, Limit: %v Offset: %v", since, opts.APIListObject.Limit, opts.APIListObject.Offset)
		resp, err := pd.ListIncidents(opts)
		if err != nil {
			log.Println("Error: Cannot list incidents from Pagerduty API:", err)
			return incidents
		}
		log.Printf("Got %d incidents", len(resp.Incidents))
		log.Printf("APIListObject %+v", resp.APIListObject)
		incidents = append(incidents, resp.Incidents...)
		opts.APIListObject.Offset = opts.APIListObject.Offset + opts.APIListObject.Limit
	}
	log.Printf("Returning %d incidents total.", len(incidents))
	return incidents
}

func pdNotify(i pagerduty.Incident) {

	var title, message string

	date, _ := time.Parse(time.RFC3339, i.CreatedAt)
	reg := regexp.MustCompile(`\[#(\d+)\] (.+)`)
	match := reg.FindStringSubmatch(i.APIObject.Summary)
	if match != nil {
		title = fmt.Sprintf("Incident %s at %s", i.Status, date.In(location).Format("15:04"))
		message = removeCharacters(match[2], "[]")
	} else {
		title = fmt.Sprintf("Incident %s", i.Status)
		message = removeCharacters(i.APIObject.Summary, "[]")
	}
	image := trayhost.Image{}

	if i.Urgency == "high" {
		image.Kind = "png"
		image.Bytes = getIcon("warning.png")
	}

	appNotify(title, message, i.APIObject.HTMLURL, &image, 0)
}
