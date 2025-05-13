package config

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address                   string        `env:"RUN_ADDRESS"`
	AccrualSystemAddress      string        `env:"ACCRUAL_SYSTEM_ADDRESS"`
	LogLevel                  string        `env:"LOG_LEVEL" envDefault:"INFO"`
	DatabaseURI               string        `env:"DATABASE_URI"`
	SecretKey                 string        `env:"SECRET_KEY" envDefault:"DontUseThatInProduction"`
	JWTExpireHours            int64         `env:"JWT_EXPIRE_HOURS" envDefault:"96"`
	DefaultChannelsBufferSize int64         `env:"DEFAULT_CHANNELS_BUFFER_SIZE" envDefault:"1024"`
	WorkersNumber             int64         `env:"WORKERS_NUMBER" envDefault:"16"`
	OrderStatusCheckPeriod    time.Duration `env:"ORDER_STATUS_CHECK_PERIOD" envDefault:"1s"`
}

func (cfg *Config) Sanitize() {
	if !strings.HasSuffix(cfg.AccrualSystemAddress, "/") {
		cfg.AccrualSystemAddress = cfg.AccrualSystemAddress + "/"
	}
}

var Settings Config

func NewConfigFromArgs(argsConfig ArgsConfig) Config {
	return Config{
		Address:              argsConfig.Address.String(),
		AccrualSystemAddress: argsConfig.AccrualSystemAddress.String(),
		DatabaseURI:          argsConfig.DatabaseURI.String(),
	}
}

type ArgsConfig struct {
	Address              NetAddress
	AccrualSystemAddress HTTPAddress
	DatabaseURI          DatabaseURI
}

var argsConfig ArgsConfig

type NetAddress struct {
	Host string
	Port int
}

func (n *NetAddress) String() string {
	return n.Host + ":" + strconv.Itoa(n.Port)
}

func (n *NetAddress) Set(flagValue string) error {
	host, port, err := net.SplitHostPort(flagValue)
	if err != nil {
		return err
	}
	if host == "" && port == "" {
		n.Host = "localhost"
		n.Port = 8080
		return nil
	}
	port = strings.TrimSuffix(port, "/")
	n.Host = host
	n.Port, err = strconv.Atoi(port)
	return err
}

type HTTPAddress struct {
	Scheme string
	Host   string
	Port   int
}

func (h *HTTPAddress) String() string {
	return fmt.Sprintf(`%s%s:%d/`, h.Scheme, h.Host, h.Port)
}

func (h *HTTPAddress) Set(flagValue string) error {
	addressParts := strings.Split(flagValue, "://")
	if addressParts == nil {
		h.Scheme = "http://"
		h.Host = "localhost"
		h.Port = 8080
		return nil
	}
	if len(addressParts) != 2 {
		fmt.Println("wrong base address format (must be schema://host:port)")
		return errors.New("wrong base address format (must be schema://host:port)")
	}
	netAddress := new(NetAddress)
	err := netAddress.Set(addressParts[1])
	if err != nil {
		fmt.Println("error setting host:port from base address:", err)
		return err
	}
	h.Scheme = addressParts[0] + "://"
	h.Host = netAddress.Host
	h.Port = netAddress.Port
	return err
}

type DatabaseURI struct {
	DSN string
}

func (d *DatabaseURI) String() string {
	return d.DSN
}
func (d *DatabaseURI) Set(flagValue string) error {
	if flagValue == "" {
		return errors.New("database DSN must not be empty")
	}
	d.DSN = flagValue
	return nil
}

func ParseFlags() {
	hostAddr := new(NetAddress)
	accrualSystemAddress := new(HTTPAddress)
	databaseURI := new(DatabaseURI)
	flag.Var(hostAddr, "a", "Address to host on host:port")
	flag.Var(accrualSystemAddress, "r", "base URL for resulting short URL (scheme://host:port)")
	flag.Var(databaseURI, "d", "DSN to connect to the database")
	flag.Parse()
	if hostAddr.Host == "" && hostAddr.Port == 0 {
		hostAddr.Host = "localhost"
		hostAddr.Port = 8081
	}
	if accrualSystemAddress.Host == "" && accrualSystemAddress.Port == 0 && accrualSystemAddress.Scheme == "" {
		accrualSystemAddress.Scheme = "http://"
		accrualSystemAddress.Host = "localhost"
		accrualSystemAddress.Port = 8080
	}

	argsConfig.Address = *hostAddr
	argsConfig.AccrualSystemAddress = *accrualSystemAddress
	argsConfig.DatabaseURI = *databaseURI
	Settings = NewConfigFromArgs(argsConfig)
}

func init() {
	Settings.Address = "localhost:8081"
	Settings.AccrualSystemAddress = "http://localhost:8080/"
	Settings.LogLevel = "INFO"
	Settings.DatabaseURI = ""
	Settings.SecretKey = "DontUseThatInProduction" // Ожидается, что настоящий ключ будет передан через env
}
