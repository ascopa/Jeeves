import os
import time
import uuid
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone

import psycopg
import pytest
import requests


for name, default in {
    "PGHOST": "localhost",
    "PGPORT": "5433",
    "PGDATABASE": "jeeves",
    "PGUSER": "jeeves",
    "PGPASSWORD": "jeeves",
}.items():
    os.environ.setdefault(name, default)

GATEWAY_URL = os.environ.get("GATEWAY_URL", "http://localhost:8080")
CARD_PROVIDER_URL = os.environ.get("CARD_PROVIDER_URL", "http://localhost:9090")
MAX_HOLDS_PER_EVENT = int(os.environ.get("MAX_HOLDS_PER_EVENT", "4"))
ADMIT_SEATS_FACTOR = int(os.environ.get("ADMIT_SEATS_FACTOR", "2"))

POLL_TIMEOUT_SECONDS = float(os.environ.get("POLL_TIMEOUT_SECONDS", "30"))
POLL_INTERVAL_SECONDS = 0.5


STARTUP_TIMEOUT_SECONDS = 90
NIL_ID = "00000000-0000-0000-0000-000000000000"


def stack_is_ready(gateway: str, card_provider: str) -> bool:
    probes = [
        ("GET", f"{gateway}/search?name=ready", {}),
        ("GET", f"{gateway}/queue/{NIL_ID}/status", {"X-User-Id": NIL_ID}),
        ("GET", f"{gateway}/events/{NIL_ID}/seats", {"X-User-Id": NIL_ID}),
        ("POST", f"{gateway}/checkout", {"X-User-Id": NIL_ID}),
        ("POST", f"{card_provider}/tokens", {}),
    ]
    try:
        return all(requests.request(m, url, headers=h, timeout=5).status_code < 500 for m, url, h in probes)
    except requests.RequestException:
        return False


def wait_for_stack(gateway: str, card_provider: str) -> bool:
    deadline = time.monotonic() + STARTUP_TIMEOUT_SECONDS
    while not stack_is_ready(gateway, card_provider):
        if time.monotonic() > deadline:
            return False
        time.sleep(1)
    return True

APPROVED_PAN = "4242424242424242"
DECLINED_PAN = "4000000000000002"

TEST_VENUE_ID = "00000000-0000-4000-8000-000000000001"
TICKET_PRICE_CENTS = 1_500_000


@dataclass(frozen=True)
class User:
    id: str

    @property
    def headers(self) -> dict[str, str]:
        return {"X-User-Id": self.id}


@dataclass(frozen=True)
class Event:
    id: str
    name: str
    ticket_ids: list[str]


@pytest.fixture(scope="session", autouse=True)
def stack_ready():
    if not wait_for_stack(GATEWAY_URL, CARD_PROVIDER_URL):
        pytest.exit(f"The services weren't ready after {STARTUP_TIMEOUT_SECONDS}s. Is the stack up (make up)?")


@pytest.fixture(scope="session")
def db():
    with psycopg.connect(autocommit=True) as conn:
        yield conn


@pytest.fixture(scope="session")
def venue(db) -> str:
    db.execute(
        """
        insert into venues (id, name, size) values (%s, 'Integration test venue', 100)
        on conflict (id) do update set name = excluded.name, size = excluded.size
        """,
        (TEST_VENUE_ID,),
    )
    return TEST_VENUE_ID


@pytest.fixture
def make_user(db):
    created: list[str] = []

    def make() -> User:
        user_id = str(uuid.uuid4())
        db.execute(
            "insert into users (id, name, email) values (%s, 'Integration test', %s)",
            (user_id, f"it-{user_id[:12]}@test.local"),
        )
        created.append(user_id)
        return User(id=user_id)

    yield make

    if created:
        db.execute("delete from payments where user_id = any(%s::uuid[])", (created,))
        db.execute("delete from users where id = any(%s::uuid[])", (created,))


