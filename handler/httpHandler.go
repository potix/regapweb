
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

type signalingClient struct {
	writeMutex sync.Mutex
	uid        string
}

type gamepadClient struct {
	writeMutex sync.Mutex
	uid        string
	peerUid    string
}

type HttpHandler struct {
        verbose               bool
        resourcePath          string
        accounts              map[string]string
	forwarder             *Forwarder
	signalingClientsMutex sync.Mutex
	signalingClients      map[*websocket.Conn]*signalingClient
	gamepadClientsMutex   sync.Mutex
	gamepadClients        map[*websocket.Conn]*gamepadClient
	remoteGamepadsMutex   sync.Mutex
	remoteGamepads        map[string] int
}

func (h *HttpHandler) remoteGamepadRegister(remoteGamepadId string) {
	h.remoteGamepadsMutex.Lock()
	defer h.remoteGamepadsMutex.Unlock()
	h.remoteGamepads[remoteGamepadId] = 1
}

func (h *HttpHandler) remoteGamepadUnregister(remoteGamepadId string) {
	h.remoteGamepadsMutex.Lock()
	defer h.remoteGamepadsMutex.Unlock()
	delete(h.remoteGamepads, remoteGamepadId)
}

func (h *HttpHandler) getRemoteGamepads() []string {
	h.remoteGamepadsMutex.Lock()
	defer h.remoteGamepadsMutex.Unlock()
	remoteGamepads := make([]string, 0, len(h.remoteGamepads))
	for k, _ := range h.remoteGamepads {
		remoteGamepads = append(remoteGamepads, k)
	}
	return remoteGamepads
}

func (h *HttpHandler) onFromTcp(msg *GamepadMessage) {
	log.Printf("onFromTcp")
	if msg.Command == "registerRequest" {
		if msg.RemoteGamepadId == "" {
			 log.Printf("no remote gamepad id in registerRequest")
			 return
		}
		h.remoteGamepadRegister(msg.RemoteGamepadId)
	} else if msg.Command == "unregisterRequest" {
		if msg.RemoteGamepadId == "" {
			 log.Printf("no remote gamepad id in unregisterRequest")
			 return
		}
		h.remoteGamepadUnregister(msg.RemoteGamepadId)
	} else if msg.Command == "vibrationRequest" {
		conn := h.getGamepadClientConn(msg.Uid, msg.PeerUid)
		if conn != nil {
			log.Printf("not found connection for gamepadVibrationRequest: uid = %v, peer uid = %v",
			    msg.Uid, msg.PeerUid)
			return
		}
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			log.Printf("can not marshal to json: %v", err)
			return
		}
		err = h.safeGamepadWriteMessage(conn, websocket.TextMessage, msgBytes)
		if err != nil {
			log.Printf("can not write message: %v", err)
			return
		}
	} else {
		log.Printf("unsupported request: %v", msg.Command)
	}
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
	authGroup.GET("/delivery.html", h.deliveryHtml)
	authGroup.GET("/webrtc", h.webrtc)
	authGroup.GET("/gamepad", h.gamepad)
	authGroup.StaticFile("/favicon.ico", favicon)
        authGroup.Static("/js", js)
        authGroup.Static("/css", css)
        authGroup.Static("/img", img)
        authGroup.Static("/font", font)
}

func (h *HttpHandler) indexHtml(c *gin.Context) {
	uuid, err := uuid.NewRandom()
        if err != nil {
                c.AbortWithStatus(500)
                return
        }
	c.HTML(http.StatusOK, "controller.html", gin.H{
		"uid": uuid.String(),
	})
}

func (h *HttpHandler) deliveryHtml(c *gin.Context) {
        uuid, err := uuid.NewRandom()
        if err != nil {
                c.AbortWithStatus(500)
                return
        }
	c.HTML(http.StatusOK, "delivery.html", gin.H{
		"uid": uuid.String(),
	})
}

