package config

// При желании конфигурацию можно вынести в internal/config.
// Организация конфига в main принуждает нас сужать API компонентов, использовать
// при их конструировании только необходимые параметры, а также уменьшает вероятность циклической зависимости.

import (
	"log"
	"os"
	"time"

	yaml "gopkg.in/yaml.v3"
)

type Config struct {
	Port                 string        `yaml:"port"`
	KeycloakURL          string        `yaml:"keycloakURL"`
	KeycloakExternalURL  string        `yaml:"keycloakExternalURL"`
	KeycloakRealm        string        `yaml:"keycloakRealm"`
	SessionCookieName    string        `yaml:"sessionCookieName"`
	SessionTTLSeconds    time.Duration `yaml:"sessionTTLSeconds"`
	StateTTLSeconds      time.Duration `yaml:"stateTTLSeconds"`
	CleanupPeriodSeconds time.Duration `yaml:"cleanupPeriodSeconds"`
	ClientID             string        `yaml:"clientID"`
	ClientSecret         string        `yaml:"clientSecret"`
	FrontendURL          string        `yaml:"frontendURL"`
	APIBaseURL           string        `yaml:"apiBaseURL"`
	ReportServiceURL     string        `yaml:"reportServiceURL"`
	Logger               LoggerConf
}

type LoggerConf struct {
	Level string
}

func NewConfig() *Config {
	return &Config{}
}

func Parse(filePath string) (*Config, error) {
	configData, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal(err)
	}

	cfg := NewConfig()
	err = yaml.Unmarshal(configData, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, err
}
