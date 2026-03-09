import argparse
import json
import os
import sys
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
    sync.add_argument("--metrics", type=str, default=None, help="Comma-separated metrics: summary,heart_rate,rhr,sleep,steps,stress,hrv,spo2,body_battery")

    subparsers.add_parser("status", help="Show sync status and record counts")

    query = subparsers.add_parser("query", help="Query stored data for a date")
    query.add_argument("date", type=parse_date, help="Date to query (YYYY-MM-DD)")
    query.add_argument("--metric", type=str, default=None, help="Specific metric to show")

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
    from garmin_data.sync import METRICS, sync_metrics

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
    print(f"Done. {db.record_count()} total records in database.")


def cmd_status():
    db = get_db()
    count = db.record_count()
    last_sync = db.get_sync_log("last_sync_date")
    print(f"Records: {count}")
    print(f"Last sync: {last_sync or 'never'}")


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
    elif args.command == "status":
        cmd_status()
    elif args.command == "query":
        cmd_query(args)


if __name__ == "__main__":
    main()