type CommonMessage struct {
	Command string
}

type signalingRequest struct {
	*CommonMessage
	Messages []string
}

type signalingResponse struct {
	*CommonMessage
	Error string
	Results []string
}

type writeMessageFunc  func(conn *websocket.Conn, messageType int, message []byte) error

func (h *HttpHandler) startPingLoop(conn *websocket.Conn, pingLoopStopChan chan int, writeMessage writeMessageFunc) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			msg := &CommonMessage{
				Command: "ping",
			}
			msgBytes, err := json.Marshal(msg)
			if err != nil {
				log.Printf("can not marshal to json: %v", err)
				break
			}
			err = writeMessage(conn, websocket.TextMessage, msgBytes)
			if err != nil {
				log.Printf("can not write ping message: %v", err)
				break
			}
		case <-pingLoopStopChan:
			return
		}
	}
}

func (h *HttpHandler) signalingClientRegister(conn *websocket.Conn) {
	h.signalingClientsMutex.Lock()
	defer h.signalingClientsMutex.Unlock()
	h.signalingClients[conn] = new(signalingClient)
}

func (h *HttpHandler) signalingClientUnregister(conn *websocket.Conn) {
	h.signalingClientsMutex.Lock()
	defer h.signalingClientsMutex.Unlock()
	delete(h.signalingClients, conn)
}

func (h *HttpHandler) signalingClientUpdate(conn *websocket.Conn, uid string) {
	h.signalingClientsMutex.Lock()
	client := h.signalingClients[conn]
	h.signalingClientsMutex.Unlock()
	client.uid = uid
}

func (h *HttpHandler) getSignalingClients(ignoreUid string) []string {
	h.signalingClientsMutex.Lock()
	defer h.signalingClientsMutex.Unlock()
	clients := make([]string, 0, len(h.signalingClients))
	for _, v := range h.signalingClients {
		if v.uid == "" || v.uid == ignoreUid {
			continue
		}
		clients = append(clients, v.uid)
	}
	return clients
}

func (h *HttpHandler) getSignalingClientConn(uid string) *websocket.Conn{
	h.signalingClientsMutex.Lock()
	defer h.signalingClientsMutex.Unlock()
	for k, v := range h.signalingClients {
		if v.uid == uid {
			return k
		}
	}
	return nil
}

func (h *HttpHandler) findPairSignalingClient(uid string, peerUid string) bool {
	if uid == "" || peerUid == "" {
		return false
	}
	h.signalingClientsMutex.Lock()
	defer h.signalingClientsMutex.Unlock()
	found := 0
	for _, v := range h.signalingClients {
		if v.uid == uid {
			found += 1
		} else if v.uid == peerUid {
			found += 1
		}
	}
	if found < 2 {
		return false
	}
	return true
}

func (h *HttpHandler) safeSignalingWriteMessage(conn *websocket.Conn, messageType int, message []byte) error {
	h.signalingClientsMutex.Lock()
	client := h.signalingClients[conn]
	h.signalingClientsMutex.Unlock()
	client.writeMutex.Lock()
	defer client.writeMutex.Unlock()
	return conn.WriteMessage(messageType, message)
}

