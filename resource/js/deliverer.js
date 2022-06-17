let localStream = null;
let websocket = null;
let stopPingLoopValue = null;
let stopLookupLoopValue = null;
let peerConnection = null;
let completeSdpOffer = false;
let completeAnswerSdp = false;

let videoInputDeviceApp = new Vue({
        el: '#div_for_video_input_devices',
        data: {
                selectedVideoInputDevice: 'default',
                videoInputDevices: [],
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
	const name = document.getElementById('name');
        let req = { MsgType: "registerReq", RegisterRequest: { ClientName: name.value } };
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
		if (msg.RegisterResponse == null || msg.RegisterResponse.ClientId == "") {
                        console.log("no parameter in registerRes");
			return
		}
		const uid = document.getElementById('uid');
		uid.value =  msg.RegisterResponse.ClientId
                console.log("done register");
                return
        } else if (msg.MsgType == "lookupRes") {
		if (msg.Error && msg.Error.Message != "") {
			console.log("could not lookup: " + msg.Error.Message);
			return
		}
		if (msg.LookupResponse == null ) {
			console.log("no parameter in lookupRes");
			return 
		}
		controllerApp.controllers = msg.LookupResponse.Controllers;
		gamepadApp.gamepads = msg.LookupResponse.Gamepads;
		console.log("done lookup clients");
        } else if (msg.MsgType == "sigOfferSdpRes") {
		if (msg.SignalingSdpResponse == null ||
		    msg.SignalingSdpResponse.DeliveryId == "" ||
		    msg.SignalingSdpResponse.ControllerId == "" ||
		    msg.SignalingSdpResponse.GamepadId == "") {
			console.log("no parameter in sigOfferSdpRes");
			hungup();
			return 
		}
		const uid = document.getElementById('uid');
		if (msg.SignalingSdpResponse.DeliveryId != uid.value ||
		    msg.SignalingSdpResponse.ControllerId != controllerApp.selectedController ||
		    msg.SignalingSdpResponse.GamepadId != gamepadApp.selectedGamepad) {
			console.log("ids are mismatch in sigOfferSdpRes");
			hungup();
			return 
		}
		if (msg.Error && msg.Error.Message != "") {
			if (msg.Error.Message == "reject") {
				alert("rejected by peer");
			} else {
				console.log("could not send offer sdp: " + msg.Error.Message);
			}
			hungup();
			return
		}
		completeSdpOffer = true
		console.log("success sendOfferSdp");
                return
        } else if (msg.MsgType == "sigAnswerSdpReq") {
		if (!completeSdpOffer) {
			console.log("not complete offerSdp");
			const uid = document.getElementById('uid');
			let res = { MsgType : "sigAnswerSdpRes",
				    Error: {
					    Message: "not complete offerSdp"
				    },
				    SignalingSdpResponse : {
					    DelivererId: uid.value,
					    ControllerId: controllerApp.selectedController,
					    GamepadId: gamepadApp.selectedGamepad,
				    }
			          };
			websocket.send(JSON.stringify(res));
			hangup()
			return
		}
		if (msg.SignalingSdpRequest == null ||
		    msg.SignalingSdpRequest.DeliveryId == "" ||
		    msg.SignalingSdpRequest.ControllerId == "" ||
		    msg.SignalingSdpRequest.GamepadId == "" ||
		    msg.SignalingSdpRequest.Sdp == "") {
			console.log("no parameter in sigAnswerSdpReq");
			const uid = document.getElementById('uid');
			let res = { MsgType : "sigAnswerSdpRes",
				    Error: {
					    Message: "no parameter in sigAnswerSdpReq"
				    },
				    SignalingSdpResponse : {
					    DelivererId: uid.value,
					    ControllerId: controllerApp.selectedController,
					    GamepadId: gamepadApp.selectedGamepad,
				    }
			          };
			websocket.send(JSON.stringify(res));
			hangup()
			return 
		}
		const uid = document.getElementById('uid');
		if (msg.SignalingSdpRequest.DeliveryId != uid.value ||
		    msg.SignalingSdpRequest.ControllerId != controllerApp.selectedController ||
		    msg.SignalingSdpRequest.GamepadId != gamepadApp.selectedGamepad) {
			console.log("ids are mismatch in sigAnswerSdpReq");
			const uid = document.getElementById('uid');
			let res = { MsgType : "sigAnswerSdpRes",
				    Error: {
					    Message: "ids are mismatch in sigAnswerSdpReq"
				    },
				    SignalingSdpResponse : {
					    DelivererId: uid.value,
					    ControllerId: controllerApp.selectedController,
					    GamepadId: gamepadApp.selectedGamepad,
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
	const uid = document.getElementById('uid');
	if (uid.value == "") {
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
	console.log(uid.value);
	console.log(controllerApp.selectedController);
	console.log(gamepadApp.selectedGamepad);
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
	const name = document.getElementById('name');
	const uid = document.getElementById('uid');
	let req = { MsgType : "sigOfferSdpReq",
		    SignalingSdpRequest : {
			    Name: name,
			    DelivererId: uid,
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
	const uid = document.getElementById('uid');
	let res = { MsgType : "sigAnswerSdpRes",
		    SignalingSdpResponse : {
			    DelivererId: uid.value,
			    ControllerId: controllerApp.selectedController,
			    GamepadId: gamepadApp.selectedGamepad,
		    }
	          };
	websocket.send(JSON.stringify(res));
    } catch(err){
        console.error('setRemoteDescription(answer) ERROR: ', err);
	const uid = document.getElementById('uid');
	let res = { MsgType : "sigAnswerSdpRes",
		    Error: {
			    Message: "could not set remote description"
		    },
		    SignalingSdpResponse : {
			    DelivererId: uid.value,
			    ControllerId: controllerApp.selectedController,
			    GamepadId: gamepadApp.selectedGamepad,
		    }
	          };
	websocket.send(JSON.stringify(res));
	hangUp()
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
	const localVideo = document.getElementById('local_video');
	localVideo.pause();
	localVideo.srcObject = localStream = null;
	completeSdpOffer = false
	completeAnswerSdp = false
}

