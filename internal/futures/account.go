package futures

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"simple-securities/pkg/apperr"
	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/outbox"
	"simple-securities/pkg/symbols"
)

// PositionView is a position with its derived risk values.
type PositionView struct {
	Position
	Mark             decimal.Decimal
	UPnL             decimal.Decimal
	Notional         decimal.Decimal
	InitialMargin    decimal.Decimal
	MaintMargin      decimal.Decimal
	LiquidationPrice decimal.Decimal
	BreakEvenPrice   decimal.Decimal
}

// AccountView is the account with its derived risk values.
type AccountView struct {
	Risk      accountRisk
	Positions []PositionView
}

// SymbolSetting is the user's leverage / margin type of a contract.
type SymbolSetting struct {
	Symbol      string
	Leverage    int
	MarginType  string
	MaxNotional decimal.Decimal
}

// TransferResult is the outcome of a futures -> spot transfer request.
type TransferResult struct {
	TransferID string
	Asset      string
	Amount     decimal.Decimal
	Status     string
}

// GetAccount returns the account with every open position.
func (s *Service) GetAccount(ctx context.Context, userID string) (AccountView, error) {
	if err := requireUser(userID); err != nil {
		return AccountView{}, err
	}
	return s.accountView(ctx, s.db, userID, "")
}

// ListPositions returns the open positions, or the position of one symbol (even when flat).
func (s *Service) ListPositions(ctx context.Context, userID, symbol string) ([]PositionView, error) {
	if err := requireUser(userID); err != nil {
		return nil, err
	}
	if symbol != "" {
		sym, err := lookupSymbol(symbol)
		if err != nil {
			return nil, err
		}
		symbol = sym.Name
	}
	v, err := s.accountView(ctx, s.db, userID, symbol)
	if err != nil {
		return nil, err
	}
	if symbol == "" {
		return v.Positions, nil
	}
	for _, p := range v.Positions {
		if p.Symbol == symbol {
			return []PositionView{p}, nil
		}
	}
	return nil, nil
}

// accountView loads and evaluates an account (display prices). include adds the (possibly flat)
// position of that symbol; missing rows are reported with default settings.
func (s *Service) accountView(ctx context.Context, db *gorm.DB, userID, include string) (AccountView, error) {
	acct, err := getAccount(ctx, db, userID)
	if err != nil && !errors.Is(err, errNotFound) {
		return AccountView{}, err
	}
	positions, err := listPositions(ctx, db, userID)
	if err != nil {
		return AccountView{}, err
	}
	om, err := openOrderMargin(ctx, db, userID)
	if err != nil {
		return AccountView{}, err
	}
	if include != "" && findPosition(positions, include) == nil {
		sym, _ := symbols.GetMarket(symbols.MarketFutures, include)
		positions = append(positions, Position{UserID: userID, Symbol: include, MarginType: MarginCrossed,
			Leverage: defaultLeverageFor(sym)})
	}
	return s.buildView(acct.WalletBalance, positions, om, include)
}

func (s *Service) buildView(wallet decimal.Decimal, positions []Position, orderMargin decimal.Decimal,
	include string) (AccountView, error) {
	r, err := computeRisk(wallet, positions, orderMargin, s.prices.Price, false)
	if err != nil {
		return AccountView{}, err
	}
	v := AccountView{Risk: r, Positions: []PositionView{}}
	for _, p := range positions {
		if p.Size.IsZero() && p.Symbol != include {
			continue
		}
		v.Positions = append(v.Positions, s.positionView(p, r))
	}
	return v, nil
}

func (s *Service) positionView(p Position, r accountRisk) PositionView {
	sym, _ := symbols.GetMarket(symbols.MarketFutures, p.Symbol)
	pr, ok := r.Positions[p.Symbol]
	if !ok { // flat: show the current mark only
		pr.Mark, _ = s.prices.Price(p.Symbol)
	}
	return PositionView{
		Position: p, Mark: pr.Mark, UPnL: pr.UPnL, Notional: pr.Notional, InitialMargin: pr.IM,
		MaintMargin: pr.MM, LiquidationPrice: liquidationPrice(p, sym, r), BreakEvenPrice: breakEvenPrice(p, sym),
	}
}

