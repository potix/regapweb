let websocket = null;
let stopPingLoopValue = null;
let peerConnection = null;
let remoteStream = new MediaStream();
let started = false;
let gamepads = {};
let gamepadTimestamp = 0;
let completeSdpOffer = false;
let completeAnswerSdp = false;
let completeConnectGamepad = false;

let audioOutputDeviceApp = new Vue({
	el: '#div_for_audio_output_devices',
	data: {
		selectedAudioOutputDevice: 'default',
		audioOutputDevices: [],
	},
	mounted : function(){ 
		this.setSkinId();
	},
	methods: {
		setSkinId: function() {
			console.log("setSkinId:" + this.selectedAudioOutputDevice);
			if (this.selectedAudioOutputDevice == "") {
				return
			}
			const remoteVideo = document.getElementById('remote_video');
			if (remoteVideo.setSinkId) {
				remoteVideo.setSinkId(this.selectedAudioOutputDevice)
				.then(function(stream) {
					console.log("done set skinId");
				})
				.catch(function(err) {
					console.log("in setSkinId: " + err.name + ": " + err.message);
				});
			} else {
				// [ firefox ]
				// about:config
				// media.setsinkid.enabled
				// change to true
				console.log("can set skinId");
			}
		},
	}
});

let gamepadApp = new Vue({
	el: '#div_for_gamepads',
	data: {
		selectedGamepad: 0,
		gamepads: [],
	},
	mounted : function(){ 
	},
	methods: {
		updateTimestamp: function() {
			let gamepad = gamepads[this.selectedGamepad];
			if (gamepad) {
				gamepadTimestamp = gamepad.timestamp;
			}
		}
	}
});

window.onload = function() {
	console.log("onload: ");
	getUserMedia();
	prepareGamepads();
}

async function start() {
	console.log('play remote video');
	try {
		started = true;
		const startLamp = document.getElementById('start_lamp');
		startLamp.setAttribute("class", "border-radius background-color-green inline-block" )
		const remoteVideo = document.getElementById('remote_video');
		await remoteVideo.play();
	} catch(err) {
		console.log('error auto play:' + err);
	}
}

function getUserMedia() {
	console.log("getUserMedia: ");
	navigator.mediaDevices.getUserMedia({video: false, audio: true })
	.then(function(stream) {
		getAudioOutDevices();
	})
	.catch(function(err) {
		console.log("in getUserMedia: " + err.name + ": " + err.message);
	});
}

function getAudioOutDevices() {
    if (!navigator.mediaDevices || !navigator.mediaDevices.enumerateDevices) {
        console.log("enumerateDevices() not supported.");
    }
    navigator.mediaDevices.enumerateDevices()
    .then(function(devices) {
        let audioOutputDevices = []; 
        devices.forEach(function(device) {
            if (device.kind == "audiooutput") {
	        audioOutputDevices.push({ "deviceId" : device.deviceId,  "label" : device.label});
	    }
        });
	console.log(audioOutputDevices);
	audioOutputDeviceApp.audioOutputDevices = audioOutputDevices;
	startWebsocket()
    })
    .catch(function(err) {
        console.log(err.name + ": " + err.message);
    });
}

