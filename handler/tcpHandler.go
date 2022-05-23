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
        forwarder *forwarder
}

func (h *TcpHander) onFromHttp(msg string) {
}

func (h *TcpHandler) Start() error {
        forwarder.StartFromHttpListener(h.onFromHttp)
}

func (h *TcpHandler) Stop() {
        forwarder.StopFromHttpListener()
}

func (h *TcpHandler) OnAccept(conn *net.Conn) {

}

func NewTcpHandler(secret string, forwarder *forwarder, opts ...TcpOption) (*TcpHandler, error) {
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

