
package handler

import (
        "log"
        "fmt"
        "path"
        "net/http"
	"sync"
	"encoding/json"
	"time"
        "github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"github.com/potix/regapweb/message"
)

type httpOptions struct {
        verbose bool
}

func defaultHttpOptions() *httpOptions {
        return &httpOptions {
                verbose: false,
        }
}

type HttpOption func(*httpOptions)

func HttpVerbose(verbose bool) HttpOption {
        return func(opts *httpOptions) {
                opts.verbose = verbose
        }
}

type relationClient struct {
	commit       bool
	delivererId  string
	controllerId string
	gamepadId    string
}

type httpClient struct {
	writeMutex     sync.Mutex
	clientType     string
	clientId       string
	relationClient *relationClient
}

type HttpHandler struct {
        verbose      bool
        resourcePath string
        accounts     map[string]string
	clientsStore *ClientsStore
	forwarder    *Forwarder
	clientsMutex sync.Mutex
	clients      map[*websocket.Conn]*httpClient
}

func (h *HttpHandler) onFromTcp(msg *message.Message) error {
	log.Printf("onFromTcp")
	if msg.MsgType == message.MsgTypeGamepadConnectRes {
		conn, client := h.getControllerByIds(
			msg.GamepadConnectResponse.DelivererId,
			msg.GamepadConnectResponse.ControllerId,
			msg.GamepadConnectResponse.GamepadId)
		if conn == nil || client == nil {
			log.Printf("not found connection for gpConnectRes: %v", msg.GamepadConnectResponse)
			return fmt.Errorf("not found connection for gpConnectRes")
		}
		if client.relationClient == nil ||
		   client.relationClient.delivererId !=  msg.GamepadConnectResponse.DelivererId ||
		   client.relationClient.controllerId !=  msg.GamepadConnectResponse.ControllerId ||
		   client.relationClient.gamepadId !=  msg.GamepadConnectResponse.GamepadId {
			log.Printf("client relation mismatch: %v, %v", client.relationClient, msg.GamepadConnectResponse)
			return fmt.Errorf("client relation mismatch")
		}
		err := h.safeWriteMessage(conn, websocket.TextMessage, msg)
		if err != nil {
			log.Printf("can not write gpConnectRes message: %v", err)
			return fmt.Errorf("can not write gpConnectRes message")
		}
	} else if msg.MsgType == message.MsgTypeGamepadVibration {
		conn, client := h.getControllerByIds(
			msg.GamepadVibration.DelivererId,
			msg.GamepadVibration.ControllerId,
			msg.GamepadVibration.GamepadId)
		if conn == nil || client == nil {
			log.Printf("not found connection for gpVibration: %v", msg.GamepadVibration)
			return nil
		}
		if client.relationClient == nil ||
		   client.relationClient.delivererId !=  msg.GamepadVibration.DelivererId ||
		   client.relationClient.controllerId !=  msg.GamepadVibration.ControllerId ||
		   client.relationClient.gamepadId !=  msg.GamepadVibration.GamepadId {
			log.Printf("client relation mismatch: %v, %v", client.relationClient, msg.GamepadConnectResponse)
			return nil
		}
		err := h.safeWriteMessage(conn, websocket.TextMessage, msg)
		if err != nil {
			log.Printf("can not write message: %v", err)
			return nil
		}
	} else {
		log.Printf("unsupported request: %v", msg.MsgType)
		return nil
	}
	return nil
}

func (h *HttpHandler) Start() error {
	h.forwarder.StartFromTcpListener(h.onFromTcp)
	return nil
}

func (h *HttpHandler) Stop() {
	h.forwarder.StopFromTcpListener()
}

func (h *HttpHandler) SetRouting(router *gin.Engine) {
	favicon := path.Join(h.resourcePath, "icon", "favicon.ico")
        js := path.Join(h.resourcePath, "js")
        css := path.Join(h.resourcePath, "css")
        img := path.Join(h.resourcePath, "img")
        font := path.Join(h.resourcePath, "font")
	templatePath := path.Join(h.resourcePath, "template", "*")
        router.LoadHTMLGlob(templatePath)
	authGroup := router.Group("/", gin.BasicAuth(h.accounts))
	authGroup.GET("/", h.indexHtml)
	authGroup.GET("/index.html", h.indexHtml)
	authGroup.GET("/controller.html", h.indexHtml)
	authGroup.GET("/deliverer.html", h.delivererHtml)
	authGroup.GET("/controllerws", h.controllerWebsocket)
	authGroup.GET("/delivererws", h.delivererWebsocket)
	authGroup.StaticFile("/favicon.ico", favicon)
        authGroup.Static("/js", js)
        authGroup.Static("/css", css)
        authGroup.Static("/img", img)
        authGroup.Static("/font", font)
}

