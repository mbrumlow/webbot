
class Robot {
	
	constructor(clientURL, videoURL, view, chatInput) {
		console.log("Robot constructor"); 
		this.clientURL = clientURL;
		this.videoURL = videoURL;
		this.view = view; 
		this.capMap = {};
		this.ctrlMap = {};
		this.lsh = 0; 
		this.lsl = 0; 
		this.chatInput = chatInput;
		this.activeUsers = {}; 
		this.vs = null;
		this.jsmpeg = null;  
		this.infoCapz = 0; 
		this.reconnect = true;
	}

	Destroy() {

		this.reconnect = false; 
		
		if(this.ws != undefined && this.ws != null) {
			this.ws.onclose = {};
			this.ws.close(); 
		}	
	
		if(this.jsmpeg != undefined || this.jsmpeg != null) { 
			this.jsmpeg.destroy();
			this.jsmpeg = null; 
		}
	}

	Connect() {
		this.ws = new WebSocket(this.clientURL); 
		this.ws.binaryType = 'arraybuffer';
		this.ws.robot = this; 
		this.ws.onopen = function() {
			
			console.log("websocket open"); 
		
			this.infoCapz = 0; 
			var elem = document.getElementById("conStatus");
			if(elem) {
				elem.parentNode.removeChild(elem);
			}

		}
		
		this.ws.onmessage = function(e) {
			var data = e.data;
			var dv = new DataView(data);
			var type = dv.getUint32(0); 
			this.robot.handleMsg(type, data.slice(4)); 
		}
		
		this.ws.onclose = function() {
			this.robot.handleDissconnect();  
			if(this.robot.reconnect) { 
				var r = this.robot; 
				setTimeout(function() {
					r.Connect()
				}, 3);
			}
		}
	}

	handleMsg(id, msg) {

		switch(id) {
			case 1: 
				this.handleStatus(msg, true); 
				break;
			case 2: 
				this.handleStatus(msg, false); 
				break;
			case 3: 
				this.handleChatCap(msg); 
				break;
			case 4: 
				this.handleCookCap(msg); 
				break;
			case 100: 
				this.handleInfoCapDef(msg);
				break;
			case 101: 
				this.handleCtrlCapDef(msg); 
				break; 
			case 102: 
				this.handleUserDef(msg); 
				break; 
			default: 
				if(this.capMap[id]) {
					this.capMap[id].function(this.capMap[id].cap, msg);
				} else {
					console.log("Unknown cap:", id);
				}
		}

	}

	handleDissconnect() {
			
		this.stopVideo(); 

		var view = document.getElementById("view"); 

		while (view.firstChild) {
			view.removeChild(view.firstChild);
		}

		this.capMap = {};
		this.ctrlMap = {};

		var c = document.getElementById("conStatus");
		if(!c) {
			c = document.createElement("canvas");
			c.id = "conStatus";
			c.width = 640; 
			c.height = 480; 

			var ctx = c.getContext("2d");
			//ctx.translate(0.5, 0.5);

			var style = "position: absolute; left: 0; top: 0; z-index: 1;";
			c.setAttribute("style", style);
			view.appendChild(c); 
		}

		var ctx = c.getContext("2d");
		ctx.font = "12px Monospace";
		ctx.clearRect(0, 0, c.width, c.height);   

		ctx.fillStyle = "red";
		ctx.fillText("Dissconnected", 5 , 472);
	}
	
