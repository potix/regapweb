package handler

import (
        "log"
        "fmt"
        "net"
        "time"
        "bufio"
        "sync"
        "github.com/google/uuid"
        "crypto/sha256"
        "encoding/json"
	"github.com/potix/regapweb/message"
)

type tcpOptions struct {
        verbose bool
}

func defaultTcpOptions() *tcpOptions {
        return &tcpOptions {
                verbose: false,
        }
}

type TcpOption func(*tcpOptions)

func TcpVerbose(verbose bool) TcpOption {
        return func(opts *tcpOptions) {
                opts.verbose = verbose
        }
}

type tcpClient struct {
        gamepadId string
}

type TcpHandler struct {
        verbose          bool
        digest           string
	clientsStore     *ClientsStore
        forwarder        *Forwarder
	tcpClientsMutex  sync.Mutex
        tcpClients       map[net.Conn]*tcpClient
}

func (t *TcpHandler) onFromWs(msg *message.Message) error {
	log.Printf("onFromWs")
	if msg.MsgType == message.MsgTypeGamepadConnectReq {
		conn := t.getClientConn(msg.GamepadConnectRequest.GamepadId)
		if conn == nil {
			log.Printf("can not find client connection: gamepadId = %v", msg.GamepadConnectRequest.GamepadId)
			return fmt.Errorf("can not find client connection")
		}
		err := t.writeMessage(conn, msg)
		if err != nil {
			log.Printf("can not write gamepad connect message: %v", err)
			return fmt.Errorf("can not find client connection")
		}
	} else if msg.MsgType == message.MsgTypeGamepadState {
		conn := t.getClientConn(msg.GamepadState.GamepadId)
		if conn == nil {
			log.Printf("can not find client connection: gamepadId = %v", msg.GamepadState.GamepadId)
			return nil
		}
		err := t.writeMessage(conn, msg)
		if err != nil {
			log.Printf("can not write gamepad state message: %v", err)
			return nil
		}
	} else {
		log.Printf("unsupported  message: %v",  msg.MsgType)
		return nil
	}
	return nil
}

func (t *TcpHandler) Start() error {
        t.forwarder.StartFromWsListener(t.onFromWs)
	return nil
}

func (t *TcpHandler) Stop() {
        t.forwarder.StopFromWsListener()
}

func (t *TcpHandler) clientRegister(conn net.Conn, gamepadId string) {
        t.tcpClientsMutex.Lock()
        defer t.tcpClientsMutex.Unlock()
        t.tcpClients[conn] = &tcpClient {
		gamepadId: gamepadId,
	}
	if t.verbose {
		log.Printf("register gamepad client: conn = %p, id = %v", conn, gamepadId)
	}
}

func (t *TcpHandler) clientUnregister(conn net.Conn) {
        t.tcpClientsMutex.Lock()
        defer t.tcpClientsMutex.Unlock()
	if t.verbose {
		log.Printf("unregister gamepad client: conn = %p", conn)
	}
        delete(t.tcpClients, conn)
}

func (t *TcpHandler) getClientConn(gamepadId string) net.Conn {
        t.tcpClientsMutex.Lock()
        defer t.tcpClientsMutex.Unlock()
	for k, v := range t.tcpClients {
		if v.gamepadId == gamepadId {
			return k
		}
	}
        return nil
}

func (t *TcpHandler) writeMessage(conn net.Conn, msg *message.Message) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("can not marshal to json for tcp: %w", err)
	}
	msgBytes = append(msgBytes, byte('\n'))
	_, err = conn.Write(msgBytes)
	if err != nil {
		return fmt.Errorf("can not write to tcp: %w", err)
	}
	return nil
}

func (t *TcpHandler) startPingLoop(conn net.Conn, pingLoopStopChan chan int) {
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()
        for {
                select {
                case <-ticker.C:
                        msg := &message.Message{
                                MsgType: "ping",
                        }
			err := t.writeMessage(conn, msg)
                        if err != nil {
				log.Printf("can not write ping message: %v", err)
				return
                        }
                case <-pingLoopStopChan:
                        return
                }
        }
}

