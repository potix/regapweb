package main

import (
        "encoding/json"
        "flag"
        "github.com/potix/utils/signal"
        "github.com/potix/utils/server"
        "github.com/potix/utils/configurator"
        "github.com/potix/regapweb/handler"
        "log"
        "log/syslog"
)

type regapwebHttpServerConfig struct {
        Mode        string `toml:"mode"`
        AddrPort    string `toml:"addrPort"`
        TlsCertPath string `toml:"tlsCertPath"`
        TlsKeyPath  string `toml:"tlsKeyPath"`
	SkipVerify  bool   `toml:"skipVerify`
}

type regapwebHttpHandlerConfig struct {
        ResourcePath string `toml:"resourcePath"`
        Accounts     map[string]string `toml:"accounts"`
}

type regapwebTcpServerConfig struct {
        AddrPort    string `toml:"addrPort"`
        TlsCertPath string `toml:"tlsCertPath"`
        TlsKeyPath  string `toml:"tlsKeyPath"`
	SkipVerify  bool   `toml:"skipVerify`
}

type regapwebTcpHandlerConfig struct {
        Secret string `toml:"secret"`
}

type regapwebLogConfig struct {
        UseSyslog bool `toml:"useSyslog"`
}

type regapwebConfig struct {
        Verbose     bool                       `toml:"verbose"`
        HttpServer  *regapwebHttpServerConfig  `toml:"httpServer"`
        HttpHandler *regapwebHttpHandlerConfig `toml:"httpHandler"`
        TcpServer   *regapwebTcpServerConfig   `toml:"tcpServer"`
        TcpHandler  *regapwebTcpHandlerConfig  `toml:"tcpHandler"`
        Log         *regapwebLogConfig         `toml:"log"`
}

type commandArguments struct {
        configFile string
}

func verboseLoadedConfig(config *regapwebConfig) {
        if !config.Verbose {
                return
        }
        j, err := json.Marshal(config)
        if err != nil {
                log.Printf("can not dump config: %v", err)
                return
        }
        log.Printf("loaded config: %v", string(j))
}

func main() {
        cmdArgs := new(commandArguments)
        flag.StringVar(&cmdArgs.configFile, "config", "./regapweb.conf", "config file")
        flag.Parse()
        cf, err := configurator.NewConfigurator(cmdArgs.configFile)
        if err != nil {
                log.Fatalf("can not create configurator: %v", err)
        }
        var conf regapwebConfig
        err = cf.Load(&conf)
        if err != nil {
                log.Fatalf("can not load config: %v", err)
        }
        if conf.HttpServer == nil || conf.HttpHandler == nil || conf.TcpServer == nil || conf.TcpHandler == nil {
                log.Fatalf("invalid config")
        }
        if conf.Log != nil && conf.Log.UseSyslog {
                logger, err := syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, "aars")
                if err != nil {
                        log.Fatalf("can not create syslog: %v", err)
                }
                log.SetOutput(logger)
        }
        verboseLoadedConfig(&conf)
	// setup forwarder
	fVerboseOpt := handler.ForwarderVerbose(conf.Verbose)
	newForwarder := handler.NewForwarder(fVerboseOpt)
	// setup tcp handler
	thVerboseOpt := handler.TcpVerbose(conf.Verbose)
	newTcpHandler, err := handler.NewTcpHandler(
                conf.TcpHandler.Secret,
		newForwarder,
                thVerboseOpt,
        )
        if err != nil {
                log.Fatalf("can not create tcp handler: %v", err)
        }
	// setup tcp server
        tsVerboseOpt := server.TcpServerVerbose(conf.Verbose)
        tsTlsOpt := server.TcpServerTls(conf.TcpServer.TlsCertPath, conf.TcpServer.TlsKeyPath)
        tsSkipVerifyOpt := server.TcpServerSkipVerify(conf.TcpServer.SkipVerify)
        newTcpServer, err := server.NewTcpServer(
                conf.TcpServer.AddrPort,
                newTcpHandler,
                tsTlsOpt,
		tsSkipVerifyOpt,
                tsVerboseOpt,
        )
        if err != nil {
                log.Fatalf("can not create tcp server: %v", err)
        }
	// setup http handler
	hhVerboseOpt := handler.HttpVerbose(conf.Verbose)
	newHttpHandler, err := handler.NewHttpHandler(
                conf.HttpHandler.ResourcePath,
                conf.HttpHandler.Accounts,
		newForwarder,
                hhVerboseOpt,
        )
        if err != nil {
                log.Fatalf("can not create http handler: %v", err)
        }
	// setup http server
        hsVerboseOpt := server.HttpServerVerbose(conf.Verbose)
        hsTlsOpt := server.HttpServerTls(conf.HttpServer.TlsCertPath, conf.HttpServer.TlsKeyPath)
        hsSkipVerifyOpt := server.HttpServerSkipVerify(conf.HttpServer.SkipVerify)
        hsModeOpt := server.HttpServerMode(conf.HttpServer.Mode)
        newHttpServer, err := server.NewHttpServer(
                conf.HttpServer.AddrPort,
                newHttpHandler,
                hsTlsOpt,
		hsSkipVerifyOpt,
                hsModeOpt,
                hsVerboseOpt,
        )
        if err != nil {
                log.Fatalf("can not create http server: %v", err)
        }
        err = newTcpServer.Start()
        if err != nil {
                log.Fatalf("can not start tcp server: %v", err)
        }
        err = newHttpServer.Start()
        if err != nil {
                log.Fatalf("can not start http server: %v", err)
        }
	newForwarder.Start()
        signal.SignalWait(nil)
	newForwarder.Stop()
        newHttpServer.Stop()
        newTcpServer.Stop()
}

