package cardprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"jeeves/checkout-service/internal/core/domain"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	return &Client{baseURL: baseURL, apiKey: apiKey, http: httpClient}
}

type chargeResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (c *Client) Charge(ctx context.Context, paymentToken string, amountCents int64, idempotencyKey string) (domain.ChargeResult, error) {
	body, err := json.Marshal(map[string]any{
		"token":          paymentToken,
		"amountCents":    amountCents,
		"idempotencyKey": idempotencyKey,
	})
	if err != nil {
		return domain.ChargeResult{}, err
	}
	res, err := c.do(ctx, http.MethodPost, "/charges", body)
	if err != nil {

		return domain.ChargeResult{}, fmt.Errorf("%w: %v", domain.ErrChargeOutcomeUnknown, err)
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusOK:
		return decodeCharge(res)
	case res.StatusCode == http.StatusUnprocessableEntity:
		return domain.ChargeResult{}, domain.ErrInvalidPaymentToken
	case res.StatusCode >= http.StatusInternalServerError:
		return domain.ChargeResult{}, fmt.Errorf("%w: card provider charge responded %d", domain.ErrChargeOutcomeUnknown, res.StatusCode)
	default:
		return domain.ChargeResult{}, fmt.Errorf("card provider rejected charge: %d", res.StatusCode)
	}
}

func (c *Client) Refund(ctx context.Context, chargeID string) error {
	body, err := json.Marshal(map[string]any{"chargeId": chargeID})
	if err != nil {
		return err
	}
	res, err := c.do(ctx, http.MethodPost, "/refunds", body)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("card provider refund responded %d", res.StatusCode)
	}
	return nil
}

func decodeCharge(res *http.Response) (domain.ChargeResult, error) {
	var body chargeResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return domain.ChargeResult{}, fmt.Errorf("decoding card provider charge: %w", err)
	}
	return domain.ChargeResult{ID: body.ID, Status: body.Status}, nil
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	return c.http.Do(req)
}
