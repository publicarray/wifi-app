export namespace main {
	
	export class AccessPoint {
	    bssid: string;
	    ssid: string;
	    vendor: string;
	    frequency: number;
	    channel: number;
	    channelWidth: number;
	    dfs: boolean;
	    signal: number;
	    signalQuality: number;
	    noise: number;
	    txPower: number;
	    security: string;
	    band: string;
	    // Go type: time
	    lastSeen: any;
	    capabilities: string[];
	    beaconInt: number;
	    bsstransition: boolean;
	    uapsd: boolean;
	    fastroaming: boolean;
	    dtim: number;
	    pmf: string;
	    wps: boolean;
	    bssLoadStations: number;
	    bssLoadUtilization: number;
	    maxPhyRate: number;
	    twtSupport: boolean;
	    neighborReport: boolean;
	    mimoStreams: number;
	    estimatedRange: number;
	    snr: number;
	    surveyUtilization: number;
	    surveyBusyMs: number;
	    surveyExtBusyMs: number;
	    maxTxPowerDbm: number;
	    securityCiphers: string[];
	    authMethods: string[];
	    bssColor: number;
	    obssPD: boolean;
	    qamSupport: number;
	    mumimo: boolean;
	    qosSupport: boolean;
	    countryCode: string;
	    apName: string;
	
	    static createFrom(source: any = {}) {
	        return new AccessPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bssid = source["bssid"];
	        this.ssid = source["ssid"];
	        this.vendor = source["vendor"];
	        this.frequency = source["frequency"];
	        this.channel = source["channel"];
	        this.channelWidth = source["channelWidth"];
	        this.dfs = source["dfs"];
	        this.signal = source["signal"];
	        this.signalQuality = source["signalQuality"];
	        this.noise = source["noise"];
	        this.txPower = source["txPower"];
	        this.security = source["security"];
	        this.band = source["band"];
	        this.lastSeen = this.convertValues(source["lastSeen"], null);
	        this.capabilities = source["capabilities"];
	        this.beaconInt = source["beaconInt"];
	        this.bsstransition = source["bsstransition"];
	        this.uapsd = source["uapsd"];
	        this.fastroaming = source["fastroaming"];
	        this.dtim = source["dtim"];
	        this.pmf = source["pmf"];
	        this.wps = source["wps"];
	        this.bssLoadStations = source["bssLoadStations"];
	        this.bssLoadUtilization = source["bssLoadUtilization"];
	        this.maxPhyRate = source["maxPhyRate"];
	        this.twtSupport = source["twtSupport"];
	        this.neighborReport = source["neighborReport"];
	        this.mimoStreams = source["mimoStreams"];
	        this.estimatedRange = source["estimatedRange"];
	        this.snr = source["snr"];
	        this.surveyUtilization = source["surveyUtilization"];
	        this.surveyBusyMs = source["surveyBusyMs"];
	        this.surveyExtBusyMs = source["surveyExtBusyMs"];
	        this.maxTxPowerDbm = source["maxTxPowerDbm"];
	        this.securityCiphers = source["securityCiphers"];
	        this.authMethods = source["authMethods"];
	        this.bssColor = source["bssColor"];
	        this.obssPD = source["obssPD"];
	        this.qamSupport = source["qamSupport"];
	        this.mumimo = source["mumimo"];
	        this.qosSupport = source["qosSupport"];
	        this.countryCode = source["countryCode"];
	        this.apName = source["apName"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ChannelInfo {
	    channel: number;
	    frequency: number;
	    band: string;
	    networkCount: number;
	    networks: string[];
	    utilization: number;
	    congestionLevel: string;
	    overlappingCount: number;
	
	    static createFrom(source: any = {}) {
	        return new ChannelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channel = source["channel"];
	        this.frequency = source["frequency"];
	        this.band = source["band"];
	        this.networkCount = source["networkCount"];
	        this.networks = source["networks"];
	        this.utilization = source["utilization"];
	        this.congestionLevel = source["congestionLevel"];
	        this.overlappingCount = source["overlappingCount"];
	    }
	}
	export class RoamingEvent {
	    // Go type: time
	    timestamp: any;
	    previousBssid: string;
	    newBssid: string;
	    previousSignal: number;
	    newSignal: number;
	    previousChannel: number;
	    newChannel: number;
	
	    static createFrom(source: any = {}) {
	        return new RoamingEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.previousBssid = source["previousBssid"];
	        this.newBssid = source["newBssid"];
	        this.previousSignal = source["previousSignal"];
	        this.newSignal = source["newSignal"];
	        this.previousChannel = source["previousChannel"];
	        this.newChannel = source["newChannel"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SignalDataPoint {
	    // Go type: time
	    timestamp: any;
	    signal: number;
	    bssid: string;
	
	    static createFrom(source: any = {}) {
	        return new SignalDataPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.signal = source["signal"];
	        this.bssid = source["bssid"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ClientStats {
	    connected: boolean;
	    interface: string;
	    ssid: string;
	    bssid: string;
	    frequency: number;
	    channel: number;
	    channelWidth: number;
	    wifiStandard: string;
	    mimoConfig: string;
	    signal: number;
	    signalAvg: number;
	    noise: number;
	    snr: number;
	    txBitrate: number;
	    rxBitrate: number;
	    txBytes: number;
	    rxBytes: number;
	    txPackets: number;
	    rxPackets: number;
	    txRetries: number;
	    txFailed: number;
	    retryRate: number;
	    connectedTime: number;
	    lastAckSignal: number;
	    signalHistory: SignalDataPoint[];
	    roamingHistory: RoamingEvent[];
	
	    static createFrom(source: any = {}) {
	        return new ClientStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.interface = source["interface"];
	        this.ssid = source["ssid"];
	        this.bssid = source["bssid"];
	        this.frequency = source["frequency"];
	        this.channel = source["channel"];
	        this.channelWidth = source["channelWidth"];
	        this.wifiStandard = source["wifiStandard"];
	        this.mimoConfig = source["mimoConfig"];
	        this.signal = source["signal"];
	        this.signalAvg = source["signalAvg"];
	        this.noise = source["noise"];
	        this.snr = source["snr"];
	        this.txBitrate = source["txBitrate"];
	        this.rxBitrate = source["rxBitrate"];
	        this.txBytes = source["txBytes"];
	        this.rxBytes = source["rxBytes"];
	        this.txPackets = source["txPackets"];
	        this.rxPackets = source["rxPackets"];
	        this.txRetries = source["txRetries"];
	        this.txFailed = source["txFailed"];
	        this.retryRate = source["retryRate"];
	        this.connectedTime = source["connectedTime"];
	        this.lastAckSignal = source["lastAckSignal"];
	        this.signalHistory = this.convertValues(source["signalHistory"], SignalDataPoint);
	        this.roamingHistory = this.convertValues(source["roamingHistory"], RoamingEvent);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Network {
	    ssid: string;
	    accessPoints: AccessPoint[];
	    bestSignal: number;
	    bestSignalAP: string;
	    channel: number;
	    security: string;
	    apCount: number;
	    hasIssues: boolean;
	    issueMessages: string[];
	
	    static createFrom(source: any = {}) {
	        return new Network(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ssid = source["ssid"];
	        this.accessPoints = this.convertValues(source["accessPoints"], AccessPoint);
	        this.bestSignal = source["bestSignal"];
	        this.bestSignalAP = source["bestSignalAP"];
	        this.channel = source["channel"];
	        this.security = source["security"];
	        this.apCount = source["apCount"];
	        this.hasIssues = source["hasIssues"];
	        this.issueMessages = source["issueMessages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

