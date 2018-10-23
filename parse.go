package ics

import (
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type traceErrFunc func(err error) bool
type timezoneLocationError struct {
	location string
}
type timezoneLocationCompatibilityError struct {
	originalLocation      string
	compatibilityLocation string
}

func (e *timezoneLocationError) Error() string { return e.location }
func (e *timezoneLocationCompatibilityError) Error() string {
	return fmt.Sprintf("%s mapped to %s", e.originalLocation, e.compatibilityLocation)
}

var (
	urlRegex                           = regexp.MustCompile(`https?:\/\/`)
	eventsRegex                        = regexp.MustCompile(`(BEGIN:VEVENT(.*\n)*?END:VEVENT\r?\n)`)
	timezoneLocationCompatibilityRegex = regexp.MustCompile(`\s[0-9]`)

	calNameRegex     = regexp.MustCompile(`X-WR-CALNAME:.*?\n`)
	calDescRegex     = regexp.MustCompile(`X-WR-CALDESC:.*?\n`)
	calVersionRegex  = regexp.MustCompile(`VERSION:.*?\n`)
	calTimezoneRegex = regexp.MustCompile(`X-WR-TIMEZONE:.*?\n`)

	eventSummaryRegex      = regexp.MustCompile(`SUMMARY:.*?\n`)
	eventStatusRegex       = regexp.MustCompile(`STATUS:.*?\n`)
	eventDescRegex         = regexp.MustCompile(`DESCRIPTION:.*?\n`)
	eventUIDRegex          = regexp.MustCompile(`UID:.*?\n`)
	eventClassRegex        = regexp.MustCompile(`CLASS:.*?\n`)
	eventSequenceRegex     = regexp.MustCompile(`SEQUENCE:.*?\n`)
	eventCreatedRegex      = regexp.MustCompile(`CREATED:.*?\n`)
	eventModifiedRegex     = regexp.MustCompile(`LAST-MODIFIED:.*?\n`)
	eventRecurrenceIDRegex = regexp.MustCompile(`RECURRENCE-ID(;TZID=.*?){0,1}:.*?\n`)
	eventDateRegex         = regexp.MustCompile(`(DTSTART|DTEND).+\n`)
	eventTimeRegex         = regexp.MustCompile(`(DTSTART|DTEND)(;TZID=.*?){0,1}:.*?\n`)
	eventWholeDayRegex     = regexp.MustCompile(`(DTSTART|DTEND);VALUE=DATE:.*?\n`)
	eventEndRegex          = regexp.MustCompile(`DTEND(;TZID=.*?){0,1}:.*?\n`)
	eventEndWholeDayRegex  = regexp.MustCompile(`DTEND;VALUE=DATE:.*?\n`)
	eventRRuleRegex        = regexp.MustCompile(`RRULE:.*?\n`)
	eventLocationRegex     = regexp.MustCompile(`LOCATION:.*?\n`)
	eventExDateRegex       = regexp.MustCompile(`EXDATE;TZID=(.*):(.*)\n`)

	attendeesRegex = regexp.MustCompile(`ATTENDEE(:|;)(.*?\r?\n)(\s.*?\r?\n)*`)
	organizerRegex = regexp.MustCompile(`ORGANIZER(:|;)(.*?\r?\n)(\s.*?\r?\n)*`)

	attendeeEmailRegex  = regexp.MustCompile(`mailto:.*?\n`)
	attendeeStatusRegex = regexp.MustCompile(`PARTSTAT=.*?;`)
	attendeeRoleRegex   = regexp.MustCompile(`ROLE=.*?;`)
	attendeeNameRegex   = regexp.MustCompile(`CN=.*?;`)
	organizerNameRegex  = regexp.MustCompile(`CN=.*?:`)
	attendeeTypeRegex   = regexp.MustCompile(`CUTYPE=.*?;`)

	untilRegex    = regexp.MustCompile(`UNTIL=(\d)*T(\d)*Z(;){0,1}`)
	intervalRegex = regexp.MustCompile(`INTERVAL=(\d)*(;){0,1}`)
	countRegex    = regexp.MustCompile(`COUNT=(\d)*(;){0,1}`)
	freqRegex     = regexp.MustCompile(`FREQ=.*?;`)
	byMonthRegex  = regexp.MustCompile(`BYMONTH=.*?;`)
	byDayRegex    = regexp.MustCompile(`BYDAY=.*?(;|){0,1}\z`)

	nonStandardTimezones = map[string]string{
		"Egypt Standard Time":             "Africa/Cairo",
		"Morocco Standard Time":           "Africa/Casablanca",
		"South Africa Standard Time":      "Africa/Johannesburg",
		"W. Central Africa Standard Time": "Africa/Lagos",
		"E. Africa Standard Time":         "Africa/Nairobi",
		"Libya Standard Time":             "Africa/Tripoli",
		"Namibia Standard Time":           "Africa/Windhoek",
		"Aleutian Standard Time":          "America/Adak",
		"Alaskan Standard Time":           "America/Anchorage",
		"Tocantins Standard Time":         "America/Araguaina",
		"Paraguay Standard Time":          "America/Asuncion",
		"Bahia Standard Time":             "America/Bahia",
		"SA Pacific Standard Time":        "America/Bogota",
		"Argentina Standard Time":         "America/Buenos_Aires",
		"Eastern Standard Time (Mexico)":  "America/Cancun",
		"Venezuela Standard Time":         "America/Caracas",
		"SA Eastern Standard Time":        "America/Cayenne",
		"Central Standard Time":           "America/Chicago",
		"Mountain Standard Time (Mexico)": "America/Chihuahua",
		"Central Brazilian Standard Time": "America/Cuiaba",
		"Mountain Standard Time":          "America/Denver",
		"Greenland Standard Time":         "America/Godthab",
		"Turks And Caicos Standard Time":  "America/Grand_Turk",
		"Central America Standard Time":   "America/Guatemala",
		"Atlantic Standard Time":          "America/Halifax",
		"Cuba Standard Time":              "America/Havana",
		"US Eastern Standard Time":        "America/Indianapolis",
		"SA Western Standard Time":        "America/La_Paz",
		"Pacific Standard Time":           "America/Los_Angeles",
		"Central Standard Time (Mexico)":  "America/Mexico_City",
		"Saint Pierre Standard Time":      "America/Miquelon",
		"Montevideo Standard Time":        "America/Montevideo",
		"Eastern Standard Time":           "America/New_York",
		"US Mountain Standard Time":       "America/Phoenix",
		"Haiti Standard Time":             "America/Port-au-Prince",
		"Magallanes Standard Time":        "America/Punta_Arenas",
		"Canada Central Standard Time":    "America/Regina",
		"Pacific SA Standard Time":        "America/Santiago",
		"E. South America Standard Time":  "America/Sao_Paulo",
		"Newfoundland Standard Time":      "America/St_Johns",
		"Pacific Standard Time (Mexico)":  "America/Tijuana",
		"Central Asia Standard Time":      "Asia/Almaty",
		"Jordan Standard Time":            "Asia/Amman",
		"Arabic Standard Time":            "Asia/Baghdad",
		"Azerbaijan Standard Time":        "Asia/Baku",
		"SE Asia Standard Time":           "Asia/Bangkok",
		"Altai Standard Time":             "Asia/Barnaul",
		"Middle East Standard Time":       "Asia/Beirut",
		"India Standard Time":             "Asia/Calcutta",
		"Transbaikal Standard Time":       "Asia/Chita",
		"Sri Lanka Standard Time":         "Asia/Colombo",
		"Syria Standard Time":             "Asia/Damascus",
		"Bangladesh Standard Time":        "Asia/Dhaka",
		"Arabian Standard Time":           "Asia/Dubai",
		"West Bank Standard Time":         "Asia/Hebron",
		"W. Mongolia Standard Time":       "Asia/Hovd",
		"North Asia East Standard Time":   "Asia/Irkutsk",
		"Israel Standard Time":            "Asia/Jerusalem",
		"Afghanistan Standard Time":       "Asia/Kabul",
		"Russia Time Zone 11":             "Asia/Kamchatka",
		"Pakistan Standard Time":          "Asia/Karachi",
		"Nepal Standard Time":             "Asia/Katmandu",
		"North Asia Standard Time":        "Asia/Krasnoyarsk",
		"Magadan Standard Time":           "Asia/Magadan",
		"N. Central Asia Standard Time":   "Asia/Novosibirsk",
		"Omsk Standard Time":              "Asia/Omsk",
		"North Korea Standard Time":       "Asia/Pyongyang",
		"Myanmar Standard Time":           "Asia/Rangoon",
		"Arab Standard Time":              "Asia/Riyadh",
		"Sakhalin Standard Time":          "Asia/Sakhalin",
		"Korea Standard Time":             "Asia/Seoul",
		"China Standard Time":             "Asia/Shanghai",
		"Singapore Standard Time":         "Asia/Singapore",
		"Russia Time Zone 10":             "Asia/Srednekolymsk",
		"Taipei Standard Time":            "Asia/Taipei",
		"West Asia Standard Time":         "Asia/Tashkent",
		"Georgian Standard Time":          "Asia/Tbilisi",
		"Iran Standard Time":              "Asia/Tehran",
		"Tokyo Standard Time":             "Asia/Tokyo",
		"Tomsk Standard Time":             "Asia/Tomsk",
		"Ulaanbaatar Standard Time":       "Asia/Ulaanbaatar",
		"Vladivostok Standard Time":       "Asia/Vladivostok",
		"Yakutsk Standard Time":           "Asia/Yakutsk",
		"Ekaterinburg Standard Time":      "Asia/Yekaterinburg",
		"Caucasus Standard Time":          "Asia/Yerevan",
		"Azores Standard Time":            "Atlantic/Azores",
		"Cape Verde Standard Time":        "Atlantic/Cape_Verde",
		"Greenwich Standard Time":         "Atlantic/Reykjavik",
		"Cen. Australia Standard Time":    "Australia/Adelaide",
		"E. Australia Standard Time":      "Australia/Brisbane",
		"AUS Central Standard Time":       "Australia/Darwin",
		"Aus Central W. Standard Time":    "Australia/Eucla",
		"Tasmania Standard Time":          "Australia/Hobart",
		"Lord Howe Standard Time":         "Australia/Lord_Howe",
		"W. Australia Standard Time":      "Australia/Perth",
		"AUS Eastern Standard Time":       "Australia/Sydney",
		"UTC":                             "Etc/GMT",
		"UTC-11":                          "Etc/GMT+11",
		"Dateline Standard Time":          "Etc/GMT+12",
		"UTC-02":                          "Etc/GMT+2",
		"UTC-08":                          "Etc/GMT+8",
		"UTC-09":                          "Etc/GMT+9",
		"UTC+12":                          "Etc/GMT-12",
		"UTC+13":                          "Etc/GMT-13",
		"Astrakhan Standard Time":         "Europe/Astrakhan",
		"W. Europe Standard Time":         "Europe/Berlin",
		"GTB Standard Time":               "Europe/Bucharest",
		"Central Europe Standard Time":    "Europe/Budapest",
		"E. Europe Standard Time":         "Europe/Chisinau",
		"Turkey Standard Time":            "Europe/Istanbul",
		"Kaliningrad Standard Time":       "Europe/Kaliningrad",
		"FLE Standard Time":               "Europe/Kiev",
		"GMT Standard Time":               "Europe/London",
		"Belarus Standard Time":           "Europe/Minsk",
		"Russian Standard Time":           "Europe/Moscow",
		"Romance Standard Time":           "Europe/Paris",
		"Russia Time Zone 3":              "Europe/Samara",
		"Saratov Standard Time":           "Europe/Saratov",
		"Central European Standard Time":  "Europe/Warsaw",
		"Mauritius Standard Time":         "Indian/Mauritius",
		"Samoa Standard Time":             "Pacific/Apia",
		"New Zealand Standard Time":       "Pacific/Auckland",
		"Bougainville Standard Time":      "Pacific/Bougainville",
		"Chatham Islands Standard Time":   "Pacific/Chatham",
		"Easter Island Standard Time":     "Pacific/Easter",
		"Fiji Standard Time":              "Pacific/Fiji",
		"Central Pacific Standard Time":   "Pacific/Guadalcanal",
		"Hawaiian Standard Time":          "Pacific/Honolulu",
		"Line Islands Standard Time":      "Pacific/Kiritimati",
		"Marquesas Standard Time":         "Pacific/Marquesas",
		"Norfolk Standard Time":           "Pacific/Norfolk",
		"West Pacific Standard Time":      "Pacific/Port_Moresby",
		"Tonga Standard Time":             "Pacific/Tongatapu",
		// Additional non-standard timezones
		"Mexico Standard Time 2":                                  "America/Chihuahua",
		"E. South America Standard Time 1":                        "America/Sao_Paulo",
		"U.S. Mountain Standard Time":                             "America/Phoenix",
		"U.S. Eastern Standard Time":                              "America/Indianapolis",
		"S.A. Pacific Standard Time":                              "America/Bogota",
		"S.A. Western Standard Time":                              "America/La_Paz",
		"Pacific S.A. Standard Time":                              "America/Santiago",
		"Newfoundland and Labrador Standard Time":                 "America/St_Johns",
		"S.A. Eastern Standard Time":                              "America/Cayenne",
		"Mid-Atlantic Standard Time":                              "Atlantic/South_Georgia",
		"Transitional Islamic State of Afghanistan Standard Time": "Asia/Kabul",
		"S.E. Asia Standard Time":                                 "Asia/Bangkok",
		"A.U.S. Central Standard Time":                            "Australia/Darwin",
		"A.U.S. Eastern Standard Time":                            "Australia/Sydney",
		"Fiji Islands Standard Time":                              "Pacific/Fiji",
		"Azerbaijan Standard Time ":                               "America/Buenos_Aires",
		"Armenian Standard Time":                                  "Asia/Yerevan",
		"Kamchatka Standard Time":                                 "Asia/Kamchatka",
	}
)

// ParseCalendar parses the calendar in the given url (can be a local path)
// and returns the parsed calendar with its events. If maxRepeats is greater
// than 0 new events will be added if an event has a repetition rule up to
// maxRepeats. If you pass a non-nil io.Writer the contents of the ics file
// will also be written to that writer.
func ParseCalendar(url string, maxRepeats int, w io.Writer) (Calendar, error) {
	content, err := getICal(url)
	if err != nil {
		return Calendar{}, err
	}

	if w != nil {
		if _, err := io.WriteString(w, content); err != nil {
			return Calendar{}, err
		}
	}

	return ParseICalContent(content, url, maxRepeats, false, nil)
}

func getICal(url string) (string, error) {
	var (
		isRemote = urlRegex.FindString(url) != ""
		content  string
		err      error
	)

	if isRemote {
		content, err = downloadFromURL(url)
		if err != nil {
			return "", err
		}
	} else {
		if !fileExists(url) {
			return "", fmt.Errorf("file %s does not exists", url)
		}

		contentBytes, err := ioutil.ReadFile(url)
		if err != nil {
			return "", err
		}
		content = string(contentBytes)
	}

	return content, nil
}

// ParseICalContent parses the calendar content as a string.
// An optional error tracing function can be passed.
func ParseICalContent(content, url string, maxRepeats int, convertDatesToUTC bool, fn traceErrFunc) (Calendar, error) {
	cal := NewCalendar()
	eventsData, info := explodeICal(content)
	cal.Name = parseICalName(info)
	cal.Description = parseICalDesc(info)
	cal.Version = parseICalVersion(info)
	cal.Timezone = parseICalTimezone(info)
	cal.URL = url

	if fn == nil {
		fn = func(err error) bool { return false }
	}

	cal.TraceErrFunc = fn
	cal.convertDatesToUTC = convertDatesToUTC
	err := parseEvents(&cal, eventsData, maxRepeats)
	if err != nil {
		return cal, err
	}

	return cal, nil
}

func explodeICal(content string) ([]string, string) {
	events := eventsRegex.FindAllString(content, -1)
	info := eventsRegex.ReplaceAllString(content, "")
	return events, info
}

func parseICalName(content string) string {
	return trimField(calNameRegex.FindString(content), "X-WR-CALNAME:")
}

func parseICalDesc(content string) string {
	return trimField(calDescRegex.FindString(content), "X-WR-CALDESC:")
}

func parseICalVersion(content string) float64 {
	version, _ := strconv.ParseFloat(trimField(calVersionRegex.FindString(content), "VERSION:"), 64)
	return version
}

func parseICalTimezone(content string) *time.Location {
	timezone := trimField(calTimezoneRegex.FindString(content), "X-WR-TIMEZONE:")
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Local
	}

	return loc
}

