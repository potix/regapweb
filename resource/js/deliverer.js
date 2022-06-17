let localStream = null;
let websocket = null;
let stopPingLoopValue = null;
let stopLookupLoopValue = null;
let peerConnection = null;
let completeSdpOffer = false;
let completeAnswerSdp = false;

let nameApp = new Vue({
        el: '#name',
        data: {
                value: '',
                readonly: false,
        },
        mounted : function(){
        },
        methods: {
                onChange: function() {
                        console.log("change name");
                },
        }
});

let videoInputDeviceApp = new Vue({
        el: '#div_for_video_input_devices',
        data: {
                selectedVideoInputDevice: 'default',
                videoInputDevices: [],
		progress: false
        },
        mounted : function(){
        },
        methods: {
        }
});

let audioInputDeviceApp = new Vue({
        el: '#div_for_audio_input_devices',
        data: {
                selectedAudioInputDevice: 'default',
                audioInputDevices: [],
		progress: false
        },
        mounted : function(){
        },
        methods: {
        }
});

let controllerApp = new Vue({
        el: '#div_for_controllers',
        data: {
                selectedController: '',
                controllers: [],
		progress: false
        },
        mounted : function(){
        },
        methods: {
        }
});

let gamepadApp = new Vue({
        el: '#div_for_gamepads',
        data: {
                selectedGamepad: '',
                gamepads: [],
		progress: false
        },
        mounted : function(){
        },
        methods: {
        }
});

window.onload = function() {
        console.log("onload: ");
        getUserMedia();
}

function getUserMedia(constraints) {
        console.log("getUserMedia: ");
        navigator.mediaDevices.getUserMedia({video: true, audio: true })
        .then(function(stream) {
                getAVInputDevices();
        })
        .catch(function(err) {
                console.log("in getUserMedia: " + err.name + ": " + err.message);
                getAVInputDevices();
        });
}

function getAVInputDevices() {
    if (!navigator.mediaDevices || !navigator.mediaDevices.enumerateDevices) {
        console.log("enumerateDevices() not supported.");
    }
    navigator.mediaDevices.enumerateDevices()
    .then(function(devices) {
        let videoInputDevices = [];
        let audioInputDevices = [];
        devices.forEach(function(device) {
            if (device.kind == "videoinput") {
                videoInputDevices.push({ "deviceId" : device.deviceId,  "label" : device.label});
            } else if (device.kind == "audioinput") {
                audioInputDevices.push({ "deviceId" : device.deviceId,  "label" : device.label});
            }
        });
        console.log(videoInputDevices);
        console.log(audioInputDevices);
        videoInputDeviceApp.videoInputDevices = videoInputDevices;
        audioInputDeviceApp.audioInputDevices = audioInputDevices;
        startWebsocket()
    })
    .catch(function(err) {
        console.log(err.name + ": " + err.message);
    });
}

