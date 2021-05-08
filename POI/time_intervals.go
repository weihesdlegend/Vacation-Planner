package POI

import (
	"errors"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"regexp"
	"strconv"
	"strings"
)

type Hour uint8

type TimeInterval struct {
	Start Hour
	End   Hour
}

type ByStartTime []TimeInterval

func (timeIntervals ByStartTime) Len() int {
	return len(timeIntervals)
}

func (timeIntervals ByStartTime) Swap(i, j int) {
	timeIntervals[i], timeIntervals[j] = timeIntervals[j], timeIntervals[i]
}

func (timeIntervals ByStartTime) Less(i, j int) bool {
	return timeIntervals[i].Start <= timeIntervals[j].Start
}

func (interval *TimeInterval) Serialize() string {
	return strconv.FormatUint(uint64(interval.Start), 10) + "_" + strconv.FormatUint(uint64(interval.End), 10)
}

// returns true if two time intervals intersect
func (interval *TimeInterval) Intersect(newInterval *TimeInterval) bool {
	if interval.End <= newInterval.Start || interval.Start >= newInterval.End {
		return false
	}
	return true
}

// returns true if the current interval includes the new interval in the parameter
func (interval *TimeInterval) Inclusive(newInterval *TimeInterval) bool {
	return newInterval.Start >= interval.Start && newInterval.End <= interval.End
}

// given a open hours data from Google, we only use the Thursday data to fill GoogleMapsTimeIntervals struct
type GoogleMapsTimeIntervals struct {
	numIntervals int
	Intervals    []TimeInterval
}

func (timeIntervals *GoogleMapsTimeIntervals) GetAllIntervals() *[]TimeInterval {
	return &timeIntervals.Intervals
}

func (timeIntervals *GoogleMapsTimeIntervals) GetInterval(idx int) (error, TimeInterval) {
	if idx < 0 || idx >= timeIntervals.numIntervals {
		return errors.New("index out of bound"), TimeInterval{}
	}
	return nil, timeIntervals.Intervals[idx]
}

// assume input having non-overlapping time intervals
// maintain intervals sorted
func (timeIntervals *GoogleMapsTimeIntervals) InsertTimeInterval(interval TimeInterval) {
	if timeIntervals.numIntervals == 0 {
		timeIntervals.Intervals = append(timeIntervals.Intervals, interval)
	} else {
		for idx, itv := range timeIntervals.Intervals {
			if itv.Start >= interval.End {
				timeIntervals.Intervals = append(timeIntervals.Intervals[:idx],
					append([]TimeInterval{interval}, timeIntervals.Intervals[idx:]...)...)
				break
			}
		}
		if interval.Start >= timeIntervals.Intervals[timeIntervals.numIntervals-1].End {
			timeIntervals.Intervals = append(timeIntervals.Intervals, interval)
		}
	}
	timeIntervals.numIntervals++
}

func (timeIntervals *GoogleMapsTimeIntervals) NumIntervals() int {
	return timeIntervals.numIntervals
}

// given a string of form "Monday: 10:45 PM - 11:78 AM", return a TimeInterval with start and end hour in [0-24]
func ParseTimeInterval(openingHour string) (interval TimeInterval, err error) {
	closed, _ := regexp.Match(`Closed`, []byte(openingHour))
	if closed {
		interval.Start = 255
		interval.End = 255
		return
	}
	hour_re := regexp.MustCompile(`[\d]{1,2}:[\d]{2}`)
	hours := hour_re.FindAll([]byte(openingHour), -1)

	am_pm_re := regexp.MustCompile(`[apAP][mM]`)
	am_pm := am_pm_re.FindAll([]byte(openingHour), -1)

	if len(hours) < 2 || len(am_pm) < 2 {
		return TimeInterval{}, errors.New("cannot parse opening hour")
	}

	interval.Start = Hour(calculateHour(string(hours[0]), string(am_pm[0])))
	interval.End = Hour(calculateHour(string(hours[1]), string(am_pm[1])))

	if interval.Start > interval.End { // late night hours
		interval.End = 24
	}

	return
}

func calculateHour(time string, am_pm string) uint8 {
	t := strings.Split(time, ":")
	hour, err := strconv.ParseUint(t[0], 10, 8)
	utils.LogErrorWithLevel(err, utils.LogError)

	if am_pm == "AM" || am_pm == "am" {
		return uint8(hour)
	} else if am_pm == "PM" || am_pm == "pm" {
		return uint8(hour) + 12
	}
	return 255 // err
}
