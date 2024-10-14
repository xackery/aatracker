package dps

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xackery/aatracker/tracker"
)

type DPS struct {
	lastZoneEvent    time.Time
	parseStart       time.Time
	zone             string
	damageEvents     map[string][]DamageEvent
	zoneDamageTotals map[string]*DamageReport
	lastDPSEvent     time.Time
	lastDPSDump      time.Time
}

type DamageEvent struct {
	Source string
	Target string
	Type   string
	Damage int
	Event  time.Time
	Origin string
}

type DamageReport struct {
	Total  int
	Melee  int
	Direct int
	Dot    int
}

var (
	instance          *DPS
	zoneRegex         = regexp.MustCompile(`You have entered (.*)`)
	meleeDamageRegex  = regexp.MustCompile(`\] (.*) for (.*) points of damage.`)
	directDamageRegex = regexp.MustCompile(`\] (.*) for (.*) points of non-melee damage.`)
	dotDamageRegex    = regexp.MustCompile(`\] (.*) has taken (.*) damage from your (.*)`)
)

func New() (*DPS, error) {
	if instance != nil {
		return nil, fmt.Errorf("dps already exists")
	}
	a := &DPS{
		zone:             "Unknown",
		lastZoneEvent:    time.Now(),
		parseStart:       time.Now(),
		damageEvents:     make(map[string][]DamageEvent),
		zoneDamageTotals: make(map[string]*DamageReport),
	}

	err := tracker.Subscribe(a.onLine)
	if err != nil {
		return nil, fmt.Errorf("tracker subscribe: %w", err)
	}
	instance = a
	return a, nil
}

func (a *DPS) onLine(event time.Time, line string) {
	if !tracker.IsLiveParse() {
		return
	}
	a.onZone(event, line)
	a.onMeleeDPS(event, line)
	a.onDirectDamageDPS(event, line)
	a.onDotDamageDPS(event, line)

	if a.lastDPSEvent.IsZero() {
		a.lastDPSDump = time.Now()
	}

	if time.Since(a.lastDPSDump) > 1*time.Minute {
		a.dumpDPS(event)
		a.lastDPSDump = time.Now()
	}
}

func (a *DPS) onZone(event time.Time, line string) {
	match := zoneRegex.FindStringSubmatch(line)
	if len(match) < 2 {
		return
	}
	a.lastZoneEvent = event
	a.zone = match[1]
	a.zoneDamageTotals = make(map[string]*DamageReport)

	a.dumpDPS(event)
}

func (a *DPS) dumpDPS(event time.Time) {
	//dpsPerSec := float64(a.totalDPSGained) / time.Since(a.parseStart).Seconds()
	//dpsPerHour := dpsPerSec * 3600

	if a.zone == "The Bazaar" {
		return
	}

	fmt.Printf("==DPS Report from %s to %s (%d)==\n", a.lastDPSDump.Format("15:04:05"), event.Format("15:04:05"), len(a.damageEvents))
	if len(a.damageEvents) == 0 {
		fmt.Printf("No new DPS events\n")
	}

	totalReceived := make(map[string]int)

	dpsMarker := float64(time.Since(a.lastZoneEvent).Seconds())
	dps60Marker := float64(60)

	for name, dmgEvents := range a.damageEvents {
		totalDamage := 0
		for _, dmgEvent := range dmgEvents {
			//fmt.Printf("%s %s %s for %d at %s src %s\n", dmgEvent.Source, dmgEvent.Type, dmgEvent.Target, dmgEvent.Damage, dmgEvent.Event.Format("15:04:05"), dmgEvent.Origin)
			totalDamage += dmgEvent.Damage
			_, ok := totalReceived[dmgEvent.Target]
			if !ok {
				totalReceived[dmgEvent.Target] = 0
			}
			totalReceived[dmgEvent.Target] += dmgEvent.Damage

			_, ok = a.zoneDamageTotals[dmgEvent.Source]
			if !ok {
				a.zoneDamageTotals[dmgEvent.Source] = &DamageReport{}
			}
			a.zoneDamageTotals[dmgEvent.Source].Total += dmgEvent.Damage
			if dmgEvent.Origin == "melee" {
				a.zoneDamageTotals[dmgEvent.Source].Melee += dmgEvent.Damage
			} else if dmgEvent.Origin == "direct" {
				a.zoneDamageTotals[dmgEvent.Source].Direct += dmgEvent.Damage
			} else if dmgEvent.Origin == "dot" {
				a.zoneDamageTotals[dmgEvent.Source].Dot += dmgEvent.Damage
			}
		}
		//fmt.Printf("%s: %d damage, %d events, %.2f dps, %.2f dps/hour\n", name, totalDamage, len(dmgEvents), dpsPerSec, dpsPerHour)
		fmt.Printf("%s: %.2f dps, %d damage\n", name, float64(totalDamage)/dps60Marker, totalDamage)
	}

	a.damageEvents = make(map[string][]DamageEvent)

	fmt.Printf("==Total Damage Received since %s (%.2f minutes, %0.2f seconds)==\n", a.lastZoneEvent.Format("15:04:05"), time.Since(a.lastZoneEvent).Minutes(), time.Since(a.lastZoneEvent).Seconds())
	for name, totalDamage := range totalReceived {
		fmt.Printf("%s: %.2f dps, %d damage received\n", name, float64(totalDamage)/dpsMarker, totalDamage)
	}

	fmt.Printf("==Zone %s Damage Totals since %s (%.2f minutes)==\n", a.zone, a.lastZoneEvent.Format("15:04:05"), time.Since(a.lastZoneEvent).Minutes())
	for name, totalDamage := range a.zoneDamageTotals {
		total := float64(totalDamage.Total)

		// Avoid division by zero if total damage is 0
		meleePct := 0.0
		directPct := 0.0
		dotPct := 0.0

		if total > 0 {
			meleePct = (float64(totalDamage.Melee) / total) * 100
			directPct = (float64(totalDamage.Direct) / total) * 100
			dotPct = (float64(totalDamage.Dot) / total) * 100
		}

		fmt.Printf("%s: %.2f dps, %d damage (", name, total/dpsMarker, totalDamage.Total)
		if meleePct > 0 {
			fmt.Printf("%.0f%% melee", meleePct)
		}
		if directPct > 0 {
			if meleePct > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%.0f%% direct", directPct)
		}
		if dotPct > 0 {
			if meleePct > 0 || directPct > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%.0f%% dot", dotPct)
		}
		fmt.Printf(")\n")
	}
}

