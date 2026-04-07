from pathlib import Path

from garminconnect import Garmin

TOKEN_DIR = Path.home() / ".garminconnect"
TOKEN_FILE = TOKEN_DIR / "garmin_tokens.json"


def _prompt_mfa() -> str:
    return input("Enter MFA code: ")


def create_client(email: str) -> Garmin:
    return Garmin(email=email, prompt_mfa=_prompt_mfa)


def login(email: str, password: str) -> Garmin:
    client = create_client(email)
    client.password = password
    client.login()
    client.client.dump(str(TOKEN_DIR))
    return client


def resume_session(email: str) -> Garmin | None:
    if not TOKEN_FILE.exists():
        return None
    client = create_client(email)
    client.login(tokenstore=str(TOKEN_DIR))
    return client


def save_session(client: Garmin):
    TOKEN_DIR.mkdir(parents=True, exist_ok=True)
    client.client.dump(str(TOKEN_DIR))
