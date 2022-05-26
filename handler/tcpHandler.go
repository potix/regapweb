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

type CommonGamepadMessage struct {
        Command string
        Error   string
}

type gamepadVibration struct {
        Duration        float64
        StartDelay      float64
        StrongMagnitude float64
        WeakMagnitude   float64
}

type gamepadButton struct {
        Pressed bool
        Touched bool
        Value   int64
}

type GamepadMessage struct {
        commonGamepadMessage
        Uid     string
        PeerUid string
        Buttons []*gamepadButton
        Axes    []float64
        Vibration gamepadVibration
}



func (t *TcpHandler) onFromWs(msg []byte) {
	// XXXX
	//log.Printf("received %v", string(msg))
}

func (t *TcpHandler) Start() error {
        t.forwarder.StartFromWsListener(t.onFromWs)
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

