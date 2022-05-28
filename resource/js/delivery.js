let localStream = null;
let signalingSocket = null;
let stopSignalingPingLoopValue = null;
let stopSignalingLookupLoopValue = null;
let peerConnection = null;

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

let peerApp = new Vue({
        el: '#div_for_peers',
        data: {
                selectedPeer: '',
                peers: [],
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
        stopSignalingLookupLoopValue = signalingLookupLoop(signalingSocket)
	const uid = document.getElementById('uid');
        let req = { Command : "registerRequest", Messages : [ uid.value ] };
        signalingSocket.send(JSON.stringify(req));
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
        } else if (msg.Command == "lookupClientsResponse") {
		if (msg.Error != "") {
			console.log("can not lookup clients: " + msg.Error);
		} else {
			console.log("done lookup clients");
			peerApp.peers = msg.Results;
		}
                return
        } else if (msg.Command == "sendOfferSdpResponse") {
		if (msg.Error != "") {
			console.log("can not send offer sdp: " + msg.Error);
		} else {
			console.log("done sendOfferSdp");
		}
                return
        } else if (msg.Command == "sendAnswerSdpRequest") {
                if (msg.Messages.length != 3) {
                        console.log("invalid send answer sdp request");
                        console.log(msg);
                        return
                }
                const uid = document.getElementById('uid');
                if (uid.value != msg.Messages[0]) {
                        console.log("uid mismatch of send answer sdp request");
                        console.log(msg);
                        return
                }
		if (peerApp.selectedPeer != msg.Messages[1]) {
                        console.log("peer uid mismatch of send answer sdp request");
                        console.log(msg);
                        return
		}
                console.log('received answer text');
                const textToReceiveSdp = document.getElementById('text_for_receive_sdp');
                textToReceiveSdp.value = msg.Messages[2];
                const sessionDescription = new RTCSessionDescription({
                        type : 'answer',
                        sdp : msg.Messages[2],
                });
                setAnswer(sessionDescription);
                return
        }
    }
    signalingSocket.onerror = event => {
	stopPingLoop(stopSignalingPingLoopValue);
	stopSignalingLookupLoop(stopSignalingLookupLoopValue)
        console.log("signaling error");
        console.log(event);
    }
    signalingSocket.onclose = event => {
	stopPingLoop(stopSignalingPingLoopValue);
	stopSignalingLookupLoop(stopSignalingLookupLoopValue)
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

function stopPingLoop(stopSignalingPingLoopValue) {
	clearInterval(stopSignalingPingLoopValue);
}

function signalingLookupLoop(socket) {
        return setInterval(() => {
		const uid = document.getElementById('uid');
		let req = { "Command" : "lookupClientsRequest", "Messages" : [ uid.value ] };
                socket.send(JSON.stringify(req));
        }, 5000);
}

function stopSignalingLookupLoop(stopSignalinglookupLoopValue) {
	clearInterval(stopSignalinglookupLoopValue);
}

function startLocalVideo() {
	console.log(peerApp.selectedPeer);
	if (peerApp.selectedPeer == '') {
		console.log("noPeer");
	}
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
	const uid = document.getElementById('uid');
	let req = { "Command" : "sendOfferSdpRequest", "Messages" : [ peerApp.selectedPeer, uid.value, sessionDescription.sdp ] };
        signalingSocket.send(JSON.stringify(req));
}

async function setAnswer(sessionDescription) {
    try{
        await peerConnection.setRemoteDescription(sessionDescription);
        console.log('setRemoteDescription(answer) succsess in promise');
    } catch(err){
        console.error('setRemoteDescription(answer) ERROR: ', err);
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
}

