package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"simple-securities/internal/credit/domain/model"
	"simple-securities/internal/credit/domain/repo"
	pkgRepo "simple-securities/pkg/db/repo"
)

type CreditTransactionRepo struct {
	*pkgRepo.BaseRepo[model.CreditTransaction]
	db           *sqlx.DB
	queryFields  string
	insertFields string
	updateFields string
	tableName    string
}

func NewCreditTransactionRepo(db *sqlx.DB) repo.ICreditTransactionRepo {
	var m model.CreditTransaction
	meta := model.GetMeta(m)
	return &CreditTransactionRepo{
		BaseRepo:     pkgRepo.NewBaseRepo[model.CreditTransaction](db),
		db:           db,
		tableName:    m.TableName(),
		queryFields:  meta.QueryFields,
		insertFields: meta.InsertFields,
		updateFields: meta.UpdateFields,
	}
}

func (r *CreditTransactionRepo) Save(ctx context.Context, tx *sqlx.Tx, txn *model.CreditTransaction) (*model.CreditTransaction, error) {
	var query string

	if txn.ID != 0 {
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

	id, err := r.SaveWithTx(ctx, tx, query, txn)
	if err != nil {
		return nil, err
	}
	txn.ID = uint64(id)

	// Use SaveWithTx to execute the chosen query
	return txn, nil
}

func (r *CreditTransactionRepo) SaveAll(ctx context.Context, tx *sqlx.Tx, txns []*model.CreditTransaction) ([]*model.CreditTransaction, error) {
	if len(txns) == 0 {
		return []*model.CreditTransaction{}, nil
	}

	values := ":" + strings.ReplaceAll(r.insertFields, ", ", ", :")

	query := fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s)`,
		r.tableName,
		r.insertFields,
		values,
	)

	_, err := r.SaveAllWithTx(ctx, tx, query, txns)
	if err != nil {
		return nil, err
	}

	// Gọi hàm SaveAllWithTx từ BaseRepo
	return txns, nil
}