function startWebsocket() {
    websocket = new WebSocket("wss://" + location.host + "/controllerws", "controller");    
    websocket.onopen = event => {
        console.log("websocket open");
	stopPingLoopValue = pingLoop(websocket)
	startRegister();
    };
    websocket.onmessage = event => {
        console.log("websocket message");
        console.log(event);
	let msg = JSON.parse(event.data);
	if (msg.MsgType == "ping") {
		console.log("ping");
		return
	} else if (msg.MsgType == "registerRes") {
		if (msg.Error && msg.Error.Message != "") {
			console.log("could not register: " + msg.Error.Message);
			return
		}
		if (msg.RegisterResponse == null || msg.RegisterResponse.ClientId == "") {
			console.log("no parameter in registerRes");
			return
		}
		if (msg.RegisterResponse.ClientType != "controller") {
			console.log("clientType mismatch in registerRes");
			return
		}
		const controllerId = document.getElementById('uid');
                controllerId.value =  msg.RegisterResponse.ClientId
                console.log("done register");
		return
	} else if (msg.MsgType == "sigOfferSdpReq") {
		if (!msg.SignalingSdpRequest ||
		    msg.SignalingSdpRequest.DelivererId == "" ||
		    msg.SignalingSdpRequest.ControllerId == "" ||
		    msg.SignalingSdpRequest.GamepadId == "" ||
		    msg.SignalingSdpRequest.Sdp == "") {
			console.log("no parameter in sigOfferSdpReq");
			// server is untrusted
			return
		}
		const controllerId = document.getElementById('uid');
		if (controllerId.value != msg.SignalingSdpRequest.ControllerId) {
			console.log("id mismatch in sigOfferSdpReq");
			// server is untrusted
			handUp();
			return
		}
		if (!confirm("There is an incoming call from " +
			msg.SignalingSdpRequest.DelivererId +
			"(" + msg.SignalingSdpRequest.Name + ")" +
			". Do you allow it?")) {
			const controllerId = document.getElementById('uid');
			let res = { MsgType: "sigOfferSdpRes",
				    Error: {
					    Message: "rejected"
				    },
				    SignalingSdpResponse: {
					    DelivererId: msg.SignalingSdpRequest.DelivererId,
					    ControllerId: controllerId.value,
					    GamepadId: msg.SignalingSdpRequest.GamepadId
				    } 
			          };
			websocket.send(JSON.stringify(res));
			return
		}
		console.log('received offer text');
		const delivererId = document.getElementById('deliverer');
		delivererId.value = msg.SignalingSdpRequest.DelivererId;
		const gamepadId = document.getElementById('gamepad');
		gamepadId.value = msg.SignalingSdpRequest.GamepadId;
		const textToReceiveSdp = document.getElementById('text_for_receive_sdp');
		textToReceiveSdp.value = msg.SignalingSdpRequest.Sdp;
	        const sessionDescription = new RTCSessionDescription({
            		type : 'offer',
			sdp : msg.SignalingSdpRequest.Sdp,
		});
		setOffer(sessionDescription);
		return
	} else if (msg.MsgType == "sigOfferSdpSrvErr") {
                if (msg.Error && msg.Error.Message != "") {
                        console.log("failed in offerSdp: " + msg.Error.Message);
			// XXX How to notify error to peer
			handUp();
                        return
                }
	} else if (msg.MsgType == "sigAnswerSdpSrvErr") {
                if (msg.Error && msg.Error.Message != "") {
                        console.log("failed in answerSdp: " + msg.Error.Message);
			// XXX How to notify error to peer
			handUp();
                        return
                }
        } else if (msg.MsgType == "sigAnswerSdpRes") {
		if (!msg.SignalingSdpResponse ||
		    msg.SignalingSdpResponse.DelivererId == "" ||
		    msg.SignalingSdpResponse.ControllerId == "" ||
		    msg.SignalingSdpResponse.GamepadId == "") {
			console.log("no parameter in sigAnswerSdpRes");
			// server is untrusted
			handUp();
			return
		}
		const controllerId = document.getElementById('uid');
		const delivererId = document.getElementById('deliverer');
		const gamepadId = document.getElementById('gamepad');
                if (msg.SignalingSdpResponse.DelivererId != delivererId.value ||
                    msg.SignalingSdpResponse.ControllerId != controllerId.value ||
                    msg.SignalingSdpResponse.GamepadId != gamepadId.value) {
                        console.log("ids are mismatch in sigOfferSdpRes");
			// server is untrusted
                        handUp();
                        return
                }
                if (msg.Error && msg.Error.Message != "") {
                        console.log("failed answerSdp: " + msg.Error.Message);
			handUp();
			return
                }
                console.log("success answerSdp");
		completeAnswerSdp = true
		// play remote video
		playRemoteVideo();
		// connect to gamepad
		let req = { MsgType: "gpConnectReq",
			    GamepadConnectRequest: {
				    DelivererId: delivererId.value,
				    ControllerId: controllerId.value,
				    GamepadId: gamepadId.value,
			    }
			  };
		websocket.send(JSON.stringify(req));
                return
        } else if (msg.MsgType == "gpConnectRes") {
		if (!msg.GamepadConnectResponse ||
		    msg.GamepadConnectResponse.DelivererId == "" ||
		    msg.GamepadConnectResponse.ControllerId == "" ||
		    msg.GamepadConnectResponse.GamepadId == "") {
			console.log("no parameter in gpConnectRes");
			// server is untrusted
			handUp();
			return
		}
		const controllerId = document.getElementById('uid');
		const delivererId = document.getElementById('deliverer');
		const gamepadId = document.getElementById('gamepad');
                if (msg.GamepadConnectResponse.DelivererId != delivererId.value ||
                    msg.GamepadConnectResponse.ControllerId != controllerId.value ||
                    msg.GamepadConnectResponse.GamepadId != gamepadId.value) {
                        console.log("ids are mismatch in gpConnectRes");
			// server is untrusted
                        handUp();
                        return
                }
                if (msg.Error && msg.Error.Message != "") {
                        console.log("failed in connect gapmepad: " + msg.Error.Message);
			handUp();
			return
                }
		completeConnectGamepad = true
		return
	} else if (msg.MsgType == "gpVibration") {
		if (!msg.GamepadVibration ||
		    msg.GamepadVibration.DelivererId == "" ||
		    msg.GamepadVibration.ControllerId == "" ||
		    msg.GamepadVibration.GamepadId == "") {
			console.log("no parameter in gpVibration");
			// server is untrusted
			return
		}
		const controllerId = document.getElementById('uid');
		const delivererId = document.getElementById('deliverer');
		const gamepadId = document.getElementById('gamepad');
                if (msg.GamepadVibration.DelivererId != delivererId.value ||
                    msg.GamepadVibration.ControllerId != controllerId.value ||
                    msg.GamepadVibration.GamepadId != gamepadId.value) {
                        console.log("ids are mismatch in gpVibration");
			// server is untrusted
                        return
                }
		gamepad = gamepads[gamepadApp.selectedGamepad];
		if (gamepad.vibrationActuator) {
			gamepad.vibrationActuator.playEffect('dual-rumble', {
				startDelay: msg.GamepadVibration.startDelay,
				duration: msg.GamepadVibration.Duration,
				weakMagnitude: msg.GamepadVibration.WeakMagnitude,
				strongMagnitude: msg.GamepadVibration.StrongMagnitude,
			});
		} else if (gamepad.hapticActuators && gamepad.hapticActuators.length > 0) {
			hapticActuator[0].pluse(msg.GamepadVibration.StrongMagnitude, msg.GamepadVibration.Duration);
		}
		return
	} else {
		console.log("unsupported message: " + msg.MsgType);
	}
    }
    websocket.onerror = event => {
        stopPingLoop(stopPingLoopValue);
        console.log("signaling error");
        console.log(event);
    }
    websocket.onclose = event => {
        stopPingLoop(stopPingLoopValue);
        console.log("signaling close");
        console.log(event);
    }
}

