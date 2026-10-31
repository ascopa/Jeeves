import argparse
import os
import random
import re
import threading
import time
import uuid
from collections import deque
from dataclasses import dataclass, field
from datetime import datetime

import psycopg
import requests
from rich.console import Console, Group
from rich.live import Live
from rich.panel import Panel
from rich.table import Table
from rich.text import Text

for name, default in {
    "PGHOST": "localhost",
    "PGPORT": "5433",
    "PGDATABASE": "jeeves",
    "PGUSER": "jeeves",
    "PGPASSWORD": "jeeves",
}.items():
    os.environ.setdefault(name, default)

DEMO_VENUE_ID = "00000000-0000-4000-8000-0000000000de"
PRICE_CENTS = 1_500_000
APPROVED_PAN = "4242424242424242"
POLL_SECONDS = 1.0


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


@dataclass
class Participant:
    name: str
    user_id: str
    is_you: bool = False
    position: int = 0
    state: str = "waiting"
    seat: str = ""
    call: str = ""


@dataclass
class Demo:
    args: argparse.Namespace
    event_id: str = ""
    event_name: str = ""
    participants: list[Participant] = field(default_factory=list)
    log: deque = field(default_factory=lambda: deque(maxlen=8))
    timeline: list[tuple[str, str]] = field(default_factory=list)
    you_status: str = "joining"
    started: float = field(default_factory=time.monotonic)
    lock: threading.Lock = field(default_factory=threading.Lock)
    stop: threading.Event = field(default_factory=threading.Event)

    def event(self, message: str):
        with self.lock:
            self.log.append(f"{datetime.now():%H:%M:%S} {message}")

    def milestone(self, message: str):
        with self.lock:
            self.timeline.append((f"{datetime.now():%H:%M:%S}", message))


def setup(db, demo: Demo):
    args = demo.args
    db.execute(
        "insert into venues (id, name, size) values (%s, 'Demo venue', %s) on conflict (id) do update set size = excluded.size",
        (DEMO_VENUE_ID, max(args.seats, 1)),
    )
    ticket_ids = [str(uuid.uuid4()) for _ in range(args.seats)]
    for n, ticket_id in enumerate(ticket_ids, start=1):
        db.execute(
            "insert into tickets (id, price_cents, seat_number) values (%s, %s, %s)",
            (ticket_id, PRICE_CENTS, f"A-{n:02d}"),
        )
    demo.event_id = str(uuid.uuid4())
    demo.event_name = f"Demo Night {demo.event_id[:4]}"
    db.execute(
        """
        insert into events (id, name, date, venue_id, ticket_ids, queue_enabled, admit_rate_per_sec, max_active_users)
        values (%s, %s, now() + interval '30 days', %s, %s::uuid[], true, %s, %s)
        """,
        (demo.event_id, demo.event_name, DEMO_VENUE_ID, ticket_ids, args.rate, args.places),
    )
    names = [f"bot-{n:02d}" for n in range(1, args.bots + 1)] + ["you"]
    for name in names:
        user_id = str(uuid.uuid4())
        db.execute(
            "insert into users (id, name, email) values (%s, %s, %s)",
            (user_id, f"Demo {name}", f"demo-{user_id[:12]}@demo.local"),
        )
        demo.participants.append(Participant(name=name, user_id=user_id, is_you=name == "you"))
    return ticket_ids


def cleanup(db, demo: Demo, ticket_ids: list[str]):
    user_ids = [p.user_id for p in demo.participants]
    db.execute("delete from payments where ticket_id = any(%s::uuid[])", (ticket_ids,))
    db.execute("delete from events where id = %s", (demo.event_id,))
    db.execute("delete from tickets where id = any(%s::uuid[])", (ticket_ids,))
    db.execute("delete from users where id = any(%s::uuid[])", (user_ids,))


