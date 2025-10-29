package config

type GrpcServerConfig struct {
	Port                  uint32 `yaml:"port" mapstructure:"port"`
	MaxConnectionIdle     int    `yaml:"max_connection_idle" mapstructure:"max_connection_idle"`
	MaxConnectionAge      int    `yaml:"max_connection_age" mapstructure:"max_connection_age"`
	MaxConnectionAgeGrace int    `yaml:"max_connection_age_grace" mapstructure:"max_connection_age_grace"`
	Time                  int    `yaml:"time" mapstructure:"time"`
	Timeout               int    `yaml:"timeout" mapstructure:"timeout"`
	MinTime               int    `yaml:"min_time" mapstructure:"min_time"`
	PermitWithoutStream   bool   `yaml:"permit_without_stream" mapstructure:"permit_without_stream"`
}

type HttpServerConfig struct {
	Addr            string `yaml:"addr" mapstructure:"addr"`
	Pprof           bool   `yaml:"pprof" mapstructure:"pprof"`
	DefaultPageSize int    `yaml:"default_page_size" mapstructure:"default_page_size"`
	MaxPageSize     int    `yaml:"max_page_size" mapstructure:"max_page_size"`
	ReadTimeout     string `yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout    string `yaml:"write_timeout" mapstructure:"write_timeout"`
}

type MetricsConfig struct {
	Addr    string `yaml:"addr" mapstructure:"addr"`
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Path    string `yaml:"path" mapstructure:"path"`
}
