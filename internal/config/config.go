package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Resolver    string            `toml:"resolver"`
	TTL         int               `toml:"ttl"`
	BindAddress string            `toml:"bind_address"`
	Hosts       map[string]string `toml:"hosts"`
	Client      *http.Client      `toml:"-"`
}

const DefaultConfigPath = "config.toml"

func Default() *Config {
	return &Config{
		Resolver:    "https://one.one.one.one/dns-query",
		TTL:         300,
		BindAddress: "0.0.0.0:53",
		Hosts: map[string]string{
			"*.home": "127.0.0.1",
		},
	}
}

func Load(path string) (*Config, error) {
	conf := Default()

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("Config file not found, using defaults", "path", path)
			if err := conf.Validate(); err != nil {
				return nil, err
			}
			if err := conf.Save(path); err != nil {
				slog.Error("Failed to save default config", "error", err)
			}
			return conf, nil
		}
		return nil, err
	}
	defer file.Close()

	if err := toml.NewDecoder(file).Decode(conf); err != nil {
		return nil, err
	}

	if err := conf.Validate(); err != nil {
		return nil, err
	}

	return conf, nil
}

func (c *Config) Save(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	encoder.SetIndentTables(true)
	return encoder.Encode(c)
}

type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	return "invalid config: " + strings.Join(e.Problems, "; ")
}

func (c *Config) Validate() error {
	var problems []string

	if strings.TrimSpace(c.Resolver) == "" {
		problems = append(problems, "resolver is required")
	} else if err := validateResolver(c.Resolver); err != nil {
		problems = append(problems, fmt.Sprintf("resolver: %v", err))
	}

	if c.TTL <= 0 {
		problems = append(problems, "ttl must be greater than zero")
	}

	if err := validateBindAddress(c.BindAddress); err != nil {
		problems = append(problems, fmt.Sprintf("bind_address: %v", err))
	}

	for _, host := range sortedHostKeys(c.Hosts) {
		if err := validateHostPattern(host); err != nil {
			problems = append(problems, fmt.Sprintf("hosts.%q: %v", host, err))
		}
		if ip := net.ParseIP(c.Hosts[host]); ip == nil {
			problems = append(problems, fmt.Sprintf("hosts.%q must map to a valid IP address", host))
		}
	}

	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

func validateResolver(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("scheme must be http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("host is required")
	}
	if parsed.Path == "" {
		return fmt.Errorf("path is required")
	}
	return nil
}

func validateBindAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if host == "" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		return nil
	}
	if strings.ContainsAny(host, " /\\") {
		return fmt.Errorf("host contains invalid characters")
	}
	return nil
}

func validateHostPattern(host string) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("host pattern is required")
	}
	if strings.Contains(host, "*") && !strings.HasPrefix(host, "*.") {
		return fmt.Errorf("wildcard must be at the start as *.")
	}
	name := strings.TrimPrefix(host, "*.")
	if name == "" {
		return fmt.Errorf("host name is required")
	}
	labels := strings.Split(name, ".")
	for _, label := range labels {
		if label == "" {
			return fmt.Errorf("host contains an empty label")
		}
	}
	return nil
}

func sortedHostKeys(hosts map[string]string) []string {
	keys := make([]string, 0, len(hosts))
	for host := range hosts {
		keys = append(keys, host)
	}
	sort.Strings(keys)
	return keys
}
