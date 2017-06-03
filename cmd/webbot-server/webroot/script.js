
var clientURL = "ws://"+window.location.host + "/client";
var videoURL = "ws://"+window.location.host + "/video";
var robot = new Robot(clientURL, videoURL, "view", "chatInput" );

document.onload = function(e) {
	robot.Connect(true); 
}(); 

document.onkeydown = function checkKey(e) {
	robot.handleKey(e,true);
}

document.onkeyup = function checkKey(e) {
	robot.handleKey(e,false);
}

