let signalingSocket = null;
let stopSignalingPingLoopValue = null;
let peerConnection = null;
let remoteStream = new MediaStream();
let started = false;
let gamepads = {};
let gamepadSocket = null;
let stopGamepadPingLoopValue = null;
let stopGamepadNotifyLoopValue = null;
let gamepadTimestamp = 0;

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
	startSignaling()
    })
    .catch(function(err) {
        console.log(err.name + ": " + err.message);
    });
}

function startSignaling() {
    signalingSocket = new WebSocket("wss://" + location.host + "/webrtc", "signaling");    
    signalingSocket.onopen = event => {
        console.log("signaling open");
	stopSignalingPingLoopValue = pingLoop(signalingSocket)
	startRegister();
    };
    signalingSocket.onmessage = event => {
        console.log("signaling message");
        console.log(event);
	let msg = JSON.parse(event.data);
	if (msg.Command == "ping") {
		console.log("ping");
		return
	} else if (msg.Command == "registerResponse") {
		if (msg.Error != "") {
			console.log("can not register: " + msg.Error);
		} else {
			console.log("done register");
		}
		return
	} else if (msg.Command == "sendOfferSdpRequest") {
		if (msg.Messages.length != 3) {
			console.log("invalid send offer sdp request");
			console.log(msg);
			return
		}
		const uid = document.getElementById('uid');
		if (uid.value != msg.Messages[0]) {
			console.log("uid mismatch of send offer sdp request");
			console.log(msg);
			return
		}
		if (!msg.Messages[1]) {
			console.log("no peer uid in send offer sdp request");
			console.log(msg);
			return
		}
		console.log('received offer text');
		const peerUid = document.getElementById('peer_uid');
		peerUid.value = msg.Messages[1]
		const textToReceiveSdp = document.getElementById('text_for_receive_sdp');
		textToReceiveSdp.value = msg.Messages[2];
	        const sessionDescription = new RTCSessionDescription({
            		type : 'offer',
			sdp : msg.Messages[2],
		});
		setOffer(sessionDescription);
		return
        } else if (msg.Command == "sendAnswerSdpResponse") {
                if (msg.Error != "") {
                        console.log("can not send answer sdp: " + msg.Error);
                } else {
                        console.log("done sendAnswerSdp");
			playRemoteVideo();
                }
                return
        } else if (msg.Command == "setupRemoteGamepadRequest") {
		if (msg.Messages.length != 3) {
			console.log("invalid setup remote gamepad request");
			console.log(msg);
			return
		}
		if (!msg.Messages[2]) {
			console.log("no remote gamepad id in setup remote gamepad request");
			console.log(msg);
		}
		const remoteGamepadId = document.getElementById('remote_gamepad_id');
		remoteGamepadId.value = msg.Messages[2]
		return

	} else {
		console.log("unsupported message: " + msg.Command);
	}
    }
    signalingSocket.onerror = event => {
        stopPingLoop(stopSignalingPingLoopValue);
        console.log("signaling error");
        console.log(event);
    }
    signalingSocket.onclose = event => {
        stopPingLoop(stopSignalingPingLoopValue);
        console.log("signaling close");
        console.log(event);
    }
}

function pingLoop(socket) {
	return setInterval(() => {
		let req = { Command : "ping" }; 
		socket.send(JSON.stringify(req));
	}, 10000);
}

function stopPingLoop(value) {
        clearInterval(value);
}

function startRegister() {
	if (started == true) {
		console.log("start register")
		const uid = document.getElementById('uid');
		let req = { Command : "registerRequest", Messages : [ uid.value ] };
		signalingSocket.send(JSON.stringify(req));
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
        makeAnswerSdp();
    } catch(err){
        console.error('setRemoteDescription(offer) ERROR: ', err);
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
        console.error(err);
    }
}

function sendAnswerSdp(sessionDescription) {
	console.log('--- sending answer sdp ---');
	const textForSendSdp = document.getElementById('text_for_send_sdp');
	textForSendSdp.value = sessionDescription.sdp;
	const uid = document.getElementById('uid');
	const peerUid = document.getElementById('peer_uid');
        let req = { "Command" : "sendAnswerSdpRequest", "Messages" : [ peerUid.value, uid.value, sessionDescription.sdp ] };
        signalingSocket.send(JSON.stringify(req));
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
        console.log('hungup');
        if(peerConnection && peerConnection.iceConnectionState !== 'closed'){
                peerConnection.close();
                peerConnection = null;
                console.log('peerConnection is closed.');
	}
        const textForSendSdp = document.getElementById('text_for_send_sdp');
        textForSendSdp.value = '';
        const textToReceiveSdp = document.getElementById('text_for_receive_sdp');
        textToReceiveSdp.value = '';
        const remoteVideo = document.getElementById('remote_video');
        //remoteVideo.pause();
        remoteVideo.srcObject = null;
	remoteStream = new MediaStream();
}

