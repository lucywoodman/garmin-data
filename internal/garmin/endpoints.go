package garmin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

func (c *Client) UserSummary(ctx context.Context, date string) (json.RawMessage, error) {
	name, err := c.DisplayName(ctx)
	if err != nil {
		return nil, err
	}
	return c.Get(ctx,
		"/usersummary-service/usersummary/daily/"+name,
		url.Values{"calendarDate": {date}},
	)
}

func (c *Client) HeartRates(ctx context.Context, date string) (json.RawMessage, error) {
	name, err := c.DisplayName(ctx)
	if err != nil {
		return nil, err
	}
	return c.Get(ctx,
		"/wellness-service/wellness/dailyHeartRate/"+name,
		url.Values{"date": {date}},
	)
}

func (c *Client) RestingHeartRate(ctx context.Context, date string) (json.RawMessage, error) {
	name, err := c.DisplayName(ctx)
	if err != nil {
		return nil, err
	}
	return c.Get(ctx,
		"/userstats-service/wellness/daily/"+name,
		url.Values{
			"fromDate":  {date},
			"untilDate": {date},
			"metricId":  {"60"},
		},
	)
}

func (c *Client) SleepData(ctx context.Context, date string) (json.RawMessage, error) {
	name, err := c.DisplayName(ctx)
	if err != nil {
		return nil, err
	}
	return c.Get(ctx,
		"/wellness-service/wellness/dailySleepData/"+name,
		url.Values{
			"date":                  {date},
			"nonSleepBufferMinutes": {"60"},
		},
	)
}

func (c *Client) StepsData(ctx context.Context, date string) (json.RawMessage, error) {
	name, err := c.DisplayName(ctx)
	if err != nil {
		return nil, err
	}
	return c.Get(ctx,
		"/wellness-service/wellness/dailySummaryChart/"+name,
		url.Values{"date": {date}},
	)
}

func (c *Client) StressData(ctx context.Context, date string) (json.RawMessage, error) {
	return c.Get(ctx, "/wellness-service/wellness/dailyStress/"+date, nil)
}

func (c *Client) HRVData(ctx context.Context, date string) (json.RawMessage, error) {
	return c.Get(ctx, "/hrv-service/hrv/"+date, nil)
}

func (c *Client) SpO2Data(ctx context.Context, date string) (json.RawMessage, error) {
	return c.Get(ctx, "/wellness-service/wellness/daily/spo2/"+date, nil)
}

func (c *Client) BodyBatteryEvents(ctx context.Context, date string) (json.RawMessage, error) {
	return c.Get(ctx, "/wellness-service/wellness/bodyBattery/events/"+date, nil)
}

// CalendarMonth returns a month of the Garmin calendar: activities, naps and
// the workouts apps like Runna and Catch schedule ahead. month0 is 0-based,
// as the endpoint expects - November is 10.
func (c *Client) CalendarMonth(ctx context.Context, year, month0 int) (json.RawMessage, error) {
	return c.Get(ctx, fmt.Sprintf("/calendar-service/year/%d/month/%d", year, month0), nil)
}

// Workout returns one workout's full definition, steps and all.
func (c *Client) Workout(ctx context.Context, id int64) (json.RawMessage, error) {
	return c.Get(ctx, "/workout-service/workout/"+strconv.FormatInt(id, 10), nil)
}

func (c *Client) ActivitiesByDate(ctx context.Context, start, end string) ([]json.RawMessage, error) {
	const pageSize = 20
	var all []json.RawMessage
	offset := 0
	for {
		raw, err := c.Get(ctx,
			"/activitylist-service/activities/search/activities",
			url.Values{
				"startDate": {start},
				"endDate":   {end},
				"start":     {strconv.Itoa(offset)},
				"limit":     {strconv.Itoa(pageSize)},
			},
		)
		if err != nil {
			return nil, err
		}
		var page []json.RawMessage
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		all = append(all, page...)
		offset += pageSize
	}
	return all, nil
}
