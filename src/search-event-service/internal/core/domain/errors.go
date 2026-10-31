package domain

type InvalidSearchError struct {
	Reason string
}

func (e InvalidSearchError) Error() string {
	return e.Reason
}