// lockUserState ensures the account and the symbol's settings row exist, then locks the account
// and every position of the user (lock order: account, positions by symbol).
func lockUserState(ctx context.Context, tx *gorm.DB, userID string, sym symbols.Symbol) (*Account, []Position, *Position, error) {
	if err := ensureAccounts(ctx, tx, []string{userID}); err != nil {
		return nil, nil, nil, err
	}
	acct, err := lockAccount(ctx, tx, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := ensurePosition(ctx, tx, userID, sym.Name, defaultLeverageFor(sym)); err != nil {
		return nil, nil, nil, err
	}
	positions, err := lockPositions(ctx, tx, []string{userID})
	if err != nil {
		return nil, nil, nil, err
	}
	pos := findPosition(positions, sym.Name)
	if pos == nil {
		return nil, nil, nil, fmt.Errorf("position %s/%s missing after ensure", userID, sym.Name)
	}
	return acct, positions, pos, nil
}

// SetLeverage changes the leverage of a contract. With an open position the new margin
// requirement must still be covered.
func (s *Service) SetLeverage(ctx context.Context, userID, symbol string, leverage int) (SymbolSetting, error) {
	if err := requireUser(userID); err != nil {
		return SymbolSetting{}, err
	}
	sym, err := lookupSymbol(symbol)
	if err != nil {
		return SymbolSetting{}, err
	}
	if leverage < 1 || leverage > sym.MaxLeverage {
		return SymbolSetting{}, apperr.Invalid("INVALID_LEVERAGE", "leverage must be between 1 and %d", sym.MaxLeverage)
	}
	var out SymbolSetting
	err = database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		acct, positions, pos, err := lockUserState(ctx, tx, userID, sym)
		if err != nil {
			return err
		}
		pos.Leverage = leverage
		if !pos.Size.IsZero() {
			if err := s.checkLeverageChange(ctx, tx, acct, positions, *pos, sym); err != nil {
				return err
			}
		}
		if err := savePosition(ctx, tx, pos); err != nil {
			return err
		}
		out = settingOf(*pos)
		return nil
	})
	return out, err
}

// checkLeverageChange verifies the margin after a leverage change of an open position.
func (s *Service) checkLeverageChange(ctx context.Context, tx *gorm.DB, acct *Account, positions []Position,
	pos Position, sym symbols.Symbol) error {
	if pos.MarginType == MarginIsolated {
		mark, ok := s.prices.Price(sym.Name)
		if !ok {
			return errMarkUnavailable(sym.Name)
		}
		pr := evalPosition(pos, sym, mark)
		if pos.IsolatedMargin.Add(pr.UPnL).LessThan(pr.IM) {
			return apperr.Precondition("INSUFFICIENT_MARGIN", "isolated margin does not cover the new initial margin")
		}
		return nil
	}
	om, err := openOrderMargin(ctx, tx, acct.UserID)
	if err != nil {
		return err
	}
	r, err := computeRisk(acct.WalletBalance, positions, om, s.prices.Price, true)
	if err != nil {
		return err
	}
	if acct.WalletBalance.Add(r.CrossUPnL).Sub(r.CrossIM).Sub(om).IsNegative() {
		return apperr.Precondition("INSUFFICIENT_MARGIN", "insufficient margin for leverage %d", pos.Leverage)
	}
	return nil
}

