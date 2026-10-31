import uuid
from concurrent.futures import ThreadPoolExecutor

import requests

from conftest import GATEWAY_URL, book, seat_map


def test_writes_over_the_per_user_limit_get_a_json_429(make_user):
    user = make_user()
    missing_ticket = str(uuid.uuid4())


    with ThreadPoolExecutor(max_workers=30) as pool:
        responses = list(pool.map(lambda _: book(user, missing_ticket), range(30)))

    limited = [r for r in responses if r.status_code == 429]
    assert limited, [r.status_code for r in responses]
    assert limited[0].json() == {"error": "rate_limited"}
    assert limited[0].headers["Retry-After"] == "1"
    assert {r.status_code for r in responses} <= {404, 429}


def test_another_users_limit_is_separate(make_user):
    noisy, quiet = make_user(), make_user()
    missing_ticket = str(uuid.uuid4())
    with ThreadPoolExecutor(max_workers=30) as pool:
        list(pool.map(lambda _: book(noisy, missing_ticket), range(30)))

    response = book(quiet, missing_ticket)

    assert response.status_code == 404, response.text


def test_seat_maps_are_gzipped(make_user, make_event):
    event = make_event(seats=20)

    response = seat_map(make_user(), event.id)

    assert response.status_code == 200, response.text
    assert response.headers.get("Content-Encoding") == "gzip"


def test_internal_routes_are_not_exposed(make_user, make_event):
    user = make_user()
    event = make_event(queue=True)

    response = requests.post(
        f"{GATEWAY_URL}/internal/queue/{event.id}/release", json={"userId": user.id}, timeout=10
    )

    assert response.status_code == 404, response.text
