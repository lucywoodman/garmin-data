package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS health_data (
    date TEXT NOT NULL,
    metric TEXT NOT NULL,
    data TEXT NOT NULL,
    synced_at REAL NOT NULL,
    PRIMARY KEY (date, metric)
);

CREATE TABLE IF NOT EXISTS activities (
    activity_id INTEGER PRIMARY KEY,
    date TEXT NOT NULL,
    data TEXT NOT NULL,
    synced_at REAL NOT NULL
);

-- Workouts an app has scheduled ahead, keyed by the day they're planned for.
CREATE TABLE IF NOT EXISTS scheduled_workouts (
    date TEXT NOT NULL,
    workout_id INTEGER NOT NULL,
    data TEXT NOT NULL,
    synced_at REAL NOT NULL,
    PRIMARY KEY (date, workout_id)
);

CREATE TABLE IF NOT EXISTS sync_log (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
`

type DB struct {
	conn *sql.DB
}

type HealthRecord struct {
	Date     string
	Metric   string
	Data     json.RawMessage
	SyncedAt float64
}

type ScheduledWorkout struct {
	Date      string
	WorkoutID int64
	Data      json.RawMessage
	SyncedAt  float64
}

type Activity struct {
	ActivityID int64
	Date       string
	Data       json.RawMessage
	SyncedAt   float64
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := conn.Exec(schema); err != nil {
		return nil, errors.Join(fmt.Errorf("create schema: %w", err), conn.Close())
	}
	return &DB{conn: conn}, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) Upsert(date, metric string, data any) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal %s/%s: %w", date, metric, err)
	}
	_, err = d.conn.Exec(
		`INSERT OR REPLACE INTO health_data (date, metric, data, synced_at) VALUES (?, ?, ?, ?)`,
		date, metric, string(encoded), nowEpoch(),
	)
	return err
}

func (d *DB) Query(date, metric string) (*HealthRecord, error) {
	row := d.conn.QueryRow(
		`SELECT date, metric, data, synced_at FROM health_data WHERE date = ? AND metric = ?`,
		date, metric,
	)
	rec := &HealthRecord{}
	var raw string
	if err := row.Scan(&rec.Date, &rec.Metric, &raw, &rec.SyncedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	rec.Data = json.RawMessage(raw)
	return rec, nil
}

func (d *DB) QueryDate(date string) ([]HealthRecord, error) {
	rows, err := d.conn.Query(
		`SELECT date, metric, data, synced_at FROM health_data WHERE date = ? ORDER BY metric`,
		date,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanHealthRecords(rows)
}

func (d *DB) SetSyncLog(key, value string) error {
	_, err := d.conn.Exec(
		`INSERT OR REPLACE INTO sync_log (key, value) VALUES (?, ?)`,
		key, value,
	)
	return err
}

func (d *DB) GetSyncLog(key string) (string, error) {
	var value string
	err := d.conn.QueryRow(`SELECT value FROM sync_log WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (d *DB) UpsertActivity(activityID int64, date string, data any) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal activity %d: %w", activityID, err)
	}
	_, err = d.conn.Exec(
		`INSERT OR REPLACE INTO activities (activity_id, date, data, synced_at) VALUES (?, ?, ?, ?)`,
		activityID, date, string(encoded), nowEpoch(),
	)
	return err
}

func (d *DB) QueryActivity(activityID int64) (*Activity, error) {
	row := d.conn.QueryRow(
		`SELECT activity_id, date, data, synced_at FROM activities WHERE activity_id = ?`,
		activityID,
	)
	act := &Activity{}
	var raw string
	if err := row.Scan(&act.ActivityID, &act.Date, &raw, &act.SyncedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	act.Data = json.RawMessage(raw)
	return act, nil
}

func (d *DB) QueryActivities(date string) ([]Activity, error) {
	rows, err := d.conn.Query(
		`SELECT activity_id, date, data, synced_at FROM activities WHERE date = ? ORDER BY activity_id`,
		date,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanActivities(rows)
}

func (d *DB) ActivityCount() (int, error) {
	return d.scalarCount(`SELECT COUNT(*) FROM activities`)
}

func (d *DB) RecordCount() (int, error) {
	return d.scalarCount(`SELECT COUNT(*) FROM health_data`)
}

func (d *DB) scalarCount(query string) (int, error) {
	var n int
	err := d.conn.QueryRow(query).Scan(&n)
	return n, err
}

func scanHealthRecords(rows *sql.Rows) ([]HealthRecord, error) {
	var out []HealthRecord
	for rows.Next() {
		var rec HealthRecord
		var raw string
		if err := rows.Scan(&rec.Date, &rec.Metric, &raw, &rec.SyncedAt); err != nil {
			return nil, err
		}
		rec.Data = json.RawMessage(raw)
		out = append(out, rec)
	}
	return out, rows.Err()
}

func scanActivities(rows *sql.Rows) ([]Activity, error) {
	var out []Activity
	for rows.Next() {
		var act Activity
		var raw string
		if err := rows.Scan(&act.ActivityID, &act.Date, &raw, &act.SyncedAt); err != nil {
			return nil, err
		}
		act.Data = json.RawMessage(raw)
		out = append(out, act)
	}
	return out, rows.Err()
}

func nowEpoch() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}

// ReplaceScheduledWorkouts sets the day's scheduled workouts to exactly these,
// keyed by workout ID. The calendar is the authority for the days it covers,
// so a session the app moved or deleted goes, and nil clears the day.
func (d *DB) ReplaceScheduledWorkouts(date string, workouts map[int64]any) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM scheduled_workouts WHERE date = ?`, date); err != nil {
		return err
	}
	for id, data := range workouts {
		encoded, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshal scheduled workout %d: %w", id, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO scheduled_workouts (date, workout_id, data, synced_at) VALUES (?, ?, ?, ?)`,
			date, id, string(encoded), nowEpoch(),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) QueryScheduledWorkouts(date string) ([]ScheduledWorkout, error) {
	rows, err := d.conn.Query(
		`SELECT date, workout_id, data, synced_at FROM scheduled_workouts WHERE date = ? ORDER BY workout_id`,
		date,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []ScheduledWorkout
	for rows.Next() {
		var (
			w   ScheduledWorkout
			raw string
		)
		if err := rows.Scan(&w.Date, &w.WorkoutID, &raw, &w.SyncedAt); err != nil {
			return nil, err
		}
		w.Data = json.RawMessage(raw)
		out = append(out, w)
	}
	return out, rows.Err()
}