func (a *DPS) onMeleeDPS(event time.Time, line string) {
	match := meleeDamageRegex.FindStringSubmatch(line)
	if len(match) < 3 {
		return
	}

	amount, err := strconv.Atoi(match[2])
	if err != nil {
		return
	}

	chunk := match[1]

	if strings.Contains(chunk, " was hit ") {
		return
	}

	pos := 0
	pickedAdj := ""
	for _, adj := range adjectives {
		pos = strings.Index(chunk, adj)
		if pos <= 0 {
			continue
		}
		pickedAdj = adj
		break
	}
	if pos <= 0 {
		return
	}

	source := chunk[:pos]
	if strings.EqualFold(source, "you") {
		source = tracker.PlayerName()
	}
	target := chunk[pos+len(pickedAdj):]

	damageEvent := DamageEvent{
		Source: source,
		Target: target,
		Type:   strings.TrimSpace(pickedAdj),
		Damage: amount,
		Event:  event,
		Origin: "melee",
	}

	_, ok := a.damageEvents[damageEvent.Source]
	if !ok {
		a.damageEvents[damageEvent.Source] = make([]DamageEvent, 0)
	}

	a.lastDPSEvent = event

	a.damageEvents[damageEvent.Source] = append(a.damageEvents[damageEvent.Source], damageEvent)
}

func (a *DPS) onDirectDamageDPS(event time.Time, line string) {
	match := directDamageRegex.FindStringSubmatch(line)
	if len(match) < 3 {
		return
	}

	amount, err := strconv.Atoi(match[2])
	if err != nil {
		return
	}

	chunk := match[1]

	pos := 0
	pickedAdj := ""
	for _, adj := range adjectives {
		pos = strings.Index(chunk, adj)
		if pos <= 0 {
			continue
		}
		pickedAdj = adj
		break
	}
	if pos <= 0 {
		return
	}

	source := chunk[:pos]
	target := chunk[pos+len(pickedAdj):]

	damageEvent := DamageEvent{
		Source: source,
		Target: target,
		Type:   strings.TrimSpace(pickedAdj),
		Damage: amount,
		Event:  event,
		Origin: "direct",
	}

	_, ok := a.damageEvents[damageEvent.Source]
	if !ok {
		a.damageEvents[damageEvent.Source] = make([]DamageEvent, 0)
	}

	a.lastDPSEvent = event

	a.damageEvents[damageEvent.Source] = append(a.damageEvents[damageEvent.Source], damageEvent)
}

func (a *DPS) onDotDamageDPS(event time.Time, line string) {
	match := dotDamageRegex.FindStringSubmatch(line)
	if len(match) < 3 {
		return
	}

	amount, err := strconv.Atoi(match[2])
	if err != nil {
		return
	}

	source := tracker.PlayerName()
	target := match[1]

	damageEvent := DamageEvent{
		Source: source,
		Target: target,
		Type:   match[3][0 : len(match[3])-2],
		Damage: amount,
		Event:  event,
		Origin: "dot",
	}

	_, ok := a.damageEvents[damageEvent.Source]
	if !ok {
		a.damageEvents[damageEvent.Source] = make([]DamageEvent, 0)
	}

	a.lastDPSEvent = event

	a.damageEvents[damageEvent.Source] = append(a.damageEvents[damageEvent.Source], damageEvent)
}

var adjectives = []string{
	" mauls ",
	" maul ",
	" bites ",
	" bite ",
	" claws ",
	" claw ",
	" gores ",
	" gore ",
	" stings ",
	" slices ",
	" slice ",
	" sting ",
	" smashes ",
	" smash ",
	" rend ",
	" rends ",
	" slash ",
	" slashes ",
	" punch ",
	" punches ",
	" hit ",
	" hits ",
	" You ",
	" yourself ",
	" YOU ",
	" himself ",
	" herself ",
	" itself ",
	" crush ",
	" crushes ",
	" pierce ",
	" pierces ",
	" kick ",
	" kicks ",
	" strike ",
	" strikes ",
	" backstab ",
	" backstabs ",
	" bash ",
	" bashes ",
}
