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

type CreditReservationItemRepo struct {
	*pkgRepo.BaseRepo[model.CreditReservationItem]
	db           *sqlx.DB
	queryFields  string
	insertFields string
	updateFields string
	tableName    string
}

func NewCreditReservationItemRepo(db *sqlx.DB) repo.ICreditReservationItemRepo {
	var m model.CreditReservationItem
	meta := model.GetMeta(m)
	return &CreditReservationItemRepo{
		BaseRepo:     pkgRepo.NewBaseRepo[model.CreditReservationItem](db),
		db:           db,
		tableName:    m.TableName(),
		queryFields:  meta.QueryFields,
		insertFields: meta.InsertFields,
		updateFields: meta.UpdateFields,
	}
}

func (r *CreditReservationItemRepo) FindItemsByReservationID(ctx context.Context, reservationID uint64) ([]*model.CreditReservationItem, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE reservation_id = $1", r.queryFields, r.tableName)
	return r.FindMany(ctx, query, reservationID)
}

func (r *CreditReservationItemRepo) SaveAll(ctx context.Context, tx *sqlx.Tx, items []*model.CreditReservationItem) ([]*model.CreditReservationItem, error) {
	if len(items) == 0 {
		return []*model.CreditReservationItem{}, nil
	}

	values := ":" + strings.ReplaceAll(r.insertFields, ", ", ", :")

	query := fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s)`,
		r.tableName,
		r.insertFields,
		values,
	)

	_, err := r.SaveAllWithTx(ctx, tx, query, items)
	if err != nil {
		return nil, err
	}
	return items, nil
}
