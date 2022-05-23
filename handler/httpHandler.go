
package handler

import (
        "log"
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

type HttpHandler struct {
        verbose               bool
        resourcePath          string
        accounts              map[string]string
	forwarder             *Forwarder
	signalincClinetsMutex sync.Mutex
	signalincClinets      map[*websocket.Conn]string
}

func (h *HttpHandler) onFromTcp(msg string) {
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
	Message string
}

type signalingResponse struct {
	Command string
	Error string
	Results []string
}

func (h *HttpHandler) signalingClientRegister(conn *websocket.Conn, id string) {
	h.signalincClinetsMutex.Lock()
	defer h.signalincClinetsMutex.Unlock()
	h.signalincClinets[conn] = id
}

func (h *HttpHandler) signalingClientUnregister(conn *websocket.Conn) {
	h.signalincClinetsMutex.Lock()
	defer h.signalincClinetsMutex.Unlock()
	delete(h.signalincClinets, conn)
}

func (h *HttpHandler) getSignalingClients() []string {
	h.signalincClinetsMutex.Lock()
	defer h.signalincClinetsMutex.Unlock()
	clients := make([]string, 0, len(h.signalincClinets))
	for _, v := range h.signalincClinets {
		if v == "" {
			continue
		}
		clients = append(clients, v)
	}
	return clients
}

func (h *HttpHandler) StartPingLoop(conn *websocket.Conn, pingLoopStopChan chan int) {
	ticker := time.NewTicker(2 * time.Second)
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
			err = conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Printf("can not write message: %v", err)
				break
			}
		case <-pingLoopStopChan:
			return
		}
	}
}

func (h *HttpHandler) signalingLoop(conn *websocket.Conn) {
	defer conn.Close()
	h.signalingClientRegister(conn, "")
	defer h.signalingClientUnregister(conn)
	pingStopChan := make(chan int)
	go h.StartPingLoop(conn, pingStopChan)
	defer close(pingStopChan)
	for {
		t, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if t != websocket.TextMessage {
			log.Printf("unsupported message type: %v", t)
			continue
		}
		var req signalingRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			log.Printf("can not unmarshal message: %v", err)
			continue
		}
		if req.Command == "ping" {
			log.Printf("ping")
			continue
		} else if req.Command == "registerRequest" {
			h.signalingClientRegister(conn, req.Message)
			res := &signalingResponse{
				Command: "registerResponse",
				Error: "",
				Results: nil,
			}
			msg, err := json.Marshal(res)
			if err != nil {
				log.Printf("can not unmarshal to json: %v", err)
				continue
			}
			err = conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Printf("can not write message: %v", err)
				continue
			}
		} else if req.Command == "lookupRequest" {
			clients := h.getSignalingClients()
			res := &signalingResponse{
				Command: "lookupResponse",
				Error: "",
				Results: clients,
			}
			msg, err := json.Marshal(res)
			if err != nil {
				log.Printf("can not unmarshal to json: %v", err)
				continue
			}
			err = conn.WriteMessage(websocket.TextMessage, msg)
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

func (h *HttpHandler) gamepad(c *gin.Context) {
	log.Printf("requested gamepad")
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
                verbose:               baseOpts.verbose,
                resourcePath:          resourcePath,
                accounts:              accounts,
                forwarder:             forwarder,
		signalincClinets:      make(map[*websocket.Conn]string),
        }, nil
}