func (h *HttpHandler) signalingLoop(conn *websocket.Conn) {
	h.signalingClientRegister(conn)
	defer h.signalingClientUnregister(conn)
	defer conn.Close()
	pingStopChan := make(chan int)
	go h.startPingLoop(conn, pingStopChan, h.safeSignalingWriteMessage)
	defer close(pingStopChan)
	for {
		t, reqMsg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if t != websocket.TextMessage {
			log.Printf("unsupported message type: %v", t)
			continue
		}
		var req signalingRequest
		if err := json.Unmarshal(reqMsg, &req); err != nil {
			log.Printf("can not unmarshal message: %v", err)
			continue
		}
		if req.Command == "ping" {
			continue
		} else if req.Command == "registerRequest" {
			errMsg := ""
			if req.Messages != nil && len(req.Messages) == 1 {
				h.signalingClientUpdate(conn, req.Messages[0])
			} else if req.Messages != nil && len(req.Messages) > 1 {
				errMsg = fmt.Sprintf("too many user id: %v", req.Messages)
			} else {
				errMsg = fmt.Sprintf("no user id: %v", req.Messages)
			}
			res := &signalingResponse{
				CommonMessage: &CommonMessage{ Command: "registerResponse" },
				Error: errMsg,
				Results: nil,
			}
			resMsg, err := json.Marshal(res)
			if err != nil {
				log.Printf("can not marshal to json: %v", err)
				continue
			}
			err = h.safeSignalingWriteMessage(conn, websocket.TextMessage, resMsg)
			if err != nil {
				log.Printf("can not write message: %v", err)
				continue
			}
		} else if req.Command == "lookupClientsRequest" {
			ignoreUid := ""
			errMsg := ""
			var clients []string = nil
			if req.Messages == nil || len(req.Messages) == 0 {
				clients = h.getSignalingClients(ignoreUid)
			} else if len(req.Messages) == 1 {
				ignoreUid = req.Messages[0]
				clients = h.getSignalingClients(ignoreUid)
			} else if len(req.Messages) > 1 {
				errMsg = fmt.Sprintf("too many user id: %v", req.Messages)
			}
			res := &signalingResponse{
				CommonMessage: &CommonMessage{ Command: "lookupClientsResponse" },
				Error: errMsg,
				Results: clients,
			}
			resMsg, err := json.Marshal(res)
			if err != nil {
				log.Printf("can not marshal to json: %v", err)
				continue
			}
			err = h.safeSignalingWriteMessage(conn, websocket.TextMessage, resMsg)
			if err != nil {
				log.Printf("can not write message: %v", err)
				continue
			}
		} else if req.Command == "sendOfferSdpRequest" {
			errMsg := ""
			if req.Messages != nil && len(req.Messages) != 3 {
				errMsg = fmt.Sprintf("invalid Message: %v", req.Messages)
			} else {
				foundConn := h.getSignalingClientConn(req.Messages[0])
				if foundConn == nil {
					errMsg = fmt.Sprintf("not found uid: %v", req.Messages)
				} else {
					err := h.safeSignalingWriteMessage(foundConn, websocket.TextMessage, reqMsg)
					if err != nil {
						errMsg = fmt.Sprintf("can not forward message: %v", req.Messages)
					}
				}
			}
			res := &signalingResponse{
				CommonMessage: &CommonMessage{ Command: "sendOfferSdpResponse" },
				Error: errMsg,
				Results: nil,
			}
			resMsg, err := json.Marshal(res)
			if err != nil {
				log.Printf("can not marshal to json: %v", err)
				continue
			}
			err = h.safeSignalingWriteMessage(conn,websocket.TextMessage, resMsg)
			if err != nil {
				log.Printf("can not write message: %v", err)
				continue
			}
		} else if req.Command == "sendAnswerSdpRequest" {
			errMsg := ""
			if req.Messages != nil && len(req.Messages) != 3 {
				errMsg = fmt.Sprintf("invalid Message: %v", req.Messages)
			} else {
				foundConn := h.getSignalingClientConn(req.Messages[0])
				if foundConn == nil {
					errMsg = fmt.Sprintf("not found uid: %v", req.Messages)
				} else {
					err := h.safeSignalingWriteMessage(foundConn, websocket.TextMessage, reqMsg)
					if err != nil {
						errMsg = fmt.Sprintf("can not forward message: %v", req.Messages)
					}
				}
			}
			res := &signalingResponse{
				CommonMessage: &CommonMessage{ Command: "sendAnswerSdpResponse" },
				Error: errMsg,
				Results: nil,
			}
			resMsg, err := json.Marshal(res)
			if err != nil {
				log.Printf("can not marshal to json: %v", err)
				continue
			}
			err = h.safeSignalingWriteMessage(conn, websocket.TextMessage, resMsg)
			if err != nil {
				log.Printf("can not write message: %v", err)
				continue
			}
		} else if req.Command == "lookupRemoteGamepadsRequest" {
			errMsg := ""
			var remoteGamepads []string
			if req.Messages == nil || len(req.Messages) == 0 {
				remoteGamepads = h.getRemoteGamepads()
			} else if len(req.Messages) > 1 {
				errMsg = fmt.Sprintf("too many parameter: %v", req.Messages)
			}
			res := &signalingResponse{
				CommonMessage: &CommonMessage{ Command: "lookupRemoteGamepadsResponse" },
				Error: errMsg,
				Results: remoteGamepads,
			}
			resMsg, err := json.Marshal(res)
			if err != nil {
				log.Printf("can not marshal to json: %v", err)
				continue
			}
			err = h.safeSignalingWriteMessage(conn, websocket.TextMessage, resMsg)
			if err != nil {
				log.Printf("can not write message: %v", err)
				continue
			}
		} else if req.Command == "setupRemoteGamepadRequest" {
			errMsg := ""
			if req.Messages != nil && len(req.Messages) != 3 {
				errMsg = fmt.Sprintf("invalid Message: %v", req.Messages)
			} else {
				foundConn := h.getSignalingClientConn(req.Messages[0])
				if foundConn == nil {
					errMsg = fmt.Sprintf("not found uid: %v", req.Messages)
				} else {
					err := h.safeSignalingWriteMessage(foundConn, websocket.TextMessage, reqMsg)
					if err != nil {
						errMsg = fmt.Sprintf("can not forward message: %v", req.Messages)
					}
				}
			}
			res := &signalingResponse{
				CommonMessage: &CommonMessage{ Command: "setupRemoteGamepadResponse" },
				Error: errMsg,
			}
			resMsg, err := json.Marshal(res)
			if err != nil {
				log.Printf("can not marshal to json: %v", err)
				continue
			}
			err = h.safeSignalingWriteMessage(conn,websocket.TextMessage, resMsg)
			if err != nil {
				log.Printf("can not write message: %v", err)
				continue
			}
		} else {
			log.Printf("unsupported request: %v", req.Command)
		}
	}
}