func (h *HttpHandler) indexHtml(c *gin.Context) {
	c.HTML(http.StatusOK, "controller.html", gin.H{})
}

func (h *HttpHandler) delivererHtml(c *gin.Context) {
	c.HTML(http.StatusOK, "deliverer.html", gin.H{})
}


func (h *HttpHandler) clientRegister(conn *websocket.Conn, clientType string, clientId string) *httpClient {
	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()
	client := &httpClient{
		 clientType: clientType,
		 clientId: clientId,
	}
	h.clients[conn] = client
	return client
}

func (h *HttpHandler) clientUnregister(conn *websocket.Conn) {
	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()
	delete(h.clients, conn)
}

func (h *HttpHandler) updateClientRelation(client *httpClient, delivererId string, controllerId string, gamepadId string, commit bool) bool {
	if client.relationClient == nil {
		relationClient := &relationClient {
			commit: commit,
			delivererId: delivererId,
			controllerId: controllerId,
			gamepadId: gamepadId,
		}
		client.relationClient = relationClient
		return true
	} else if client.relationClient.commit == false {
		client.relationClient.delivererId = delivererId
		client.relationClient.controllerId = controllerId
		client.relationClient.gamepadId = gamepadId
		return true
	}
	return false
}

func (h *HttpHandler) commitClientRelation(client *httpClient) bool {
	if client.relationClient == nil {
		return false
	}
	if client.relationClient.commit == true {
		return false
	}
	client.relationClient.commit = true
	return true
}

func (h *HttpHandler) getClient(clientId string) (*websocket.Conn, *httpClient){
	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()
	for conn, client := range h.clients {
		if client.clientId == clientId {
			return conn, client
		}
	}
	return nil, nil
}

func (h *HttpHandler) getControllerByIds(delivererId string, controllerId string, gamepadId string) (*websocket.Conn, *httpClient) {
	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()
	for conn, client := range h.clients {
		if client.clientType != message.ClientTypeController {
			continue
		}
		if client.relationClient.commit == true &&
		   client.relationClient.delivererId == delivererId &&
		   client.relationClient.controllerId == controllerId &&
		   client.relationClient.gamepadId == gamepadId {
			   return conn, client
		 }
	}
	return nil, nil
}

func (h *HttpHandler) safeWriteMessage(conn *websocket.Conn, messageType int, msg *message.Message) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("can not marshal to json: %v", err)
	}
	h.clientsMutex.Lock()
	client := h.clients[conn]
	h.clientsMutex.Unlock()
	client.writeMutex.Lock()
	defer client.writeMutex.Unlock()
	return conn.WriteMessage(messageType, msgBytes)
}

func (h *HttpHandler) startPingLoop(conn *websocket.Conn, pingLoopStopChan chan int) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			msg := &message.Message{
				MsgType: message.MsgTypePing,
			}
			err := h.safeWriteMessage(conn, websocket.TextMessage, msg)
			if err != nil {
				log.Printf("can not write ping message: %v", err)
				return
			}
		case <-pingLoopStopChan:
			return
		}
	}
}

