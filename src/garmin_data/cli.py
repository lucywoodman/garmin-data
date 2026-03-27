import argparse
import json
import os
import sys
import urllib.request
from datetime import date, datetime, timedelta
from getpass import getpass
from pathlib import Path

from garmin_data.database import Database

DEFAULT_DB_PATH = Path.home() / ".local" / "share" / "garmin-data" / "garmin.db"
DEFAULT_SYNC_DAYS = 7


def parse_date(value: str) -> date:
    try:
        return datetime.strptime(value, "%Y-%m-%d").date()
    except ValueError:
        raise argparse.ArgumentTypeError(
            f"Invalid date format: {value!r} (expected YYYY-MM-DD)"
        )


def get_email() -> str:
    email = os.environ.get("GARMIN_EMAIL", "").strip()
    if not email:
        print("Error: GARMIN_EMAIL environment variable is required.", file=sys.stderr)
        sys.exit(1)
    return email


def get_db() -> Database:
    db_path = os.environ.get("GARMIN_DB_PATH", str(DEFAULT_DB_PATH))
    path = Path(db_path)
    path.parent.mkdir(parents=True, exist_ok=True)
    return Database(str(path))


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="garmin-data",
        description="Sync Garmin Connect data to a local SQLite database.",
        epilog="Run 'garmin-data <command> --help' for more info on a command.",
    )
    subparsers = parser.add_subparsers(dest="command")

    subparsers.add_parser("login", help="Login to Garmin Connect")

    sync = subparsers.add_parser("sync", help="Sync health data from Garmin")
    sync.add_argument("--start", type=parse_date, default=None, help="Start date (YYYY-MM-DD)")
    sync.add_argument("--end", type=parse_date, default=None, help="End date (YYYY-MM-DD)")
    from garmin_data.sync import METRICS

    available = ", ".join(METRICS.keys())
    sync.add_argument("--metrics", type=str, default=None, help=f"Comma-separated metrics to sync (available: {available})")

    subparsers.add_parser("status", help="Show sync status and record counts")

    activities = subparsers.add_parser("activities", help="Show activities for a date")
    activities.add_argument("date", type=parse_date, help="Date to query (YYYY-MM-DD)")

    query = subparsers.add_parser("query", help="Query stored data for a date")
    query.add_argument("date", type=parse_date, help="Date to query (YYYY-MM-DD)")
    query.add_argument("--metric", type=str, default=None, help="Specific metric to show")

    daily = subparsers.add_parser("daily", help="Output daily log YAML for a date")
    daily.add_argument("date", type=parse_date, help="Date to query (YYYY-MM-DD)")

    push = subparsers.add_parser("push", help="Sync from Garmin and push daily data to Genki Tracker")
    push.add_argument("--start", type=parse_date, default=None, help="Start date (YYYY-MM-DD, default: yesterday)")
    push.add_argument("--end", type=parse_date, default=None, help="End date (YYYY-MM-DD, default: today)")

    return parser


def cmd_login(email: str):
    from garmin_data.auth import login

    password = getpass("Garmin password: ")
    try:
        login(email, password)
        print("Login successful. Tokens saved.")
    except Exception as e:
        print(f"Login failed: {e}", file=sys.stderr)
        sys.exit(1)


def cmd_sync(args, email: str):
    from garmin_data.auth import resume_session
    from garmin_data.sync import METRICS, sync_activities, sync_metrics

    client = resume_session(email)
    if client is None:
        print("Error: No saved session. Run 'garmin-data login' first.", file=sys.stderr)
        sys.exit(1)

    end = args.end or date.today()
    start = args.start or (end - timedelta(days=DEFAULT_SYNC_DAYS - 1))

    metric_names = None
    if args.metrics:
        metric_names = [m.strip() for m in args.metrics.split(",")]
        invalid = set(metric_names) - set(METRICS.keys())
        if invalid:
            print(f"Error: Unknown metrics: {', '.join(invalid)}", file=sys.stderr)
            print(f"Available: {', '.join(METRICS.keys())}", file=sys.stderr)
            sys.exit(1)

    db = get_db()
    print(f"Syncing {start} to {end}...")
    sync_metrics(client, db, start, end, metric_names=metric_names)
    sync_activities(client, db, start, end)
    print(f"Done. {db.record_count()} health records, {db.activity_count()} activities in database.")


def cmd_activities(args):
    db = get_db()
    date_str = args.date.isoformat()
    rows = db.query_activities(date_str)
    if not rows:
        print(f"No activities for {date_str}")
        return
    for row in rows:
        data = json.loads(row["data"])
        print(f"{data.get('activityName', 'Unknown')}:")
        print(json.dumps(data, indent=2))
        print()


def cmd_status():
    db = get_db()
    count = db.record_count()
    activities = db.activity_count()
    last_sync = db.get_sync_log("last_sync_date")
    print(f"Health records: {count}")
    print(f"Activities: {activities}")
    print(f"Last sync: {last_sync or 'never'}")


ACTIVITY_TYPE_MAP = {
    "running": "run",
    "swimming": "swim",
    "pool_swimming": "swim",
    "open_water_swimming": "swim",
    "cycling": "bike",
    "road_biking": "bike",
    "mountain_biking": "bike",
    "strength_training": "strength",
    "hiking": "hill_walk",
}


