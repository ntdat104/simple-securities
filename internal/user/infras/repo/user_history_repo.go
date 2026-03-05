package repo

import (
	"context"
	"fmt"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"
	"strings"

	pkgRepo "simple-securities/pkg/db/repo"

	"github.com/jmoiron/sqlx"
)

type UserHistoryRepo struct {
	*pkgRepo.BaseRepo[model.UserHistory]
	db           *sqlx.DB
	queryFields  string
	insertFields string
	updateFields string
	tableName    string
}

func NewUserHistoryRepo(db *sqlx.DB) repo.IUserHistoryRepo {
	var m model.UserHistory
	meta := model.GetMeta(m)
	return &UserHistoryRepo{
		BaseRepo:     pkgRepo.NewBaseRepo[model.UserHistory](db),
		db:           db,
		tableName:    m.TableName(),
		queryFields:  meta.QueryFields,
		insertFields: meta.InsertFields,
		updateFields: meta.UpdateFields,
	}
}

func (r *UserHistoryRepo) FindAllByPageAndSize(ctx context.Context, page int, size int) ([]*model.UserHistory, error) {
	query := fmt.Sprintf("SELECT %s FROM %s LIMIT ? OFFSET ?", r.queryFields, r.tableName)
	return r.FindAll(ctx, query, size, page)
}

func (r *UserHistoryRepo) CountTotal(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", r.tableName)
	return r.Count(ctx, query)
}

func (r *UserHistoryRepo) FindByIdAndEmailAndUuid(ctx context.Context, id uint64, email string, uuid string) (*model.UserHistory, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1 AND email = $2 AND uuid = $3 LIMIT 1", r.queryFields, r.tableName)
	return r.FindOne(ctx, query, id, email, uuid)
}

func (r *UserHistoryRepo) FindById(ctx context.Context, id uint64) (*model.UserHistory, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1 LIMIT 1", r.queryFields, r.tableName)
	return r.FindOne(ctx, query, id)
}

func (r *UserHistoryRepo) FindByUuid(ctx context.Context, uuid string) (*model.UserHistory, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE uuid = $1 LIMIT 1", r.queryFields, r.tableName)
	return r.FindOne(ctx, query, uuid)
}

func (r *UserHistoryRepo) FindByUuidIn(ctx context.Context, uuids []string) ([]*model.UserHistory, error) {
	return nil, nil
}

func (r *UserHistoryRepo) FindByEmail(ctx context.Context, email string) (*model.UserHistory, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE email = $1 LIMIT 1", r.queryFields, r.tableName)
	return r.FindOne(ctx, query, email)
}

func (r *UserHistoryRepo) FindByIdIn(ctx context.Context, ids []uint64) ([]*model.UserHistory, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id IN (?)", r.queryFields, r.tableName)
	return r.FindMany(ctx, query, ids)
}

// Save thực hiện INSERT hoặc UPDATE một UserHistory.
// Nếu bạn muốn hỗ trợ Transaction, có thể truyền tx vào, nếu không hãy truyền nil.
func (r *UserHistoryRepo) Save(ctx context.Context, tx *sqlx.Tx, user *model.UserHistory) (*model.UserHistory, error) {
	var query string

	if user.ID != 0 {
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

	id, err := r.SaveWithTx(ctx, tx, query, user)
	if err != nil {
		return nil, err
	}
	user.ID = uint64(id)

	// Use SaveWithTx to execute the chosen query
	return user, nil
}

// SaveAll thực hiện lưu một danh sách UserHistory
func (r *UserHistoryRepo) SaveAll(ctx context.Context, tx *sqlx.Tx, users []*model.UserHistory) ([]*model.UserHistory, error) {
	if len(users) == 0 {
		return []*model.UserHistory{}, nil
	}

	values := ":" + strings.ReplaceAll(r.insertFields, ", ", ", :")

	query := fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s)`,
		r.tableName,
		r.insertFields,
		values,
	)

	_, err := r.SaveAllWithTx(ctx, tx, query, users)
	if err != nil {
		return nil, err
	}

	// Gọi hàm SaveAllWithTx từ BaseRepo
	return users, nil
}
