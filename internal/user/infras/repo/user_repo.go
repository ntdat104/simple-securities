package repo

import (
	"context"
	"fmt"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"
	"strings"

	pkgRepo "simple-securities/pkg/db/repo"
	"simple-securities/pkg/pagination"

	"github.com/jmoiron/sqlx"
)

type UserRepo struct {
	*pkgRepo.BaseRepo[model.User]
	db           *sqlx.DB
	queryFields  string
	insertFields string
	updateFields string
	tableName    string
}

func NewUserRepo(db *sqlx.DB) repo.IUserRepo {
	var m model.User
	meta := model.GetMeta(m)
	return &UserRepo{
		BaseRepo:     pkgRepo.NewBaseRepo[model.User](db),
		db:           db,
		tableName:    m.TableName(),
		queryFields:  meta.QueryFields,
		insertFields: meta.InsertFields,
		updateFields: meta.UpdateFields,
	}
}

func (r *UserRepo) findAllCursor(ctx context.Context, query string, args ...any) ([]*model.User, string, error) {
	val, err := r.FindAll(ctx, query, args...)
	if err != nil {
		return []*model.User{}, "", err
	}
	var cursor string
	if len(val) > 0 {
		last := val[len(val)-1]
		cursor = pagination.NewCursor(last.ID, last.CreatedAt).Encode()
	}
	return val, cursor, nil
}

func (r *UserRepo) FindAllByCursor(ctx context.Context, cursor string, size int) ([]*model.User, string, error) {
	var query string

	if cursor == "" {
		query = fmt.Sprintf(`SELECT %s FROM %s ORDER BY created_at DESC, id DESC LIMIT $1`, r.queryFields, r.tableName)
		return r.findAllCursor(ctx, query, size)
	}

	c, err := pagination.DecodeCursor(cursor)
	if err != nil {
		return []*model.User{}, "", err
	}

	query = fmt.Sprintf(`SELECT %s FROM %s WHERE (created_at, id) < ($1, $2) ORDER BY created_at DESC, id DESC LIMIT $3`, r.queryFields, r.tableName)
	return r.findAllCursor(ctx, query, c.CreatedAt, c.Id, size)
}

func (r *UserRepo) FindAllByPageAndSize(ctx context.Context, page int, size int) ([]*model.User, error) {
	query := fmt.Sprintf("SELECT %s FROM %s LIMIT ? OFFSET ?", r.queryFields, r.tableName)
	return r.FindAll(ctx, query, size, page)
}

func (r *UserRepo) CountTotal(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", r.tableName)
	return r.Count(ctx, query)
}

func (r *UserRepo) FindByIdAndEmailAndUuid(ctx context.Context, id uint64, email string, uuid string) (*model.User, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1 AND email = $2 AND uuid = $3 LIMIT 1", r.queryFields, r.tableName)
	return r.FindOne(ctx, query, id, email, uuid)
}

func (r *UserRepo) FindById(ctx context.Context, id uint64) (*model.User, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1 LIMIT 1", r.queryFields, r.tableName)
	return r.FindOne(ctx, query, id)
}

func (r *UserRepo) FindByUuid(ctx context.Context, uuid string) (*model.User, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE uuid = $1 LIMIT 1", r.queryFields, r.tableName)
	return r.FindOne(ctx, query, uuid)
}

func (r *UserRepo) FindByUuidIn(ctx context.Context, uuids []string) ([]*model.User, error) {
	return nil, nil
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE email = $1 LIMIT 1", r.queryFields, r.tableName)
	return r.FindOne(ctx, query, email)
}

func (r *UserRepo) FindByIdIn(ctx context.Context, ids []uint64) ([]*model.User, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id IN (?)", r.queryFields, r.tableName)
	return r.FindMany(ctx, query, ids)
}

// Save thực hiện INSERT hoặc UPDATE một User.
// Nếu bạn muốn hỗ trợ Transaction, có thể truyền tx vào, nếu không hãy truyền nil.
func (r *UserRepo) Save(ctx context.Context, tx *sqlx.Tx, user *model.User) (*model.User, error) {
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

// SaveAll thực hiện lưu một danh sách User
func (r *UserRepo) SaveAll(ctx context.Context, tx *sqlx.Tx, users []*model.User) ([]*model.User, error) {
	if len(users) == 0 {
		return []*model.User{}, nil
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