	handleStatus(data, online) {

		var dv = new DataView(data);
		var th = dv.getUint32(0); 
		var tl = dv.getUint32(4); 

		if(this.lsh != 0 && this.lsl != 0 && this.lsh > th && this.lsl > tl ) {
			return
		}

		this.lsh = th; 
		this.lsl = tl;

		if(online) {
			console.log("Robot online.") 
		} else {
			console.log("Robot offline.") 
		}

		var view = document.getElementById("view"); 

		if(!online) { 
			while (view.firstChild) {
				view.removeChild(view.firstChild);
			}
			this.capMap = {};
			this.ctrlMap = {};
		}

		var c = document.getElementById("capStatus");
		if(!c) {
			c = document.createElement("canvas");
			c.id = "capStatus";
			c.width = 640; 
			c.height = 480; 

			var ctx = c.getContext("2d");
			var style = "position: absolute; left: 0; top: 0; z-index: 1;";
			c.setAttribute("style", style);
			view.appendChild(c); 
		}

		var ctx = c.getContext("2d");
		ctx.font = "12px Monospace";
		ctx.clearRect(0, 0, c.width, c.height);   

		if(online) {
			Robot.drawStatusText(c, "Robot Online", 5, 472, "#41F427"); 
			this.startVideo(); 
		} else {
			var ctx = c.getContext("2d");
			ctx.font = "12px Monospace";
			ctx.clearRect(0, 0, c.width, c.height);   
			ctx.fillStyle = "red";
			ctx.fillText("Robot Offline", 5 , 472);
			this.stopVideo(); 
		}
	}

	startVideo() {

		var view = document.getElementById("view"); 

		var c = document.createElement("canvas");
		c.width = 640; 
		c.height = 480; 

		var style = "position: relative; left: 0; top: 0; z-index: 0;";
		c.setAttribute("style", style);
		view.appendChild(c); 

		this.jsmpeg = new JSMpeg.Player(this.videoURL, {canvas: c, disableGl: false, pauseWhenHidden: false, chunkSize: 512 });
	
	}

	stopVideo() {

		if(this.jsmpeg == null) {
			return;
		}

		this.jsmpeg.destroy();
		this.jsmpeg = null; 
	}

	handleCookCap(data) {
		
		var dv = new DataView(data);
		var th = dv.getUint32(0); 
		var tl = dv.getUint32(4); 
		
		var msg = String.fromCharCode.apply(null, new Uint8Array(data.slice(12)));

		var d = new Date();
		d.setTime(d.getTime() + (30*24*60*60*1000));
		var expires = "expires="+d.toUTCString();
		document.cookie = "CHAT_NAME=" + msg + "; " + expires;
	}

	handleUserDef(data) {
		
		var dv = new DataView(data);
		var th = dv.getUint32(0); 
		var tl = dv.getUint32(4); 
		var i   = dv.getUint32(8); 
		var idh = dv.getUint32(12); 
		var idl = dv.getUint32(16); 
		
		var msg = String.fromCharCode.apply(null, new Uint8Array(data.slice(20)));

		if(i > 0) { 
		
			if( this.activeUsers[idh] == undefined ) {
				this.activeUsers[idh] = {} 
			}

			this.activeUsers[idh][idl] = { 
				name: msg, 
				th: th, 
				tl: tl, 
			}

		} else {
			// TODO delete keys if empty. 
			delete this.activeUsers[idh][idl]
		}
	}

	handleInfoCapDef(data) {

		var dv = new DataView(data);
		var id = dv.getUint32(0); 
		var v  = dv.getUint32(4); 
		var f  = dv.getUint32(8);
		var th = dv.getUint32(12); 
		var tl = dv.getUint32(16); 

		var msg = String.fromCharCode.apply(null, new Uint8Array(data.slice(20)));
		var cap = JSON.parse(msg);

		var c = document.createElement("canvas");
		c.id = "infoCap" + id;
		c.width = 640; 
		c.height = 480; 

		var ctx = c.getContext("2d");
		//ctx.translate(0.5, 0.5);

		ctx.mozImageSmoothingEnabled = false;
		ctx.webkitImageSmoothingEnabled = false;
		ctx.msImageSmoothingEnabled = false;
		ctx.imageSmoothingEnabled = false;

		this.infoCapz += 1
		var style = "position: absolute; left: 0; top: 0; z-index: " + this.infoCapz + ";";
		c.setAttribute("style", style);

		var view = document.getElementById("view"); 
		view.appendChild(c); 

		// If the one we have is newer lets keep that. 
		if(this.capMap[id] && this.capMap[id].th > th && this.capMap[id].tl > tl) {
			return
		}

		this.capMap[id] ={
			th: th, 
			tl: tl, 
			cth: 0, 
			ctl: 0, 
			cap: cap, 
			function: function(cap, data) {

				var dv = new DataView(data);
				var cth = dv.getUint32(0); 
				var ctl = dv.getUint32(4); 

				if( cap.cth > cth && cap.ctl > ctl) {
					return;
				}

				var msg = String.fromCharCode.apply(null, new Uint8Array(data.slice(8)));

				cap.cth = cth; 
				cap.ctl = ctl; 

				Robot.drawStatusTextID("infoCap"+id, cap.Lable + ": " +msg,cap.X, cap.Y, "white");
			}, 
		};
	}