func eventIsDuplicated(events []Event, event *Event) (int, bool) {
	for i, e := range events {
		if event.Equals(&e) {
			return i, true
		}
	}
	return 0, false
}

func diff(events []Event, events2 []Event) []Event {
	var result []Event
OuterLoop:
	for _, e := range events {
		for _, e2 := range events2 {
			if (&e).Equals(&e2) {
				continue OuterLoop
			}
		}
		result = append(result, e)
	}
	return result
}

func parseEvents(cal *Calendar, eventsData []string, maxRepeats int) error {
	var excluded []Event
	for _, eventData := range eventsData {
		event := NewEvent()

		start, startTz, err := parseEventDate("DTSTART", eventData)
		if err != nil {
			if _, ok := err.(*timezoneLocationError); ok {
				cal.TraceErrFunc(fmt.Errorf("Unmapped timezone location '%s' for iCal '%s'. Falling back to UTC", err.Error(), cal.URL))
			} else if _, ok := err.(*timezoneLocationCompatibilityError); ok {
				cal.TraceErrFunc(fmt.Errorf("Compatibility mode used, '%s' for iCal '%s'", err.Error(), cal.URL))
			} else {
				return err
			}
		}

		end, endTz, err := parseEventDate("DTEND", eventData)
		if err != nil {
			if _, ok := err.(*timezoneLocationError); ok {
				cal.TraceErrFunc(fmt.Errorf("Unmapped timezone location '%s' for iCal '%s'. Falling back to UTC", err.Error(), cal.URL))
			} else if _, ok := err.(*timezoneLocationCompatibilityError); ok {
				cal.TraceErrFunc(fmt.Errorf("Compatibility mode used, '%s' for iCal '%s'", err.Error(), cal.URL))
			} else {
				return err
			}
		}

		if startTz == nil {
			startTz = cal.Timezone
		}

		if endTz == nil {
			endTz = cal.Timezone
		}

		if end.IsZero() {
			end = time.Date(start.Year(), start.Month(), start.Day(), 23, 59, 59, 0, start.Location())
		}

		if cal.convertDatesToUTC {
			start = start.UTC()
			end = end.UTC()
		}

		wholeDay := start.Hour() == 0 && end.Hour() == 0 && start.Minute() == 0 && end.Minute() == 0 && start.Second() == 0 && end.Second() == 0

		event.Status = parseEventStatus(eventData)
		event.Summary = parseEventSummary(eventData)
		event.Description = parseEventDescription(eventData)
		event.ID = parseEventID(eventData)
		event.Class = parseEventClass(eventData)
		event.Sequence = parseEventSequence(eventData)
		event.Created = parseEventCreated(eventData)
		event.Modified = parseEventModified(eventData)
		event.RRule = parseEventRRule(eventData)
		exclusions, err := parseExcludedDates(eventData, cal.convertDatesToUTC)
		if err != nil {
			return err
		}
		event.ExDates = exclusions
		event.RecurrenceID, _, err = parseEventRecurrenceID(eventData)
		if err != nil {
			return err
		}

		event.Location = parseEventLocation(eventData)
		event.Start = start
		event.End = end
		event.StartTimezone = startTz
		event.EndTimezone = endTz
		event.WholeDayEvent = wholeDay
		event.Attendees = parseEventAttendees(eventData)
		event.Organizer = parseEventOrganizer(eventData)
		duration := end.Sub(start)
		cal.Events = append(cal.Events, *event)

		if maxRepeats > 0 && event.RRule != "" {
			until := parseUntil(event.RRule)
			interval := parseInterval(event.RRule)
			count := parseCount(event.RRule, maxRepeats)
			freq := trimField(freqRegex.FindString(event.RRule), `(FREQ=|;)`)
			byMonth := trimField(byMonthRegex.FindString(event.RRule), `(BYMONTH=|;)`)
			byDay := trimField(byDayRegex.FindString(event.RRule), `(BYDAY=|;)`)

			var years, days, months int

			switch freq {
			case "DAILY":
				days = interval
			case "WEEKLY":
				days = 7
			case "MONTHLY":
				months = interval
			case "YEARLY":
				years = interval
			}

			current := 0
			freqDate := event.Start

			for {
				weekDays := freqDate
				commitEvent := func() {
					current++
					count--
					newEvent := event.Clone()
					newEvent.Start = weekDays
					newEvent.End = weekDays.Add(duration)
					newEvent.Sequence = current

					for _, e := range exclusions {
						if e.Equal(weekDays) {
							excluded = append(excluded, *newEvent)
							return
						}
					}

					if until.IsZero() || (!until.IsZero() && (until.After(weekDays) || until.Equal(weekDays))) {
						cal.Events = append(cal.Events, *newEvent)
					}
				}

				if byMonth == "" || strings.Contains(byMonth, weekDays.Format("1")) {
					if byDay != "" {
						for i := 0; i < 7; i++ {
							day := parseDayNameToIcsName(weekDays.Format("Mon"))
							if strings.Contains(byDay, day) && weekDays != event.Start {
								commitEvent()
							}
							weekDays = weekDays.AddDate(0, 0, 1)
						}
					} else {
						if weekDays != event.Start {
							commitEvent()
						}
					}
				}

				freqDate = freqDate.AddDate(years, months, days)
				if current > maxRepeats || count == 0 {
					break
				}

				if !until.IsZero() && (until.Before(freqDate) || until.Equal(freqDate)) {
					break
				}
			}
		}
	}

	sort.Sort(byDate(cal.Events))
	cal.Events = diff(ExcludeRecurrences(cal.Events), excluded)

	return nil
}

