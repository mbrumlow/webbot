var ws;
var powerL = 0; 
var powerR = 0; 

var powerLinked = true; 
var username = ""; 
var authenticated = false;
var dir = 0; 

// Web events 
const ROBOT_COMMAND = 2; 

// Robot event commands 
const COMMAND_STOP     = 0; 
const COMMAND_LEFT     = 1; 
const COMMAND_RIGHT    = 2; 
const COMMAND_FORWARD  = 3; 
const COMMAND_BACKWARD = 4; 

document.onload = createWebSocket(); 

function createWebSocket() {

	ws = new WebSocket("ws://" +window.location.host + "/client");  

	ws.onopen =  function () { 

		// Attempt to log in using cookie information. 

		clearChatLog();

		username = getCookie("username"); 
		var authToken = getCookie("authToken");
		authToken = authToken.replace(/['"]+/g, '')

			if( username.length > 0  && authToken.length > 0 ) {

				autoAuth(); 

				ws.send(JSON.stringify({
					Name: username,
					Token: authToken,
				}));

			} else {
				needName();   
			}

	};  

	ws.onmessage = function(event) {

		var je = JSON.parse(event.data);

		switch(je.Type) {

			case 1: // AUTH_OK  
				authOk(je); 
				break;

			case 2: // AUTH_ERROR
				break;

			case 3: // AUTH_USERNAME_IN_USE
				authUserInUse();
				break; 

			case 4: // AUTH_REQUIRE_PASSWORD 
				authPassRequired();
				break;

			case 5: // AUTH_BAD_PASSWORD
				authBadPass(); 
				break;

			case 6: // AUTH_BAD_NAME
				authBadName(); 
				break;

				// ACTIONS

			case 32: // TrackPower
				handleEvent(je, "actionLog") 
					break;
			case 64: // Chat 
				handleEvent(je, "chatLog") 
					break;
			default: 
				console.log("Unknown event: ", je.Type) 
		}

	};

	ws.onclose = function() {
		setTimeout(function() {
			createWebSocket()
		}, 3);
	};  
}

function autoAuth() {

	document.getElementById('authScreen').className = 'visible';    
	document.getElementById('authLoading').className = 'visible';
	document.getElementById('authInput').className = 'visible';
	document.getElementById('authName').className = 'hidden';
	document.getElementById('authPass').className = 'hidden';
	document.getElementById('authRegister').className = 'hidden';
	document.getElementById('authErrorInUse').className = 'authError hidden';
	document.getElementById('AuthErrorBadPass').className = 'authError hidden';
	document.getElementById('AuthErrorBadName').className = 'authError hidden';

	authenticated = false; 
}

function needName() {

	document.getElementById('authScreen').className = 'visible';    
	document.getElementById('authLoading').className = 'hidden';
	document.getElementById('authInput').className = 'visible';
	document.getElementById('authName').className = 'visible';
	document.getElementById('authPass').className = 'hidden';
	document.getElementById('authRegister').className = 'hidden';
	document.getElementById('authErrorInUse').className = 'authError hidden';
	document.getElementById('AuthErrorBadPass').className = 'authError hidden';
	document.getElementById('AuthErrorBadName').className = 'authError hidden';

	document.getElementById("nameInput").value = ""; 
	document.getElementById("nameInput").focus(); 

	authenticated = false; 
}

function authOk(je) {

	token = JSON.parse(je.Event);

	document.getElementById('authScreen').className = 'hidden';    
	document.getElementById('authLoading').className = 'hidden';
	document.getElementById('authInput').className = 'hidden';
	document.getElementById('authName').className = 'hidden';
	document.getElementById('authPass').className = 'hidden';
	document.getElementById('authRegister').className = 'hidden';
	document.getElementById('authErrorInUse').className = 'authError hidden';
	document.getElementById('AuthErrorBadPass').className = 'authError hidden';

	setCookie("authToken", token); 
	setCookie("username", username);

	document.getElementById("userName").innerHTML = username;

	authenticated = true;

}

function authUserInUse() {

	document.getElementById('authScreen').className = 'visible';    
	document.getElementById('authLoading').className = 'hidden';
	document.getElementById('authInput').className = 'visible';
	document.getElementById('authName').className = 'visible';
	document.getElementById('authPass').className = 'hidden';
	document.getElementById('authRegister').className = 'hidden';
	document.getElementById('authErrorInUse').className = 'authError visible';
	document.getElementById('AuthErrorBadPass').className = 'authError hidden';
	document.getElementById('AuthErrorBadName').className = 'authError hidden';

	authenticated = false;
}

function authPassRequired() {

	document.getElementById('authScreen').className = 'visible';    
	document.getElementById('authLoading').className = 'hidden';
	document.getElementById('authInput').className = 'visible';
	document.getElementById('authName').className = 'hidden';
	document.getElementById('authPass').className = 'visible';
	document.getElementById('authRegister').className = 'hidden';
	document.getElementById('authErrorInUse').className = 'authError hidden';
	document.getElementById('AuthErrorBadPass').className = 'authError hidden';
	document.getElementById('AuthErrorBadName').className = 'authError hidden';

	document.getElementById("passInput").value = ""; 
	document.getElementById("passInput").focus(); 

	authenticated = false;
}

function authBadPass() {

	document.getElementById('authScreen').className = 'visible';    
	document.getElementById('authLoading').className = 'hidden';
	document.getElementById('authInput').className = 'visible';
	document.getElementById('authName').className = 'hidden';
	document.getElementById('authPass').className = 'visible';
	document.getElementById('authRegister').className = 'hidden';
	document.getElementById('authErrorInUse').className = 'authError hidden';
	document.getElementById('AuthErrorBadPass').className = 'authError visible';
	document.getElementById('AuthErrorBadName').className = 'authError hidden';

	document.getElementById("passInput").value = ""; 
	document.getElementById("passInput").focus(); 

	authenticated = false;
}

function authBadName() {

	document.getElementById('authScreen').className = 'visible';    
	document.getElementById('authLoading').className = 'hidden';
	document.getElementById('authInput').className = 'visible';
	document.getElementById('authName').className = 'visible';
	document.getElementById('authPass').className = 'hidden';
	document.getElementById('authRegister').className = 'hidden';
	document.getElementById('authErrorInUse').className = 'authError hidden';
	document.getElementById('AuthErrorBadPass').className = 'authError hidden';
	document.getElementById('AuthErrorBadName').className = 'authError visible';

	document.getElementById("nameInput").value = ""; 
	document.getElementById("nameInput").focus(); 

	authenticated = false;
}

function authRegister() {

	document.getElementById('authScreen').className = 'visible';    
	document.getElementById('authLoading').className = 'hidden';
	document.getElementById('authInput').className = 'visible';
	document.getElementById('authName').className = 'hidden';
	document.getElementById('authPass').className = 'hidden';
	document.getElementById('authRegister').className = 'visible';
	document.getElementById('authErrorInUse').className = 'authError hidden';
	document.getElementById('AuthErrorBadPass').className = 'authError hidden';
	document.getElementById('AuthErrorBadName').className = 'authError hidden';

}

document.onkeydown = function checkKey(e) {

	e = e || window.event;

	var chatEnabled = false; 
	if( document.getElementById("txtArea").value.length != 0 ) {
		chatEnabled = true;   
	}

	switch(e.keyCode) {
		case 38:
			if(!chatEnabled) {
				e.preventDefault();
				if(dir != 1 ) {
					fullForward();
				}
			}
			break;  
		case 40: 
			if(!chatEnabled) { 
				e.preventDefault();
				if( dir != -1 ) { 
					fullReverse(); 
				}
			}
			break; 
		case 37:
			if(!chatEnabled) { 
				e.preventDefault();
				if( dir != 3 ) {
					rotateLeft();
				}
			}
			break;
		case 39:
			if(!chatEnabled) {  
				e.preventDefault();
				if( dir != 2 ) { 
					rotateRight();
				}
			}
			break;
		case 13:
			if(chatEnabled) {  
				sendChat(); 
			}
			break; 
		default: 
	}
}

document.onkeyup = function checkKey(e) {

	e = e || window.event;

	var chatEnabled = false; 
	if( document.getElementById("txtArea").value.length != 0 ) {
		chatEnabled = true;   
	}

	switch(e.keyCode) {
		case 38:
		case 40: 
		case 37:
		case 39:
			if(!chatEnabled) {  
				e.preventDefault();
				fullStop();
			}
			break;
		default: 
	}
}
function clearChatLog() {
	var node = document.getElementById("chatLog");
	while (node && node.firstChild) {
		node.removeChild(node.firstChild);
	}
}

function handleEvent(je, type) {

	ev = JSON.parse(je.Event)

		var elem = document.getElementById(type);
	children = elem.children;

	if( children.length > 100 ) {
		elem.removeChild(elem.firstChild);    
	}

	var lastChatIndex = -1;
	if( children.length > 0 ) {
		lastChatIndex = elem.lastChild.getAttribute("chatIndex");
	}

	if( lastChatIndex < ev.Id ) {
		insertEventFast(je, ev, type) 	
	} else {
		insertEventSlow(je, ev, type) 	
	}

} 

function insertEventSlow(je, ev, type) {

	var elem = document.getElementById(type);
	children = elem.children;

	var node = document.createElement("div");
	var textnode = document.createTextNode(ev.Time + ": " + je.UserInfo.Name.substring(0, 10)  + " > " + ev.Action);
	node.title = je.UserInfo.Name;
	node.setAttribute("chatIndex", ev.Id); 
	node.appendChild(textnode); 

	for(var i = 0; i < children.length; i++) {
		lastChatIndex = children[i].getAttribute("chatIndex");
		if(lastChatIndex > ev.Id) {
			elem.insertBefore(node, children[i]);
			elem.scrollTop = elem.scrollHeight;	
			return 
		}	
		if(lastChatIndex == ev.Id) {
			return
		}
	}

	elem.appendChild(node); 
	elem.scrollTop = elem.scrollHeight;

}

function insertEventFast(je, ev, type) {

	var elem = document.getElementById(type);

	var node = document.createElement("div");
	var textnode = document.createTextNode(ev.Time + ": " + je.UserInfo.Name.substring(0, 10)  + " > " + ev.Action);
	node.title = je.UserInfo.Name;
	node.setAttribute("chatIndex", ev.Id); 
	node.appendChild(textnode); 

	elem.appendChild(node); 
	elem.scrollTop = elem.scrollHeight;

}

function sendName() {

	username = document.getElementById("nameInput").value; 

	ws.send(JSON.stringify({
		Name: username,
	}));

	document.getElementById("nameInput").value = "";

	return false;

}

function sendRegister() {

	var password = document.getElementById("passInputA").value; 
	var password = SHA256(password); 

	var info = {};
	info["Type"] = 128; // REGISTER_EVENT
	info["Event"] = password;

	document.getElementById("passInputA").value = "";
	document.getElementById("passInputB").value = "";

	ws.send(JSON.stringify(info));
	return false;

}

function sendPass() {

	password = document.getElementById("passInput").value;
	password = SHA256(password); 
	ws.send(JSON.stringify({
		Pass: password,
	}));

	document.getElementById("passInput").value = ""; 

	autoAuth(); 

	return false;
}

function sendCommand(command) {

	var je = {}; 
	je["Type"] = ROBOT_COMMAND;  
	je["Event"] = JSON.stringify(command);

	ws.send(JSON.stringify(je));
}

function powerRight(p) {
	powerR = p; 
}

function powerLeft(p) {
	powerL = p;   
}

function rotateLeft() {
	dir = 3; 
	sendCommand(COMMAND_LEFT); 
}

function rotateRight() {
	dir = 2; 
	sendCommand(COMMAND_RIGHT); 
}

function fullForward() {
	dir = 1; 
	sendCommand(COMMAND_FORWARD); 
}

function fullStop() {
	dir = 0; 
	sendCommand(COMMAND_STOP);
}

function fullReverse() {
	dir = -1;
	sendCommand(COMMAND_BACKWARD); 
}

function toggleLinked() {

	b = document.getElementById("linkedButton"); 

	if(powerLinked) {
		powerLinked = false;
		b.innerHTML = "&nhArr;" 
	} else {
		powerLinked = true; 
		b.innerHTML = "&hArr;" 
	}
}

function sendChat() {

	if(!authenticated) {
		return;
	}

	if( document.getElementById("txtArea").value.length == 0 ) {
		return;
	}

	// Local commands
	if( document.getElementById("txtArea").value == "/register" ) {
		authRegister();
		document.getElementById("txtArea").value = '';
		return;
	}

	if( document.getElementById("txtArea").value == "/logout" ) {
		logOut();
		document.getElementById("txtArea").value = '';
		return;
	}

	var info = {};
	info["Type"] = 64; // CHAT_EVENT
	info["Event"] = document.getElementById("txtArea").value;

	ws.send(JSON.stringify(info));
	document.getElementById("txtArea").value = '';

}

function logOut() {
	deleteCookie("username"); 
	deleteCookie("authToken"); 
	ws.close();
	document.getElementById("userName").innerHTML = '';
	return false;
}

function deleteCookie(cname) {
	setCookie(cname, "", -1) 
}

function setCookie(cname, cvalue, exdays) {
	var d = new Date();
	d.setTime(d.getTime() + (exdays*24*60*60*1000));
	var expires = "expires="+d.toUTCString();
	document.cookie = cname + "=" + cvalue + "; " + expires;
}

function getCookie(cname) {
	var name = cname + "=";
	var ca = document.cookie.split(';');
	for(var i=0; i<ca.length; i++) {
		var c = ca[i];
		while (c.charAt(0)==' ') c = c.substring(1);
		if (c.indexOf(name) == 0) return c.substring(name.length,c.length);
	}
	return "";
}

function canvasEvent(event) {
	var ex = event.pageX - vcanvas.offsetLeft; 
	var ey = event.pageY - vcanvas.offsetTop;
	handleCanvasXY(ex,ey); 
}

function canvasTouchEvent(event) {
	var ex = event.touches[0].pageX - vcanvas.offsetLeft;
	var ey = event.touches[0].pageY - vcanvas.offsetTop;
	handleCanvasXY(ex,ey); 
}

function handleCanvasXY(ex, ey) {

	if( ey < 150 ) {
		// Forward 
		fullForward();
		return;
	} else if ( ey > 330 ) {
		// Backward 
		fullReverse(); 
		return;
	}

	if( ex < 150 ) {
		// Left
		rotateLeft();
		return;
	} else if ( ex > 490 ) {
		// Right
		rotateRight();
		return;
	}	

	fullStop();
	// All else STOP!
}

var vcanvas;
function  initializeCavasTouch() { 
	vcanvas = document.getElementById('videoCanvas');

	vcanvas.addEventListener('touchstart', function(event) {
		if(event.touches) {
			canvasTouchEvent(event);	
		} else {
			canvasEvent(event);
		}	
		event.preventDefault();
	}, false);

	vcanvas.addEventListener('mousedown', function(event) {
		canvasEvent(event);
		event.preventDefault();
	}, false);
	
	vcanvas.addEventListener('mouseup', function(event) {
		fullStop();
		event.preventDefault();
	}, false);

	vcanvas.addEventListener('touchend', function(event) {
		fullStop();
		event.preventDefault();
	}, false);
	
	vcanvas.addEventListener('touchcancel', function(event) {
		fullStop();
		event.preventDefault();
	}, false);

	document.addEventListener('gesturestart', function (e) {
		event.preventDefault();
	}, false);
}