func (h *HttpHandler) websocketLoop(conn *websocket.Conn, clientType string) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		log.Printf("can not create uuid: %v", err)
		return
	}
	clientId := uuid.String()
	client := h.clientRegister(conn, clientType, clientId)
	defer h.clientUnregister(conn)
	defer conn.Close()
	pingStopChan := make(chan int)
	go h.startPingLoop(conn, pingStopChan)
	defer close(pingStopChan)
	for {
		t, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if t != websocket.TextMessage {
			log.Printf("unsupported message type: %v", t)
			continue
		}
		var msg message.Message
		err = json.Unmarshal(msgBytes, &msg)
		if err != nil {
			log.Printf("can not unmarshal message: %v", err)
			continue
		}
		if msg.MsgType == message.MsgTypePing {
			if h.verbose {
				log.Printf("recieved ping")
			}
		} else if msg.MsgType == message.MsgTypeRegisterReq {
			if msg.RegisterRequest == nil {
				log.Printf("no register request parameter: %v", msg.RegisterRequest)
				resMsg := &message.Message {
					MsgType: message.MsgTypeRegisterRes,
					Error: &message.Error{
						Message: "no register request parameter",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write register response message: %v", err)
					return
				}
			}
			resMsg := &message.Message {
				MsgType: message.MsgTypeRegisterRes,
				RegisterResponse: &message.RegisterResponse {
					ClientType: clientType,
					ClientId: clientId,
				},
			}
			err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
			if err != nil {
				log.Printf("can not write register response message: %v", err)
				return
			}
			if clientType == message.ClientTypeDeliverer {
				h.clientsStore.AddDeliverer(clientId, msg.RegisterRequest.ClientName)

			} else if clientType == message.ClientTypeController {
				h.clientsStore.AddController(clientId, msg.RegisterRequest.ClientName)
			}
		} else if msg.MsgType == message.MsgTypeLookupReq {
			controllers := h.clientsStore.GetControllers()
			gamepads := h.clientsStore.GetGamepads()
			resMsg := &message.Message {
				MsgType: message.MsgTypeLookupRes,
				LookupResponse: &message.LookupResponse {
					Controllers: controllers,
					Gamepads: gamepads,
				},
			}
			err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
			if err != nil {
				log.Printf("can not write lookup response message: %v", err)
				return
			}
		} else if msg.MsgType == message.MsgTypeSignalingOfferSdpReq {
			if msg.SignalingSdpRequest == nil ||
			   msg.SignalingSdpRequest.DelivererId == "" ||
			   msg.SignalingSdpRequest.ControllerId == "" ||
			   msg.SignalingSdpRequest.GamepadId == "" ||
			   msg.SignalingSdpRequest.Sdp == "" {
				log.Printf("no sigOfferSdpReq parameter: %v", msg.SignalingSdpRequest)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingOfferSdpServerError,
					Error: &message.Error {
						Message: "no sigOfferSdpReq parameter",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigOfferSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			if msg.SignalingSdpRequest.DelivererId != client.clientId {
				log.Printf("deliverer id mismatch: act %v, exp %v",
					msg.SignalingSdpRequest.DelivererId, client.clientId)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingOfferSdpServerError,
					Error: &message.Error {
						Message: "deliverer id mismatch",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigOfferSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			foundConn, _ := h.getClient(msg.SignalingSdpRequest.ControllerId)
			if foundConn == nil {
				log.Printf("not found controller id: %v", msg.SignalingSdpRequest.ControllerId)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingOfferSdpServerError,
					Error: &message.Error {
						Message: "not found controller id",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigOfferSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			ok := h.updateClientRelation(client,
				msg.SignalingSdpRequest.DelivererId,
				msg.SignalingSdpRequest.ControllerId,
				msg.SignalingSdpRequest.GamepadId,
				false)
			if !ok {
				log.Printf("can not update client relation: %v", msg.SignalingSdpRequest)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingOfferSdpServerError,
					Error: &message.Error {
						Message: "can not update client relation",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigOfferSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			// forward to controller
			err = h.safeWriteMessage(foundConn, websocket.TextMessage, &msg)
			if err != nil {
				log.Printf("can not forward sigOfferSdpReq message: %v", msg)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingOfferSdpServerError,
					Error: &message.Error {
						Message: "can not forward sigOfferSdpReq message",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigOfferSdpSrvErr message: %v", err)
					return
				}
				continue
			}
		} else if msg.MsgType == message.MsgTypeSignalingOfferSdpRes {
			if msg.SignalingSdpResponse == nil ||
			   msg.SignalingSdpResponse.DelivererId == "" ||
			   msg.SignalingSdpResponse.ControllerId == "" ||
			   msg.SignalingSdpResponse.GamepadId == "" {
				log.Printf("no sigOfferSdpRes parameter: %v", msg.SignalingSdpResponse)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingOfferSdpServerError,
					Error: &message.Error {
						Message: "no sigOfferSdpRes parameter",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigOfferSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			if msg.SignalingSdpResponse.ControllerId != client.clientId {
				log.Printf("controller id mismatch: act %v, exp %v",
					msg.SignalingSdpResponse.ControllerId, client.clientId)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingOfferSdpServerError,
					Error: &message.Error {
						Message: "controller id mismatch",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigOfferSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			foundConn, foundClient := h.getClient(msg.SignalingSdpResponse.DelivererId)
			if foundConn == nil || foundClient == nil {
				log.Printf("not found deliverer id: %v", msg.SignalingSdpResponse.DelivererId)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingOfferSdpServerError,
					Error: &message.Error {
						Message: "not found deliverer id",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigOfferSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			if foundClient.relationClient == nil ||
			   foundClient.relationClient.delivererId !=  msg.SignalingSdpResponse.DelivererId ||
			   foundClient.relationClient.controllerId !=  msg.SignalingSdpResponse.ControllerId ||
			   foundClient.relationClient.gamepadId !=  msg.SignalingSdpResponse.GamepadId {
				log.Printf("found client relation mismatch: %v, %v", foundClient.relationClient, msg.SignalingSdpResponse)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingOfferSdpServerError,
					Error: &message.Error {
						Message: "client relation mismatch",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigOfferSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			if msg.Error != nil && msg.Error.Message != "" {
				// forward error to deliverer
				err = h.safeWriteMessage(foundConn, websocket.TextMessage, &msg)
				if err != nil {
					log.Printf("can not forward sigOfferSdpRes message: %v", msg)
					resMsg := &message.Message{
						MsgType: message.MsgTypeSignalingOfferSdpServerError,
						Error: &message.Error {
							Message: "can not forward sigOfferSdpRes message",
						},
					}
					err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
					if err != nil {
						log.Printf("can not write sigOfferSdpSrvErr message: %v", err)
						return
					}
					continue
				}
				continue
			}
			ok := h.commitClientRelation(foundClient)
			if !ok {
				log.Printf("can not commit found client relation: %v", msg.SignalingSdpResponse)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingOfferSdpServerError,
					Error: &message.Error {
						Message: "can not update client relation",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigOfferSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			ok = h.updateClientRelation(client,
				msg.SignalingSdpResponse.DelivererId,
				msg.SignalingSdpResponse.ControllerId,
				msg.SignalingSdpResponse.GamepadId,
				true)
			if !ok {
				log.Printf("can not update client relation: %v", msg.SignalingSdpResponse)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingOfferSdpServerError,
					Error: &message.Error {
						Message: "can not update client relation",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigOfferSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			// forward to deliverer
			err = h.safeWriteMessage(foundConn, websocket.TextMessage, &msg)
			if err != nil {
				log.Printf("can not forward sigOfferSdpRes message: %v", msg)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingOfferSdpServerError,
					Error: &message.Error {
						Message: "can not forward sigOfferSdpRes message",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigOfferSdpSrvErr message: %v", err)
					return
				}
				continue
			}
		} else if msg.MsgType == message.MsgTypeSignalingAnserSdpReq {
			if msg.SignalingSdpRequest == nil ||
			   msg.SignalingSdpRequest.DelivererId == "" ||
			   msg.SignalingSdpRequest.ControllerId == "" ||
			   msg.SignalingSdpRequest.GamepadId == "" ||
			   msg.SignalingSdpRequest.Sdp == "" {
				log.Printf("no sigAnserSdpReq parameter: %v", msg.SignalingSdpRequest)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingAnserSdpServerError,
					Error: &message.Error {
						Message: "no sigAnserSdpReq parameter",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigAnserSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			if msg.SignalingSdpRequest.ControllerId != client.clientId {
				log.Printf("controller id mismatch: act %v, exp %v",
					msg.SignalingSdpRequest.ControllerId, client.clientId)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingAnserSdpServerError,
					Error: &message.Error {
						Message: "controller id mismatch",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigAnserSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			foundConn, foundClient := h.getClient(msg.SignalingSdpRequest.DelivererId)
			if foundConn == nil || foundClient == nil {
				log.Printf("not found deliverer id: %v", msg.SignalingSdpRequest.DelivererId)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingAnserSdpServerError,
					Error: &message.Error {
						Message: "not found deliverer id",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigAnserSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			if client.relationClient == nil ||
			   client.relationClient.delivererId != msg.SignalingSdpRequest.DelivererId ||
			   client.relationClient.controllerId != msg.SignalingSdpRequest.ControllerId ||
			   client.relationClient.gamepadId != msg.SignalingSdpRequest.GamepadId ||
			   foundClient.relationClient == nil ||
			   foundClient.relationClient.delivererId !=  msg.SignalingSdpRequest.DelivererId ||
			   foundClient.relationClient.controllerId !=  msg.SignalingSdpRequest.ControllerId ||
			   foundClient.relationClient.gamepadId !=  msg.SignalingSdpRequest.GamepadId {
				log.Printf("client relation mismatch: %v, %v, %v",
					client.relationClient, foundClient.relationClient, msg.SignalingSdpRequest)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingAnserSdpServerError,
					Error: &message.Error {
						Message: "client relation mismatch",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigAnserSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			// forward to deliverer
			err = h.safeWriteMessage(foundConn, websocket.TextMessage, &msg)
			if err != nil {
				log.Printf("can not forward sigAnserSdpReq message: %v", msg)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingAnserSdpServerError,
					Error: &message.Error {
						Message: "can not forward sigAnserSdpReq message",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigAnserSdpSrvErr message: %v", err)
					return
				}
				continue
			}
		} else if msg.MsgType == message.MsgTypeSignalingAnserSdpRes {
			if msg.SignalingSdpResponse == nil ||
			   msg.SignalingSdpResponse.DelivererId == "" ||
			   msg.SignalingSdpResponse.ControllerId == "" ||
			   msg.SignalingSdpResponse.GamepadId == "" {
				log.Printf("no sigAnserSdpRes parameter: %v", msg.SignalingSdpResponse)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingAnserSdpServerError,
					Error: &message.Error {
						Message: "no sigAnserSdpRes parameter",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigAnserSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			if msg.SignalingSdpResponse.DelivererId != client.clientId {
				log.Printf("deliverer id mismatch: act %v, exp %v",
					msg.SignalingSdpResponse.DelivererId, client.clientId)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingAnserSdpServerError,
					Error: &message.Error {
						Message: "deliverer id mismatch",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigAnserSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			foundConn, foundClient := h.getClient(msg.SignalingSdpResponse.ControllerId)
			if foundConn == nil || foundClient == nil {
				log.Printf("not found controller id: %v", msg.SignalingSdpResponse.ControllerId)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingAnserSdpServerError,
					Error: &message.Error {
						Message: "not found controller Id",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigAnserSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			if client.relationClient == nil ||
			   client.relationClient.delivererId != msg.SignalingSdpResponse.DelivererId ||
			   client.relationClient.controllerId != msg.SignalingSdpResponse.ControllerId ||
			   client.relationClient.gamepadId != msg.SignalingSdpResponse.GamepadId ||
			   foundClient.relationClient == nil ||
			   foundClient.relationClient.delivererId !=  msg.SignalingSdpResponse.DelivererId ||
			   foundClient.relationClient.controllerId !=  msg.SignalingSdpResponse.ControllerId ||
			   foundClient.relationClient.gamepadId !=  msg.SignalingSdpResponse.GamepadId {
				log.Printf("client relation mismatch: %v, %v, %v",
					client.relationClient, foundClient.relationClient, msg.SignalingSdpResponse)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingAnserSdpServerError,
					Error: &message.Error {
						Message: "client relation mismatch",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigAnserSdpSrvErr message: %v", err)
					return
				}
				continue
			}
			// forward to controller
			err = h.safeWriteMessage(foundConn, websocket.TextMessage, &msg)
			if err != nil {
				log.Printf("can not forward sigAnserSdpRes message: %v", msg)
				resMsg := &message.Message{
					MsgType: message.MsgTypeSignalingAnserSdpServerError,
					Error: &message.Error {
						Message: "can not forward sigAnserSdpRes message",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write sigAnserSdpSrvErr message: %v", err)
					return
				}
				continue
			}
		} else if msg.MsgType == message.MsgTypeGamepadConnectReq {
			if msg.GamepadConnectRequest == nil ||
			   msg.GamepadConnectRequest.DelivererId == "" ||
			   msg.GamepadConnectRequest.ControllerId == "" ||
			   msg.GamepadConnectRequest.GamepadId == "" {
				log.Printf("no gamepad connect request parameter: %v", msg.GamepadConnectRequest)
				resMsg := &message.Message{
					MsgType: message.MsgTypeGamepadConnectServerError,
					Error: &message.Error {
						Message: "no gamepad connect request parameter",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write gpConnectSrvErr message: %v", err)
					return
				}
				continue
			}
			if msg.GamepadConnectRequest.ControllerId != client.clientId {
				log.Printf("controller id mismatch: act %v, exp %v",
					msg.GamepadConnectRequest.ControllerId, client.clientId)
				resMsg := &message.Message{
					MsgType: message.MsgTypeGamepadConnectServerError,
					Error: &message.Error {
						Message: "controller id mismatch",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write gpConnectSrvErr message: %v", err)
					return
				}
				continue
			}
			if client.relationClient == nil ||
			   client.relationClient.delivererId != msg.GamepadConnectRequest.DelivererId ||
			   client.relationClient.controllerId != msg.GamepadConnectRequest.ControllerId ||
			   client.relationClient.gamepadId != msg.GamepadConnectRequest.GamepadId {
				log.Printf("client relation mismatch: %v, %v",
					client.relationClient, msg.GamepadConnectRequest)
				resMsg := &message.Message{
					MsgType: message.MsgTypeGamepadConnectServerError,
					Error: &message.Error {
						Message: "client relation mismatch",
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write gpConnectSrvErr message: %v", err)
					return
				}
				continue
			}
			h.forwarder.ToTcp(&msg, func(err error) {
				log.Printf("error callback: %v", err)
				resMsg := &message.Message{
					MsgType: message.MsgTypeGamepadConnectServerError,
					Error: &message.Error {
						Message: err.Error(),
					},
				}
				err = h.safeWriteMessage(conn, websocket.TextMessage, resMsg)
				if err != nil {
					log.Printf("can not write gpConnectSrvErr message: %v", err)
					return
				}
			})
		} else if msg.MsgType == message.MsgTypeGamepadState {
			if msg.GamepadState == nil ||
			   msg.GamepadState.DelivererId == "" ||
			   msg.GamepadState.ControllerId == "" ||
			   msg.GamepadState.GamepadId == "" {
				log.Printf("no gamepad state request parameter: %v", msg.GamepadState)
				continue
			}
			if msg.GamepadState.ControllerId != client.clientId {
				log.Printf("controller id mismatch: act %v, exp %v",
					msg.GamepadState.ControllerId, client.clientId)
				continue
			}
			if client.relationClient == nil ||
			   client.relationClient.delivererId != msg.GamepadState.DelivererId ||
			   client.relationClient.controllerId != msg.GamepadState.ControllerId ||
			   client.relationClient.gamepadId != msg.GamepadState.GamepadId {
				log.Printf("client relation mismatch: %v, %v",
					client.relationClient, msg.GamepadConnectRequest)
				continue
			}
			h.forwarder.ToTcp(&msg, nil)
		} else {
			log.Printf("unsupported request: %v", msg.MsgType)
		}
	}
}

func (h *HttpHandler) delivererWebsocket(c *gin.Context) {
	log.Printf("requested /delivererws")
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		Subprotocols: []string{"deliverer"},
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Failed to set websocket upgrade: %+v", err)
                c.AbortWithStatus(400)
		return
	}
	go h.websocketLoop(conn, message.ClientTypeDeliverer)
}


func (h *HttpHandler) controllerWebsocket(c *gin.Context) {
	log.Printf("requested /controllerws")
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		Subprotocols: []string{"controller"},
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Failed to set websocket upgrade: %+v", err)
                c.AbortWithStatus(400)
		return
	}
	go h.websocketLoop(conn, message.ClientTypeController)
}


func NewHttpHandler(resourcePath string, accounts map[string]string, clientsStore *ClientsStore, forwarder *Forwarder, opts ...HttpOption) (*HttpHandler, error) {
        baseOpts := defaultHttpOptions()
        for _, opt := range opts {
                if opt == nil {
                        continue
                }
                opt(baseOpts)
        }
	return &HttpHandler{
                verbose:          baseOpts.verbose,
                resourcePath:     resourcePath,
                accounts:         accounts,
		clientsStore:     clientsStore,
                forwarder:        forwarder,
		clients:          make(map[*websocket.Conn]*httpClient),
        }, nil
}
