package config

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

type Location struct {
	Name      string  `yaml:"name"`
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
}

var (
	instance *Config
	once     sync.Once
)

// Config - can/will add more later
type Config struct {
	Weather struct {
		MonitoredFields []string `yaml:"monitored_fields"`
	} `yaml:"weather"`
	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
		Stream   string `yaml:"stream"`
	} `yaml:"redis"`
	Locations []Location `yaml:"locations"`
}

func Load(configPath string) (*Config, error) {
	var err error
	once.Do(func() {
		instance = &Config{}

		data, readErr := os.ReadFile(configPath)
		if readErr != nil {
			err = fmt.Errorf("failed to read config file %s: %w", configPath, readErr)
			return
		}

		if parseErr := yaml.Unmarshal(data, instance); parseErr != nil {
			err = fmt.Errorf("failed to parse config: %w", parseErr)
			return
		}

		if validateErr := instance.validate(); validateErr != nil {
			err = validateErr
			return
		}
	})

	return instance, err
}

func Get() *Config {
	if instance == nil {
		panic("config not loaded - call config.Load() first")
	}
	return instance
}

func (c *Config) validate() error {
	if len(c.Weather.MonitoredFields) == 0 {
		return fmt.Errorf("weather.monitored_fields cannot be empty")
	}
	return nil
}
