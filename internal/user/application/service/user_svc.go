package service

import (
	"context"
	"errors"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"
	"strings"
	"time"

	common "simple-securities/gen/common/v1"
	userpb "simple-securities/gen/user/v1"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	JwtSecret = "secret-key"
	TokenType = "Bearer"
)

type UserSvc interface {
	Register(ctx context.Context, req *userpb.RegisterRequest) (*userpb.RegisterResponse, error)
	Login(ctx context.Context, req *userpb.LoginRequest) (*userpb.LoginResponse, error)
	GetUserProfile(ctx context.Context, req *userpb.GetUserProfileRequest) (*userpb.GetUserProfileResponse, error)
}

type userSvc struct {
	userRepo repo.IUserRepo
}

func NewUserSvc(userRepo repo.IUserRepo) UserSvc {
	return &userSvc{
		userRepo: userRepo,
	}
}

func (s *userSvc) Register(ctx context.Context, req *userpb.RegisterRequest) (*userpb.RegisterResponse, error) {
	user, _ := s.userRepo.FindByEmail(ctx, req.Email)
	if user != nil {
		return nil, status.Error(codes.AlreadyExists, "Email is already registered.")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to hash password: %v", err)
	}

	newUser := model.NewUser(req.Email, string(hashedPassword))
	user, err = s.userRepo.Save(ctx, newUser)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Internal system error during user save: %v", err)
	}

	accessToken, exp, err := generateAccessToken(user)
	if err != nil {
		return nil, err // Propagate gRPC status error from helper
	}

	return &userpb.RegisterResponse{
		User:        userToDto(user),
		AccessToken: accessToken,
		TokenType:   TokenType,
		Exp:         exp,
	}, nil
}

func (s *userSvc) Login(ctx context.Context, req *userpb.LoginRequest) (*userpb.LoginResponse, error) {
	user, _ := s.userRepo.FindByEmail(ctx, req.Email)
	if user == nil {
		return nil, status.Error(codes.Unauthenticated, "Invalid email or password.")
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password))
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "Invalid email or password.")
	}

	accessToken, exp, err := generateAccessToken(user)
	if err != nil {
		return nil, err // Propagate gRPC status error from helper
	}

	return &userpb.LoginResponse{
		User:        userToDto(user),
		AccessToken: accessToken,
		TokenType:   TokenType,
		Exp:         exp,
	}, nil
}

func (s *userSvc) GetUserProfile(ctx context.Context, req *userpb.GetUserProfileRequest) (*userpb.GetUserProfileResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "Authorization metadata not found.")
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return nil, status.Error(codes.Unauthenticated, "Authorization header missing.")
	}

	authHeader := authHeaders[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, status.Error(codes.Unauthenticated, "Invalid authorization scheme. Expected 'Bearer <token>'.")
	}

	accessToken := strings.TrimPrefix(authHeader, "Bearer ")
	if accessToken == "" {
		return nil, status.Error(codes.Unauthenticated, "Access token is empty.")
	}

	type UserClaims struct {
		UserID   uint64 `json:"user_id"`
		UserUUID string `json:"user_uuid"`
		Email    string `json:"email"`
		jwt.RegisteredClaims
	}

	claims := &UserClaims{}
	token, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (interface{}, error) {
		// Kiểm tra thuật toán ký
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(JwtSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, status.Errorf(codes.Unauthenticated, "Invalid or expired token: %v", err)
	}

	userUUID := claims.UserUUID
	if userUUID == "" {
		return nil, status.Error(codes.Unauthenticated, "Token claims missing user UUID.")
	}

	user, err := s.userRepo.FindByUuid(ctx, userUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to query user details: %v", err)
	}
	if user == nil {
		return nil, status.Error(codes.NotFound, "User profile not found.")
	}

	createdAtMilli := user.CreatedAt.UnixMilli()

	return &userpb.GetUserProfileResponse{
		User:      userToDto(user),
		Status:    common.UserStatus(common.UserStatus_value[string(user.Status)]),
		KycStatus: common.KycStatus_KYC_STATUS_VERIFIED,
		CreatedAt: createdAtMilli,
	}, nil
}

func generateAccessToken(user *model.User) (string, int64, error) {
	expireTime := time.Now().Add(24 * time.Hour)
	expireUnix := expireTime.Unix()

	claims := jwt.MapClaims{
		"user_id":   user.ID,
		"user_uuid": user.Uuid,
		"email":     user.Email,
		"exp":       expireUnix, // expires in 24h
		"iat":       time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	accessToken, err := token.SignedString([]byte(JwtSecret))
	if err != nil {
		return "", 0, status.Errorf(codes.Internal, "Failed to generate token: %v", err)
	}

	return accessToken, expireUnix, nil
}

func userToDto(user *model.User) *common.UserDto {
	now := time.Now().UnixMilli()
	return &common.UserDto{
		Id:          user.ID,
		Uuid:        user.Uuid,
		Email:       user.Email,
		LastLoginAt: &now,
		Status:      string(user.Status),
	}
}
