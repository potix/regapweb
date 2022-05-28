package handler

import (
        "log"
        "fmt"
        "net"
        "time"
        "bufio"
	"io"
        "sync"
        "github.com/google/uuid"
        "crypto/sha256"
        "encoding/json"
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
        remoteGamepadId string
}

type TcpHandler struct {
        verbose          bool
        digest           string
        forwarder        *Forwarder
	tcpClientsMutex  sync.Mutex
        tcpClients       map[net.Conn]*tcpClient
}

func (t *TcpHandler) onFromWs(msg *GamepadMessage) {
	log.Printf("onFromWs")
	if msg.Command == "stateRequest" {
		conn := t.getTcpClientConn(msg.RemoteGamepadId)
		if conn == nil {
			log.Printf("can not find client connection: gamepadId = %v", msg.RemoteGamepadId)
			return
		}
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			log.Printf("can not marshal to json in communicationLoop: %v", err)
			return
		}
		msgBytes = append(msgBytes, byte('\n'))
		_, err = conn.Write(msgBytes)
		if err != nil {
			log.Printf("can not write gamepad message: %v", err)
			return
		}
	} else {
		log.Printf("unsupported  message: %v",  msg.Command)
		return
	}
}

func (t *TcpHandler) Start() error {
        t.forwarder.StartFromWsListener(t.onFromWs)
	return nil
}

func (t *TcpHandler) Stop() {
        t.forwarder.StopFromWsListener()
}

func (t *TcpHandler) tcpClientRegister(conn net.Conn, remoteGamepadId string) {
        t.tcpClientsMutex.Lock()
        defer t.tcpClientsMutex.Unlock()
        t.tcpClients[conn] = &tcpClient {
		remoteGamepadId: remoteGamepadId,
	}
}

func (t *TcpHandler) tcpClientUnregister(conn net.Conn) {
        t.tcpClientsMutex.Lock()
        defer t.tcpClientsMutex.Unlock()
        delete(t.tcpClients, conn)
}

func (t *TcpHandler) getTcpClientConn(remoteGamepadId string) net.Conn {
        t.tcpClientsMutex.Lock()
        defer t.tcpClientsMutex.Unlock()
	for k, v := range t.tcpClients {
		if v.remoteGamepadId == remoteGamepadId {
			return k
		}
	}
        return nil
}

func (t *TcpHandler) checkRemoteGamepadId(conn net.Conn, remoteGamepadId string) bool {
        t.tcpClientsMutex.Lock()
        defer t.tcpClientsMutex.Unlock()
	client := t.tcpClients[conn]
	if client.remoteGamepadId == remoteGamepadId {
		return true
	}
	return false
}

func (h *TcpHandler) startPingLoop(conn net.Conn, pingLoopStopChan chan int) {
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
			msgBytes = append(msgBytes, byte('\n'))
                        _, err = conn.Write(msgBytes)
                        if err != nil {
				if err == io.EOF {
					return
				} else {
					log.Printf("can not write ping message: %v", err)
				}
                        }
                case <-pingLoopStopChan:
                        return
                }
        }
}

type TcpClientRegisterRequest struct {
	CommonMessage
	Digest string
}

type TcpClientRegisterResponse struct {
	CommonMessage
	Error           string
	RemoteGamepadId string
}

func (t *TcpHandler) handshake(conn net.Conn) (string, error) {
	var remoteGamepadId string
        msgBytes := make([]byte, 0, 2048)
        rbufio := bufio.NewReader(conn)
        for {
		err := conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			return remoteGamepadId, fmt.Errorf("can not set read deadline: %w", err)
		}
                patialMsgBytes, isPrefix, err := rbufio.ReadLine()
                if err != nil {
			return remoteGamepadId, fmt.Errorf("can not read tcpClientRegisterRquest: %w", err)
                } else if isPrefix {
                        // patial message
                        msgBytes = append(msgBytes, patialMsgBytes...)
                        continue
                } else {
                        // entire message
                        msgBytes = append(msgBytes, patialMsgBytes...)
                        var msg TcpClientRegisterRequest
                        if err := json.Unmarshal(msgBytes, &msg); err != nil {
                                msgBytes = msgBytes[:0]
                                return remoteGamepadId, fmt.Errorf("can not unmarshal message: %w", err)
                        }
                        msgBytes = msgBytes[:0]
                        if msg.Command == "registerRequest" {
				if msg.Digest != t.digest {
					return remoteGamepadId, fmt.Errorf("digest mismatch: act: %v, exp: %v", msg.Digest, t.digest)
				}
				uuid, err := uuid.NewRandom()
				if err != nil {
					return remoteGamepadId, fmt.Errorf("can not create remote gamepad id: %w", err)
				}
				remoteGamepadId := uuid.String()
				forwardMsg := &GamepadMessage{
                                        CommonMessage: &CommonMessage{
                                                Command: "registerRequest",
                                        },
					RemoteGamepadId: remoteGamepadId,
				}
				t.forwarder.ToWs(forwardMsg)
				t.tcpClientRegister(conn, remoteGamepadId)
                                resMsg := &GamepadMessage{
                                        CommonMessage: &CommonMessage{
                                                Command: "registerResponse",
                                        },
					RemoteGamepadId: remoteGamepadId,
                                }
                                resMsgBytes, err := json.Marshal(resMsg)
                                if err != nil {
					return remoteGamepadId, fmt.Errorf("can not marshal to json for TcpClientRegisterResponse: %w", err)
                                }
				resMsgBytes = append(resMsgBytes, byte('\n'))
                                _, err = conn.Write(resMsgBytes)
                                if err != nil {
					return remoteGamepadId, fmt.Errorf("can not write TcpClientRegisterResponse: %w", err)
                                }
				return remoteGamepadId, nil
                        } else {
				return remoteGamepadId, fmt.Errorf("recieved invalid message: %w", msg.Command)
			}
		}
	}
}