// SetMarginType switches CROSSED/ISOLATED; only allowed without position and open orders.
func (s *Service) SetMarginType(ctx context.Context, userID, symbol, marginType string) (SymbolSetting, error) {
	if err := requireUser(userID); err != nil {
		return SymbolSetting{}, err
	}
	sym, err := lookupSymbol(symbol)
	if err != nil {
		return SymbolSetting{}, err
	}
	mt, err := normalizeMarginType(marginType)
	if err != nil {
		return SymbolSetting{}, err
	}
	var out SymbolSetting
	err = database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		_, _, pos, err := lockUserState(ctx, tx, userID, sym)
		if err != nil {
			return err
		}
		if pos.MarginType != mt {
			var open int64
			if err := tx.WithContext(ctx).Model(&Order{}).
				Where("user_id = ? AND symbol = ? AND status IN ?", userID, sym.Name, openStatuses).
				Count(&open).Error; err != nil {
				return err
			}
			if !pos.Size.IsZero() || open > 0 || pos.Liquidating {
				return apperr.Precondition("POSITION_OR_ORDERS_EXIST",
					"margin type cannot be changed with an open position or open orders")
			}
			pos.MarginType = mt
			pos.IsolatedMargin = decimal.Zero
			if err := savePosition(ctx, tx, pos); err != nil {
				return err
			}
		}
		out = settingOf(*pos)
		return nil
	})
	return out, err
}

func settingOf(p Position) SymbolSetting {
	return SymbolSetting{Symbol: p.Symbol, Leverage: p.Leverage, MarginType: p.MarginType, MaxNotional: maxValue}
}

// AdjustIsolatedMargin adds margin to / removes margin from an isolated position.
func (s *Service) AdjustIsolatedMargin(ctx context.Context, userID, symbol, amount, typ string) (PositionView, error) {
	if err := requireUser(userID); err != nil {
		return PositionView{}, err
	}
	sym, err := lookupSymbol(symbol)
	if err != nil {
		return PositionView{}, err
	}
	amt, err := parseMoney(amount)
	if err != nil {
		return PositionView{}, err
	}
	typ = strings.ToUpper(strings.TrimSpace(typ))
	if typ != "ADD" && typ != "REDUCE" {
		return PositionView{}, apperr.Invalid("INVALID_TYPE", "type must be ADD or REDUCE")
	}
	var out PositionView
	err = database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		acct, positions, pos, err := lockUserState(ctx, tx, userID, sym)
		if err != nil {
			return err
		}
		if pos.Size.IsZero() {
			return apperr.Precondition("NO_POSITION", "no open %s position", sym.Name)
		}
		if pos.MarginType != MarginIsolated {
			return apperr.Precondition("POSITION_NOT_ISOLATED", "position %s is not isolated", sym.Name)
		}
		om, err := openOrderMargin(ctx, tx, userID)
		if err != nil {
			return err
		}
		r, err := computeRisk(acct.WalletBalance, positions, om, s.prices.Price, true)
		if err != nil {
			return err
		}
		delta := amt
		if typ == "ADD" {
			if amt.GreaterThan(r.Available) {
				return apperr.Precondition("INSUFFICIENT_MARGIN", "amount exceeds available balance %s", r.Available)
			}
		} else {
			pr := r.Positions[sym.Name]
			if amt.GreaterThan(pos.IsolatedMargin) || pos.IsolatedMargin.Sub(amt).Add(pr.UPnL).LessThan(pr.IM) {
				return apperr.Precondition("INSUFFICIENT_MARGIN", "amount exceeds the removable isolated margin")
			}
			delta = amt.Neg()
		}
		acct.WalletBalance = acct.WalletBalance.Sub(delta)
		pos.IsolatedMargin = pos.IsolatedMargin.Add(delta)
		if err := saveAccount(ctx, tx, acct); err != nil {
			return err
		}
		if err := savePosition(ctx, tx, pos); err != nil {
			return err
		}
		// Income is signed from the wallet's point of view: ADD moves cash out of the wallet.
		if err := insertIncome(ctx, tx, []Income{{UserID: userID, Symbol: sym.Name, IncomeType: IncomeIsolatedMargin,
			Income: delta.Neg(), Asset: Asset, Info: typ, Time: s.now()}}); err != nil {
			return err
		}
		if err := s.emitAccountUpdate(ctx, tx, acct, positions, om, reasonMarginTransfer,
			func(p Position) bool { return p.Symbol == sym.Name }); err != nil {
			return err
		}
		v, err := s.buildView(acct.WalletBalance, positions, om, sym.Name)
		if err != nil {
			return err
		}
		for _, p := range v.Positions {
			if p.Symbol == sym.Name {
				out = p
			}
		}
		return nil
	})
	if err != nil {
		return PositionView{}, err
	}
	s.disp.Notify()
	return out, nil
}

