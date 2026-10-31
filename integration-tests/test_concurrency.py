from concurrent.futures import ThreadPoolExecutor

from conftest import MAX_HOLDS_PER_EVENT, book, ticket_row


def test_200_users_racing_for_one_seat_get_exactly_one_hold(db, make_user, make_event):
    users = [make_user() for _ in range(200)]
    event = make_event(seats=1)
    ticket_id = event.ticket_ids[0]

    with ThreadPoolExecutor(max_workers=len(users)) as pool:
        responses = list(pool.map(lambda u: book(u, ticket_id), users))

    codes = [r.status_code for r in responses]
    winners = [u for u, r in zip(users, responses) if r.status_code == 200]
    assert len(winners) == 1, sorted(set(codes))
    assert codes.count(409) == len(users) - 1, sorted(set(codes))

    assert {r.json()["error"] for r in responses if r.status_code == 409} <= {"ticket_unavailable", "ticket_busy"}
    assert ticket_row(db, ticket_id)[:2] == ("booked", winners[0].id)


def test_one_user_booking_past_the_limit_at_once_gets_exactly_the_limit(db, make_user, make_event):
    user = make_user()
    event = make_event(seats=MAX_HOLDS_PER_EVENT * 2)

    with ThreadPoolExecutor(max_workers=len(event.ticket_ids)) as pool:
        responses = list(pool.map(lambda t: book(user, t), event.ticket_ids))

    held = [t for t, r in zip(event.ticket_ids, responses) if r.status_code == 200]
    rejected = [r.json()["error"] for r in responses if r.status_code == 409]
    assert len(held) == MAX_HOLDS_PER_EVENT, [r.status_code for r in responses]
    assert rejected == ["hold_limit_reached"] * MAX_HOLDS_PER_EVENT
    assert sum(ticket_row(db, t)[:2] == ("booked", user.id) for t in event.ticket_ids) == MAX_HOLDS_PER_EVENT

    again = book(user, held[0])
    assert again.status_code == 200, again.text