function pingLoop(socket) {
	return setInterval(() => {
		let req = { MsgType : "ping" }; 
		socket.send(JSON.stringify(req));
	}, 10000);
}

function stopPingLoop(value) {
        clearInterval(value);
}

function startRegister() {
	if (started == true) {
		console.log("start register")
		const name = document.getElementById('name');
		let req = { MsgType: "registerReq", RegisterRequest: { ClientName: name.value } };
		websocket.send(JSON.stringify(req));
	} else {
		console.log("retry register")
		setTimeout( () => {
			startRegister();
		}, 1000);
	}
}

function prepareNewConnection(isOffer) {
    console.log('prepareNewConnection');
    const iceServers = [
	{
	  urls: "stun:stun.webrtc.ecl.ntt.com:3478",
	},
        //{
        //    url: 'turn:numb.viagenie.ca',
        //    credential: 'muazkh',
        //    username: 'webrtc@live.com'
        //},
    ]
    const peer = new RTCPeerConnection({"iceServers": iceServers});

    peer.onnegotiationneeded = async () => {
        console.log('onnegotiationneeded');
    }

    // ICE Candidateを収集したときのイベント
    peer.onicecandidate = evt => {
        console.log('onicecandidate');
        if (evt.candidate) {
            console.log(evt.candidate);
        } else {
            console.log('empty ice event');
	    console.log(peer.localDescription);
	    sendAnswerSdp(peer.localDescription);
        }
    };

    peer.oniceconnectionstatechange = function() {
        console.log('ICE connection Status has changed to ' + peer.iceConnectionState);
        switch (peer.iceConnectionState) {
            case 'closed':
            case 'failed':
            case 'disconnected':
		hangUp();
                break;
        }
    };

    // リモートのMediaStreamTrackを受信した時
    peer.ontrack = evt => {
        console.log('-- peer.ontrack()');
        console.log(evt.track);
        console.log('Adding remote stream...');
	remoteStream.addTrack(evt.track);
    };

    return peer;
}

