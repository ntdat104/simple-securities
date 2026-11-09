package futures

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-securities/pkg/database"
	"simple-securities/pkg/symbols"
)

// settleDueFunding settles every contract whose latest funding boundary is not settled yet by
// this leader. On election the latest boundary is attempted once (catch-up of a boundary missed
// while no leader ran); settlement is idempotent through the funding_rates primary key.
func (s *Service) settleDueFunding(ctx context.Context, st *leaderState, now time.Time) {
	boundary := fundingBoundary(now, s.cfg.FundingInterval)
	for _, sym := range symbols.AllMarket(symbols.MarketFutures) {
		if last, ok := st.settled[sym.Name]; ok && !last.Before(boundary) {
			continue
		}
		mark, ok := s.prices.Mark(sym.Name)
		if !ok {
			continue // retried on the next tick
		}
		ms := st.mark(sym.Name)
		rate := ms.estimatedRate()
		n, err := s.settleFunding(ctx, sym, boundary, rate, mark.Mark)
		if err != nil {
			if ctx.Err() == nil {
				s.log.Warn("funding settlement failed", zap.String("symbol", sym.Name), zap.Error(err))
			}
			continue
		}
		st.settled[sym.Name] = boundary
		ms.resetWindow()
		if n >= 0 {
			s.log.Info("funding settled", zap.String("symbol", sym.Name), zap.Time("funding_time", boundary),
				zap.String("rate", rate.String()), zap.String("mark", mark.Mark.String()), zap.Int("positions", n))
		}
	}
}

// settleFunding pays funding for every open position of a contract at one boundary:
// payment = size · M · rate (longs pay when rate > 0); CROSSED from/to wallet_balance, ISOLATED
// from/to isolated_margin. It returns the number of positions charged, or -1 when the boundary
// was already settled.
func (s *Service) settleFunding(ctx context.Context, sym symbols.Symbol, boundary time.Time,
	rate, mark decimal.Decimal) (int, error) {
	charged := -1
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		charged = -1
		res := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&FundingRecord{
			Symbol: sym.Name, FundingTime: boundary, Rate: rate, MarkPrice: mark, CreatedAt: s.now(),
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // already settled
		}
		charged = 0
		var users []string
		if err := tx.WithContext(ctx).Model(&Position{}).Where("symbol = ? AND size <> 0", sym.Name).
			Order("user_id").Pluck("user_id", &users).Error; err != nil {
			return err
		}
		if len(users) == 0 {
			return nil
		}
		accounts, err := lockAccounts(ctx, tx, users)
		if err != nil {
			return err
		}
		var positions []Position
		if err := tx.WithContext(ctx).Clauses(forUpdate).Where("symbol = ? AND user_id IN ?", sym.Name, users).
			Order("user_id").Find(&positions).Error; err != nil {
			return err
		}
		now := s.now()
		var incomes []Income
		touched := map[string]bool{}
		for i := range positions {
			p := &positions[i]
			acct, ok := accounts[p.UserID]
			if p.Size.IsZero() || !ok {
				continue
			}
			delta := money(p.Size.Mul(mark).Mul(rate)).Neg()
			if delta.IsZero() {
				continue
			}
			if p.MarginType == MarginIsolated {
				p.IsolatedMargin = p.IsolatedMargin.Add(delta)
				if err := savePosition(ctx, tx, p); err != nil {
					return err
				}
			} else {
				acct.WalletBalance = acct.WalletBalance.Add(delta)
				touched[p.UserID] = true
			}
			incomes = append(incomes, Income{UserID: p.UserID, Symbol: sym.Name, IncomeType: IncomeFundingFee,
				Income: delta, Asset: Asset, Info: rate.String(), Time: now})
			charged++
		}
		if err := insertIncome(ctx, tx, incomes); err != nil {
			return err
		}
		for _, p := range positions {
			if _, ok := accounts[p.UserID]; !ok || p.Size.IsZero() {
				continue
			}
			acct := accounts[p.UserID]
			if touched[p.UserID] {
				if err := saveAccount(ctx, tx, acct); err != nil {
					return err
				}
			}
			if err := s.emitAccountUpdateLoaded(ctx, tx, acct, reasonFunding,
				func(q Position) bool { return q.Symbol == sym.Name }); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if charged > 0 {
		s.disp.Notify()
	}
	return charged, nil
}
