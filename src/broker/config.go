package broker

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"awesomeProject/beacon/mqtt-broker-sn/logger"
	"go.uber.org/zap"
)

type Config struct {
	RegularWorker int     `json:"regularWorkerNum"`
	SpecialWorker int     `json:"specialWorkerNum"`
	SupremeWorker int     `json:"supremeWorkerNum"`
	Host          string  `json:"host"`
	Port          string  `json:"port"`
	TlsHost       string  `json:"tlsHost"`
	TlsPort       string  `json:"tlsPort"`
	WsPath        string  `json:"wsPath"`
	WsPort        string  `json:"wsPort"`
	WsTLS         bool    `json:"wsTLS"`
	TlsInfo       TLSInfo `json:"tlsInfo"`
	Acl           bool    `json:"acl"`
	AclConf       string  `json:"aclConf"`
	Debug         bool    `json:"debug"`
}

type TLSInfo struct {
	Verify   bool   `json:"verify"`
	CaFile   string `json:"caFile"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

var (
	DefaultConfig = &Config{
		RegularWorker: 1024,
		SpecialWorker: 1024,
		SupremeWorker: 1024,
		Host:          "0.0.0.0",
		Port:          "1883",
		Acl:           false,
	}

	log *zap.Logger
)

func showHelp() {
	fmt.Printf("%s\n", usageStr)
	os.Exit(0)
}

func ConfigureConfig(args []string) (*Config, error) {
	config := &Config{}
	var (
		help       bool
		configFile string
	)
	fs := flag.NewFlagSet("mqtt-broker", flag.ExitOnError)
	fs.Usage = showHelp

	fs.BoolVar(&help, "h", false, "Show this message.")
	fs.BoolVar(&help, "help", false, "Show this message.")
	fs.IntVar(&config.RegularWorker, "rew", 1024, "worker num to process message, perfer (client num)/10.")
	fs.IntVar(&config.RegularWorker, "regularworker", 1024, "worker num to process message, perfer (client num)/10.")
	fs.IntVar(&config.SpecialWorker, "spw", 1024, "worker num to process message, perfer (client num)/10.")
	fs.IntVar(&config.SpecialWorker, "specialworker", 1024, "worker num to process message, perfer (client num)/10.")
	fs.IntVar(&config.SupremeWorker, "suw", 1024, "worker num to process message, perfer (client num)/10.")
	fs.IntVar(&config.SupremeWorker, "supremeworker", 1024, "worker num to process message, perfer (client num)/10.")
	fs.StringVar(&config.Port, "port", "1883", "Port to listen on.")
	fs.StringVar(&config.Port, "p", "1883", "Port to listen on.")
	fs.StringVar(&config.Host, "host", "0.0.0.0", "Network host to listen on")
	fs.StringVar(&config.WsPort, "ws", "", "port for ws to listen on")
	fs.StringVar(&config.WsPort, "wsport", "", "port for ws to listen on")
	fs.StringVar(&config.WsPath, "wsp", "", "path for ws to listen on")
	fs.StringVar(&config.WsPath, "wspath", "", "path for ws to listen on")
	fs.StringVar(&configFile, "config", "", "config file for mqtt-broker")
	fs.StringVar(&configFile, "c", "", "config file for mqtt-broker")
	fs.BoolVar(&config.Debug, "debug", false, "enable Debug logging.")
	fs.BoolVar(&config.Debug, "d", false, "enable Debug logging.")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if help {
		showHelp()
		return nil, nil
	}

	if configFile != "" {
		tmpConfig, e := LoadConfig(configFile)
		if e != nil {
			return nil, e
		} else {
			config = tmpConfig
		}
	}

	logger.InitLogger(config.Debug)
	log = logger.Get().Named("Broker")

	if err := config.check(); err != nil {
		return nil, err
	}

	return config, nil

}

func LoadConfig(filename string) (*Config, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Error("Read config file error: ", zap.Error(err))
		return nil, err
	}
	var config Config
	err = json.Unmarshal(content, &config)
	if err != nil {
		log.Error("Unmarshal config file error: ", zap.Error(err))
		return nil, err
	}

	return &config, nil
}

func (config *Config) check() error {

	if config.RegularWorker == 0 {
		config.RegularWorker = 1024
	}

	if config.SpecialWorker == 0 {
		config.SpecialWorker = 1024
	}

	if config.SupremeWorker == 0 {
		config.SupremeWorker = 1024
	}

	if config.Port != "" {
		if config.Host == "" {
			config.Host = "0.0.0.0"
		}
	}

	if config.TlsPort != "" {
		if config.TlsInfo.CertFile == "" || config.TlsInfo.KeyFile == "" {
			log.Error("tls config error, no cert or key file.")
			return errors.New("config/check: tls config error, no cert or key file")
		}
		if config.TlsHost == "" {
			config.TlsHost = "0.0.0.0"
		}
	}
	return nil
}

func NewTLSConfig(tlsInfo TLSInfo) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(tlsInfo.CertFile, tlsInfo.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("error parsing X509 certificate/key pair: %v", zap.Error(err))
	}
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("error parsing certificate: %v", zap.Error(err))
	}

	// Create TLSConfig
	// We will determine the cipher suites that we prefer.
	config := tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	// Require client certificates as needed
	if tlsInfo.Verify {
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}
	// Add in CAs if applicable.
	if tlsInfo.CaFile != "" {
		rootPEM, err := ioutil.ReadFile(tlsInfo.CaFile)
		if err != nil || rootPEM == nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		ok := pool.AppendCertsFromPEM(rootPEM)
		if !ok {
			return nil, fmt.Errorf("failed to parse root ca certificate")
		}
		config.ClientCAs = pool
	}

	return &config, nil
}
