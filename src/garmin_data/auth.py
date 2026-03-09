from getpass import getpass
from pathlib import Path

from garminconnect import Garmin

TOKEN_DIR = Path.home() / ".garminconnect"


def _prompt_mfa() -> str:
    return input("Enter MFA code: ")


def create_client(email: str) -> Garmin:
    return Garmin(email=email, prompt_mfa=_prompt_mfa)


def login(email: str, password: str) -> Garmin:
    client = create_client(email)
    client.password = password
    client.login()
    client.garth.dump(str(TOKEN_DIR))
    return client


def resume_session(email: str) -> Garmin | None:
    if not TOKEN_DIR.exists():
        return None
    client = create_client(email)
    client.login(tokenstore=str(TOKEN_DIR))
    return client
