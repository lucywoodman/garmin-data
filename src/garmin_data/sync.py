import time
from datetime import date, timedelta

from garminconnect import GarminConnectTooManyRequestsError

from garmin_data.database import Database

METRICS = {
    "summary": "get_user_summary",
    "heart_rate": "get_heart_rates",
    "rhr": "get_rhr_day",
    "sleep": "get_sleep_data",
    "steps": "get_steps_data",
    "stress": "get_stress_data",
    "hrv": "get_hrv_data",
    "spo2": "get_spo2_data",
    "body_battery": "get_body_battery_events",
}

RATE_LIMIT_DELAY = 0.5
BACKOFF_DELAY = 60
MAX_RETRIES = 3


def sync_metrics(
    client,
    db: Database,
    start: date,
    end: date,
    metric_names: list[str] | None = None,
):
    metrics = {k: v for k, v in METRICS.items() if metric_names is None or k in metric_names}
    current = start
    while current <= end:
        date_str = current.isoformat()
        first_call = True
        for metric_name, method_name in metrics.items():
            if not first_call:
                time.sleep(RATE_LIMIT_DELAY)
            first_call = False

            data = _fetch_with_retry(client, method_name, date_str)
            if data is not None:
                db.upsert(date_str, metric_name, data)

        current += timedelta(days=1)

    db.set_sync_log("last_sync_date", end.isoformat())


def _fetch_with_retry(client, method_name: str, date_str: str) -> dict | None:
    for attempt in range(MAX_RETRIES):
        try:
            return getattr(client, method_name)(date_str)
        except GarminConnectTooManyRequestsError:
            if attempt < MAX_RETRIES - 1:
                time.sleep(BACKOFF_DELAY)
    return None
