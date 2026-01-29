package main

import (
	"context"
	"log"
	"time"

	// Import package Go được tạo ra từ user.proto
	userpb "simple-securities/gen/user/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status" // Dùng để xử lý lỗi gRPC tốt hơn
)

const (
	address = "localhost:50054" // Địa chỉ của gRPC Server
	// Dữ liệu dùng thử
	testEmail    = "testuser@example.com"
	testPassword = "password123"
)

func main() {
	// 1. Kết nối tới gRPC Server
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("❌ Lỗi: Không thể kết nối tới server %s: %v", address, err)
	}
	defer conn.Close()

	// 2. Tạo Client stub
	client := userpb.NewUserServiceClient(conn)

	// Biến để lưu trữ token từ Login cho cuộc gọi GetUserProfile
	var accessToken string

	// 3. Gọi các RPC methods

	// --- Ví dụ 1: Register ---
	// Gọi Register và cố gắng lấy token
	if token, ok := callRegister(client); ok {
		accessToken = token
	}

	// --- Ví dụ 2: Login ---
	// Gọi Login và cố gắng lấy token
	if token, ok := callLogin(client); ok {
		accessToken = token
	}

	// --- Ví dụ 3: GetUserProfile (Chỉ gọi nếu có token) ---
	if accessToken != "" {
		callGetUserProfile(client, accessToken)
	} else {
		log.Println("⚠️ Bỏ qua GetUserProfile vì không có Access Token hợp lệ.")
	}
}

// Hàm gọi RPC Register
func callRegister(client userpb.UserServiceClient) (string, bool) {
	log.Println("--- Đang gọi RPC: Register ---")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Tăng timeout
	defer cancel()

	req := &userpb.RegisterRequest{
		Email:    testEmail,
		Password: testPassword,
	}

	res, err := client.Register(ctx, req)
	if err != nil {
		// Log lỗi chi tiết hơn nếu là lỗi gRPC Status
		if st, ok := status.FromError(err); ok && st.Code() == 7 { // Code 7 là AlreadyExists
			log.Printf("⚠️ Lỗi gọi Register: Tài khoản đã tồn tại. Thử tiếp tục với Login: %v", st.Message())
			return "", false
		}
		log.Fatalf("❌ Lỗi nghiêm trọng khi gọi Register: %v", err)
	}

	// In thông tin phản hồi dựa trên cấu trúc mới (RegisterResponse có UserDto, access_token)
	log.Printf("✅ Phản hồi Register thành công:")
	log.Printf("   User ID: %d", res.GetUser().GetId())
	log.Printf("   User UUID: %s", res.GetUser().GetUuid())
	log.Printf("   Email: %s", res.GetUser().GetEmail())
	log.Printf("   Access Token: %s...", res.GetAccessToken()[:20]) // Cắt ngắn token
	log.Printf("   Loại Token: %s (Hết hạn sau %d giây)", res.GetTokenType(), res.GetExp())
	log.Println("----------------------------------")
	return res.GetAccessToken(), true
}

// Hàm gọi RPC Login
func callLogin(client userpb.UserServiceClient) (string, bool) {
	log.Println("--- Đang gọi RPC: Login ---")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Tăng timeout
	defer cancel()

	req := &userpb.LoginRequest{
		Email:    testEmail,
		Password: testPassword,
	}

	res, err := client.Login(ctx, req)
	if err != nil {
		log.Fatalf("❌ Lỗi khi gọi Login: %v", err)
	}

	// In thông tin phản hồi dựa trên cấu trúc mới (LoginResponse có UserDto, access_token)
	log.Printf("✅ Phản hồi Login thành công:")
	log.Printf("   User ID: %d", res.GetUser().GetId())
	log.Printf("   User UUID: %s", res.GetUser().GetUuid())
	log.Printf("   Email: %s", res.GetUser().GetEmail())
	log.Printf("   Access Token: %s...", res.GetAccessToken()[:20]) // Cắt ngắn token
	log.Printf("   Loại Token: %s (Hết hạn sau %d giây)", res.GetTokenType(), res.GetExp())
	log.Println("----------------------------------")
	return res.GetAccessToken(), true
}

// Hàm gọi RPC GetUserProfile
// Bây giờ nhận vào accessToken để gửi qua metadata
func callGetUserProfile(client userpb.UserServiceClient, accessToken string) {
	log.Println("--- Đang gọi RPC: GetUserProfile (Có Auth Token) ---")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// ❗ Bổ sung: Thêm Authorization Token vào context (metadata)
	authHeader := "Bearer " + accessToken
	md := metadata.Pairs("authorization", authHeader)
	ctx = metadata.NewOutgoingContext(ctx, md)

	req := &userpb.GetUserProfileRequest{}

	res, err := client.GetUserProfile(ctx, req)
	if err != nil {
		log.Fatalf("❌ Lỗi khi gọi GetUserProfile: %v", err)
	}

	// In thông tin phản hồi dựa trên cấu trúc mới (GetUserProfileResponse có UserDto, status, kyc_status)
	log.Printf("✅ Phản hồi GetUserProfile thành công:")
	log.Printf("   User ID: %d", res.GetUser().GetId())
	log.Printf("   Email: %s", res.GetUser().GetEmail())
	log.Printf("   Trạng thái tài khoản: %v", res.GetUser().GetStatus()) // UserStatus
	log.Println("----------------------------------")
}
