INSERT INTO users (id, name, email) VALUES
    ('11111111-1111-1111-1111-111111111111', 'Ada Lovelace', 'ada@example.com'),
    ('22222222-2222-2222-2222-222222222222', 'Alan Turing', 'alan@example.com'),
    ('33333333-3333-3333-3333-333333333333', 'Grace Hopper', 'grace@example.com');

INSERT INTO venues (id, name, size) VALUES
    ('aaaaaaaa-0000-0000-0000-000000000001', 'Luna Park', 10),
    ('aaaaaaaa-0000-0000-0000-000000000002', 'Teatro Colon', 8);

INSERT INTO events (id, name, date, venue_id, queue_enabled, admit_rate_per_sec) VALUES
    ('eeeeeeee-0000-0000-0000-000000000001', 'Rock en el Parque', '2026-12-05 21:00:00-03', 'aaaaaaaa-0000-0000-0000-000000000001', true, 2),
    ('eeeeeeee-0000-0000-0000-000000000002', 'Rock Nacional Night', '2026-12-12 22:00:00-03', 'aaaaaaaa-0000-0000-0000-000000000002', false, 50),
    ('eeeeeeee-0000-0000-0000-000000000003', 'Jazz Sessions', '2026-11-20 20:30:00-03', 'aaaaaaaa-0000-0000-0000-000000000002', false, 50),
    ('eeeeeeee-0000-0000-0000-000000000004', 'Symphony No. 9', '2027-01-15 19:00:00-03', 'aaaaaaaa-0000-0000-0000-000000000001', false, 50);

CREATE TEMP TABLE seed_seats AS
SELECT e.id AS event_id,
       s AS seat,
       md5(e.id::text || '-seat-' || s)::uuid AS ticket_id
FROM events e
CROSS JOIN generate_series(1, 8) AS s;

INSERT INTO tickets (id, price_cents, seat_number, type)
SELECT ticket_id,
       CASE WHEN seat <= 2 THEN 2500000 ELSE 1200000 END,
       'A-' || lpad(seat::text, 2, '0'),
       CASE WHEN seat <= 2 THEN 'vip' ELSE 'general' END::ticket_type
FROM seed_seats;

UPDATE events e
SET ticket_ids = (SELECT array_agg(ticket_id ORDER BY seat) FROM seed_seats WHERE seed_seats.event_id = e.id);

UPDATE tickets t
SET status = 'sold', booked_by = '33333333-3333-3333-3333-333333333333',
    booked_at = now() - interval '2 days', hold_expires_at = now() - interval '2 days' + interval '10 minutes',
    sold_at = now() - interval '2 days'
FROM seed_seats s
WHERE t.id = s.ticket_id AND s.event_id = 'eeeeeeee-0000-0000-0000-000000000001' AND s.seat = 8;

UPDATE tickets t
SET status = 'booked', booked_by = '22222222-2222-2222-2222-222222222222',
    booked_at = now(), hold_expires_at = now() + interval '1 year'
FROM seed_seats s
WHERE t.id = s.ticket_id AND s.event_id = 'eeeeeeee-0000-0000-0000-000000000001' AND s.seat = 7;

UPDATE tickets t
SET status = 'booked', booked_by = '22222222-2222-2222-2222-222222222222',
    booked_at = now() - interval '1 hour', hold_expires_at = now() - interval '50 minutes'
FROM seed_seats s
WHERE t.id = s.ticket_id AND s.event_id = 'eeeeeeee-0000-0000-0000-000000000001' AND s.seat = 6;

DROP TABLE seed_seats;
