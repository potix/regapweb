package handler

import (
	"log"
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

type Forwarder struct {
	opts            *forwarderOptions
        toTcpChan       chan *GamepadMessage
        toWsChan        chan *GamepadMessage
        stopFromTcpChan chan int
        stopFromWsChan  chan int
}

func (f *Forwarder)ToTcp(msg *GamepadMessage) {
	f.toTcpChan <- msg
}

func (f *Forwarder)ToWs(msg *GamepadMessage) {
	f.toWsChan <- msg
}

type OnFromTcp func(*GamepadMessage)

func (f *Forwarder) StartFromTcpListener(fn OnFromTcp) {
	go func() {
		log.Printf("start from tcp listener")
		for {
			select {
			case v := <-f.toWsChan:
				fn(v)
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

type OnFromWs func(*GamepadMessage)

func (f *Forwarder) StartFromWsListener(fn OnFromWs) {
	go func() {
		log.Printf("start from http listener")
		for {
			select {
			case v := <-f.toTcpChan:
				fn(v)
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
		toTcpChan:       make(chan *GamepadMessage),
		toWsChan:        make(chan *GamepadMessage),
		stopFromTcpChan: make(chan int),
		stopFromWsChan:  make(chan int),
	}
}