func parseEventSummary(eventData string) string {
	return trimField(eventSummaryRegex.FindString(eventData), "SUMMARY:")
}

func parseEventStatus(eventData string) string {
	return trimField(eventStatusRegex.FindString(eventData), "STATUS:")
}

func parseEventDescription(eventData string) string {
	return trimField(eventDescRegex.FindString(eventData), "DESCRIPTION:")
}

func parseEventID(eventData string) string {
	return trimField(eventUIDRegex.FindString(eventData), "DSTAMP:")
}

func parseEventClass(eventData string) string {
	return trimField(eventClassRegex.FindString(eventData), "CLASS:")
}

func parseEventSequence(eventData string) int {
	seq, _ := strconv.Atoi(trimField(eventSequenceRegex.FindString(eventData), "SEQUENCE:"))
	return seq
}

func parseEventCreated(eventData string) time.Time {
	created := trimField(eventCreatedRegex.FindString(eventData), "CREATED:")
	t, _ := time.Parse(icsFormat, created)
	return t
}

func parseEventModified(eventData string) time.Time {
	date := trimField(eventModifiedRegex.FindString(eventData), "LAST-MODIFIED:")
	t, _ := time.Parse(icsFormat, date)
	return t
}

func parseEventRecurrenceID(eventData string) (time.Time, *time.Location, error) {
	rec := eventRecurrenceIDRegex.FindString(eventData)
	if rec == "" {
		return time.Time{}, nil, nil
	}

	return parseDatetime(rec)
}

