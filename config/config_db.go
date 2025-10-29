package config

type MySQLConfig struct {
	User         string `yaml:"user" mapstructure:"user"`
	Password     string `yaml:"password" mapstructure:"password"`
	Host         string `yaml:"host" mapstructure:"host"`
	Port         int    `yaml:"port" mapstructure:"port"`
	Database     string `yaml:"database" mapstructure:"database"`
	MaxIdleConns int    `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
	MaxOpenConns int    `yaml:"max_open_conns" mapstructure:"max_open_conns"`
	MaxLifeTime  string `yaml:"max_life_time" mapstructure:"max_life_time"`
	MaxIdleTime  string `yaml:"max_idle_time" mapstructure:"max_idle_time"`
	CharSet      string `yaml:"char_set" mapstructure:"char_set"`
	ParseTime    bool   `yaml:"parse_time" mapstructure:"parse_time"`
	TimeZone     string `yaml:"time_zone" mapstructure:"time_zone"`
}

type PostgreSQLConfig struct {
	User            string `yaml:"user" mapstructure:"user"`
	Password        string `yaml:"password" mapstructure:"password"`
	Host            string `yaml:"host" mapstructure:"host"`
	Port            int    `yaml:"port" mapstructure:"port"`
	Database        string `yaml:"database" mapstructure:"database"`
	SSLMode         string `yaml:"ssl_mode" mapstructure:"ssl_mode"`
	Options         string `yaml:"options" mapstructure:"options"`
	MaxConnections  int32  `yaml:"max_connections" mapstructure:"max_connections"`
	MinConnections  int32  `yaml:"min_connections" mapstructure:"min_connections"`
	MaxConnLifetime int    `yaml:"max_conn_lifetime" mapstructure:"max_conn_lifetime"`
	IdleTimeout     int    `yaml:"idle_timeout" mapstructure:"idle_timeout"`
	ConnectTimeout  int    `yaml:"connect_timeout" mapstructure:"connect_timeout"`
	TimeZone        string `yaml:"time_zone" mapstructure:"time_zone"`
}

type RedisConfig struct {
	Host         string `yaml:"host" mapstructure:"host"`
	Port         int    `yaml:"port" mapstructure:"port"`
	Password     string `yaml:"password" mapstructure:"password"`
	DB           int    `yaml:"db" mapstructure:"db"`
	PoolSize     int    `yaml:"poolSize" mapstructure:"poolSize"`
	IdleTimeout  int    `yaml:"idleTimeout" mapstructure:"idleTimeout"`
	MinIdleConns int    `yaml:"minIdleConns" mapstructure:"minIdleConns"`
}

type MongoDBConfig struct {
	Host        string `yaml:"host" mapstructure:"host"`
	Port        int    `yaml:"port" mapstructure:"port"`
	Database    string `yaml:"database" mapstructure:"database"`
	User        string `yaml:"user" mapstructure:"user"`
	Password    string `yaml:"password" mapstructure:"password"`
	AuthSource  string `yaml:"auth_source" mapstructure:"auth_source"`
	Options     string `yaml:"options" mapstructure:"options"`
	MinPoolSize int    `yaml:"min_pool_size" mapstructure:"min_pool_size"`
	MaxPoolSize int    `yaml:"max_pool_size" mapstructure:"max_pool_size"`
	IdleTimeout int    `yaml:"idle_timeout" mapstructure:"idle_timeout"`
}