// TransferToSpot debits the futures wallet (amount <= max withdraw) and emits
// transfer.futures_to_spot for the wallet service. Idempotent on (user, idempotency_key).
func (s *Service) TransferToSpot(ctx context.Context, userID, asset, amount, key string) (TransferResult, error) {
	if err := requireUser(userID); err != nil {
		return TransferResult{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(asset), Asset) {
		return TransferResult{}, apperr.Invalid("INVALID_ASSET", "only %s can be transferred", Asset)
	}
	amt, err := parseMoney(amount)
	if err != nil {
		return TransferResult{}, err
	}
	key = strings.TrimSpace(key)
	if len(key) > maxClientOrderIDLen {
		return TransferResult{}, apperr.Invalid("INVALID_IDEMPOTENCY_KEY", "idempotency_key longer than %d", maxClientOrderIDLen)
	}
	if key != "" {
		if t, err := getTransferByKey(ctx, s.db, userID, key); err == nil {
			return transferResult(t), nil
		} else if !errors.Is(err, errNotFound) {
			return TransferResult{}, err
		}
	}
	t := Transfer{TransferID: uuid.Must(uuid.NewV7()).String(), UserID: userID, Direction: directionFuturesToSpot,
		Asset: Asset, Amount: amt, IdempotencyKey: key, CreatedAt: s.now()}
	err = database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		acct, err := lockAccount(ctx, tx, userID)
		if errors.Is(err, errNotFound) {
			return apperr.Precondition("INSUFFICIENT_BALANCE", "futures account has no balance")
		}
		if err != nil {
			return err
		}
		positions, err := lockPositions(ctx, tx, []string{userID})
		if err != nil {
			return err
		}
		om, err := openOrderMargin(ctx, tx, userID)
		if err != nil {
			return err
		}
		r, err := computeRisk(acct.WalletBalance, positions, om, s.prices.Price, true)
		if err != nil {
			return err
		}
		if amt.GreaterThan(r.MaxWithdraw) {
			return apperr.Precondition("INSUFFICIENT_BALANCE", "amount exceeds max withdraw amount %s", r.MaxWithdraw)
		}
		acct.WalletBalance = acct.WalletBalance.Sub(amt)
		if err := saveAccount(ctx, tx, acct); err != nil {
			return err
		}
		if _, err := insertTransfer(ctx, tx, &t); err != nil {
			return err
		}
		if err := insertIncome(ctx, tx, []Income{{UserID: userID, IncomeType: IncomeTransfer, Income: amt.Neg(),
			Asset: Asset, Info: t.TransferID, Time: t.CreatedAt}}); err != nil {
			return err
		}
		env, err := events.New(ServiceName, events.TypeFuturesToSpot, t.TransferID, events.Transfer{
			TransferID: t.TransferID, UserID: userID, Asset: Asset, Amount: amt.String(), Time: t.CreatedAt,
		})
		if err != nil {
			return err
		}
		if err := outbox.Add(ctx, tx, events.TopicFuturesTransfers, userID, env); err != nil {
			return err
		}
		return s.emitAccountUpdate(ctx, tx, acct, positions, om, reasonWithdraw, nil)
	})
	if err != nil {
		if key != "" && database.IsUniqueViolation(err) {
			existing, gerr := getTransferByKey(ctx, s.db, userID, key)
			if gerr != nil {
				return TransferResult{}, fmt.Errorf("load duplicate transfer: %w", gerr)
			}
			return transferResult(existing), nil
		}
		return TransferResult{}, err
	}
	s.disp.Notify()
	return transferResult(t), nil
}

func transferResult(t Transfer) TransferResult {
	return TransferResult{TransferID: t.TransferID, Asset: t.Asset, Amount: t.Amount, Status: "PENDING"}
}
