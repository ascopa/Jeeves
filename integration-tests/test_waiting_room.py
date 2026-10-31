import time
import uuid

from conftest import (
    ADMIT_SEATS_FACTOR,
    POLL_TIMEOUT_SECONDS,
    admit,
    book,
    card_token,
    checkout,
    join_queue,
    queue_status,
    seat_map,
    sell,
    wait_for,
    wait_until_admitted,
)


def admitted_count(users, event_id) -> int:
    return sum(queue_status(u, event_id).json()["status"] == "admitted" for u in users)


def test_joining_an_event_without_a_queue_is_rejected(make_user, make_event):
    event = make_event(queue=False)

    response = join_queue(make_user(), event.id)

    assert response.status_code == 409, response.text
    assert response.json()["error"] == "queue_not_enabled"


def test_joining_an_unknown_event_is_404(make_user):
    response = join_queue(make_user(), str(uuid.uuid4()))

    assert response.status_code == 404, response.text
    assert response.json()["error"] == "event_not_found"


def test_status_before_joining_is_404(make_user, make_event):
    event = make_event(queue=True)

    response = queue_status(make_user(), event.id)

    assert response.status_code == 404, response.text
    assert response.json()["error"] == "not_in_queue"


def test_positions_are_handed_out_in_order_and_joining_again_keeps_yours(make_user, make_event):
    event = make_event(queue=True, max_active_users=1)
    users = [make_user() for _ in range(3)]

    positions = [join_queue(u, event.id).json()["position"] for u in users]
    rejoined = join_queue(users[0], event.id).json()["position"]
    wait_until_admitted(users[0], event.id)
    waiting = queue_status(users[2], event.id)

    assert positions == [1, 2, 3]
    assert rejoined == 1
    assert waiting.status_code == 200, waiting.text
    body = waiting.json()
    assert (body["status"], body["position"], body["ahead"]) == ("waiting", 3, 2)
    assert body["estimatedWaitSeconds"] > 0
    assert int(waiting.headers["Retry-After"]) >= 1


def test_admission_token_unlocks_the_seat_map_and_booking(make_user, make_event):
    user, other = make_user(), make_user()
    event = make_event(queue=True)
    other_event = make_event(queue=True)

    admission = admit(user, event.id)
    other_events_admission = admit(user, other_event.id)

    assert seat_map(user, event.id).status_code == 403
    assert seat_map(user, event.id, admission).status_code == 200
    assert seat_map(other, event.id, admission).status_code == 403
    assert seat_map(user, event.id, other_events_admission).status_code == 403
    assert seat_map(user, event.id, "not-a-token").status_code == 403
    assert book(other, event.ticket_ids[0], admission).status_code == 403
    response = book(user, event.ticket_ids[0], admission)
    assert response.status_code == 200, response.text


def test_full_slots_hold_the_queue_until_a_purchase_frees_one(make_user, make_event):
    event = make_event(queue=True, max_active_users=1)
    first, second = make_user(), make_user()

    admission = admit(first, event.id)
    assert join_queue(second, event.id).status_code == 200
    time.sleep(3)
    assert queue_status(second, event.id).json()["status"] == "waiting"

    assert book(first, event.ticket_ids[0], admission).status_code == 200
    response = checkout(first, event.ticket_ids[0], card_token(), str(uuid.uuid4()))
    assert response.status_code == 200, response.text


    wait_until_admitted(second, event.id)


def test_admissions_are_capped_by_the_seats_left(make_user, make_event):
    seats = 2
    event = make_event(queue=True, seats=seats, admit_rate=50, max_active_users=1000)
    users = [make_user() for _ in range(ADMIT_SEATS_FACTOR * seats + 2)]
    for user in users:
        assert join_queue(user, event.id).status_code == 200

    wait_for(lambda: admitted_count(users, event.id), lambda n: n > 0, "the first admissions")
    time.sleep(3)

    assert admitted_count(users, event.id) == ADMIT_SEATS_FACTOR * seats


def test_admission_rate_is_not_multiplied_by_replicas(make_user, make_event):
    rate, joiners = 2, 20
    event = make_event(queue=True, seats=joiners, admit_rate=rate)
    users = [make_user() for _ in range(joiners)]
    for user in users:
        assert join_queue(user, event.id).status_code == 200


    first, last = users[0], users[-1]
    wait_until_admitted(first, event.id)
    started = time.monotonic()
    deadline = started + POLL_TIMEOUT_SECONDS
    while queue_status(last, event.id).json()["status"] != "admitted":
        assert time.monotonic() < deadline, "the last user was never admitted"
        time.sleep(0.2)
    elapsed = time.monotonic() - started

    expected = (joiners - rate) / rate
    assert expected - 2 <= elapsed <= expected + 2, f"last user admitted {elapsed:.1f}s after the first"


def test_a_sold_out_event_closes_its_queue(db, make_user, make_event):
    buyer, waiting = make_user(), make_user()

    event = make_event(queue=True, seats=2, max_active_users=1)
    admit(buyer, event.id)
    assert join_queue(waiting, event.id).status_code == 200
    assert queue_status(waiting, event.id).json()["status"] == "waiting"

    for ticket_id in event.ticket_ids:
        sell(db, ticket_id, buyer)


    wait_for(lambda: queue_status(waiting, event.id).json()["status"], lambda s: s == "closed", "the queue to close")
    late = wait_for(lambda: join_queue(make_user(), event.id), lambda r: r.status_code == 409, "joins to be refused")
    assert late.json()["error"] == "queue_closed"
