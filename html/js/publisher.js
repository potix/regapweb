const localVideo = document.getElementById('local_video');
const remoteVideo = document.getElementById('remote_video');
const videoDevices = document.getElementById('div_for_video_devices');
const audioDevices = document.getElementById('div_for_audio_devices');
const audioOutDevices = document.getElementById('div_for_audio_out_devices');
let localStream = null;
let remoteStream = null;

function getDevices() {
    if (!navigator.mediaDevices || !navigator.mediaDevices.enumerateDevices) {
        console.log("enumerateDevices() not supported.");
        return;
    }
    navigator.mediaDevices.enumerateDevices()
    .then(function(devices) {

        let videoSelect = "<select id='select_for_video_devices' >";
        let audioSelect = "<select id='select_for_audio_devices' >";
        let audioOutSelect = "<select id='select_for_audio_out_devices' >";
        devices.forEach(function(device) {
            if (device.kind == "videoinput") {
                videoSelect += "<option value='" + device.deviceId + "'>" + device.label + "</option>";
            } else if (device.kind == "audioinput") {
                audioSelect += "<option value='" + device.deviceId + "'>" + device.label + "</option>";
            } else if (device.kind == "audiooutput") {
	        audioOutSelect += "<option value='" + device.deviceId + "'>" + device.label + "</option>";
	    }
        });
        videoSelect += "</select>";
        audioSelect += "</select>";
        audioOutSelect += "</select>";
        videoDevices.innerHTML = videoSelect;
        audioDevices.innerHTML = audioSelect;
        audioOutDevices.innerHTML = audioOutSelect;
    })
    .catch(function(err) {
        console.log(err.name + ": " + err.message);
    });
}

function getLocalStream() {
    return localStream;
}

async function startVideo() {
    let video = document.getElementById('select_for_video_devices');
    let audio = document.getElementById('select_for_audio_devices');
    try{
        localStream = await navigator.mediaDevices.getUserMedia({video: { deviceId: video.value }, audio: { deviceId: audio.value} });
        playLocalVideo(localStream);
    } catch(err){
        console.error('mediaDevice.getUserMedia() error:', err);
    }
}

// Videoの再生を開始する
async function playLocalVideo(stream) {
    if (localVideo) {
        localVideo.srcObject = stream;
        try {
            await localVideo.play();
        } catch(err) {
            console.log('error auto play:' + err);
        }
    } else {
        console.log('error no local video');
    }
}

function getRemoteStream() {
    if (!remoteStream) {
        console.log('new remote video');
        remoteStream = new MediaStream();
        remoteVideo.srcObject = remoteStream;
    }
    return remoteStream;
}

// Videoの再生を開始する
async function playRemoteVideo() {
    console.log('play remote video');
    try {
        await remoteVideo.play();
    } catch(err) {
        console.log('error auto play:' + err);
    }
}

function cleanupRemoteVideo() {
    remoteVideo.pause();
    remoteVideo.srcObject = null;
    remoteStream = null;
}

getDevices();
