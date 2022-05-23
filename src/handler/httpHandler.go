
package handler

import (
        "log"
        "path"
        "net/http"
        "github.com/gin-gonic/gin"
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
        verbose      bool
        resourcePath string
        accounts     map[string]string
	forwarder    *Forwarder
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
        c.HTML(http.StatusOK, "controller.tmpl", gin.H{})
}

func (h *HttpHandler) deliveryHtml(c *gin.Context) {
        c.HTML(http.StatusOK, "delivery.tmpl", gin.H{})
}

func (h *HttpHandler) webrtc(c *gin.Context) {
	log.Printf("requested webrtc")
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
                verbose: baseOpts.verbose,
                resourcePath: resourcePath,
                accounts: accounts,
                forwarder: forwarder,
        }, nil
}