function startWebsocket() {
    websocket = new WebSocket("wss://" + location.host + "/delivererws", "deliverer");
    websocket.onopen = event => {
        console.log("websocket open");
        stopPingLoopValue = pingLoop(websocket)
        stopLookupLoopValue = lookupLoop(websocket)
        let req = { MsgType: "registerReq", RegisterRequest: { ClientName: nameApp.value } };
        websocket.send(JSON.stringify(req));
    };
    websocket.onmessage = event => {
        console.log("websocket message");
        console.log(event);
	let msg = JSON.parse(event.data);
        if (msg.MsgType== "ping") {
                console.log("ping");
                return
        } else if (msg.MsgType == "registerRes") {
                if (msg.Error && msg.Error.Message != "") {
                        console.log("could not register: " + msg.Error.Message);
			return
                }
		if (!msg.RegisterResponse || msg.RegisterResponse.ClientId == "") {
                        console.log("no parameter in registerRes");
			return
		}
		if (msg.RegisterResponse.ClientType != "deliverer") {
			console.log("clientType mismatch in registerRes");
			return
                }
		const delivererId = document.getElementById('uid');
		delivererId.value =  msg.RegisterResponse.ClientId
                console.log("done register");
                return
        } else if (msg.MsgType == "lookupRes") {
		if (msg.Error && msg.Error.Message != "") {
			console.log("could not lookup: " + msg.Error.Message);
			return
		}
		if (!msg.LookupResponse) {
			console.log("no parameter in lookupRes");
			return 
		}
		controllerApp.controllers = msg.LookupResponse.Controllers;
		gamepadApp.gamepads = msg.LookupResponse.Gamepads;
		console.log("done lookup clients");
        } else if (msg.MsgType == "sigOfferSdpSrvErr") {
		if (msg.Error && msg.Error.Message != "") {
			console.log("failed in offerSdp: " + msg.Error.Message);
			hangUp();
			return
		}
        } else if (msg.MsgType == "sigOfferSdpRes") {
		if (!msg.SignalingSdpResponse ||
		    msg.SignalingSdpResponse.DelivererId == "" ||
		    msg.SignalingSdpResponse.ControllerId == "" ||
		    msg.SignalingSdpResponse.GamepadId == "") {
			console.log("no parameter in sigOfferSdpRes");
			// server is untrusted
			hangUp();
			return 
		}
		const delivererId = document.getElementById('uid');
		if (msg.SignalingSdpResponse.DelivererId != delivererId.value ||
		    msg.SignalingSdpResponse.ControllerId != controllerApp.selectedController ||
		    msg.SignalingSdpResponse.GamepadId != gamepadApp.selectedGamepad) {
			console.log("ids are mismatch in sigOfferSdpRes");
			// server is untrusted
			hangUp();
			return 
		}
		if (msg.Error && msg.Error.Message != "") {
			if (msg.Error.Message == "rejected") {
				alert("rejected by peer");
			} else {
				console.log("failed in offerSdp: " + msg.Error.Message);
			}
			hangUp();
			return
		}
		// server commited ids
		completeSdpOffer = true
		console.log("success sendOfferSdp");
                return
        } else if (msg.MsgType == "sigAnswerSdpReq") {
		if (!completeSdpOffer) {
			console.log("not complete offerSdp");
			const delivererId = document.getElementById('uid');
			let res = { MsgType : "sigAnswerSdpRes",
				    Error: {
					    Message: "not complete offerSdp"
				    },
				    SignalingSdpResponse : {
					    DelivererId: delivererId.value,
					    ControllerId: controllerApp.selectedController,
					    GamepadId: gamepadApp.selectedGamepad,
				    }
			          };
			websocket.send(JSON.stringify(res));
			hangup()
			return
		}
		if (!msg.SignalingSdpRequest ||
		    msg.SignalingSdpRequest.DelivererId == "" ||
		    msg.SignalingSdpRequest.ControllerId == "" ||
		    msg.SignalingSdpRequest.GamepadId == "" ||
		    msg.SignalingSdpRequest.Sdp == "") {
			console.log("no parameter in sigAnswerSdpReq");
			// server is untrusted
			hangup()
			return 
		}
		const delivererId = document.getElementById('uid');
		if (msg.SignalingSdpRequest.DelivererId != delivererId.value ||
		    msg.SignalingSdpRequest.ControllerId != controllerApp.selectedController ||
		    msg.SignalingSdpRequest.GamepadId != gamepadApp.selectedGamepad) {
			console.log("ids are mismatch in sigAnswerSdpReq");
			const delivererId = document.getElementById('uid');
			let res = { MsgType : "sigAnswerSdpRes",
				    Error: {
					    Message: "ids are mismatch in sigAnswerSdpReq"
				    },
				    SignalingSdpResponse : {
					    DelivererId: msg.SignalingSdpRequest.DelivererId,
					    ControllerId: msg.SignalingSdpRequest.ControllerId,
					    GamepadId: msg.SignalingSdpRequest.GamepadId
				    }
			          };
			websocket.send(JSON.stringify(res));
			hangup()
			return 
		}
                console.log('received answer sdp');
                const textToReceiveSdp = document.getElementById('text_for_receive_sdp');
                textToReceiveSdp.value = msg.SignalingSdpRequest.Sdp;
                const sessionDescription = new RTCSessionDescription({
                        type : 'answer',
                        sdp : msg.SignalingSdpRequest.Sdp,
                });
                setAnswer(sessionDescription);
                return
        } else if (msg.MsgType == "sigAnswerSdpSrvErr") {
		if (msg.Error && msg.Error.Message != "") {
			console.log("failed in answerSdp: " + msg.Error.Message);
			// XXX How to notify error to peer
			hangUp();
			return
		}
        } else {
		 console.log("unsupported message: " + msg.MsgType);
	}
    }
    websocket.onerror = event => {
	stopPingLoop(stopPingLoopValue);
	stopLookupLoop(stopLookupLoopValue)
        console.log("signaling error");
        console.log(event);
    }
    websocket.onclose = event => {
	stopPingLoop(stopPingLoopValue);
	stopLookupLoop(stopLookupLoopValue)
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

function stopPingLoop(stopPingLoopValue) {
	clearInterval(stopPingLoopValue);
}

function lookupLoop(socket) {
        return setInterval(() => {
		let req = { MsgType : "lookupReq" };
                socket.send(JSON.stringify(req));
        }, 2000);
}

function stopLookupLoop(stopLookupLoopValue) {
	clearInterval(stopLookupLoopValue);
}

function startLocalVideo() {
	const delivererId = document.getElementById('uid');
	if (delivererId.value == "") {
		console.log("no uid");
		return
	}
	if (controllerApp.selectedController == "") {
		console.log("no select controller");
		return
	}
	if (gamepadApp.selectedGamepad == "") {
		console.log("no select gamepad");
		return
	}
	console.log(delivererId.value);
	console.log(controllerApp.selectedController);
	console.log(gamepadApp.selectedGamepad);
	nameApp.readonly = true;
	videoInputDeviceApp.progress = true;
	audioInputDeviceApp.progress = true;
	controllerApp.progress = true;
	gamepadApp.progress = true;
	const localVideo = document.getElementById('local_video'); 
	localVideo.pause();
	console.log(videoInputDeviceApp.selectedVideoInputDevice);
	console.log(audioInputDeviceApp.selectedAudioInputDevice);
	navigator.mediaDevices.getUserMedia({
		video: { deviceId: videoInputDeviceApp.selectedVideoInputDevice },
		audio: { deviceId: audioInputDeviceApp.selectedAudioInputDevice }
	}).then(function(stream) {
		playLocalVideo(stream);
        })
        .catch(function(err) {
                console.log("in startLocalVideo: " + err.name + ": " + err.message);
        });
}

async function playLocalVideo(stream) {
	const localVideo = document.getElementById('local_video'); 
        localVideo.srcObject = localStream = stream;
        try {
            await localVideo.play();
            connectWebrtc();
        } catch(err) {
            console.log('error auto play:' + err);
        }
}

function connectWebrtc() {
	console.log('make Offer');
	peerConnection = prepareNewConnection(true);
	// ローカルのMediaStreamを利用できるようにする
	// peerConnectionのonnegotiationneededが発生する
	console.log('Adding local stream...');
	for (const track of localStream.getTracks()) {
		console.log(track);
		peerConnection.addTrack(track);
	}
}

function prepareNewConnection() {
	console.log('prepareNewConnection');
	const iceServers = [
		{
			urls: "stun:stun.webrtc.ecl.ntt.com:3478",
		},
		//{
		//	url: 'turn:numb.viagenie.ca',
		//	credential: 'muazkh',
		//	username: 'webrtc@live.com'
		//},
	]
	const peer = new RTCPeerConnection({"iceServers": iceServers });

	peer.onnegotiationneeded = async () => {
		console.log('onnegotiationneeded');
		try {
			let offer = await peer.createOffer();
			console.log('createOffer() succsess in promise');
			await peer.setLocalDescription(offer);
			console.log('setLocalDescription() succsess in promise');
		} catch(err){
			console.error('setLocalDescription(offer) ERROR: ', err);
		}
		// この処理の後にonicecandidateが呼ばれる
	}

	// ICE Candidateを収集したときのイベント
	peer.onicecandidate = evt => {
		console.log('onicecandidate');
		if (evt.candidate) {
		    console.log(evt.candidate);
		} else {
		    console.log('empty ice event');
		    console.log(peer.localDescription);
		    // candidateの収集が終わるまで待つ
		    sendOfferSdp(peer.localDescription);
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

	return peer;
}

function sendOfferSdp(sessionDescription) {
	console.log('--- sending offer sdp ---');
	const textForSendSdp = document.getElementById('text_for_send_sdp');
	textForSendSdp.value = sessionDescription.sdp;
	const delivererId = document.getElementById('uid');
	let req = { MsgType: "sigOfferSdpReq",
		    SignalingSdpRequest: {
			    Name: nameApp.value,
			    DelivererId: delivererId.value,
			    ControllerId: controllerApp.selectedController,
			    GamepadId: gamepadApp.selectedGamepad,
			    Sdp: sessionDescription.sdp
                    }
	          };
        websocket.send(JSON.stringify(req));
}

async function setAnswer(sessionDescription) {
    try{
        await peerConnection.setRemoteDescription(sessionDescription);
        console.log('setRemoteDescription(answer) succsess in promise');
	const delivererId = document.getElementById('uid');
	let res = { MsgType : "sigAnswerSdpRes",
		    SignalingSdpResponse : {
			    DelivererId: delivererId.value,
			    ControllerId: controllerApp.selectedController,
			    GamepadId: gamepadApp.selectedGamepad,
		    }
	          };
	websocket.send(JSON.stringify(res));
        console.log('succsess answerSdp');
	completeAnswerSdp = true;
    } catch(err){
        console.error('setRemoteDescription(answer) ERROR: ', err);
	const delivererId = document.getElementById('uid');
	let res = { MsgType : "sigAnswerSdpRes",
		    Error: {
			    Message: "could not set remote description"
		    },
		    SignalingSdpResponse : {
			    DelivererId: delivererId.value,
			    ControllerId: controllerApp.selectedController,
			    GamepadId: gamepadApp.selectedGamepad,
		    }
	          };
	websocket.send(JSON.stringify(res));
	hangUp()
    }
}

function hangUp(){
	console.log('hangUp');
	if(peerConnection && peerConnection.iceConnectionState !== 'closed'){
		peerConnection.close();
		peerConnection = null;
		console.log('peerConnection is closed.');
	}
	const textForSendSdp = document.getElementById('text_for_send_sdp');
	textForSendSdp.value = '';
	const textToReceiveSdp = document.getElementById('text_for_receive_sdp');
	textToReceiveSdp.value = '';
	const localVideo = document.getElementById('local_video');
	localVideo.pause();
	localVideo.srcObject = localStream = null;
	completeSdpOffer = false;
	completeAnswerSdp = false;
	nameApp.readonly = false;
	videoInputDeviceApp.progress = false;
	audioInputDeviceApp.progress = false;
	controllerApp.progress = false;
	gamepadApp.progress = false;
}