func parseEventDate(start, eventData string) (time.Time, *time.Location, error) {
	ts := eventDateRegex.FindAllString(eventData, -1)
	t := findWithStart(start, ts)
	tWholeDay := eventWholeDayRegex.FindString(t)
	if tWholeDay != "" {
		return parseDate(strings.TrimSpace(tWholeDay))
	}

	if t == "" {
		return time.Time{}, nil, nil
	}

	return parseDatetime(t)
}

func findWithStart(start string, list []string) string {
	for _, t := range list {
		if strings.HasPrefix(t, start) {
			return t
		}
	}

	return ""
}

func parseDatetime(data string) (time.Time, *time.Location, error) {
	data = strings.TrimSpace(data)
	var dataTz string
	timeString := data
	if strings.Contains(data, ":") {
		dataParts := strings.Split(data, ":")
		dataTz = dataParts[0]
		timeString = dataParts[1]
	}

	if !strings.Contains(timeString, "Z") {
		timeString = timeString + "Z"
	}

	t, err := time.Parse(icsFormat, timeString)
	if err != nil {
		return t, nil, err
	}

	if strings.Contains(dataTz, "TZID") {
		loc, err := parseLocation(strings.Split(dataTz, "=")[1])

		return t, loc, err
	}

	return t, nil, nil
}