@pytest.fixture
def make_event(db, venue, make_user):
    created: list[Event] = []

    def make(seats: int = 4, queue: bool = False, admit_rate: int = 50, max_active_users: int = 1000) -> Event:
        ticket_ids = [str(uuid.uuid4()) for _ in range(seats)]
        for n, ticket_id in enumerate(ticket_ids, start=1):
            db.execute(
                "insert into tickets (id, price_cents, seat_number, type) values (%s, %s, %s, %s)",
                (ticket_id, TICKET_PRICE_CENTS, f"A-{n:02d}", "vip" if n == 1 else "general"),
            )
        event_id = str(uuid.uuid4())
        name = f"Integration test {event_id[:8]}"
        db.execute(
            """
            insert into events (id, name, date, venue_id, ticket_ids, queue_enabled, admit_rate_per_sec, max_active_users)
            values (%s, %s, now() + interval '30 days', %s, %s::uuid[], %s, %s, %s)
            """,
            (event_id, name, venue, ticket_ids, queue, admit_rate, max_active_users),
        )
        event = Event(id=event_id, name=name, ticket_ids=ticket_ids)
        created.append(event)
        return event

    yield make

    for event in created:
        db.execute("delete from payments where ticket_id = any(%s::uuid[])", (event.ticket_ids,))
        db.execute("delete from events where id = %s", (event.id,))
        db.execute("delete from tickets where id = any(%s::uuid[])", (event.ticket_ids,))


def hold(db, ticket_id: str, user: User, expires_in: timedelta = timedelta(minutes=10)):
    db.execute(
        "update tickets set status = 'booked', booked_by = %s, booked_at = now(), hold_expires_at = now() + %s "
        "where id = %s",
        (user.id, expires_in, ticket_id),
    )


def sell(db, ticket_id: str, user: User):
    db.execute(
        "update tickets set status = 'sold', booked_by = %s, booked_at = now(), hold_expires_at = now(), "
        "sold_at = now() where id = %s",
        (user.id, ticket_id),
    )


def ticket_row(db, ticket_id: str):
    return db.execute(
        "select status::text, booked_by::text, hold_expires_at from tickets where id = %s", (ticket_id,)
    ).fetchone()


def payments_for(db, ticket_id: str):
    return db.execute(
        "select id::text, status, provider_charge_id, idempotency_key, amount_cents from payments "
        "where ticket_id = %s order by created_at",
        (ticket_id,),
    ).fetchall()


def search(name: str) -> requests.Response:
    return requests.get(f"{GATEWAY_URL}/search", params={"name": name}, timeout=10)


def seat_map(user: User, event_id: str, admission: str | None = None) -> requests.Response:
    return requests.get(
        f"{GATEWAY_URL}/events/{event_id}/seats", headers=_with_admission(user, admission), timeout=10
    )


def book(user: User, ticket_id: str, admission: str | None = None) -> requests.Response:
    return requests.post(
        f"{GATEWAY_URL}/book", json={"ticketId": ticket_id}, headers=_with_admission(user, admission), timeout=10
    )


def checkout(user: User, ticket_id: str, payment_token: str, idempotency_key: str | None = None) -> requests.Response:
    headers = dict(user.headers)
    if idempotency_key is not None:
        headers["Idempotency-Key"] = idempotency_key
    return requests.post(
        f"{GATEWAY_URL}/checkout",
        json={"ticketId": ticket_id, "paymentToken": payment_token},
        headers=headers,
        timeout=15,
    )


def join_queue(user: User, event_id: str) -> requests.Response:
    return requests.post(f"{GATEWAY_URL}/queue/{event_id}/join", headers=user.headers, timeout=10)


def queue_status(user: User, event_id: str) -> requests.Response:
    return requests.get(f"{GATEWAY_URL}/queue/{event_id}/status", headers=user.headers, timeout=10)


def card_token(pan: str = APPROVED_PAN) -> str:
    response = requests.post(
        f"{CARD_PROVIDER_URL}/tokens",
        json={"pan": pan, "expMonth": 12, "expYear": datetime.now().year + 3, "cvv": "123"},
        timeout=10,
    )
    assert response.status_code == 201, response.text
    return response.json()["token"]


def _with_admission(user: User, admission: str | None) -> dict[str, str]:
    headers = dict(user.headers)
    if admission is not None:
        headers["Admission-Token"] = admission
    return headers


def wait_for(fetch, done, description):
    deadline = time.monotonic() + POLL_TIMEOUT_SECONDS
    while True:
        value = fetch()
        if done(value):
            return value
        if time.monotonic() > deadline:
            pytest.fail(f"timed out after {POLL_TIMEOUT_SECONDS}s waiting for {description} (last: {value})")
        time.sleep(POLL_INTERVAL_SECONDS)


def wait_until_admitted(user: User, event_id: str) -> str:
    body = wait_for(
        lambda: queue_status(user, event_id).json(),
        lambda b: b.get("status") == "admitted",
        f"user {user.id} to be admitted to {event_id}",
    )
    return body["admissionToken"]


def admit(user: User, event_id: str) -> str:
    response = join_queue(user, event_id)
    assert response.status_code == 200, response.text
    return wait_until_admitted(user, event_id)
