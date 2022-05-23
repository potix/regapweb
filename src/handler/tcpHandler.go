package handler

import (
        "log"
        "net"
)

type tcpOptions struct {
        verbose bool
}

func defaultTcpOptions() *tcpOptions {
        return &tcpOptions {
                verbose: false,
        }
}

type TcpOption func(*tcpOptions)

func TcpVerbose(verbose bool) TcpOption {
        return func(opts *tcpOptions) {
                opts.verbose = verbose
        }
}

type TcpHandler struct {
        verbose   bool
        secret    string
        forwarder *Forwarder
}

func (t *TcpHandler) onFromHttp(msg string) {
}

func (t *TcpHandler) Start() error {
        t.forwarder.StartFromWsListener(t.onFromHttp)
	return nil
}

func (t *TcpHandler) Stop() {
        t.forwarder.StopFromWsListener()
}

func (t *TcpHandler) OnAccept(conn *net.Conn) {
	log.Printf("on accept")

}

func NewTcpHandler(secret string, forwarder *Forwarder, opts ...TcpOption) (*TcpHandler, error) {
        baseOpts := defaultTcpOptions()
        for _, opt := range opts {
                if opt == nil {
                        continue
                }
                opt(baseOpts)
        }
        return &TcpHandler{
                verbose: baseOpts.verbose,
                secret: secret,
                forwarder: forwarder,
        }, nil
}