async function setOffer(sessionDescription) {
    peerConnection = prepareNewConnection();
    try{
        await peerConnection.setRemoteDescription(sessionDescription);
        console.log('setRemoteDescription(offer) succsess in promise');
	const controllerId = document.getElementById('uid');
	const delivererId = document.getElementById('deliverer');
	const gamepadId = document.getElementById('gamepad');
	let res = { MsgType: "sigOfferSdpRes",
		    SignalingSdpResponse: {
			    DelivererId: delivererId.value,
			    ControllerId: controllerId.value,
			    GamepadId: gamepadId.value
		    } 
	          };
	websocket.send(JSON.stringify(res));
	// server commited ids
        console.log('succsess OfferSdp');
	completeSdpOffer = true;
        makeAnswerSdp();
    } catch(err){
        console.error('setRemoteDescription(offer) ERROR: ', err);
	const controllerId = document.getElementById('uid');
	const delivererId = document.getElementById('deliverer');
	const gamepadId = document.getElementById('gamepad');
	let res = { MsgType: "sigOfferSdpRes",
		    Error: {
			    Message: "could not set remote description"
		    },
		    SignalingSdpResponse: {
			    DelivererId: delivererId.value,
			    ControllerId: controllerId.value,
			    GamepadId: gamepadId.value
		    } 
	          };
	websocket.send(JSON.stringify(res));
	handUp();
    }
}

async function makeAnswerSdp() {
    console.log('sending Answer. Creating remote session description...' );
    try{
        let answer = await peerConnection.createAnswer();
        console.log('createAnswer() succsess in promise');
        await peerConnection.setLocalDescription(answer);
        console.log('setLocalDescription() succsess in promise');
    } catch(err){
        console.error("setLocalDescription(answer) ERROR:", err);
	// XXX How to notify error to peer
	hangup();
    }
}

function sendAnswerSdp(sessionDescription) {
	console.log('--- sending answer sdp ---');
	const textForSendSdp = document.getElementById('text_for_send_sdp');
	textForSendSdp.value = sessionDescription.sdp;
	const controllerId = document.getElementById('uid');
	const delivererId = document.getElementById('deliverer');
	const gamepadId = document.getElementById('gamepad');
	const name = document.getElementById('name');
        let req = { MsgType: "sigAnswerSdpReq",
		    SignalingSdpRequest: {
			    Name: name.value,
			    DelivererId: delivererId.value,
			    ControllerId: controllerId.value,
			    GamepadId: gamepadId.value,
			    Sdp: sessionDescription.sdp
		    }
	          };
        websocket.send(JSON.stringify(req));
}

