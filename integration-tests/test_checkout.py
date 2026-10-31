import uuid
from datetime import timedelta

import pytest

from conftest import (
    DECLINED_PAN,
    TICKET_PRICE_CENTS,
    card_token,
    checkout,
    hold,
    payments_for,
    seat_map,
    sell,
    ticket_row,
)


def test_checkout_sells_the_held_seat(db, make_user, make_event):
    user = make_user()
    event = make_event()
    ticket_id = event.ticket_ids[1]
    hold(db, ticket_id, user)

    response = checkout(user, ticket_id, card_token(), str(uuid.uuid4()))

    assert response.status_code == 200, response.text
    body = response.json()
    assert (body["ticketId"], body["status"], body["amountCents"]) == (ticket_id, "sold", TICKET_PRICE_CENTS)
    assert ticket_row(db, ticket_id)[:2] == ("sold", user.id)
    [(payment_id, status, charge_id, _, amount)] = payments_for(db, ticket_id)
    assert (payment_id, status, amount) == (body["paymentId"], "approved", TICKET_PRICE_CENTS)
    assert charge_id.startswith("ch_")
    seats = seat_map(user, event.id).json()["tickets"]
    assert seats[1]["status"] == "sold"


def test_retrying_with_the_same_key_replays_the_outcome(db, make_user, make_event):
    user = make_user()
    event = make_event()
    ticket_id = event.ticket_ids[0]
    hold(db, ticket_id, user)
    key, token = str(uuid.uuid4()), card_token()

    first = checkout(user, ticket_id, token, key)
    replay = checkout(user, ticket_id, token, key)

    assert first.status_code == replay.status_code == 200, replay.text
    assert replay.json() == first.json()
    assert len(payments_for(db, ticket_id)) == 1


def test_reusing_a_key_for_another_ticket_is_rejected(db, make_user, make_event):
    user = make_user()
    event = make_event()
    hold(db, event.ticket_ids[0], user)
    hold(db, event.ticket_ids[1], user)
    key = str(uuid.uuid4())
    assert checkout(user, event.ticket_ids[0], card_token(), key).status_code == 200

    response = checkout(user, event.ticket_ids[1], card_token(), key)

    assert response.status_code == 422, response.text
    assert response.json()["error"] == "idempotency_key_reused"
    assert ticket_row(db, event.ticket_ids[1])[0] == "booked"


def test_a_declined_card_keeps_the_hold_for_another_try(db, make_user, make_event):
    user = make_user()
    event = make_event()
    ticket_id = event.ticket_ids[0]
    hold(db, ticket_id, user)

    declined = checkout(user, ticket_id, card_token(DECLINED_PAN), str(uuid.uuid4()))
    retried = checkout(user, ticket_id, card_token(), str(uuid.uuid4()))

    assert declined.status_code == 402, declined.text
    assert declined.json()["error"] == "payment_declined"
    assert retried.status_code == 200, retried.text
    assert [p[1] for p in payments_for(db, ticket_id)] == ["declined", "approved"]


def test_an_unknown_payment_token_is_rejected(db, make_user, make_event):
    user = make_user()
    event = make_event()
    ticket_id = event.ticket_ids[0]
    hold(db, ticket_id, user)

    response = checkout(user, ticket_id, "tok_unknown", str(uuid.uuid4()))

    assert response.status_code == 422, response.text
    assert response.json()["error"] == "invalid_payment_token"
    assert ticket_row(db, ticket_id)[0] == "booked"
    assert [p[1] for p in payments_for(db, ticket_id)] == ["error"]


@pytest.mark.parametrize(
    "seat_state, reason",
    [
        ("available", "not_booked"),
        ("held_by_someone_else", "not_your_booking"),
        ("hold_expired", "hold_expired"),
        ("sold", "already_sold"),
    ],
)
def test_checkout_requires_your_own_active_hold(db, make_user, make_event, seat_state, reason):
    user, other = make_user(), make_user()
    event = make_event()
    ticket_id = event.ticket_ids[0]
    if seat_state == "held_by_someone_else":
        hold(db, ticket_id, other)
    elif seat_state == "hold_expired":
        hold(db, ticket_id, user, expires_in=timedelta(minutes=-1))
    elif seat_state == "sold":
        sell(db, ticket_id, other)

    response = checkout(user, ticket_id, card_token(), str(uuid.uuid4()))

    assert response.status_code == 409, response.text
    assert response.json()["error"] == reason
    assert payments_for(db, ticket_id) == []


def test_checkout_requires_an_idempotency_key(db, make_user, make_event):
    user = make_user()
    event = make_event()
    hold(db, event.ticket_ids[0], user)

    response = checkout(user, event.ticket_ids[0], card_token(), idempotency_key=None)

    assert response.status_code == 400, response.text
    assert payments_for(db, event.ticket_ids[0]) == []
