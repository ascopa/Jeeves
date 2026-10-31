MODULES := src/search-event-service src/booking-service src/checkout-service src/waiting-room-service \
           src/utils/card-provider-mock

.PHONY: up down reset logs tidy build integration-test demo demo-queue

up:
	docker compose up --build -d --wait

down:
	docker compose --profile tools down --volumes --remove-orphans --rmi local

reset:
	docker compose down -v

logs:
	docker compose logs -f

tidy:
	@for m in $(MODULES); do (cd $$m && go mod tidy) || exit 1; done

build:
	@for m in $(MODULES); do (cd $$m && go build ./... && go vet ./...) || exit 1; done

integration-test:
	@docker compose --progress quiet build integration-tests
	@docker compose --progress quiet run --rm integration-tests

demo:
	@docker compose --progress quiet run --rm newman

demo-queue:
	@docker compose --progress quiet build queue-demo
	@docker compose --progress quiet run --rm queue-demo $(ARGS)
