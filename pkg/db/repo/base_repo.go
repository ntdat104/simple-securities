package repo

import (
	"context"
	"database/sql"
	"fmt"
	"simple-securities/common/constants"
	"simple-securities/pkg/logger"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

type BaseRepo[T any] struct {
	db *sqlx.DB
}

func NewBaseRepo[T any](db *sqlx.DB) *BaseRepo[T] {
	return &BaseRepo[T]{db: db}
}

// FindAll thực thi truy vấn và scan kết quả vào một slice của model T.
// Câu lệnh query và các tham số args (bao gồm phân trang nếu có) đã được build sẵn từ bên ngoài.
func (r *BaseRepo[T]) FindAll(ctx context.Context, query string, args ...any) ([]*T, error) {
	start := time.Now()

	// Khởi tạo slice rỗng để tránh trả về nil (trả về [] thay vì nil tốt hơn cho JSON API)
	models := make([]*T, 0)

	err := r.db.SelectContext(ctx, &models, query, args...)
	elapsed := time.Since(start)

	if err != nil {
		// Log lỗi đồng nhất với FindOne
		logger.Info(ctx, "🛑 [FindAll] Executing query has error",
			zap.String(constants.Query, query),
			zap.Any(constants.Arg, args),
			zap.Any(constants.Error, err),
			zap.Any(constants.Duration, elapsed))

		var model T
		return nil, fmt.Errorf("findAll failed for model %T: %w", model, err)
	}

	// Log thành công
	logger.Info(ctx, "✅ [FindAll] Executing query has success",
		zap.String(constants.Query, query),
		zap.Any(constants.Arg, args),
		zap.Int(constants.Count, len(models)), // Log thêm số lượng row tìm thấy
		zap.Any(constants.Duration, elapsed))

	return models, nil
}

// FindOne is now generic. It accepts a specific instance of M to scan into.
func (r *BaseRepo[T]) FindOne(ctx context.Context, query string, args ...any) (*T, error) {
	start := time.Now()

	var model T

	err := r.db.GetContext(ctx, &model, query, args...)
	elapsed := time.Since(start)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		logger.Info(ctx, "🛑 [FindOne] Executing query has error",
			zap.String(constants.Query, query),
			zap.Any(constants.Arg, args),
			zap.Any(constants.Error, err),
			zap.Any(constants.Duration, elapsed))
		return nil, fmt.Errorf("findOne failed for model %T: %w", model, err)
	}

	logger.Info(ctx, "✅ [FindOne] Executing query has success",
		zap.String(constants.Query, query),
		zap.Any(constants.Arg, args),
		zap.Any(constants.Duration, elapsed))
	return &model, nil
}

// FindMany is now generic, returning a slice of the model type M.
func (r *BaseRepo[T]) FindMany(ctx context.Context, query string, value ...any) ([]*T, error) {
	start := time.Now()

	if len(value) == 0 {
		return []*T{}, nil
	}

	// Xử lý IN clause
	queryIn, args, err := sqlx.In(query, value...)
	if err != nil {
		return nil, fmt.Errorf("FindMany failed: generating IN clause: %w", err)
	}

	queryIn = r.db.Rebind(queryIn)

	// Thực thi query
	models := make([]*T, 0)
	err = r.db.SelectContext(ctx, &models, queryIn, args...)
	elapsed := time.Since(start)

	if err != nil {
		// Log lỗi tương tự FindOne
		logger.Info(ctx, "🛑 [FindMany] Executing query has error",
			zap.String(constants.Query, queryIn),
			zap.Any(constants.Arg, args),
			zap.Any(constants.Error, err),
			zap.Any(constants.Duration, elapsed))

		var model T // Để lấy type name cho message lỗi
		return nil, fmt.Errorf("findMany failed for model %T: %w", model, err)
	}

	// Log thành công
	logger.Info(ctx, "✅ [FindMany] Executing query has success",
		zap.String(constants.Query, queryIn),
		zap.Any(constants.Arg, args),
		zap.Int(constants.Count, len(models)), // Thêm số lượng record lấy được để tiện debug
		zap.Any(constants.Duration, elapsed))

	return models, nil
}

// Count thực thi truy vấn để đếm số lượng bản ghi (thường dùng với SELECT COUNT(*))
func (r *BaseRepo[T]) Count(ctx context.Context, query string, args ...any) (int64, error) {
	start := time.Now()

	var total int64
	err := r.db.GetContext(ctx, &total, query, args...)
	elapsed := time.Since(start)

	if err != nil {
		// Log lỗi tương tự FindOne
		logger.Info(ctx, "🛑 [Count] Executing query has error",
			zap.String(constants.Query, query),
			zap.Any(constants.Arg, args),
			zap.Any(constants.Error, err),
			zap.Any(constants.Duration, elapsed))

		var model T // Để xác định context của repo đang gọi
		return 0, fmt.Errorf("count failed for model %T: %w", model, err)
	}

	// Log thành công
	logger.Info(ctx, "✅ [Count] Executing query has success",
		zap.String(constants.Query, query),
		zap.Any(constants.Arg, args),
		zap.Int64("total", total),
		zap.Any(constants.Duration, elapsed))

	return total, nil
}