//async function playRemoteVideo() {
function playRemoteVideo() {
    console.log('play remote video');
    try {
	//remoteVideo.pause();
        const remoteVideo = document.getElementById('remote_video');
        remoteVideo.srcObject = remoteStream;
	//await remoteVideo.play();
    } catch(err) {
        console.log('error auto play:' + err);
    }
}

function hangUp(){
        console.log('handUp');
        if(peerConnection && peerConnection.iceConnectionState !== 'closed'){
                peerConnection.close();
                peerConnection = null;
                console.log('peerConnection is closed.');
	}
	const delivererId = document.getElementById('deliverer');
	delivererId.value = '';
	const gamepadId = document.getElementById('gamepad');
	gamepadId.value = '';
        const textForSendSdp = document.getElementById('text_for_send_sdp');
        textForSendSdp.value = '';
        const textToReceiveSdp = document.getElementById('text_for_receive_sdp');
        textToReceiveSdp.value = '';
        const remoteVideo = document.getElementById('remote_video');
        //remoteVideo.pause();
        remoteVideo.srcObject = null;
	remoteStream = new MediaStream();
	completeSdpOffer = false;
        completeAnswerSdp = false;
        completeConnectGamepad = false;
}

function prepareGamepads() {
	if ('GamepadEvent' in window) {
		window.addEventListener("gamepadconnected", connectHandler);
		window.addEventListener("gamepaddisconnected", disconnectHandler);
	} else if ('WebKitGamepadEvent' in window) {
		window.addEventListener("webkitgamepadconnected", connectHandler);
		window.addEventListener("webkitgamepaddisconnected", disconnectHandler);
	} else {
		console.log("cloud not use gamepad");
	}
}

function connectHandler(e) {
	gamepads[e.gamepad.index] = e.gamepad;
	gamepadApp.gamepads = gamepads;
	const rAF = window.requestAnimationFrame ||
	            window.mozRequestAnimationFrame ||
	            window.webkitRequestAnimationFrame
	rAF(updateGamepadsStatus);
}

function disconnectHandler(e) {
	delete gamepads[e.gamepad.index];
}

function scanGamepads() {
	let gps = navigator.getGamepads ?  navigator.getGamepads() :
	  	  (navigator.webkitGetGamepads ?  navigator.webkitGetGamepads() : []);
	for (var i = 0; i < gps.length; i++) {
		if (gps[i] && (gps[i].index in gamepads)) {
			gamepads[gps[i].index] = gps[i];
		}
	}
}

function updateGamepadsStatus() {
	scanGamepads();
	let changed = false;
	gamepad = gamepads[gamepadApp.selectedGamepad];
	const controllerId = document.getElementById('uid');
	const delivererId = document.getElementById('deliverer');
	const gmepadId = document.getElementById('gamepad');
	if (controllerId.value != "" &&
	    delivererId.value != "" &&
	    gmepadId.value != "" &&
	    completeSdpOffer &&
            completeAnswerSdp &&
            completeConnectGamepad) {
		buttons = [];
		for (let v of gamepad.buttons) {
			buttons.push({ "Pressed" : v.pressed, "Touched" : v.touched, "Value" : v.value })
		}
		let msg = {
			MsgType: "gpState",
			GamepadState: {
				DelivererId: delivererId.value,
				ControllerId: controllerId.value,
				GamepadId: gmepadId.value
			},
			Buttons: buttons,
			Axes: gamepad.axes,
		};
		gamepadSocket.send(JSON.stringify(req));
		gamepadTimestamp = gamepad.timestamp;
	}
	const rAF = window.requestAnimationFrame ||
	 	    window.mozRequestAnimationFrame ||
                    window.webkitRequestAnimationFrame
	rAF(updateGamepadsStatus);
}
