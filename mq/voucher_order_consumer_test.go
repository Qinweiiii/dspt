package mq

import "testing"

func TestParseOrder(t *testing.T) {
	values := map[string]interface{}{
		"id":        "1001",
		"userId":    "2002",
		"voucherId": "3003",
	}

	order, err := parseOrder(values)
	if err != nil {
		t.Fatalf("parseOrder returned error: %v", err)
	}
	if order.ID != 1001 || order.UserID != 2002 || order.VoucherID != 3003 {
		t.Fatalf("unexpected order: %#v", order)
	}
}

func TestParseOrderRejectsMissingField(t *testing.T) {
	_, err := parseOrder(map[string]interface{}{
		"id":     "1001",
		"userId": "2002",
	})
	if err == nil {
		t.Fatalf("expected missing voucherId error")
	}
}

func TestParseOrderRejectsNonStringField(t *testing.T) {
	_, err := parseOrder(map[string]interface{}{
		"id":        int64(1001),
		"userId":    "2002",
		"voucherId": "3003",
	})
	if err == nil {
		t.Fatalf("expected non-string field error")
	}
}

func TestParseOrderRejectsInvalidNumber(t *testing.T) {
	_, err := parseOrder(map[string]interface{}{
		"id":        "not-a-number",
		"userId":    "2002",
		"voucherId": "3003",
	})
	if err == nil {
		t.Fatalf("expected invalid number error")
	}
}

func TestShouldMoveToDeadLetter(t *testing.T) {
	cases := []struct {
		name          string
		deliveryCount int64
		want          bool
	}{
		{name: "below max retry", deliveryCount: maxRetry - 1, want: false},
		{name: "equal max retry", deliveryCount: maxRetry, want: false},
		{name: "above max retry", deliveryCount: maxRetry + 1, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldMoveToDeadLetter(tc.deliveryCount); got != tc.want {
				t.Fatalf("shouldMoveToDeadLetter(%d) = %v, want %v", tc.deliveryCount, got, tc.want)
			}
		})
	}
}