function prepareGamepads() {
	if ('GamepadEvent' in window) {
		window.addEventListener("gamepadconnected", connectHandler);
		window.addEventListener("gamepaddisconnected", disconnectHandler);
	} else if ('WebKitGamepadEvent' in window) {
		window.addEventListener("webkitgamepadconnected", connectHandler);
		window.addEventListener("webkitgamepaddisconnected", disconnectHandler);
	} else {
		console.log("can not use gamepad");
	}
	startForwardGamepad();
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
	console.log(gamepad)
	if (gamepad && gamepad.timestamp != gamepadTimestamp) {
		console.log(gamepad);
		const uid = document.getElementById('uid');
		const peerUid = document.getElementById('peer_uid');
		const remoteGamepadId = document.getElementById('remote_gamepad_id');
		if (uid.value != "" && peerUid.value != "" && remoteGamepadId.value != "") {
			buttons = [];
			for (let v of gamepad.buttons) {
				buttons.push({ "Pressed" : v.pressed, "Touched" : v.touched, "Value" : v.value })
			}
			let req = {
				"Command": "stateRequest",
				"Uid":     uid.value,
				"PeerUid": peerUid.value,
				"RemoteGamepadId": remoteGamepadId.value,
				"State": {
					"Buttons": buttons,
					"Axes":    gamepad.axes,
				}
			};
			gamepadSocket.send(JSON.stringify(req));
			gamepadTimestamp = gamepad.timestamp;
		}
	}
	const rAF = window.requestAnimationFrame ||
	 	    window.mozRequestAnimationFrame ||
                    window.webkitRequestAnimationFrame
	rAF(updateGamepadsStatus);
}

function notifyLoop(socket) {
	return setInterval(() => {
		const uid = document.getElementById('uid');
		const peerUid = document.getElementById('peer_uid');
		if (uid.value != "" && peerUid.value != "") {
			let req = {
				Command: "notify",
				Uid:     uid.value,
				PeerUid: peerUid.value,
			};
			console.log(req);
			socket.send(JSON.stringify(req));
		}
	}, 2000);
}

function stopNotifyLoop(value) {
        clearInterval(value);
}

function startForwardGamepad() {
    gamepadSocket = new WebSocket("wss://" + location.host + "/gamepad", "gamepad");    
    gamepadSocket.onopen = event => {
        console.log("gamepad open");
	stopGamepadPingLoopValue = pingLoop(gamepadSocket)
	stopGamepadNotifyLoopValue = notifyLoop(gamepadSocket)
    };
    gamepadSocket.onmessage = event => {
        //console.log("gamepadSocket message");
        //console.log(event);
	let msg = JSON.parse(event.data);
	if (msg.Command == "ping") {
		console.log("ping");
		return
	} else if (msg.Command == "stateResponse") {
		if (msg.Error != "") {
			console.log("can not forward gamepad state: " + msg.Error);
		} else {
			console.log("done forward gamepad state");
		}
		return
	} else if (msg.Command == "vibrationRequest") {
		console.log("vibration request");
		console.log(msg);
		gamepad = gamepads[gamepadApp.selectedGamepad];
		if (gamepad.vibrationActuator) {
			gamepad.vibrationActuator.playEffect('dual-rumble', {
				startDelay: msg.Vibration.startDelay,
				duration: msg.Vibration.Duration,
				weakMagnitude: msg.Vibration.WeakMagnitude,
				strongMagnitude: msg.Vibration.StrongMagnitude,
			});
		} else if (gamepad.hapticActuators && gamepad.hapticActuators.length > 0) {
			hapticActuator[0].pluse(msg.Vibration.StrongMagnitude, msg.Vibration.Duration);
		}
		return
	} else {
		console.log("unsupported message: " + msg.Command);
	}
    }
    gamepadSocket.onerror = event => {
        stopPingLoop(stopGamepadPingLoopValue);
        stopNotifyLoop(stopGamepadNotifyLoopValue);
        console.log("gamepad error");
        console.log(event);
    }
    gamepadSocket.onclose = event => {
        stopPingLoop(stopGamepadPingLoopValue);
        stopNotifyLoop(stopGamepadNotifyLoopValue);
        console.log("gamepad close");
        console.log(event);
    }
}
