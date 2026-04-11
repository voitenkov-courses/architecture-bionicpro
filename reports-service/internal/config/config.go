package config

// При желании конфигурацию можно вынести в internal/config.
// Организация конфига в main принуждает нас сужать API компонентов, использовать
// при их конструировании только необходимые параметры, а также уменьшает вероятность циклической зависимости.

import (
	"log"
	"os"

	yaml "gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConf  `yaml:"server"`
	Storage StorageConf `yaml:"storage"`
	Cdn     Cdn         `yaml:"cdn"`
	Logger  LoggerConf  `yaml:"logger"`
}

type ServerConf struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
}

type StorageConf struct {
	Host  string `yaml:"host"`
	Port  string `yaml:"port"`
	DB    string `yaml:"db"`
	Table string `yaml:"table"`
}

type Cdn struct {
	S3Endpoint  string `yaml:"s3Endpoint"`
	S3AccessKey string `yaml:"s3AccessKey"`
	S3SecretKey string `yaml:"s3SecretKey"`
	BaseURL     string `yaml:"baseURL"`
	Bucket      string `yaml:"bucket"`
}

type LoggerConf struct {
	Level string `yaml:"level"`
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
