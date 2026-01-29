package config

type InternalServiceConfig struct {
	UserService         string `yaml:"user_service" mapstructure:"user_service"`
	NotificationService string `yaml:"notification_service" mapstructure:"notification_service"`
	CryptoService       string `yaml:"crypto_service" mapstructure:"crypto_service"`
	MarketService       string `yaml:"market_service" mapstructure:"market_service"`
}

type ExternalServiceConfig struct {
	IpInfoService string `yaml:"ip_info_service" mapstructure:"ip_info_service"`
}
