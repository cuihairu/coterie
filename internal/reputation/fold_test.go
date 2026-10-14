package reputation

import "testing"

// White-box tests for fold: the single derivation both the
// authenticated report and the marketplace badge (D18) run through, so
// the projection cannot drift between the two.

func TestFoldNoHistory(t *testing.T) {
	rep := fold("u1", nil)
	if rep.UserID != "u1" {
		t.Fatalf("user_id = %q, want u1", rep.UserID)
	}
	if rep.Contributions != (ContributionCounts{}) {
		t.Fatalf("counts = %+v, want all zero", rep.Contributions)
	}
	if len(rep.Currencies) != 0 {
		t.Fatalf("currencies = %+v, want none", rep.Currencies)
	}
	if rep.PaymentRatio != nil {
		t.Fatalf("payment_ratio = %v, want null without chargeable history", *rep.PaymentRatio)
	}
}

func TestFoldBucketsByStatus(t *testing.T) {
	rep := fold("u1", []statusRow{
		{Status: "paid", Currency: "USD", N: 3, Total: "30.00"},
		{Status: "pending", Currency: "USD", N: 1, Total: "10.00"},
		{Status: "waived", Currency: "USD", N: 2, Total: "20.00"},
		{Status: "cancelled", Currency: "USD", N: 1, Total: "10.00"},
	})
	want := ContributionCounts{Paid: 3, Pending: 1, Waived: 2, Cancelled: 1}
	if rep.Contributions != want {
		t.Fatalf("counts = %+v, want %+v", rep.Contributions, want)
	}
	if len(rep.Currencies) != 1 {
		t.Fatalf("currencies = %+v, want one", rep.Currencies)
	}
	cs := rep.Currencies[0]
	if cs.Currency != "USD" || cs.Paid != "30.00" || cs.Pending != "10.00" {
		t.Fatalf("currency stats = %+v, want USD 30.00/10.00", cs)
	}
}

func TestFoldCurrencyWithOnlyPending(t *testing.T) {
	rep := fold("u1", []statusRow{
		{Status: "waived", Currency: "EUR", N: 1, Total: "5.00"},
	})
	if len(rep.Currencies) != 1 {
		t.Fatalf("currencies = %+v, want one", rep.Currencies)
	}
	cs := rep.Currencies[0]
	if cs.Paid != "0.00" || cs.Pending != "0.00" {
		t.Fatalf("currency stats = %+v, want zeroed totals", cs)
	}
}

func TestFoldSortsCurrencies(t *testing.T) {
	rep := fold("u1", []statusRow{
		{Status: "paid", Currency: "USD", N: 1, Total: "1.00"},
		{Status: "paid", Currency: "EUR", N: 1, Total: "2.00"},
		{Status: "paid", Currency: "CNY", N: 1, Total: "3.00"},
		{Status: "paid", Currency: "AUD", N: 1, Total: "4.00"},
	})
	want := []string{"AUD", "CNY", "EUR", "USD"}
	if len(rep.Currencies) != len(want) {
		t.Fatalf("currencies = %+v, want %v", rep.Currencies, want)
	}
	for i, cur := range want {
		if rep.Currencies[i].Currency != cur {
			t.Fatalf("currencies[%d] = %q, want %q (order %+v)", i, rep.Currencies[i].Currency, cur, rep.Currencies)
		}
	}
}

func TestFoldPaymentRatio(t *testing.T) {
	t.Run("counts only chargeable shares", func(t *testing.T) {
		// 3 paid + 1 pending out of 7 rows: waived and cancelled were
		// never owed, so they stay out of the denominator.
		rep := fold("u1", []statusRow{
			{Status: "paid", Currency: "USD", N: 3, Total: "30.00"},
			{Status: "pending", Currency: "USD", N: 1, Total: "10.00"},
			{Status: "waived", Currency: "USD", N: 2, Total: "20.00"},
			{Status: "cancelled", Currency: "USD", N: 1, Total: "10.00"},
		})
		if rep.PaymentRatio == nil {
			t.Fatal("payment_ratio is null despite chargeable history")
		}
		if *rep.PaymentRatio != "0.75" {
			t.Fatalf("payment_ratio = %q, want 0.75", *rep.PaymentRatio)
		}
	})

	t.Run("fully settled is 1.00", func(t *testing.T) {
		rep := fold("u1", []statusRow{
			{Status: "paid", Currency: "USD", N: 2, Total: "20.00"},
		})
		if rep.PaymentRatio == nil || *rep.PaymentRatio != "1.00" {
			t.Fatalf("payment_ratio = %v, want 1.00", rep.PaymentRatio)
		}
	})

	t.Run("nothing settled is 0.00", func(t *testing.T) {
		rep := fold("u1", []statusRow{
			{Status: "pending", Currency: "USD", N: 2, Total: "20.00"},
		})
		if rep.PaymentRatio == nil || *rep.PaymentRatio != "0.00" {
			t.Fatalf("payment_ratio = %v, want 0.00", rep.PaymentRatio)
		}
	})

	t.Run("no chargeable history stays null", func(t *testing.T) {
		rep := fold("u1", []statusRow{
			{Status: "waived", Currency: "USD", N: 2, Total: "20.00"},
			{Status: "cancelled", Currency: "USD", N: 1, Total: "10.00"},
		})
		if rep.PaymentRatio != nil {
			t.Fatalf("payment_ratio = %q, want null", *rep.PaymentRatio)
		}
	})
}

func TestFoldIgnoresUnknownStatus(t *testing.T) {
	// A status the projection does not know must not silently land in
	// a bucket or the chargeable denominator.
	rep := fold("u1", []statusRow{
		{Status: "refunded", Currency: "USD", N: 4, Total: "40.00"},
	})
	if rep.Contributions != (ContributionCounts{}) {
		t.Fatalf("counts = %+v, want all zero", rep.Contributions)
	}
	if rep.PaymentRatio != nil {
		t.Fatalf("payment_ratio = %q, want null", *rep.PaymentRatio)
	}
}

func TestFmtRatioRoundsToTwoDecimals(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{2.0 / 3.0, "0.67"},
		{1.0 / 3.0, "0.33"},
		{0, "0.00"},
		{1, "1.00"},
	}
	for _, tc := range cases {
		if got := fmtRatio(tc.in); got != tc.want {
			t.Fatalf("fmtRatio(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
