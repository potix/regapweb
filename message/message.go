package message

const (
	MsgTypePing                   string = "ping"              // client     <------> server (periodic 10 sec)
	MsgTypeRegisterReq                   = "registerReq"       // client      ------> server
	MsgTypeRegisterRes                   = "registerRes"       // client      ------> server
	MsgTypeLookupReq                     = "lookupReq"         // deliverer   ------> server (periodic 3 sec)
	MsgTypeLookupRes                     = "lookupRes"         // deliverer  <------  server
	MsgTypeSignalingOfferSdpReq          = "sigOfferSdpReq"    // deliverer   ------> server  ------> controller
	MsgTypeSignalingOfferSdpRes          = "sigOfferSdpRes"    // deliverer  <------  server <------  controller
	MsgTypeSignalingOfferSdpServerError  = "sigOfferSdpSrvErr" // deliverer  <------  server ------>  controller
	MsgTypeSignalingAnswerSdpReq         = "sigAnswerSdpReq"    // deliverer  <------  server <------  controller
	MsgTypeSignalingAnswerSdpRes         = "sigAnswerSdpRes"    // deliverer   ------> server  ------> controller
	MsgTypeSignalingAnswerSdpServerError = "sigAnswerSdpSrvErr" // deliverer  <------  server  ------> controller
	MsgTypeGamepadHandshakeReq           = "gpHandshakeReq"    // gamepad     ------> server
	MsgTypeGamepadHandshakeRes           = "gpHandshakeRes"    // gamepad    <------> server
	MsgTypeGamepadConnectReq             = "gpConnectReq"      // controller  ------> server  ------> gamepad
	MsgTypeGamepadConnectRes             = "gpConnectRes"      // controller <------  server <------  gamepad
	MsgTypeGamepadConnectServerError     = "gpConnectSrvErr"   // controller <------  server ------>  gamepad
	MsgTypeGamepadState                  = "gpState"           // controller  ------> server  ------> gamepad (perodic 1000 / 60 msec)
	MsgTypeGamepadVibration              = "gpVibration"       // controller <------  server <------  gamepad
)

const (
	ClientTypeDeliverer  string = "deliverer"
	ClientTypeController        = "controller"
	ClientTypeGamepad           = "gamepad"
)

type Error struct {
	Message string
}

type RegisterRequest struct {
	ClientName string
}

type RegisterResponse struct {
	ClientType string
	ClientId   string
}

type LookupResponse struct {
	Controllers []*NameAndId
	Gamepads []*NameAndId
}

type NameAndId struct {
	Name string
	Id  string
}

type SignalingSdpRequest struct {
	Name         string
	DelivererId  string
	ControllerId string
	GamepadId    string
	Sdp          string
}

type SignalingSdpResponse struct {
	DelivererId  string
	ControllerId string
	GamepadId    string
}

type GamepadHandshakeRequest struct {
	Name string
	Digest string
}

type GamepadHandshakeResponse struct {
	GamepadId string
}

type GamepadConnectRequest struct {
	DelivererId  string
	ControllerId string
	GamepadId    string
}

type GamepadConnectResponse struct {
	DelivererId  string
	ControllerId string
	GamepadId    string
}

type GamepadState struct {
	DelivererId  string
	ControllerId string
	GamepadId    string
        Buttons      []*GamepadButtonState
        Axes         []float64
}

type GamepadButtonState struct {
        Pressed bool
        Touched bool
        Value   float64
}

type GamepadVibration struct {
	DelivererId     string
	ControllerId    string
	GamepadId       string
	Duration        float64
        StartDelay      float64
        StrongMagnitude float64
        WeakMagnitude   float64
}

type Message struct {
	MsgType                  string
	Error                    *Error                    `json:"omitempty"`
	RegisterRequest          *RegisterRequest          `json:"omitempty"`
	RegisterResponse         *RegisterResponse         `json:"omitempty"`
	LookupResponse           *LookupResponse           `json:"omitempty"`
	SignalingSdpRequest      *SignalingSdpRequest      `json:"omitempty"`
	SignalingSdpResponse     *SignalingSdpResponse     `json:"omitempty"`
	GamepadHandshakeRequest  *GamepadHandshakeRequest  `json:"GamepadHandshakeRequest,omitempty"`
	GamepadHandshakeResponse *GamepadHandshakeResponse `json:"GamepadHandshakeResponse,omitempty"`
	GamepadConnectRequest    *GamepadConnectRequest    `json:"omitempty"`
	GamepadConnectResponse   *GamepadConnectResponse   `json:"omitempty"`
	GamepadState             *GamepadState             `json:"omitempty"`
	GamepadVibration         *GamepadVibration         `json:"omitempty"`
}



