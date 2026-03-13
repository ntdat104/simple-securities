# Hybrid Cache Documentation

Tài liệu này mô tả chi tiết về cấu trúc, ý tưởng triển khai và hiệu quả của hệ thống **Hybrid Cache** mã nguồn mở được tích hợp trong project `simple-securities`.

## 1. Cấu Trúc Cache (Architecture)

Hybrid Cache được thiết kế theo mô hình **Multi-level Caching** (Cache đa tầng) để tối ưu hóa giữa tốc độ truy xuất và khả năng chia sẻ dữ liệu:

- **L1 (Local Memory Cache):** Sử dụng `ristretto`. Đây là tầng cache cực nhanh nằm trực tiếp trên RAM của application process. Phù hợp cho "Hot Data" được truy cập liên tục.
- **L2 (Distributed Cache):** Sử dụng `Redis`. Đảm bảo tính nhất quán dữ liệu giữa nhiều instance của service (Distributed system) và có dung lượng lưu trữ lớn hơn.

## 2. Ý Tưởng Triển Khai (Implementation Details)

### A. Quy trình truy vấn (Get Logic)
Khi gọi hàm `Get`, hệ thống thực hiện tuần tự:
1. **Build Key:** Kết hợp `KeyPrefix`, `Key` và **Version** hiện tại từ Redis để tạo ra một key hoàn chỉnh (ví dụ: `user:123:v1`).
2. **Check L1:** Nếu có trong RAM (Memory Hit), trả về ngay lập tức (~ vài μs).
3. **Check L2:** Nếu L1 miss, kiểm tra Redis. Nếu tìm thấy (Redis Hit), ghi ngược vào L1 và trả về (~ vài ms).
4. **Singleflight Protection:** Nếu cả hai đều miss, `singleflight` sẽ đảm bảo chỉ có duy nhất 1 request đi vào hàm `fetcher()` (Database). Các request đồng thời khác cho cùng một key sẽ đứng đợi kết quả.
5. **Backfill:** Sau khi lấy dữ liệu từ DB, hệ thống tự động cập nhật vào cả Redis và Memory Cache.

### B. Cơ chế Versioning (Invalidation)
Thay vì xóa cache theo kiểu truyền thống (dễ gây lỗi hoặc tốn tài nguyên khi Scan), hệ thống sử dụng **Versioning**:
- Mỗi nhóm dữ liệu (Prefix) có một số phiên bản (Version) lưu trên Redis.
- `DeleteRegistry()`: Tăng số version (ví dụ v1 -> v2). Ngay lập tức, tất cả Key cũ (v1) sẽ bị "vô hiệu hóa" vì hệ thống chỉ tìm kiếm theo Key v2.
- Cơ chế này cực kỳ mạnh mẽ khi cần clear cache theo nhóm lớn mà không gây áp lực lên Performance.

## 3. Hiệu Quả (Performance & Benefits)

- **Latency:** Giảm tải tối đa cho mạng bằng cách ưu tiên RAM nội bộ.
- **Scalability:** Chống hiện tượng **Cache Stampede** (nhiều request cùng làm sập DB khi cache hết hạn) nhờ `singleflight`.
- **Consistency:** Đảm bảo dữ liệu đồng bộ giữa các node thông qua Redis.
- **Safety:** Tránh hiện tượng **Cache Avalanche** bằng cách thêm độ lệch ngẫu nhiên (jitter) vào TTL của Redis.

## 4. Hướng dẫn sử dụng

Dựa trên cách triển khai thực tế trong `GetUserProfileSvc`:

### Bước 1: Khai báo Registry
Định nghĩa các hằng số cache trong file `constant/cache.go`:

```go
// Tạo function registry với prefix "user:v1:profile"
var UserProfile = registry.NewDefaultCache("user:v1:profile")
```

### Bước 2: Sử dụng trong Service Layer
Triển khai logic lấy dữ liệu tự động fallback từ cache:

```go
func (s *getUserProfileSvc) Execute(ctx context.Context, userId int64) (*dto.UserDto, error) {
	// 1. Khởi tạo key cụ thể cho user (ví dụ: user:v1:profile:789)
	cacheRegistry := constant.UserProfile(fmt.Sprintf("%d", userId))

	// 2. Truy vấn qua Hybrid Cache
	userExist, err := cache.Get(s.hybridCache, ctx, cacheRegistry, func() (*model.User, error) {
		// Hàm này chỉ chạy khi L1 và L2 đều không có dữ liệu
		logger.Info(ctx, "Cache miss, fetching from Database", zap.Int64("userId", userId))
		return s.userRepo.FindById(ctx, userId)
	})

	if err != nil {
		return nil, err
	}

	return mapper.ToUserDto(userExist), nil
}
```

### Bước 3: Invalidation (Khi cập nhật dữ liệu)
- **Xóa 1 item cụ thể:** `cache.Delete(s.hybridCache, ctx, cacheRegistry)`
- **Vô hiệu hóa toàn bộ cấu trúc:** `cache.DeleteRegistry(s.hybridCache, ctx, constant.UserProfile())`