def extract_daily(db: Database, date_str: str) -> dict | None:
    """Extract daily health data for a date into Genki Tracker format."""
    result = {"date": date_str}

    sleep_row = db.query(date_str, "sleep")
    if sleep_row:
        data = json.loads(sleep_row["data"])
        sleep_seconds = data.get("dailySleepDTO", {}).get("sleepTimeSeconds")
        if sleep_seconds is not None:
            result["sleep_hours"] = round(sleep_seconds / 3600, 2)

    summary_row = db.query(date_str, "summary")
    if summary_row:
        data = json.loads(summary_row["data"])
        summary_fields = {
            "steps": "totalSteps",
            "body_battery_at_wake": "bodyBatteryAtWakeTime",
            "body_battery_highest": "bodyBatteryHighestValue",
            "body_battery_lowest": "bodyBatteryLowestValue",
            "stress_avg": "averageStressLevel",
            "stress_max": "maxStressLevel",
            "active_calories": "activeKilocalories",
            "floors_ascended": "floorsAscended",
        }
        for target, source in summary_fields.items():
            value = data.get(source)
            if value is not None:
                result[target] = value

    rhr_row = db.query(date_str, "rhr")
    if rhr_row:
        data = json.loads(rhr_row["data"])
        try:
            metrics = data["allMetrics"]["metricsMap"]["WELLNESS_RESTING_HEART_RATE"]
            result["resting_heart_rate"] = metrics[0]["value"]
        except (KeyError, IndexError):
            pass

    hrv_row = db.query(date_str, "hrv")
    if hrv_row:
        data = json.loads(hrv_row["data"])
        summary = data.get("hrvSummary", {})
        for target, source in [("hrv_weekly_avg", "weeklyAvg"), ("hrv_last_night_avg", "lastNightAvg"), ("hrv_status", "status")]:
            value = summary.get(source)
            if value is not None:
                result[target] = value

    spo2_row = db.query(date_str, "spo2")
    if spo2_row:
        data = json.loads(spo2_row["data"])
        for target, source in [("spo2_avg", "averageSpO2"), ("spo2_lowest", "lowestSpO2")]:
            value = data.get(source)
            if value is not None:
                result[target] = value

    activity_rows = db.query_activities(date_str)
    best = None
    for row in activity_rows:
        data = json.loads(row["data"])
        type_key = data.get("activityType", {}).get("typeKey", "")
        mapped = ACTIVITY_TYPE_MAP.get(type_key)
        if mapped is None:
            continue
        duration = data.get("duration", 0)
        if best is None or duration > best[1]:
            best = (mapped, duration)

    if best:
        exercise_type, duration_seconds = best
        result["exercise_type"] = exercise_type
        result["exercise_duration"] = round(duration_seconds / 60)

    if len(result) > 1:
        return result
    return None


def cmd_daily(args):
    db = get_db()
    daily = extract_daily(db, args.date.isoformat())
    if daily:
        for key, value in daily.items():
            if key == "date":
                continue
            if key == "sleep_hours":
                print(f"{key}: {value:.2f}")
            else:
                print(f"{key}: {value}")


def cmd_push(args, email: str):
    from garmin_data.auth import resume_session
    from garmin_data.sync import sync_activities, sync_metrics

    genki_url = os.environ.get("GENKI_URL", "").rstrip("/")
    genki_password = os.environ.get("GENKI_PASSWORD", "")
    if not genki_url or not genki_password:
        print("Error: GENKI_URL and GENKI_PASSWORD environment variables are required.", file=sys.stderr)
        sys.exit(1)

    client = resume_session(email)
    if client is None:
        print("Error: No saved session. Run 'garmin-data login' first.", file=sys.stderr)
        sys.exit(1)

    end = args.end or date.today()
    start = args.start or (end - timedelta(days=1))

    db = get_db()
    print(f"Syncing {start} to {end}...")
    sync_metrics(client, db, start, end)
    sync_activities(client, db, start, end)

    current = start
    pushed = 0
    while current <= end:
        daily = extract_daily(db, current.isoformat())
        if daily:
            _post_to_genki(genki_url, genki_password, daily)
            print(f"  Pushed {current.isoformat()}")
            pushed += 1
        current += timedelta(days=1)

    print(f"Done. Pushed {pushed} day(s) to Genki Tracker.")


def _post_to_genki(url: str, password: str, data: dict):
    body = json.dumps(data).encode()
    req = urllib.request.Request(
        f"{url}/api/garmin",
        data=body,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {password}",
        },
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())


def cmd_query(args):
    db = get_db()
    date_str = args.date.isoformat()

    if args.metric:
        row = db.query(date_str, args.metric)
        if row is None:
            print(f"No data for {date_str} / {args.metric}")
            return
        data = json.loads(row["data"])
        print(f"{args.metric}:")
        print(json.dumps(data, indent=2))
    else:
        rows = db.query_date(date_str)
        if not rows:
            print(f"No data for {date_str}")
            return
        for row in rows:
            data = json.loads(row["data"])
            print(f"{row['metric']}:")
            print(json.dumps(data, indent=2))
            print()


def main():
    parser = build_parser()
    args = parser.parse_args()

    if args.command is None:
        parser.print_help()
        return

    if args.command == "login":
        email = get_email()
        cmd_login(email)
    elif args.command == "sync":
        email = get_email()
        cmd_sync(args, email)
    elif args.command == "push":
        email = get_email()
        cmd_push(args, email)
    elif args.command == "activities":
        cmd_activities(args)
    elif args.command == "status":
        cmd_status()
    elif args.command == "query":
        cmd_query(args)
    elif args.command == "daily":
        cmd_daily(args)


if __name__ == "__main__":
    main()