	static drawStatusTextID(id, text, x, y, color) {
		var c = document.getElementById(id);
		Robot.drawStatusText(c, text, x, y, color); 
	}
	
	static drawStatusText(c, text, x, y, color) {

		var scale = 2; 
		var ocanvas = document.createElement('canvas');
		var octx = ocanvas.getContext('2d');

		ocanvas.width = c.width * scale; 
		ocanvas.height = c.height * scale;

		var fs = 14 * scale; 
		octx.font = "bold " + fs + "px Monospace";
		octx.strokeStyle = 'black';
		octx.lineWidth = 2;

		octx.strokeText(text, x*scale, y*scale);
		octx.fillStyle = color;
		octx.fillText(text, x*scale, y*scale);

		var ctx = c.getContext("2d");
		ctx.clearRect(0, 0, c.width, c.height);   
		ctx.drawImage(ocanvas, 0, 0, c.width, c.height);
	}

	static drawStatusTextNB(c, text, x, y, color) {

		var ctx = c.getContext("2d");

		ctx.font = "bold 14px Monospace";
		ctx.clearRect(0, 0, c.width, c.height);   
		ctx.fillStyle = color;
		ctx.fillText(text, x, y);
	}

	handleCtrlCapDef(data) {

		var dv = new DataView(data);
		var id = dv.getUint32(0); 
		var v  = dv.getUint32(4); 
		var f  = dv.getUint32(8);
		var th = dv.getUint32(12); 
		var tl = dv.getUint32(16); 

		var msg = String.fromCharCode.apply(null, new Uint8Array(data.slice(20)));
		var cap = JSON.parse(msg);

		// If the one we have is newer lets keep that. 
		if(this.capMap[id] && this.capMap[id].th > th && this.capMap[id].tl > tl) {
			return
		}

		this.capMap[id] ={
			th: th, 
			tl: tl, 
			cap: cap, 
		};

		this.ctrlMap[cap.KeyCode] ={
			id: id, 
			th: th, 
			tl: tl, 
			cap: cap, 
			down: false, 
		};

	}

	getUserName(idh, idl) {

		if( this.activeUsers[idh] != undefined && 
			this.activeUsers[idh][idl] != undefined ) {
			return this.activeUsers[idh][idl].name;
		}

		return "unknown"; 
	}

	handleChatCap(data) {

		var dv = new DataView(data);
		var idh = dv.getUint32(0); 
		var idl = dv.getUint32(4); 
		var coh = dv.getUint32(8); 
		var col = dv.getUint32(12); 
	
		var name = this.getUserName(idh, idl); 

		var str = String.fromCharCode.apply(null, new Uint16Array(data.slice(16)));
	
		var elem = document.getElementById("chatLog");
		var children = elem.children;
	
		if( children.length > 100 ) {
			elem.removeChild(elem.firstChild);    
		}

		var lcoh = -1;
		var lcol = -1;
		if( children.length > 0 ) {
			var lastChild = elem.lastChild;
			lcoh = parseInt(lastChild.getAttribute("coh"));
			lcol = parseInt(lastChild.getAttribute("col"));
		}

		if( lcoh <= coh && lcol < col ) {
			this.insertChatFast(name, str, coh, col, "chatLog") 
		} else { 
			this.insertChatSlow(name, str, coh, col, "chatLog") 
		}
	}
	