func (h *HttpHandler) webrtc(c *gin.Context) {
	log.Printf("requested /webrtc")
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		Subprotocols: []string{"signaling"},
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Failed to set websocket upgrade: %+v", err)
                c.AbortWithStatus(400)
		return
	}
	go h.signalingLoop(conn)
}

type GamepadVibrationMessage struct {
        Duration        float64
        StartDelay      float64
        StrongMagnitude float64
        WeakMagnitude   float64
}

type GamepadButtonMessage struct {
        Pressed bool
        Touched bool
        Value   float64
}

type GamepadStateMessage struct {
        Buttons []*GamepadButtonMessage
        Axes    []float64
}

type GamepadMessage struct {
	*CommonMessage
	Error          string
	Uid            string
	PeerUid        string
	RemoteGamepadId string
	State          *GamepadStateMessage
        Vibration      *GamepadVibrationMessage
}

func (h *HttpHandler) gamepadClientRegister(conn *websocket.Conn) {
	h.gamepadClientsMutex.Lock()
	defer h.gamepadClientsMutex.Unlock()
	h.gamepadClients[conn] = new(gamepadClient)
}

func (h *HttpHandler) gamepadClientUnregister(conn *websocket.Conn) {
	h.gamepadClientsMutex.Lock()
	defer h.gamepadClientsMutex.Unlock()
	delete(h.gamepadClients, conn)
}

func (h *HttpHandler) gamepadClientUpdate(conn *websocket.Conn, uid string, peerUid string) {
	h.gamepadClientsMutex.Lock()
	client := h.gamepadClients[conn]
	h.gamepadClientsMutex.Unlock()
	client.uid = uid
	client.peerUid = peerUid
}

