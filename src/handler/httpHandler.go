
package handler

import (
        "log"
        "path"
        "net/http"
        "github.com/gin-gonic/gin"
)

type HttpHandler struct {
        verbose          bool
        resourcePath     string
        accounts         map[string]string
	forwarder        *forwarder
}

func (h *HttpHander) onFromTcp(msg string)

func (h *HttpHandler) Start() error {
	forwarder.StartFromTcpListener(h.onFromTcp)
}

func (h *HttpHandler) Stop() {
	forwarder.StopFromTcpListener()
}

func (h *HttpHandler) SetRouting(router *gin.Engine) {
	favicon := path.Join(h.resourcePath, "icon", "favicon.ico")
        js := path.Join(h.resourcePath, "js")
        css := path.Join(h.resourcePath, "css")
        img := path.Join(h.resourcePath, "img")
        font := path.Join(h.resourcePath, "font")
	templatePath := path.Join(h.resourcePath, "template", "*")
	authGroup := r.Group("/", gin.BasicAuth(h.accounts))
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

func (h *Handler) indexHtml(c *gin.Context) {
        c.HTML(http.StatusOK, "controller.tmpl", gin.H{})
}

func (h *Handler) deliveryHtml(c *gin.Context) {
        c.HTML(http.StatusOK, "delivery.tmpl", gin.H{})
}

func (h *Handler) webrtc(c *gin.Context) {
	log.Printf("requested webrtc")
}

func (h *Handler) gamepad(c *gin.Context) {
	log.Printf("requested gamepad")
}

func NewHttpHandler(resourcePath string, accounts map[string]string, forwarder *forwarder, opts ...Option) (*HttpHandler, error) {
        baseOpts := defaultHttpOptions()
        for _, opt := range opts {
                if opt == nil {
                        continue
                }
                opt(baseOpts)
        }
	return &Handler{
                verbose: baseOpts.verbose,
                resourcePath: resourcePath,
                accounts: accounts,
                forwarder: forwarder,
        }, nil
}