func (t *TcpHandler) handshake(conn net.Conn, gamepadId string) (error) {
        msgBytes := make([]byte, 0, 2048)
        rbufio := bufio.NewReader(conn)
        for {
		err := conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			return fmt.Errorf("can not set read deadline: %w", err)
		}
                patialMsgBytes, isPrefix, err := rbufio.ReadLine()
                if err != nil {
			return fmt.Errorf("can not read gpHandshakeRquest: %w", err)
                } else if isPrefix {
                        // patial message
                        msgBytes = append(msgBytes, patialMsgBytes...)
                        continue
                } else {
                        // entire message
                        msgBytes = append(msgBytes, patialMsgBytes...)
                        var msg message.Message
                        if err = json.Unmarshal(msgBytes, &msg); err != nil {
                                msgBytes = msgBytes[:0]
                                return fmt.Errorf("can not unmarshal message: %w", err)
                        }
                        msgBytes = msgBytes[:0]
                        if msg.MsgType != message.MsgTypeGamepadHandshakeReq {
				return fmt.Errorf("recieved invalid message: %w", msg.MsgType)
			}
			if msg.GamepadHandshakeRequest == nil ||
			   msg.GamepadHandshakeRequest.Digest == "" {
				resMsg := &message.Message{
					MsgType: message.MsgTypeGamepadHandshakeRes,
					Error: &message.Error{
						Message: "no request parameter",
					},
				}
				err = t.writeMessage(conn, resMsg)
				if err != nil {
					return fmt.Errorf("can not write gpHandshakeRes: %w", err)
				}
				return fmt.Errorf("no request parameter: %v", msg.GamepadHandshakeRequest)
			}
			if msg.GamepadHandshakeRequest.Digest != t.digest {
				resMsg := &message.Message{
					MsgType: message.MsgTypeGamepadHandshakeRes,
					Error: &message.Error{
						Message: "digest mismatch",
					},
				}
				err = t.writeMessage(conn, resMsg)
				if err != nil {
					return fmt.Errorf("can not write gpHandshakeRes: %w", err)
				}
				return fmt.Errorf("digest mismatch: act: %v, exp: %v", msg.GamepadHandshakeRequest.Digest, t.digest)
			}
			// create gamepad id
		        resMsg := &message.Message{
			        MsgType: message.MsgTypeGamepadHandshakeRes,
				GamepadHandshakeResponse: &message.GamepadHandshakeResponse{
					GamepadId: gamepadId,
				},
			}
			err = t.writeMessage(conn, resMsg)
		        if err != nil {
				return fmt.Errorf("can not write gpHandshakeRes: %w", err)
			}
			t.clientsStore.AddGamepad(gamepadId, msg.GamepadHandshakeRequest.Name)
			return nil
		}
	}
}