	insertChatFast(n, msg, coh, col, type) {

		var elem = document.getElementById(type);

		var node = document.createElement("div");
		var p = document.createElement("p");
		
		p.setAttribute("style", "margin: 0px;" );

		node.setAttribute("coh", coh); 
		node.setAttribute("col", col); 

		var spanNodeName = document.createElement("span"); 
		spanNodeName.innerHTML = n + ": ";
		spanNodeName.setAttribute("style", "font-weight: bold;" );
		var spanNodeChat = document.createElement("span"); 
		spanNodeChat.innerHTML = msg; 
		p.appendChild(spanNodeName);
		p.appendChild(spanNodeChat);
		node.appendChild(p); 

		elem.appendChild(node); 
		elem.scrollTop = elem.scrollHeight;

	}

	insertChatSlow(n, msg, coh, col, type) {

		var elem = document.getElementById(type);
		var children = elem.children;

		var node = document.createElement("div");
		var p = document.createElement("p");
		
		p.setAttribute("style", "margin: 0px;" );

		node.setAttribute("coh", coh); 
		node.setAttribute("col", col); 
		
		var spanNodeName = document.createElement("span"); 
		spanNodeName.innerHTML = n + ": ";
		spanNodeName.setAttribute("style", "font-weight: bold;" );
		var spanNodeChat = document.createElement("span"); 
		spanNodeChat.innerHTML = msg; 
		p.appendChild(spanNodeName);
		p.appendChild(spanNodeChat);
		node.appendChild(p); 

		for(var i = 0; i < children.length; i++) {
			var lcoh = children[i].getAttribute("coh");
			var lcol = children[i].getAttribute("col");
			if(lcoh > coh && lcol > col) {
				elem.insertBefore(node, children[i]);
				elem.scrollTop = elem.scrollHeight;	
				return 
			}	
			if(lcoh == coh && lcol == col) {
				return
			}
		}

		elem.appendChild(node); 
		elem.scrollTop = elem.scrollHeight;

	}

	sendChat() {
		
		if( document.getElementById(this.chatInput).innerHTML == "" ) {
			return;
		}

		var str = document.getElementById(this.chatInput).innerHTML.replace(/&nbsp;/gi, '');;
		var buf = new ArrayBuffer(8 + str.length*2);
		var dv = new DataView(buf);
		dv.setUint32(0, buf.byteLength - 4); 
		dv.setUint32(4, 3); // CHAT CAP

		for(var i = 0, len=str.length; i<len; i++) {
			dv.setUint16(8 + (i * 2),str.charCodeAt(i), true); 
		}

		this.ws.send(buf); 

		document.getElementById(this.chatInput).innerHTML = ""; 
	}

	handleKey(e, d) {
				
		if(d && e.keyCode == 13) {
			e.preventDefault();
			if( document.getElementById(this.chatInput).innerHTML != "" ) {
				this.sendChat(); 
			}
			return;
		}

		if(this.ctrlMap[e.keyCode]) {
			if(this.ctrlMap[e.keyCode] && 
				this.ctrlMap[e.keyCode].cap.Alt == e.altKey && 
				this.ctrlMap[e.keyCode].cap.Ctrl == e.ctrlKey && 
				this.ctrlMap[e.keyCode].cap.Shift == e.shiftKey ) {
		
				e.preventDefault();

				if(this.ctrlMap[e.keyCode].down != d) { 

					this.ctrlMap[e.keyCode].down = d;
					if(this.ctrlMap[e.keyCode].cap.Toggle && !d) { 
						return
					}

					var buf = new ArrayBuffer(16);
					var dv = new DataView(buf);
					dv.setUint32(0, 12); 
					dv.setUint32(4, 101); // CTRL CAP
					dv.setUint32(8, this.ctrlMap[e.keyCode].id); 
					if(d) { 
						dv.setUint32(12,1); 
					} else {
						dv.setUint32(12,0); 
					}
					this.ws.send(buf); 
				}
			}
		}
	}
}