class Client:
    def __init__(self, demo: Demo, participant: Participant):
        self.demo = demo
        self.me = participant
        self.http = requests.Session()
        self.http.headers["X-User-Id"] = participant.user_id

    def url(self, path: str) -> str:
        return f"{self.demo.args.gateway}/{path}"

    def join(self) -> dict:
        response = self.http.post(self.url(f"queue/{self.demo.event_id}/join"), timeout=10)
        response.raise_for_status()
        return response.json()

    def status(self) -> dict:
        response = self.http.get(self.url(f"queue/{self.demo.event_id}/status"), timeout=10)
        response.raise_for_status()
        return response.json()

    def available_seats(self, admission: str) -> list[dict]:
        response = self.http.get(
            self.url(f"events/{self.demo.event_id}/seats"), headers={"Admission-Token": admission}, timeout=10
        )
        response.raise_for_status()
        return [t for t in response.json()["tickets"] if t["status"] == "available"]

    def hold(self, admission: str, seats: list[dict]) -> dict | None:
        for seat in seats:
            response = self.http.post(
                self.url("book"), json={"ticketId": seat["id"]}, headers={"Admission-Token": admission}, timeout=10
            )
            if response.status_code == 200:
                return seat
        return None

    def checkout(self, seat: dict) -> dict:
        card = requests.post(
            f"{self.demo.args.card_provider}/tokens",
            json={"pan": APPROVED_PAN, "expMonth": 12, "expYear": datetime.now().year + 3, "cvv": "123"},
            timeout=10,
        )
        card.raise_for_status()
        response = self.http.post(
            self.url("checkout"),
            json={"ticketId": seat["id"], "paymentToken": card.json()["token"]},
            headers={"Idempotency-Key": str(uuid.uuid4())},
            timeout=15,
        )
        response.raise_for_status()
        return response.json()


def wait_for_admission(client: Client, on_waiting=None) -> dict:
    while not client.demo.stop.is_set():
        status = client.status()
        if status["status"] == "admitted":
            return status
        if on_waiting:
            on_waiting(status)
        time.sleep(POLL_SECONDS)
    raise SystemExit


def step(client: Client, call: str, action):
    demo = client.demo
    with demo.lock:
        client.me.call = call
    time.sleep(demo.args.step)
    return action()


def buy(client: Client, admission: str) -> dict | None:
    seats = step(client, "GET /events/…/seats", lambda: client.available_seats(admission))
    random.shuffle(seats)
    seat = step(client, "POST /book", lambda: client.hold(admission, seats))
    if seat is None:
        return None
    step(client, f"POST /checkout  {seat['seatNumber']}", lambda: client.checkout(seat))
    with client.demo.lock:
        client.me.state, client.me.seat, client.me.call = "bought", seat["seatNumber"], ""
    client.demo.event(f"{client.me.name} bought {seat['seatNumber']}, freeing a place")
    return seat


def run_bot(demo: Demo, bot: Participant):
    client = Client(demo, bot)
    try:
        status = wait_for_admission(client)
        with demo.lock:
            bot.state = "inside"
        if not demo.stop.is_set():
            buy(client, status["admissionToken"])
    except (requests.RequestException, SystemExit) as err:
        if not demo.stop.is_set():
            demo.event(f"{bot.name} gave up: {err}")


def run_you(demo: Demo, you: Participant):
    client = Client(demo, you)
    last = {}

    def on_waiting(status: dict):
        ahead, eta = status.get("ahead"), status.get("estimatedWaitSeconds", 0)
        with demo.lock:
            demo.you_status = f"#{you.position} · {ahead} ahead · ~{eta}s"
        if ahead != last.get("ahead"):
            demo.milestone(f"{ahead} ahead, about {eta}s to go")
            last.update(ahead=ahead, eta=eta, stalled=False)
        elif not last["stalled"] and eta >= last["eta"] + 5:
            demo.milestone(f"{ahead} ahead, about {eta}s to go  (every place is taken: waiting for purchases)")
            last.update(eta=eta, stalled=True)

    status = wait_for_admission(client, on_waiting)
    with demo.lock:
        you.state = "inside"
        demo.you_status = "admitted"
    expires_at = re.sub(r"\.\d+", "", status["expiresAt"]).replace("Z", "+00:00")
    expires = datetime.fromisoformat(expires_at).astimezone()
    demo.milestone(f"admitted, with an admission token valid until {expires:%H:%M:%S}")

    seat = buy(client, status["admissionToken"])
    if seat is None:
        demo.milestone("no seat left")
        return None
    demo.milestone(f"held seat {seat['seatNumber']}, then paid: sold")
    with demo.lock:
        demo.you_status = f"bought {seat['seatNumber']}"
    return seat


SLOT = 2


