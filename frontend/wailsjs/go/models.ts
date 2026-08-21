export namespace battlelog {
	
	export class DeckCard {
	    id: number;
	    name: string;
	    imageUrl: string;
	    form: string;
	    isSpell: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeckCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.imageUrl = source["imageUrl"];
	        this.form = source["form"];
	        this.isSpell = source["isSpell"];
	    }
	}
	export class DeckMatch {
	    battleTime: string;
	    cards: DeckCard[];
	
	    static createFrom(source: any = {}) {
	        return new DeckMatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.battleTime = source["battleTime"];
	        this.cards = this.convertValues(source["cards"], DeckCard);
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
	export class Result {
	    playerTag: string;
	    matches: DeckMatch[];
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.playerTag = source["playerTag"];
	        this.matches = this.convertValues(source["matches"], DeckMatch);
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

export namespace main {
	
	export class ActionResult {
	    success: boolean;
	    code: string;
	
	    static createFrom(source: any = {}) {
	        return new ActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.code = source["code"];
	    }
	}
	export class AppInfo {
	    version: string;
	    dataDirectory: string;
	    tokenStorage: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.dataDirectory = source["dataDirectory"];
	        this.tokenStorage = source["tokenStorage"];
	    }
	}
	export class AppStatus {
	    state: string;
	    hasToken: boolean;
	    errorCode: string;
	
	    static createFrom(source: any = {}) {
	        return new AppStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.hasToken = source["hasToken"];
	        this.errorCode = source["errorCode"];
	    }
	}
	export class TokenSetupResult {
	    success: boolean;
	    code: string;
	
	    static createFrom(source: any = {}) {
	        return new TokenSetupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.code = source["code"];
	    }
	}

}

export namespace opponentscout {
	
	export class Candidate {
	    playerTag: string;
	    nicknameOriginal: string;
	    rankedRating: number;
	    countryCode: string;
	
	    static createFrom(source: any = {}) {
	        return new Candidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.playerTag = source["playerTag"];
	        this.nicknameOriginal = source["nicknameOriginal"];
	        this.rankedRating = source["rankedRating"];
	        this.countryCode = source["countryCode"];
	    }
	}
	export class SearchResult {
	    candidates: Candidate[];
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.candidates = this.convertValues(source["candidates"], Candidate);
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

