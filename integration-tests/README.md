# Integration tests

End-to-end pytest suite, run through the gateway against the running stack.

The stack must be running (`make up` from the repository root). Then, from the root:

```bash
make integration-test
```

That builds a small Python image with `requirements.txt` and runs `pytest -v` in it, on the stack's network. Nothing needs to be installed on the host.

To run it on the host instead (Python 3.10 or newer):

```bash
cd integration-tests && python3 -m venv .venv && .venv/bin/pip install -r requirements.txt
```

```bash
.venv/bin/pytest -v
```

The defaults match `docker-compose.yml` as seen from the host. To point the suite elsewhere, set:
- `GATEWAY_URL` (default `http://localhost:8080`) and `CARD_PROVIDER_URL` (default `http://localhost:9090`);
- `MAX_HOLDS_PER_EVENT` (default 4) and `ADMIT_SEATS_FACTOR` (default 2), matching the services;
- the libpq variables `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER` and `PGPASSWORD` (default `localhost:5433`, `jeeves` for the rest);
- `POLL_TIMEOUT_SECONDS`.
