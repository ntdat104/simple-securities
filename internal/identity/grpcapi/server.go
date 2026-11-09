// Package grpcapi exposes the identity use cases over gRPC (and, via grpc-gateway, HTTP/JSON).
package grpcapi

import (
	"context"
	"time"

	identityv1 "simple-securities/gen/identity/v1"
	"simple-securities/internal/identity/app"
	"simple-securities/internal/identity/store"
)

// Server implements identityv1.IdentityServiceServer.
type Server struct {
	identityv1.UnimplementedIdentityServiceServer
	svc *app.Service
}

// NewServer returns the gRPC server implementation.
func NewServer(svc *app.Service) *Server { return &Server{svc: svc} }

// Register creates an account.
func (s *Server) Register(ctx context.Context, req *identityv1.RegisterRequest) (*identityv1.AuthResponse, error) {
	sess, err := s.svc.Register(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, err
	}
	return toAuth(sess), nil
}

// Login authenticates with e-mail and password.
func (s *Server) Login(ctx context.Context, req *identityv1.LoginRequest) (*identityv1.AuthResponse, error) {
	sess, err := s.svc.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, err
	}
	return toAuth(sess), nil
}

// Refresh rotates a refresh token.
func (s *Server) Refresh(ctx context.Context, req *identityv1.RefreshRequest) (*identityv1.AuthResponse, error) {
	sess, err := s.svc.Refresh(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, err
	}
	return toAuth(sess), nil
}

// Logout revokes a refresh token.
func (s *Server) Logout(ctx context.Context, req *identityv1.LogoutRequest) (*identityv1.LogoutResponse, error) {
	if err := s.svc.Logout(ctx, req.GetRefreshToken()); err != nil {
		return nil, err
	}
	return &identityv1.LogoutResponse{}, nil
}

// GetUser returns a user profile.
func (s *Server) GetUser(ctx context.Context, req *identityv1.GetUserRequest) (*identityv1.User, error) {
	u, err := s.svc.GetUser(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	return toUser(u), nil
}

func toAuth(s app.Session) *identityv1.AuthResponse {
	return &identityv1.AuthResponse{
		User:            toUser(s.User),
		AccessToken:     s.AccessToken,
		RefreshToken:    s.RefreshToken,
		AccessExpiresIn: int64(s.AccessTTL / time.Second),
	}
}

func toUser(u store.User) *identityv1.User {
	return &identityv1.User{UserId: u.ID, Email: u.Email, CreatedAt: u.CreatedAt.UTC().Format(time.RFC3339)}
}