func parseLocation(location string) (*time.Location, error) {
	timezone, err := time.LoadLocation(location)
	if err != nil {
		loc, found := nonStandardTimezones[location]
		if found {
			timezone, err = time.LoadLocation(loc)
		} else {
			trimmedLoc := timezoneLocationCompatibilityRegex.ReplaceAllString(location, "")
			loc, found = nonStandardTimezones[trimmedLoc]
			if found {
				timezone, err = time.LoadLocation(loc)
				if err != nil {
					return timezone, err
				}

				err = &timezoneLocationCompatibilityError{
					originalLocation:      location,
					compatibilityLocation: trimmedLoc,
				}
			}
		}

		if timezone == nil {
			timezone = time.UTC
			err = &timezoneLocationError{
				location: location,
			}
		}

		return timezone, err
	}

	return timezone, nil
}

func parseDate(data string) (time.Time, *time.Location, error) {
	return parseDatetime(data + "T000000")
}

func parseEventRRule(eventData string) string {
	return trimField(eventRRuleRegex.FindString(eventData), "RRULE:")
}

func parseExcludedDates(eventData string, convertDatesToUTC bool) ([]time.Time, error) {
	var dates []time.Time
	excl := eventExDateRegex.FindAllStringSubmatch(eventData, -1)

	for _, e := range excl {
		if len(e) < 3 {
			continue
		}

		tz, err := parseLocation(e[1])
		if err != nil {
			return nil, err
		}

		exDates := strings.Split(e[2], ",")

		for _, dateStr := range exDates {
			dt := strings.TrimSpace(dateStr)
			if !strings.Contains(dt, "Z") {
				dt += "Z"
			}

			t, err := time.Parse(icsFormat, dt)
			if err != nil {
				continue
			}

			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), tz)

			if convertDatesToUTC {
				t = t.UTC()
			}

			dates = append(dates, t)
		}
	}

	return dates, nil
}

