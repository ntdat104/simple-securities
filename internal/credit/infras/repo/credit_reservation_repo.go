package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"simple-securities/internal/credit/domain/model"
	"simple-securities/internal/credit/domain/repo"
	pkgRepo "simple-securities/pkg/db/repo"
)

type CreditReservationRepo struct {
	*pkgRepo.BaseRepo[model.CreditReservation]
	db           *sqlx.DB
	queryFields  string
	insertFields string
	updateFields string
	tableName    string
}

func NewCreditReservationRepo(db *sqlx.DB) repo.ICreditReservationRepo {
	var m model.CreditReservation
	meta := model.GetMeta(m)
	return &CreditReservationRepo{
		BaseRepo:     pkgRepo.NewBaseRepo[model.CreditReservation](db),
		db:           db,
		tableName:    m.TableName(),
		queryFields:  meta.QueryFields,
		insertFields: meta.InsertFields,
		updateFields: meta.UpdateFields,
	}
}

func (r *CreditReservationRepo) FindByUuid(ctx context.Context, uuid string) (*model.CreditReservation, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE uuid = $1 LIMIT 1", r.queryFields, r.tableName)
	return r.FindOne(ctx, query, uuid)
}

func (r *CreditReservationRepo) FindByIdempotencyKey(ctx context.Context, key string) (*model.CreditReservation, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE idempotency_key = $1 LIMIT 1", r.queryFields, r.tableName)
	return r.FindOne(ctx, query, key)
}

func (r *CreditReservationRepo) Save(ctx context.Context, tx *sqlx.Tx, res *model.CreditReservation) (*model.CreditReservation, error) {
	var query string

	if res.ID != 0 {
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

	id, err := r.SaveWithTx(ctx, tx, query, res)
	if err != nil {
		return nil, err
	}
	res.ID = uint64(id)

	// Use SaveWithTx to execute the chosen query
	return res, nil
}

func (r *CreditReservationRepo) FindExpiredPending(ctx context.Context, limit int) ([]*model.CreditReservation, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE status = 'PENDING' AND expires_at < $1 LIMIT $2", r.queryFields, r.tableName)
	return r.FindMany(ctx, query, time.Now(), limit)
}
