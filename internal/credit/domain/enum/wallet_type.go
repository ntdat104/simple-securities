package enum

type WalletType string

const (
	WalletTypeMembership WalletType = "MEMBERSHIP" // credit tặng từ membership   — có expire
	WalletTypeCourse     WalletType = "COURSE"     // credit tặng từ mua khoá học — có expire
	WalletTypePurchased  WalletType = "PURCHASED"  // credit mua                  — không expire
	WalletTypeGiveaway   WalletType = "GIVEAWAY"   // credit give away            — có expire
)

func (w WalletType) HasExpiry() bool {
	return w != WalletTypePurchased
}
