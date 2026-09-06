export namespace backend {
	
	export class ConnectParams {
	    peerAddr: string;
	    password: string;
	    hashes: string[];
	    deviceId?: string;
	    workers?: number;
	    captchaMode?: string;
	    obfsMode?: string;
	    fingerprint?: string;
	    turnTcp?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConnectParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.peerAddr = source["peerAddr"];
	        this.password = source["password"];
	        this.hashes = source["hashes"];
	        this.deviceId = source["deviceId"];
	        this.workers = source["workers"];
	        this.captchaMode = source["captchaMode"];
	        this.obfsMode = source["obfsMode"];
	        this.fingerprint = source["fingerprint"];
	        this.turnTcp = source["turnTcp"];
	    }
	}
	export class LogEntry {
	    level: string;
	    message: string;
	    time: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.message = source["message"];
	        this.time = source["time"];
	        this.count = source["count"];
	    }
	}
	export class ProfileData {
	    peer: string;
	    password: string;
	    hashes: string[];
	    listen: string;
	    turn: string;
	    port: string;
	    device_id: string;
	    turn_tcp: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProfileData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.peer = source["peer"];
	        this.password = source["password"];
	        this.hashes = source["hashes"];
	        this.listen = source["listen"];
	        this.turn = source["turn"];
	        this.port = source["port"];
	        this.device_id = source["device_id"];
	        this.turn_tcp = source["turn_tcp"];
	    }
	}
	export class UpdateInfo {
	    available: boolean;
	    version: string;
	    url: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.version = source["version"];
	        this.url = source["url"];
	        this.body = source["body"];
	    }
	}

}