func parseEventLocation(eventData string) string {
	return trimField(eventLocationRegex.FindString(eventData), "LOCATION:")
}

func parseEventAttendees(eventData string) []Attendee {
	attendeesList := []Attendee{}
	attendees := attendeesRegex.FindAllString(eventData, -1)

	for _, a := range attendees {
		if a == "" {
			continue
		}
		attendee := parseAttendee(strings.Replace(strings.Replace(a, "\r", "", 1), "\n ", "", 1))
		if attendee.Email != "" || attendee.Name != "" {
			attendeesList = append(attendeesList, attendee)
		}
	}

	return attendeesList
}

func parseEventOrganizer(eventData string) Attendee {
	organizer := organizerRegex.FindString(eventData)
	if organizer == "" {
		return Attendee{}
	}

	organizer = strings.Replace(strings.Replace(organizer, "\r", "", 1), "\n ", "", 1)
	return Attendee{
		Email: parseAttendeeMail(organizer),
		Name:  parseOrganizerName(organizer),
	}
}

func parseAttendee(data string) Attendee {
	return Attendee{
		Email:  parseAttendeeMail(data),
		Name:   parseAttendeeName(data),
		Role:   parseAttendeeRole(data),
		Status: parseAttendeeStatus(data),
		Type:   parseAttendeeType(data),
	}
}

