package handler

import (
	"log"
	"sync"
	"github.com/potix/regapweb/message"
)

type client struct {
	name string
}

type clientsStoreOptions struct {
        verbose bool
}

func defaultClientsStoreOptions() *clientsStoreOptions {
        return &clientsStoreOptions {
                verbose: false,
        }
}

type ClientsStoreOption func(*clientsStoreOptions)

func ClientsStoreVerbose(verbose bool) ClientsStoreOption {
        return func(opts *clientsStoreOptions) {
                opts.verbose = verbose
        }
}

type ClientsStore struct {
	verbose                bool
	delivererClientsMutex  sync.Mutex
	delivererClients       map[string]*client
	controllerClientsMutex sync.Mutex
	controllerClients      map[string]*client
	gamepadClientsMutex    sync.Mutex
	gamepadClients         map[string]*client
}

func (c *ClientsStore) baseAddClient(clients map[string]*client, clientId string, clientName string) {
	clnt, ok := clients[clientId]
	if !ok {
		clients[clientId] = &client{
			name: clientName,
		}
	} else {
		clnt.name = clientName
	}
}

func (c *ClientsStore) AddDeliverer(clientId string, clientName string) {
	c.delivererClientsMutex.Lock()
        defer c.delivererClientsMutex.Unlock()
	c.baseAddClient(c.delivererClients, clientId, clientName)
	if c.verbose {
		log.Printf("add or update deliverer: id = %v, name = %v", clientId, clientName)
	}
}

func (c *ClientsStore) AddController(clientId string, clientName string) {
	c.controllerClientsMutex.Lock()
        defer c.controllerClientsMutex.Unlock()
	c.baseAddClient(c.controllerClients, clientId, clientName)
	if c.verbose {
		log.Printf("add or update controller: id = %v, name = %v", clientId, clientName)
	}
}

func (c *ClientsStore) AddGamepad(clientId string, clientName string) {
	c.gamepadClientsMutex.Lock()
        defer c.gamepadClientsMutex.Unlock()
	c.baseAddClient(c.gamepadClients, clientId, clientName)
	if c.verbose {
		log.Printf("add or update gamepad: id = %v, name = %v", clientId, clientName)
	}
}

func (c *ClientsStore) baseDeleteClient(clients map[string]*client, clientId string) {
	_, ok := clients[clientId]
	if ok {
		delete(clients, clientId)
	}
}

func (c *ClientsStore) DeleteDeliverer(clientId string) {
	c.delivererClientsMutex.Lock()
        defer c.delivererClientsMutex.Unlock()
	if c.verbose {
		log.Printf("delete deliverer: id = %v", clientId)
	}
	c.baseDeleteClient(c.delivererClients, clientId)
}

func (c *ClientsStore) DeleteController(clientId string) {
	c.controllerClientsMutex.Lock()
        defer c.controllerClientsMutex.Unlock()
	if c.verbose {
		log.Printf("delete controller: id = %v", clientId)
	}
	c.baseDeleteClient(c.controllerClients, clientId)
}

func (c *ClientsStore) DeleteGamepad(clientId string) {
	c.gamepadClientsMutex.Lock()
        defer c.gamepadClientsMutex.Unlock()
	if c.verbose {
		log.Printf("delete gamepad: id = %v", clientId)
	}
	c.baseDeleteClient(c.gamepadClients, clientId)
}

func (c *ClientsStore) baseGetClients(clients map[string]*client) []*message.NameAndId {
	newClients := make([]*message.NameAndId, 0, len(clients))
	for id, clnt := range clients {
		newClients = append(newClients, &message.NameAndId{ Name: clnt.name, Id: id})
	}
	return newClients
}

func (c *ClientsStore) GetDeliverers() []*message.NameAndId {
	c.delivererClientsMutex.Lock()
        defer c.delivererClientsMutex.Unlock()
	return c.baseGetClients(c.delivererClients)
}

func (c *ClientsStore) GetControllers() []*message.NameAndId {
	c.controllerClientsMutex.Lock()
        defer c.controllerClientsMutex.Unlock()
	return c.baseGetClients(c.controllerClients)
}

func (c *ClientsStore) GetGamepads() []*message.NameAndId {
	c.gamepadClientsMutex.Lock()
        defer c.gamepadClientsMutex.Unlock()
	return c.baseGetClients(c.gamepadClients)
}

func NewClientsStore(opts ...ClientsStoreOption) *ClientsStore {
	baseOpts := defaultClientsStoreOptions()
        for _, opt := range opts {
                if opt == nil {
                        continue
                }
                opt(baseOpts)
        }
	return &ClientsStore {
		verbose:           baseOpts.verbose,
		delivererClients:  make(map[string]*client),
		controllerClients: make(map[string]*client),
		gamepadClients:    make(map[string]*client),
	}
}