func (t *TcpHandler) OnAccept(conn net.Conn) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		log.Printf("can not create gamepad id")
		return
	}
	gamepadId := uuid.String()
	t.clientRegister(conn, gamepadId)
	defer t.clientUnregister(conn)
	if t.verbose {
		log.Printf("start handshake")
	}
	err = t.handshake(conn, gamepadId)
	if err != nil {
		log.Printf("can not handshakea: %v", err)
		return
	}
	if t.verbose {
		log.Printf("end handshake")
	}
	defer t.clientsStore.DeleteGamepad(gamepadId)
	conn.SetDeadline(time.Time{})
	pingStopChan := make(chan int)
        go t.startPingLoop(conn, pingStopChan)
	defer close(pingStopChan)
        msgBytes := make([]byte, 0, 2048)
        rbufio := bufio.NewReader(conn)
        for {
                patialMsgBytes, isPrefix, err := rbufio.ReadLine()
                if err != nil {
			log.Printf("can not read message: %v", err)
			return
                } else if isPrefix {
                        // patial message
                        msgBytes = append(msgBytes, patialMsgBytes...)
                        continue
                } else {
                        // entire message
                        msgBytes = append(msgBytes, patialMsgBytes...)
			var msg message.Message
                        if err := json.Unmarshal(msgBytes, &msg); err != nil {
                                log.Printf("can not unmarshal message: %v, %v", string(msgBytes), err)
                                msgBytes = msgBytes[:0]
                                continue
                        }
                        msgBytes = msgBytes[:0]
                        if msg.MsgType == message.MsgTypePing {
				if t.verbose {
					log.Printf("recieved ping")
				}
                                continue
                        } else if msg.MsgType == message.MsgTypeGamepadConnectRes {
				if msg.GamepadConnectResponse == nil ||
				   msg.GamepadConnectResponse.GamepadId == "" ||
				   msg.GamepadConnectResponse.DelivererId == "" ||
				   msg.GamepadConnectResponse.ControllerId == "" {
					log.Printf("no gamepad connect response parameter: %v",  msg.GamepadConnectResponse)
					resMsg := &message.Message{
						MsgType: message.MsgTypeGamepadConnectServerError,
						Error: &message.Error {
							Message: "no gamepad connect response parameter",
						},
					}
					err = t.writeMessage(conn, resMsg)
					if err != nil {
						log.Printf("can not write gpConnectSrvErr message")
						return
					}
					continue
				}
				if msg.GamepadConnectResponse.GamepadId != gamepadId {
					log.Printf("gamepad id is mismatch: act %v, exp %v",  msg.GamepadConnectResponse.GamepadId, gamepadId)
					resMsg := &message.Message{
						MsgType: message.MsgTypeGamepadConnectServerError,
						Error: &message.Error {
							Message: "gamepad id is mismatch",
						},
					}
					err = t.writeMessage(conn, resMsg)
					if err != nil {
						log.Printf("can not write gpConnectSrvErr message")
						return
					}
					continue
				}
				t.forwarder.ToWs(&msg, func(err error){
					log.Printf("error callback %v",  err)
					resMsg := &message.Message{
						MsgType: message.MsgTypeGamepadConnectServerError,
						Error: &message.Error {
							Message: err.Error(),
						},
					}
					err = t.writeMessage(conn, resMsg)
					if err != nil {
						log.Printf("can not write gpConnectSrvErr message")
						return
					}
				})
                        } else if msg.MsgType == message.MsgTypeGamepadVibration {
				if msg.GamepadVibration == nil ||
				   msg.GamepadVibration.GamepadId == "" ||
				   msg.GamepadVibration.DelivererId == "" ||
				   msg.GamepadVibration.ControllerId == "" {
					log.Printf("no gamepad vibration parameter: %v",  msg.GamepadVibration)
					continue
				}
				if msg.GamepadVibration.GamepadId != gamepadId {
					log.Printf("gamepad id is mismatch: act %v, exp %v",  msg.GamepadVibration.GamepadId, gamepadId)
					continue
				}
				t.forwarder.ToWs(&msg, nil)
                        } else {
				log.Printf("unsupportede message: %v", msg.MsgType)
			}
                }
        }
}

func NewTcpHandler(secret string, clientsStore *ClientsStore, forwarder *Forwarder, opts ...TcpOption) (*TcpHandler, error) {
        baseOpts := defaultTcpOptions()
        for _, opt := range opts {
                if opt == nil {
                        continue
                }
                opt(baseOpts)
        }
	sha := sha256.New()
	digest := fmt.Sprintf("%x", sha.Sum([]byte(secret)))
        return &TcpHandler{
                verbose:      baseOpts.verbose,
                digest:       digest,
                clientsStore: clientsStore,
                forwarder:    forwarder,
		tcpClients:   make(map[net.Conn]*tcpClient),
        }, nil
}