func (t *TcpHandler) forwardUnregisterRequest(remoteGamepadId string) {
	forwardMsg := &GamepadMessage{
		CommonMessage: &CommonMessage{
			Command: "unregisterRequest",
		},
		RemoteGamepadId: remoteGamepadId,
	}
	t.forwarder.ToWs(forwardMsg)
}

func (t *TcpHandler) OnAccept(conn net.Conn) {
	log.Printf("on accept")
	remoteGamepadId, err := t.handshake(conn)
	if err != nil {
		log.Printf("can not handshakea: %v", err)
		return
	}
	defer t.forwardUnregisterRequest(remoteGamepadId)
	conn.SetDeadline(time.Time{})
	pingStopChan := make(chan int)
        go t.startPingLoop(conn, pingStopChan)
	defer close(pingStopChan)
        msgBytes := make([]byte, 0, 2048)
        rbufio := bufio.NewReader(conn)
        for {
                patialMsgBytes, isPrefix, err := rbufio.ReadLine()
                if err != nil {
                        if err == io.EOF {
                                return
                        } else {
				log.Printf("can not read message: %v", err)
                                continue
                        }
                } else if isPrefix {
                        // patial message
                        msgBytes = append(msgBytes, patialMsgBytes...)
                        continue
                } else {
                        // entire message
                        msgBytes = append(msgBytes, patialMsgBytes...)
                        var msg GamepadMessage
                        if err := json.Unmarshal(msgBytes, &msg); err != nil {
                                log.Printf("can not unmarshal message: %v, %v", string(msgBytes), err)
                                msgBytes = msgBytes[:0]
                                continue
                        }
                        msgBytes = msgBytes[:0]
                        if msg.Command == "ping" {
                                continue
                        } else if msg.Command == "stateResponse" {
                                if msg.Error != "" {
                                        log.Printf("error gamepadStateResponse: %v", msg.Error)
                                }
                        } else if msg.Command == "vibrationRequest" {
				errMsg := ""
				if t.checkRemoteGamepadId(conn, msg.RemoteGamepadId) &&
				   msg.Uid != "" && msg.PeerUid != "" && msg.Vibration != nil  {
					t.forwarder.ToWs(&msg)
				} else {
					errMsg = fmt.Sprintf("invalid vibrationRequest: %+v",  msg)
				}
                                resMsg := &GamepadMessage{
                                        CommonMessage: &CommonMessage{
                                                Command: "vibrationResponse",
                                        },
					Error: errMsg,
                                }
                                resMsgBytes, err := json.Marshal(resMsg)
                                if err != nil {
                                        log.Printf("can not unmarshal to json in communicationLoop: %v", err)
                                        continue
                                }
				resMsgBytes = append(resMsgBytes, byte('\n'))
                                _, err = conn.Write(resMsgBytes)
                                if err != nil {
					if err == io.EOF {
						return
					} else {
						log.Printf("can not write state response message: %v", err)
					}
                                }
                        } else {
				log.Printf("unsupportede message: %v", msg.Command)
			}
                }
        }
}

func NewTcpHandler(secret string, forwarder *Forwarder, opts ...TcpOption) (*TcpHandler, error) {
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
                verbose:    baseOpts.verbose,
                digest:     digest,
                forwarder:  forwarder,
		tcpClients: make(map[net.Conn]*tcpClient),
        }, nil
}

