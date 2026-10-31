package domain

const (
	ChargeApproved  = "approved"
	ChargeDeclined  = "declined"
	ChargeRefunded  = "refunded"
	DeclinedTestPAN = "4000000000000002"
)

type Card struct {
	Token    string
	Last4    string
	Brand    string
	Declines bool
}

type Charge struct {
	ID             string
	Token          string
	AmountCents    int64
	IdempotencyKey string
	Status         string
}
