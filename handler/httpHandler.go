
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
}

type HttpHandler struct {
        verbose               bool
        resourcePath          string
        accounts              map[string]string
	forwarder             *Forwarder
	signalincClientsMutex sync.Mutex
	signalincClients      map[*websocket.Conn]*signalingClient
	gamepadClientsMutex   sync.Mutex
	gamepadClients        map[*websocket.Conn]*gamepadClient
}

func (h *HttpHandler) onFromTcp(msg []byte) {
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

type commonRequest struct {
	Command string
}

type signalingRequest struct {
	commonRequest
	Messages []string
}

type signalingResponse struct {
	Command string
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
			req := &commonRequest{
				Command: "ping",
			}
			msg, err := json.Marshal(req)
			if err != nil {
				log.Printf("can not unmarshal to json: %v", err)
				break
			}
			err = writeMessage(conn, websocket.TextMessage, msg)
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
	h.signalincClientsMutex.Lock()
	defer h.signalincClientsMutex.Unlock()
	h.signalincClients[conn] = new(signalingClient)
}

func (h *HttpHandler) signalingClientUnregister(conn *websocket.Conn) {
	h.signalincClientsMutex.Lock()
	defer h.signalincClientsMutex.Unlock()
	delete(h.signalincClients, conn)
}

func (h *HttpHandler) signalingClientUpdate(conn *websocket.Conn, uid string) {
	h.signalincClientsMutex.Lock()
	client := h.signalincClients[conn]
	h.signalincClientsMutex.Unlock()
	client.uid = uid
}

func (h *HttpHandler) getSignalingClients(ignoreUid string) []string {
	h.signalincClientsMutex.Lock()
	defer h.signalincClientsMutex.Unlock()
	clients := make([]string, 0, len(h.signalincClients))
	for _, v := range h.signalincClients {
		if v.uid == "" || v.uid == ignoreUid {
			continue
		}
		clients = append(clients, v.uid)
	}
	return clients
}

func (h *HttpHandler) getSignalingClientConn(uid string) *websocket.Conn{
	h.signalincClientsMutex.Lock()
	defer h.signalincClientsMutex.Unlock()
	for k, v := range h.signalincClients {
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
	h.signalincClientsMutex.Lock()
	defer h.signalincClientsMutex.Unlock()
	found := 0
	for _, v := range h.signalincClients {
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
	h.signalincClientsMutex.Lock()
	client := h.signalincClients[conn]
	h.signalincClientsMutex.Unlock()
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
			if len(req.Messages) == 1 {
				h.signalingClientUpdate(conn, req.Messages[0])
			} else if len(req.Messages) > 1 {
				errMsg = fmt.Sprintf("too many user id: %v", req.Messages)
			} else {
				errMsg = fmt.Sprintf("no user id: %v", req.Messages)
			}
			res := &signalingResponse{
				Command: "registerResponse",
				Error: errMsg,
				Results: nil,
			}
			resMsg, err := json.Marshal(res)
			if err != nil {
				log.Printf("can not unmarshal to json: %v", err)
				continue
			}
			err = h.safeSignalingWriteMessage(conn, websocket.TextMessage, resMsg)
			if err != nil {
				log.Printf("can not write message: %v", err)
				continue
			}
		} else if req.Command == "lookupRequest" {
			ignoreUid := ""
			errMsg := ""
			var clients []string = nil
			if len(req.Messages) == 0 {
				clients = h.getSignalingClients(ignoreUid)
			} else if len(req.Messages) == 1 {
				ignoreUid = req.Messages[0]
				clients = h.getSignalingClients(ignoreUid)
			} else if len(req.Messages) > 1 {
				errMsg = fmt.Sprintf("too many user id: %v", req.Messages)
			}
			res := &signalingResponse{
				Command: "lookupResponse",
				Error: errMsg,
				Results: clients,
			}
			resMsg, err := json.Marshal(res)
			if err != nil {
				log.Printf("can not unmarshal to json: %v", err)
				continue
			}
			err = h.safeSignalingWriteMessage(conn, websocket.TextMessage, resMsg)
			if err != nil {
				log.Printf("can not write message: %v", err)
				continue
			}
		} else if req.Command == "sendOfferSdpRequest" {
			errMsg := ""
			if len(req.Messages) != 3 {
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
				Command: "sendOfferSdpResponse",
				Error: errMsg,
				Results: nil,
			}
			resMsg, err := json.Marshal(res)
			if err != nil {
				log.Printf("can not unmarshal to json: %v", err)
				continue
			}
			err = h.safeSignalingWriteMessage(conn,websocket.TextMessage, resMsg)
			if err != nil {
				log.Printf("can not write message: %v", err)
				continue
			}
		} else if req.Command == "sendAnswerSdpRequest" {
			errMsg := ""
			if len(req.Messages) != 3 {
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
				Command: "sendAnswerSdpResponse",
				Error: errMsg,
				Results: nil,
			}
			resMsg, err := json.Marshal(res)
			if err != nil {
				log.Printf("can not unmarshal to json: %v", err)
				continue
			}
			err = h.safeSignalingWriteMessage(conn, websocket.TextMessage, resMsg)
			if err != nil {
				log.Printf("can not write message: %v", err)
				continue
			}
		}
	}
}

func (h *HttpHandler) webrtc(c *gin.Context) {
	log.Printf("requested webrtc")
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

type gamepadButton struct {
	Pressed bool
	Touched bool
	Value   int64
}

type gamepadRequest struct {
	commonRequest
	Uid     string
	PeerUid string
	Buttons []*gamepadButton
	Axes    []float64
}

type gamepadResponse struct {
	Command string
	Error string
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
		t, reqMsg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if t != websocket.TextMessage {
			log.Printf("unsupported message type: %v", t)
			continue
		}
		var req gamepadRequest
		if err := json.Unmarshal(reqMsg, &req); err != nil {
			log.Printf("can not unmarshal message: %v", err)
			continue
		}
		if req.Command == "ping" {
			continue
		} else if req.Command == "gamepadRequest" {
			errMsg := ""
			if h.findPairSignalingClient(req.Uid, req.PeerUid) &&
			    (req.Buttons != nil || len(req.Buttons) != 0) &&
			    (req.Axes != nil || len(req.Axes) != 0) {
				h.forwarder.ToTcp(reqMsg)
			} else {
				errMsg = fmt.Sprintf("invalid gamepadRequest: %v", string(reqMsg))
			}
			res := &gamepadResponse{
				Command: "gamepadResponse",
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
		}
	}
}

func (h *HttpHandler) gamepad(c *gin.Context) {
	log.Printf("requested gamepad")
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
		signalincClients: make(map[*websocket.Conn]*signalingClient),
		gamepadClients:   make(map[*websocket.Conn]*gamepadClient),
        }, nil
}
