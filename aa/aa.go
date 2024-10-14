package aa

import (
	"fmt"
	"regexp"
	"time"

	"github.com/xackery/aatracker/tracker"
)

type AA struct {
	lastZoneEvent time.Time
	lastAAExpDing time.Time
	parseStart    time.Time
	totalAAGained int
	zone          string
}

var (
	instance    *AA
	zoneRegex   = regexp.MustCompile(`You have entered (.*)`)
	aaGainRegex = regexp.MustCompile(`You have gained an ability point!`)
)

func New() (*AA, error) {
	if instance != nil {
		return nil, fmt.Errorf("aa already exists")
	}
	a := &AA{
		zone:          "Unknown",
		lastZoneEvent: time.Now(),
		parseStart:    time.Now(),
	}

	err := tracker.Subscribe(a.onLine)
	if err != nil {
		return nil, fmt.Errorf("tracker subscribe: %w", err)
	}
	instance = a
	return a, nil
}

func (a *AA) onLine(event time.Time, line string) {
	a.onZone(event, line)
	a.onAA(event, line)
}

func (a *AA) onZone(event time.Time, line string) {
	match := zoneRegex.FindStringSubmatch(line)
	if len(match) < 2 {
		return
	}
	if match[1] == "The Bazaar" {
		return
	}
	a.lastZoneEvent = event
	a.zone = match[1]
	a.totalAAGained = 0

	fmt.Println("You have entered", match[1])
}

func (a *AA) onAA(event time.Time, line string) {
	match := aaGainRegex.FindStringSubmatch(line)
	if len(match) < 1 {
		return
	}
	sinceLastDing := event.Sub(a.lastAAExpDing).Minutes()
	a.lastAAExpDing = event
	a.totalAAGained++

	elapsedTime := event.Sub(a.lastZoneEvent).Hours()
	if elapsedTime <= 0 {
		return
	}
	aaPerHour := float64(a.totalAAGained) / elapsedTime
	fmt.Printf("Total AA gained: %d / per hour: %.2f, Time in zone: %0.2f hours, last AA ding: %0.2f minutes\n", a.totalAAGained, aaPerHour, elapsedTime, sinceLastDing)
}