def render(demo: Demo) -> Group:
    args = demo.args
    with demo.lock:
        joined = sorted((p for p in demo.participants if p.position), key=lambda p: p.position)
        waiting = [p for p in joined if p.state == "waiting"]
        inside = [(p.name, p.is_you, p.call) for p in joined if p.state == "inside"]
        bought = sum(p.state == "bought" for p in joined)
        log, timeline, you_status = list(demo.log), list(demo.timeline), demo.you_status


    spots = max(len(joined), 1)
    corridor = Text("· " * (spots - len(waiting)), style="grey35")
    for p in reversed(waiting):
        corridor.append("◉ " if p.is_you else "○ ", style="bold cyan" if p.is_you else "grey62")
    width = spots * SLOT
    line = Group(
        Text(f"waiting room · {len(waiting)} in line", style="bold"),
        corridor,
        Text("back".ljust(width - len("front ")) + "front", style="grey50"),
    )

    full = len(inside) >= args.places
    rows = Table.grid(padding=(0, 2))
    for name, is_you, call in inside:
        rows.add_row(Text(name, style="bold cyan" if is_you else "yellow"), Text(call or "admitted", style="grey85"))
    for _ in range(args.places - len(inside)):
        rows.add_row(Text("·", style="grey42"), Text("free", style="grey42"))
    title = f"inside {len(inside)}/{args.places}" + (" · full until someone buys" if full else "")
    room = Panel(rows, title=title, title_align="left", border_style="red" if full else "grey50", width=46)

    layout = Table.grid(padding=(0, 1))
    layout.add_column(vertical="middle")
    layout.add_column(vertical="middle")
    layout.add_column(vertical="middle")
    layout.add_row(line, Text("──▶", style="grey62"), room)

    you_table = Table.grid(padding=(0, 2))
    you_table.add_row(Text("You", style="bold cyan"), Text(you_status, style="bold"))
    for at, message in timeline[-10:]:
        you_table.add_row(Text(at, style="grey50"), message)

    log_text = Text("\n".join(log) if log else "no purchases yet", style="grey70")
    elapsed = time.monotonic() - demo.started
    header = Text(
        f"{demo.event_name} · {args.rate} admission/s · {args.places} places · "
        f"{args.seats - bought} seats left · {elapsed:.0f}s",
        style="bold",
    )
    return Group(
        header,
        Text(),
        layout,
        Text("◉ you   ○ waiting", style="grey50"),
        Text(),
        Panel(you_table, title="your status", title_align="left"),
        Panel(log_text, title="purchases", title_align="left"),
    )


def main():
    parser = argparse.ArgumentParser(description="Live terminal demo of the waiting room.")
    parser.add_argument("--bots", type=int, default=10, help="users ahead of you in the queue (default 10)")
    parser.add_argument("--places", type=int, default=3, help="users allowed inside at once (default 3)")
    parser.add_argument("--rate", type=int, default=2, help="admissions per second (default 2)")
    parser.add_argument("--seats", type=int, default=40, help="seats in the demo event (default 40)")
    parser.add_argument("--step", type=float, default=2, help="seconds each call is shown (default 2)")
    parser.add_argument("--gateway", default=os.environ.get("GATEWAY_URL", "http://localhost:8080"))
    parser.add_argument("--card-provider", default=os.environ.get("CARD_PROVIDER_URL", "http://localhost:9090"))
    args = parser.parse_args()

    demo = Demo(args=args)
    console = Console()
    with console.status("Waiting for the services to be ready…"):
        ready = wait_for_stack(args.gateway, args.card_provider)
    if not ready:
        raise SystemExit(f"The services weren't ready after {STARTUP_TIMEOUT_SECONDS}s. Is the stack up (make up)?")
    with psycopg.connect(autocommit=True) as db:
        ticket_ids = setup(db, demo)
        threads: list[threading.Thread] = []
        try:
            for p in demo.participants:
                p.position = Client(demo, p).join()["position"]
            you = demo.participants[-1]
            demo.milestone(f"joined the queue at #{you.position}")

            for bot in demo.participants[:-1]:
                thread = threading.Thread(target=run_bot, args=(demo, bot), daemon=True)
                thread.start()
                threads.append(thread)

            result: dict = {}
            worker = threading.Thread(target=lambda: result.update(seat=run_you(demo, you)), daemon=True)
            worker.start()
            with Live(render(demo), console=console, refresh_per_second=8) as live:
                while worker.is_alive():
                    live.update(render(demo))
                    time.sleep(0.125)
                time.sleep(1.5)
                live.update(render(demo))

            waited = time.monotonic() - demo.started
            seat = result.get("seat")
            changes = sum(1 for _, m in demo.timeline if "ahead" in m)
            if seat:
                console.print(
                    f"\n[bold green]Done:[/] you joined at #{you.position}, saw the queue move {changes} times "
                    f"and bought {seat['seatNumber']} after {waited:.0f}s."
                )
        except KeyboardInterrupt:
            console.print("\nStopped.")
        finally:
            demo.stop.set()
            for thread in threads:
                thread.join(timeout=20)
            cleanup(db, demo, ticket_ids)


if __name__ == "__main__":
    main()
