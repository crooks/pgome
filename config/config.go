package config

import (
	"flag"
	"os"

	"gopkg.in/yaml.v2"
)

// Flags are the command line Flags
type Flags struct {
	Config   string
	Debug    bool
	Warranty bool
}

// Config contains the njmon_exporter configuration data
type Config struct {
	API struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		CertFile string `yaml:"certfile"`
		URL      string `yaml:"url"`
	} `yaml:"api"`
	Logging struct {
		Journal  bool   `yaml:"journal"`
		LevelStr string `yaml:"level"`
	} `yaml:"logging"`
	Output struct {
		Filename string `yaml:"filename"`
		FQDN     bool   `yaml:"fqdn"`
	} `yaml:"output"`
}

// ParseConfig imports a yaml formatted config file into a Config struct
func ParseConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{}
	d := yaml.NewDecoder(file)
	if err := d.Decode(&config); err != nil {
		return nil, err
	}
	return config, nil
}

// WriteConfig will create a YAML formatted config file from a Config struct
func (c *Config) WriteConfig(filename string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

// parseFlags processes arguments passed on the command line in the format
// standard format: --foo=bar
func ParseFlags() *Flags {
	f := new(Flags)
	flag.StringVar(&f.Config, "config", "pgome.yml", "Path to pgome configuration file")
	flag.BoolVar(&f.Debug, "debug", false, "Expand logging with Debug level messaging and format")
	flag.BoolVar(&f.Warranty, "warranty", false, "Output warranty information")
	flag.Parse()
	return f
}