func parseAttendeeMail(attendeeData string) string {
	return trimField(attendeeEmailRegex.FindString(attendeeData), "mailto:")
}

func parseAttendeeStatus(attendeeData string) string {
	return trimField(attendeeStatusRegex.FindString(attendeeData), `(PARTSTAT=|;)`)
}

func parseAttendeeRole(attendeeData string) string {
	return trimField(attendeeRoleRegex.FindString(attendeeData), `(ROLE=|;)`)
}

func parseAttendeeName(attendeeData string) string {
	return trimField(attendeeNameRegex.FindString(attendeeData), `(CN=|;)`)
}

func parseOrganizerName(orgData string) string {
	return trimField(organizerNameRegex.FindString(orgData), `(CN=|:)`)
}

func parseAttendeeType(attendeeData string) string {
	return trimField(attendeeTypeRegex.FindString(attendeeData), `(CUTYPE=|;)`)
}

func parseUntil(rrule string) time.Time {
	until := trimField(untilRegex.FindString(rrule), `(UNTIL=|;)`)
	var t time.Time
	if until == "" {
	} else {
		t, _ = time.Parse(icsFormat, until)
	}
	return t
}

func parseInterval(rrule string) int {
	interval := trimField(intervalRegex.FindString(rrule), `(INTERVAL=|;)`)
	i, _ := strconv.Atoi(interval)
	if i == 0 {
		i = 1
	}

	return i
}

func parseCount(rrule string, maxRepeats int) int {
	c := trimField(countRegex.FindString(rrule), `(COUNT=|;)`)
	count, _ := strconv.Atoi(c)
	if count == 0 {
		count = maxRepeats
	}

	return count
}
