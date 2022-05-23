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
        toTcpChan       chan string
        toWsChan        chan string
        stopFromTcpChan chan int
        stopFromWsChan  chan int
}

func (f *Forwarder)ToTcp(msg string) {
	f.toTcpChan <- msg
}

func (f *Forwarder)ToWs(msg string) {
	f.toWsChan <- msg
}

func (f *Forwarder) StartFromTcpListener(func onFromTcp(msg string)) {
	go func() {
		log.Printf("start from tcp listener")
		for {
			select {
			case v <- f.toTcpChan:
				onFromTcp(v)
			case <-f.stopFromTcpChan:
				return
			}
		}
		log.Printf("finish from tcp listener")
	}()
}

func (f *Forwarder) StartFromWsListener(func onFromWs(msg string)) {
	go func() {
		log.Printf("start from http listener")
		for {
			select {
			case v <- f.toWsChan:
				onFromWs(v)
			case <- f.stopFromWsChan:
				return
			}
		}
		log.Printf("finish from http listener")
	}()
}

func (f *Forwarder) StopFromTcpListener() {
	close(f.stopFromTcpChan)
}

func (f *Forwarder) StopFromTcpListener() {
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
		toTcpChan:       make(chan string),
		toWsChan:        make(chan string),
		stopFromTcpChan: make(chan int),
		stopFromWsChan:  make(chan int),
	}
}
