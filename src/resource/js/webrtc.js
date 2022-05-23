const textForSendSdp = document.getElementById('text_for_send_sdp');
const textToReceiveSdp = document.getElementById('text_for_receive_sdp');
let peerConnection = null;
let isOffer = false;

function prepareNewConnection(isOffer) {
    console.log('prepareNewConnection');
    // const iceServers = [ {"urls":"stun:stun.webrtc.ecl.ntt.com:3478"} ]
    const iceServers = [
        {
	  urls: "stun:stun1.l.google.com:19302",
        },
        {
          urls: "stun:stun2.l.google.com:19302",
        },
        {
          urls: "stun:stun3.l.google.com:19302",
        },
        {
          urls: "stun:stun4.l.google.com:19302",
        },
	{
	  urls: "stun:stun.webrtc.ecl.ntt.com:3478",
	},
        //{
        //    url: 'turn:numb.viagenie.ca',
        //    credential: 'muazkh',
        //    username: 'webrtc@live.com'
        //},
    ]
    const pc_config = {"iceServers": iceServers };
    const peer = new RTCPeerConnection(pc_config);

    peer.onnegotiationneeded = async () => {
        console.log('onnegotiationneeded');
        try {
            if(isOffer){
                let offer = await peer.createOffer();
                console.log('createOffer() succsess in promise');
                await peer.setLocalDescription(offer);
                console.log('setLocalDescription() succsess in promise');
            }
        } catch(err){
            console.error('setLocalDescription(offer) ERROR: ', err);
        }
    }

    // ICE Candidateを収集したときのイベント
    peer.onicecandidate = evt => {
        console.log('onicecandidate');
        if (evt.candidate) {
            console.log(evt.candidate);
        } else {
            console.log('empty ice event');
	    console.log(peer.localDescription);
            sendSdp(peer.localDescription);
        }
    };

    peer.oniceconnectionstatechange = function() {
        console.log('ICE connection Status has changed to ' + peer.iceConnectionState);
        switch (peer.iceConnectionState) {
            case 'closed':
            case 'failed':
                if (peerConnection) {
                    hangUp();
                }
                break;
            case 'disconnected':
                break;
        }
    };

    // リモートのMediaStreamTrackを受信した時
    peer.ontrack = evt => {
        console.log('-- peer.ontrack()');
        console.log(evt.track);
        console.log('Adding remote stream...');
	remoteStream = getRemoteStream();
	remoteStream.addTrack(evt.track);
    };

    // ローカルのMediaStreamを利用できるようにする
    localStream = getLocalStream();
    if (localStream) {
        console.log('Adding local stream...');
	for (const track of localStream.getTracks()) {
	    console.log(track);
            peer.addTrack(track);
        }
    } else {
        console.warn('no local stream, but continue.');
    }

    return peer;
}

// 手動シグナリングのための処理を追加する
function sendSdp(sessionDescription) {
    console.log('---sending sdp ---');
    textForSendSdp.value = sessionDescription.sdp;
    textForSendSdp.focus();
    textForSendSdp.select();
}

function connect() {
    if (! peerConnection) {
        console.log('make Offer');
        peerConnection = prepareNewConnection(true);
    }
    else {
        console.warn('peer already exist.');
    }
}

async function makeAnswer() {
    console.log('sending Answer. Creating remote session description...' );
    if (! peerConnection) {
        console.error('peerConnection NOT exist!');
        return;
    }
    try{
        let answer = await peerConnection.createAnswer();
        console.log('createAnswer() succsess in promise');
        await peerConnection.setLocalDescription(answer);
        console.log('setLocalDescription() succsess in promise');
        sendSdp(peerConnection.localDescription);
    } catch(err){
        console.error(err);
    }
}

// Receive remote SDPボタンが押されたらOffer側とAnswer側で処理を分岐
function onSdpText() {
    const text = textToReceiveSdp.value;
    if (peerConnection) {
        console.log('Received answer text...');
        const answer = new RTCSessionDescription({
            type : 'answer',
            sdp : text,
        });
        setAnswer(answer);
    }
    else {
        console.log('Received offer text...');
        const offer = new RTCSessionDescription({
            type : 'offer',
            sdp : text,
        });
        setOffer(offer);
    }
    textToReceiveSdp.value ='';
}

// Offer側のSDPをセットする処理
async function setOffer(sessionDescription) {
    if (peerConnection) {
        console.error('peerConnection alreay exist!');
    }
    peerConnection = prepareNewConnection(false);
    try{
        await peerConnection.setRemoteDescription(sessionDescription);
        console.log('setRemoteDescription(offer) succsess in promise');
        makeAnswer();
	playRemoteVideo();
    } catch(err){
        console.error('setRemoteDescription(offer) ERROR: ', err);
    }
}

// Answer側のSDPをセットする場合
async function setAnswer(sessionDescription) {
    if (! peerConnection) {
        console.error('peerConnection NOT exist!');
        return;
    }
    try{
        await peerConnection.setRemoteDescription(sessionDescription);
        console.log('setRemoteDescription(answer) succsess in promise');
	playRemoteVideo();
    } catch(err){
        console.error('setRemoteDescription(answer) ERROR: ', err);
    }
}

// P2P通信を切断する
function hangUp(){
    if (peerConnection) {
        if(peerConnection.iceConnectionState !== 'closed'){
            peerConnection.close();
            peerConnection = null;
            //negotiationneededCounter = 0;
            cleanupRemoteVideo();
            textForSendSdp.value = '';
            return;
        }
    }
    console.log('peerConnection is closed.');
}
