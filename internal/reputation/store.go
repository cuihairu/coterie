package reputation

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// Store holds the reputation aggregation queries.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// UserExists reports whether the user id is known.
func (s *Store) UserExists(ctx context.Context, userID string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).
		Table("users").
		Where("id = ?", userID).
		Limit(1).Count(&n).Error
	return n > 0, err
}

// statusRow is one (status, currency) bucket of the aggregation.
type statusRow struct {
	Status   string
	Currency string
	N        int64
	Total    string
}

// Aggregate folds the user's contribution history into a Report.
func (s *Store) Aggregate(ctx context.Context, userID string) (*Report, error) {
	var rows []statusRow
	err := s.db.WithContext(ctx).
		Table("contributions co").
		Select("co.status AS status, s.currency AS currency, COUNT(*) AS n, SUM(co.amount) AS total").
		Joins("JOIN billing_periods bp ON bp.id = co.billing_period_id").
		Joins("JOIN subscriptions s ON s.id = bp.subscription_id").
		Joins("JOIN members m ON m.id = co.member_id").
		Where("m.user_id = ?", userID).
		Group("co.status, s.currency").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	rep := &Report{UserID: userID}
	byCur := map[string]*CurrencyStats{}
	for _, r := range rows {
		switch r.Status {
		case "paid":
			rep.Contributions.Paid += r.N
		case "pending":
			rep.Contributions.Pending += r.N
		case "waived":
			rep.Contributions.Waived += r.N
		case "cancelled":
			rep.Contributions.Cancelled += r.N
		}
		cs := byCur[r.Currency]
		if cs == nil {
			cs = &CurrencyStats{Currency: r.Currency, Paid: "0.00", Pending: "0.00"}
			byCur[r.Currency] = cs
		}
		if r.Status == "paid" {
			cs.Paid = r.Total
		} else if r.Status == "pending" {
			cs.Pending = r.Total
		}
	}
	for _, cs := range byCur {
		rep.Currencies = append(rep.Currencies, *cs)
	}
	// Stable order when several currencies ever appear.
	if len(rep.Currencies) > 1 {
		for i := 1; i < len(rep.Currencies); i++ {
			for j := i; j > 0 && rep.Currencies[j].Currency < rep.Currencies[j-1].Currency; j-- {
				rep.Currencies[j], rep.Currencies[j-1] = rep.Currencies[j-1], rep.Currencies[j]
			}
		}
	}

	// Ratio over chargeable shares only — waived/cancelled were never
	// owed. No chargeable history → null rather than a fake 100%.
	if chargeable := rep.Contributions.Paid + rep.Contributions.Pending; chargeable > 0 {
		ratio := fmtRatio(float64(rep.Contributions.Paid) / float64(chargeable))
		rep.PaymentRatio = &ratio
	}
	return rep, nil
}

// fmtRatio renders the settled share with two decimals ("0.75").
func fmtRatio(v float64) string {
	return fmt.Sprintf("%.2f", v)
}
