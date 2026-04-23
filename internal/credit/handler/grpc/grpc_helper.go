package grpc

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	personalizev1 "simple-securities/gen/personalize/v1"
	"simple-securities/internal/credit/domain/enum"
	"simple-securities/pkg/errors"
)

func toGrpcError(err error) error {
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*errors.AppError); ok {
		switch appErr.Type {
		case errors.ErrorTypeNotFound:
			return status.Error(codes.NotFound, appErr.Error())
		case errors.ErrorTypeBusiness:
			return status.Error(codes.InvalidArgument, appErr.Error())
		case errors.ErrorTypeUnauthorized:
			return status.Error(codes.Unauthenticated, appErr.Error())
		}
	}
	return status.Error(codes.Internal, err.Error())
}

func parseUint64(s string) uint64 {
	var v uint64
	fmt.Sscanf(s, "%d", &v)
	return v
}

func toProtoWalletType(wt enum.WalletType) personalizev1.WalletType {
	switch wt {
	case enum.WalletTypeMembership:
		return personalizev1.WalletType_WALLET_TYPE_MEMBERSHIP
	case enum.WalletTypeCourse:
		return personalizev1.WalletType_WALLET_TYPE_COURSE
	case enum.WalletTypePurchased:
		return personalizev1.WalletType_WALLET_TYPE_PURCHASED
	case enum.WalletTypeGiveaway:
		return personalizev1.WalletType_WALLET_TYPE_GIVEAWAY
	default:
		return personalizev1.WalletType_WALLET_TYPE_UNSPECIFIED
	}
}
