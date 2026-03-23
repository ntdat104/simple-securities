package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"simple-securities/internal/credit/domain/enum"
	"simple-securities/internal/credit/domain/model"
	"simple-securities/internal/credit/domain/repo"
	pkgRepo "simple-securities/pkg/db/repo"
)

type CreditWalletRepo struct {
	*pkgRepo.BaseRepo[model.CreditWallet]
	db           *sqlx.DB
	queryFields  string
	insertFields string
	updateFields string
	tableName    string
}

func NewCreditWalletRepo(db *sqlx.DB) repo.ICreditWalletRepo {
	var m model.CreditWallet
	meta := model.GetMeta(m)
	return &CreditWalletRepo{
		BaseRepo:     pkgRepo.NewBaseRepo[model.CreditWallet](db),
		db:           db,
		tableName:    m.TableName(),
		queryFields:  meta.QueryFields,
		insertFields: meta.InsertFields,
		updateFields: meta.UpdateFields,
	}
}

func (r *CreditWalletRepo) FindByUserID(ctx context.Context, userID uint64) ([]*model.CreditWallet, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE user_id = $1 AND (expire_at IS NULL OR expire_at > $2) ORDER BY CASE WHEN expire_at IS NULL THEN 1 ELSE 0 END ASC, expire_at ASC", r.queryFields, r.tableName)
	return r.FindMany(ctx, query, userID, time.Now())
}

func (r *CreditWalletRepo) FindByUserIDAndType(ctx context.Context, userID uint64, walletType enum.WalletType) (*model.CreditWallet, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE user_id = $1 AND wallet_type = $2 LIMIT 1", r.queryFields, r.tableName)
	return r.FindOne(ctx, query, userID, walletType)
}

func (r *CreditWalletRepo) FindByID(ctx context.Context, id uint64) (*model.CreditWallet, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1 LIMIT 1", r.queryFields, r.tableName)
	return r.FindOne(ctx, query, id)
}

func (r *CreditWalletRepo) Save(ctx context.Context, tx *sqlx.Tx, wallet *model.CreditWallet) (*model.CreditWallet, error) {
	var query string

	if wallet.ID != 0 {
		query = fmt.Sprintf(
			`UPDATE %s SET %s WHERE id = :id`,
			r.tableName,
			r.updateFields,
		)
	} else {
		query = fmt.Sprintf(
			`INSERT INTO %s (%s) VALUES (:%s) RETURNING id`,
			r.tableName,
			r.insertFields,
			strings.ReplaceAll(r.insertFields, ", ", ", :"),
		)
	}

	id, err := r.SaveWithTx(ctx, tx, query, wallet)
	if err != nil {
		return nil, err
	}
	wallet.ID = uint64(id)

	// Use SaveWithTx to execute the chosen query
	return wallet, nil
}

func (r *CreditWalletRepo) SaveAll(ctx context.Context, tx *sqlx.Tx, wallets []*model.CreditWallet) ([]*model.CreditWallet, error) {
	if len(wallets) == 0 {
		return []*model.CreditWallet{}, nil
	}

	values := ":" + strings.ReplaceAll(r.insertFields, ", ", ", :")

	query := fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s)`,
		r.tableName,
		r.insertFields,
		values,
	)

	_, err := r.SaveAllWithTx(ctx, tx, query, wallets)
	if err != nil {
		return nil, err
	}

	// Gọi hàm SaveAllWithTx từ BaseRepo
	return wallets, nil
}
