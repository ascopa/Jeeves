# Demo collection

Import `jeeves.postman_collection.json` into Postman (File → Import) with the stack running (`make up`). Run the folders in order, request by request or with the Collection Runner. `make demo` runs the whole collection from the terminal.

| Folder | What it shows |
|---|---|
| 1. Open event | Search Jazz Sessions, see its seats, hold one, tokenize a card and check out. Checking out again with the same key returns the same purchase. |
| 2. Queued event | Rock en el Parque: the seat map is closed without admission; join the queue, check the status until admitted, then see the seats, hold one and check out. |
| 3. When things go wrong | Another user tries to book the seat sold in folder 1, then a card is declined and the same seat is bought with another card. |

- **Users.** Requests identify the user with the `X-User-Id` header: `userId` is Ada and `otherUserId` is Alan, both from the seed. Change `userId` to act as someone else.
- **Requests feed each other.** Each one saves what the next needs: the event, the seat, the payment token, the idempotency key and the admission token.
- **Waiting.** When sent by hand, send "Check the queue status" again until it says `admitted`. The Collection Runner and `make demo` repeat it on their own, waiting as long as the response's `Retry-After` asks.
- **Running it again.** Each run buys two of Jazz Sessions' eight seats and one in Rock en el Parque, so it can run a few times before they sell out. `make reset up` restores the seed.
