from datetime import timedelta

import pytest

from conftest import hold, search, sell


def test_search_is_public():
    response = search("integration")

    assert response.status_code == 200, response.text


def test_search_finds_event_by_partial_name_with_live_counts(db, make_user, make_event):
    user = make_user()
    event = make_event(seats=4)
    hold(db, event.ticket_ids[0], user)
    hold(db, event.ticket_ids[1], user, expires_in=timedelta(minutes=-1))
    sell(db, event.ticket_ids[2], user)


    response = search(event.name[-6:].upper())

    assert response.status_code == 200, response.text
    matches = [e for e in response.json()["events"] if e["id"] == event.id]
    assert len(matches) == 1
    found = matches[0]
    assert found["name"] == event.name
    assert (found["totalTickets"], found["availableCount"]) == (4, 2)
    assert found["queueEnabled"] is False
    assert "tickets" not in found


def test_search_with_no_match_returns_an_empty_list():
    response = search("no event is called this zqxj")

    assert response.status_code == 200, response.text
    assert response.json()["events"] == []


@pytest.mark.parametrize("name", ["", "x" * 101], ids=["missing", "too-long"])
def test_search_rejects_invalid_names(name):
    response = search(name)

    assert response.status_code == 400, response.text


def test_search_is_open_while_an_event_is_queued(make_user, make_event):
    event = make_event(queue=True)

    response = search(event.name)

    assert response.status_code == 200, response.text
    [found] = [e for e in response.json()["events"] if e["id"] == event.id]
    assert found["queueEnabled"] is True
