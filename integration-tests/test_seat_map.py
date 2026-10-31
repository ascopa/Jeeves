import uuid
from datetime import timedelta

from conftest import TICKET_PRICE_CENTS, hold, seat_map, sell


def test_seat_map_lists_every_seat_in_order_with_its_effective_status(db, make_user, make_event):
    user = make_user()
    event = make_event(seats=4)
    hold(db, event.ticket_ids[0], user)
    hold(db, event.ticket_ids[1], user, expires_in=timedelta(minutes=-1))
    sell(db, event.ticket_ids[2], user)

    response = seat_map(user, event.id)

    assert response.status_code == 200, response.text
    body = response.json()
    assert body["id"] == event.id
    assert body["name"] == event.name
    assert body["queueEnabled"] is False
    assert (body["totalTickets"], body["availableCount"]) == (4, 2)
    assert [(t["id"], t["seatNumber"], t["status"]) for t in body["tickets"]] == [
        (event.ticket_ids[0], "A-01", "booked"),
        (event.ticket_ids[1], "A-02", "available"),
        (event.ticket_ids[2], "A-03", "sold"),
        (event.ticket_ids[3], "A-04", "available"),
    ]
    assert body["tickets"][0]["type"] == "vip"
    assert all(t["priceCents"] == TICKET_PRICE_CENTS for t in body["tickets"])


def test_seat_map_of_unknown_event_is_404(make_user):
    response = seat_map(make_user(), str(uuid.uuid4()))

    assert response.status_code == 404, response.text
    assert response.json()["error"] == "event_not_found"


def test_seat_map_rejects_an_invalid_event_id(make_user):
    response = seat_map(make_user(), "not-a-uuid")

    assert response.status_code == 400, response.text


def test_seat_map_of_queued_event_requires_admission(make_user, make_event):
    event = make_event(queue=True)

    response = seat_map(make_user(), event.id)

    assert response.status_code == 403, response.text
    assert response.json()["error"] == "admission_required"
