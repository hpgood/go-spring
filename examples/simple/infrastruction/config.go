package infrastruction

import "log"

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
}

func (cfg *Config) Before() {
	log.Println("config do something")
	cfg.Host = "127.0.0.1"
	cfg.Port = 8080
	cfg.User = "admin"
	cfg.Password = "your_password"
}
func (cfg *Config) Start() {

}

func (cfg Config) Name() string {

	return "config"
}