// Save thực hiện lưu instance của T vào database.
// Nếu tx khác nil, hàm sẽ chạy trong transaction đó.
func (r *BaseRepo[T]) SaveWithTx(ctx context.Context, tx *sqlx.Tx, query string, model *T) (uint64, error) {
	start := time.Now()

	// 1. Xác định executor (Transaction hoặc DB)
	var err error
	var result sql.Result

	if tx != nil {
		result, err = tx.NamedExecContext(ctx, query, model)
	} else {
		result, err = r.db.NamedExecContext(ctx, query, model)
	}

	elapsed := time.Since(start)

	// 2. Logging & Error Handling
	if err != nil {
		logger.Info(ctx, "🛑 [Save] Executing query has error",
			zap.String(constants.Query, query),
			zap.Any(constants.Error, err),
			zap.Any(constants.Duration, elapsed))

		return 0, fmt.Errorf("save failed for model %T: %w", model, err)
	}

	lastID, _ := result.LastInsertId()
	rowsAffected, _ := result.RowsAffected()

	// 3. Log thành công
	logger.Info(ctx, "✅ [Save] Executing query has success",
		zap.String(constants.Query, query),
		zap.Int64(constants.RowsAffected, rowsAffected),
		zap.Int64(constants.LastId, lastID),
		zap.Any(constants.Duration, elapsed))

	return uint64(lastID), nil
}

// SaveAll thực hiện lưu một danh sách các model T.
// Nếu tx khác nil, hàm chạy trong transaction đó.
func (r *BaseRepo[T]) SaveAllWithTx(ctx context.Context, tx *sqlx.Tx, query string, models []*T) ([]int64, error) {
	if len(models) == 0 {
		return []int64{}, nil
	}

	start := time.Now()
	var err error
	var result sql.Result

	// 1. Thực thi (Hỗ trợ cả Transaction hoặc DB trực tiếp)
	if tx != nil {
		result, err = tx.NamedExecContext(ctx, query, models)
	} else {
		result, err = r.db.NamedExecContext(ctx, query, models)
	}

	elapsed := time.Since(start)

	// 2. Logging tương tự FindMany
	if err != nil {
		logger.Info(ctx, "🛑 [SaveAll] Executing query has error",
			zap.String(constants.Query, query),
			zap.Int("count", len(models)),
			zap.Any(constants.Error, err),
			zap.Any(constants.Duration, elapsed))

		var model T
		return nil, fmt.Errorf("saveAll failed for model %T: %w", model, err)
	}

	// Lấy ID đầu tiên trong batch (MySQL)
	firstID, _ := result.LastInsertId()
	rowsAffected, _ := result.RowsAffected()

	// Tạo danh sách ID dự đoán (Chỉ chính xác nếu ID tự tăng liên tục)
	ids := make([]int64, 0, rowsAffected)
	for i := int64(0); i < rowsAffected; i++ {
		ids = append(ids, firstID+i)
	}

	// 3. Log thành công
	logger.Info(ctx, "✅ [SaveAll] Executing query has success",
		zap.String(constants.Query, query),
		zap.Int64("rows_affected", rowsAffected),
		zap.Int("total_input", len(models)),
		zap.Any(constants.Duration, elapsed))

	return ids, nil
}

// func (s *UserService) UpdateUserAndLogHistory(ctx context.Context, user *model.User, history *model.UserHistory) error {
//     // 1. Dùng Atomic để khởi tạo Transaction
//     return s.userRepo.Atomic(ctx, func(tx *sqlx.Tx) error {

//         // 2. Lưu User (truyền tx vào)
//         userQuery := `UPDATE users SET email = :email WHERE id = :id`
//         _, err := s.userRepo.SaveWithTx(ctx, tx, user, "", userQuery)
//         if err != nil {
//             return err // Rollback tự động
//         }

//         // 3. Lưu History (truyền cùng tx vào)
//         historyQuery := `INSERT INTO user_histories (user_id, action) VALUES (:user_id, :action)`
//         _, err = s.historyRepo.SaveWithTx(ctx, tx, history, historyQuery, "")
//         if err != nil {
//             return err // Rollback cả User ở bước 2
//         }

//         return nil // Commit tự động
//     })
// }
