package handler

import (
	"log"
	"github.com/potix/regapweb/message"
)

type forwarderOptions struct {
	verbose bool
}

func defaultForwarderOptions() *forwarderOptions {
	return &forwarderOptions {
		verbose: false,
	}
}

type ForwarderOption func(*forwarderOptions)

func ForwarderVerbose(verbose bool) ForwarderOption {
        return func(opts *forwarderOptions) {
                opts.verbose = verbose
        }
}

type ErrorCb func(error)

type OnFromTcp func(*message.Message) error

type OnFromWs func(*message.Message) error

type msgAndErrCb struct {
	msg *message.Message
	errCb ErrorCb
}

type Forwarder struct {
	opts            *forwarderOptions
        toTcpChan       chan *msgAndErrCb
        toWsChan        chan *msgAndErrCb
        stopFromTcpChan chan int
        stopFromWsChan  chan int
	started         bool
}

func (f *Forwarder)Start() {
	f.started = true
}

func (f *Forwarder)Stop() {
	f.started = false
}

func (f *Forwarder)ToTcp(msg *message.Message, errCb ErrorCb) {
	if f.started {
		f.toTcpChan <- &msgAndErrCb{ msg: msg, errCb: errCb }
	}
}

func (f *Forwarder)ToWs(msg *message.Message, errCb ErrorCb) {
	if f.started {
		f.toWsChan <- &msgAndErrCb{ msg: msg, errCb: errCb }
	}
}

func (f *Forwarder) StartFromTcpListener(fn OnFromTcp) {
	go func() {
		log.Printf("start from tcp listener")
		for {
			select {
			case v := <-f.toWsChan:
				err := fn(v.msg)
				if err != nil && v.errCb != nil {
					v.errCb(err)
				}
			case <-f.stopFromTcpChan:
				return
			}
		}
		log.Printf("finish from tcp listener")
	}()
}

func (f *Forwarder) StopFromTcpListener() {
	close(f.stopFromTcpChan)
}

func (f *Forwarder) StartFromWsListener(fn OnFromWs) {
	go func() {
		log.Printf("start from http listener")
		for {
			select {
			case v := <-f.toTcpChan:
				err := fn(v.msg)
				if err != nil && v.errCb != nil {
					v.errCb(err)
				}
			case <-f.stopFromWsChan:
				return
			}
		}
		log.Printf("finish from http listener")
	}()
}

func (f *Forwarder) StopFromWsListener() {
	close(f.stopFromWsChan)
}

func NewForwarder(opts ...ForwarderOption) *Forwarder {
	baseOpts := defaultForwarderOptions()
        for _, opt := range opts {
                if opt == nil {
                        continue
                }
                opt(baseOpts)
        }
	return &Forwarder{
		opts:            baseOpts,
		toTcpChan:       make(chan *msgAndErrCb),
		toWsChan:        make(chan *msgAndErrCb),
		stopFromTcpChan: make(chan int),
		stopFromWsChan:  make(chan int),
		started:         false,
	}
}