func (h *HttpHandler) checkIdsGamepadClient(conn *websocket.Conn, uid string, peerUid string) bool {
	if uid == "" || peerUid == "" {
		return false
	}
	h.gamepadClientsMutex.Lock()
	client := h.gamepadClients[conn]
	h.gamepadClientsMutex.Unlock()
	if client.uid == uid && client.peerUid == peerUid {
		return true
	}
	return false
}

func (h *HttpHandler) getGamepadClientConn(uid string, peerUid string) *websocket.Conn {
	if uid == "" || peerUid == "" {
		return nil
	}
	h.gamepadClientsMutex.Lock()
	defer h.gamepadClientsMutex.Unlock()
	for k, v := range h.gamepadClients {
		if v.uid == uid && v.peerUid == peerUid {
			return k
		}
	}
	return nil
}

func (h *HttpHandler) safeGamepadWriteMessage(conn *websocket.Conn, messageType int, message []byte) error {
	h.gamepadClientsMutex.Lock()
	client := h.gamepadClients[conn]
	h.gamepadClientsMutex.Unlock()
	client.writeMutex.Lock()
	defer client.writeMutex.Unlock()
	return conn.WriteMessage(messageType, message)
}

func (h *HttpHandler) gamepadLoop(conn *websocket.Conn) {
	h.gamepadClientRegister(conn)
	defer h.gamepadClientUnregister(conn)
	defer conn.Close()
	pingStopChan := make(chan int)
	go h.startPingLoop(conn, pingStopChan, h.safeGamepadWriteMessage)
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
		var msg GamepadMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			log.Printf("can not unmarshal message: %v, %v", string(msgBytes), err)
			continue
		}
		if msg.Command == "ping" {
			continue
		} else if msg.Command == "notify" {
			if h.findPairSignalingClient(msg.Uid, msg.PeerUid) {
				h.gamepadClientUpdate(conn, msg.Uid, msg.PeerUid)
			} else {
				log.Printf("invalid notify request: %v", string(msgBytes))
			}
		} else if msg.Command == "stateRequest" {
			errMsg := ""
			if h.checkIdsGamepadClient(conn, msg.Uid, msg.PeerUid) && msg.RemoteGamepadId != "" &&
			    msg.State.Buttons != nil && len(msg.State.Buttons) != 0 &&
			    msg.State.Axes != nil && len(msg.State.Axes) != 0 {
				h.forwarder.ToTcp(&msg)
			} else {
				errMsg = fmt.Sprintf("invalid gamepadRequest: %v", string(msgBytes))
			}
			res := &GamepadMessage{
				CommonMessage: &CommonMessage{ Command: "stateResponse" },
				Error: errMsg,
			}
			resMsg, err := json.Marshal(res)
			if err != nil {
				log.Printf("can not unmarshal to json: %v", err)
				continue
			}
			err = h.safeGamepadWriteMessage(conn, websocket.TextMessage, resMsg)
			if err != nil {
				log.Printf("can not write message: %v", err)
				continue
			}
		} else {
			 log.Printf("unsupported message: %v", msg.Command)
		}
	}
}

func (h *HttpHandler) gamepad(c *gin.Context) {
	log.Printf("requested /gamepad")
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		Subprotocols: []string{"gamepad"},
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Failed to set websocket upgrade: %+v", err)
                c.AbortWithStatus(400)
		return
	}
	go h.gamepadLoop(conn)
}

func NewHttpHandler(resourcePath string, accounts map[string]string, forwarder *Forwarder, opts ...HttpOption) (*HttpHandler, error) {
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
                forwarder:        forwarder,
		signalingClients: make(map[*websocket.Conn]*signalingClient),
		gamepadClients:   make(map[*websocket.Conn]*gamepadClient),
		remoteGamepads:   make(map[string] int),
        }, nil
}
