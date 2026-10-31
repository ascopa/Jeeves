import uuid
from datetime import datetime, timedelta, timezone

import pytest
import requests

from conftest import GATEWAY_URL, MAX_HOLDS_PER_EVENT, User, book, hold, sell, ticket_row


def test_book_holds_the_seat_and_points_to_checkout(db, make_user, make_event):
    user = make_user()
    event = make_event()
    ticket_id = event.ticket_ids[0]

    response = book(user, ticket_id)

    assert response.status_code == 200, response.text
    body = response.json()
    assert (body["ticketId"], body["status"], body["seatNumber"]) == (ticket_id, "booked", "A-01")
    assert body["next"]["href"] == "/checkout"
    status, booked_by, expires_at = ticket_row(db, ticket_id)
    assert (status, booked_by) == ("booked", user.id)
    assert expires_at > datetime.now(timezone.utc)


def test_seat_held_by_someone_else_is_unavailable(db, make_user, make_event):
    holder, other = make_user(), make_user()
    event = make_event()
    hold(db, event.ticket_ids[0], holder)

    response = book(other, event.ticket_ids[0])

    assert response.status_code == 409, response.text
    assert (response.json()["error"], response.json()["status"]) == ("ticket_unavailable", "booked")
    assert ticket_row(db, event.ticket_ids[0])[1] == holder.id


def test_booking_your_own_hold_again_refreshes_it(db, make_user, make_event):
    user = make_user()
    event = make_event()
    hold(db, event.ticket_ids[0], user, expires_in=timedelta(minutes=1))
    before = ticket_row(db, event.ticket_ids[0])[2]

    response = book(user, event.ticket_ids[0])

    assert response.status_code == 200, response.text
    assert ticket_row(db, event.ticket_ids[0])[2] > before


def test_an_expired_hold_can_be_taken_by_someone_else(db, make_user, make_event):
    holder, other = make_user(), make_user()
    event = make_event()
    hold(db, event.ticket_ids[0], holder, expires_in=timedelta(minutes=-1))

    response = book(other, event.ticket_ids[0])

    assert response.status_code == 200, response.text
    assert ticket_row(db, event.ticket_ids[0])[1] == other.id


def test_sold_seat_is_unavailable(db, make_user, make_event):
    buyer, other = make_user(), make_user()
    event = make_event()
    sell(db, event.ticket_ids[0], buyer)

    response = book(other, event.ticket_ids[0])

    assert response.status_code == 409, response.text
    assert (response.json()["error"], response.json()["status"]) == ("ticket_unavailable", "sold")


def test_hold_limit_applies_per_user_and_event(make_user, make_event):
    user = make_user()
    event = make_event(seats=MAX_HOLDS_PER_EVENT + 1)
    other_event = make_event(seats=1)

    for ticket_id in event.ticket_ids[:MAX_HOLDS_PER_EVENT]:
        assert book(user, ticket_id).status_code == 200
    over_limit = book(user, event.ticket_ids[MAX_HOLDS_PER_EVENT])
    again = book(user, event.ticket_ids[0])
    elsewhere = book(user, other_event.ticket_ids[0])

    assert over_limit.status_code == 409, over_limit.text
    assert over_limit.json()["error"] == "hold_limit_reached"
    assert again.status_code == 200, again.text
    assert elsewhere.status_code == 200, elsewhere.text


def test_unknown_ticket_is_404(make_user):
    response = book(make_user(), str(uuid.uuid4()))

    assert response.status_code == 404, response.text
    assert response.json()["error"] == "ticket_not_found"


def test_an_unknown_user_is_rejected(make_event):
    event = make_event()

    response = book(User(id=str(uuid.uuid4())), event.ticket_ids[0])

    assert response.status_code == 403, response.text
    assert response.json()["error"] == "unknown_user"


@pytest.mark.parametrize("user_id", [None, "not-a-uuid"], ids=["missing", "malformed"])
def test_booking_needs_a_user_id(make_event, user_id):
    event = make_event()
    headers = {"X-User-Id": user_id} if user_id else {}

    response = requests.post(f"{GATEWAY_URL}/book", json={"ticketId": event.ticket_ids[0]}, headers=headers, timeout=10)

    assert response.status_code == 401, response.text


def test_book_rejects_an_invalid_ticket_id(make_user):
    response = book(make_user(), "not-a-uuid")

    assert response.status_code == 400, response.text


def test_queued_event_requires_admission_to_book(db, make_user, make_event):
    user = make_user()
    event = make_event(queue=True)

    response = book(user, event.ticket_ids[0])

    assert response.status_code == 403, response.text
    assert response.json()["error"] == "admission_required"
    assert ticket_row(db, event.ticket_ids[0])[0] == "available"
