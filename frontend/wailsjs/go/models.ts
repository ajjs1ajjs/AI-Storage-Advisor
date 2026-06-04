export namespace cleanup {
	
	export class DryRunResult {
	    total_count: number;
	    total_size: number;
	    total_size_formatted: string;
	    writable_files: any[];
	    restricted_files: any[];
	    can_proceed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DryRunResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_count = source["total_count"];
	        this.total_size = source["total_size"];
	        this.total_size_formatted = source["total_size_formatted"];
	        this.writable_files = source["writable_files"];
	        this.restricted_files = source["restricted_files"];
	        this.can_proceed = source["can_proceed"];
	    }
	}

}

export namespace forecast {
	
	export class TrendPoint {
	    days: number;
	    scan_time: string;
	    actual_size: number;
	    projected_size: number;
	
	    static createFrom(source: any = {}) {
	        return new TrendPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.days = source["days"];
	        this.scan_time = source["scan_time"];
	        this.actual_size = source["actual_size"];
	        this.projected_size = source["projected_size"];
	    }
	}
	export class ForecastResult {
	    status: string;
	    message: string;
	    days_remaining: number;
	    daily_growth_bytes: number;
	    free_bytes: number;
	    total_bytes: number;
	    trend_points: TrendPoint[];
	
	    static createFrom(source: any = {}) {
	        return new ForecastResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.message = source["message"];
	        this.days_remaining = source["days_remaining"];
	        this.daily_growth_bytes = source["daily_growth_bytes"];
	        this.free_bytes = source["free_bytes"];
	        this.total_bytes = source["total_bytes"];
	        this.trend_points = this.convertValues(source["trend_points"], TrendPoint);
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
	
	export class ConnectionResult {
	    success: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	    }
	}

}

export namespace providers {
	
	export class ChatMessage {
	    role: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	    }
	}

}

export namespace scanner {
	
	export class FileInfo {
	    path: string;
	    name: string;
	    size: number;
	    size_formatted: string;
	    ext: string;
	    last_access: string;
	    last_modified: string;
	    last_modified_ts: number;
	    category: string;
	    rule_match?: string;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.size_formatted = source["size_formatted"];
	        this.ext = source["ext"];
	        this.last_access = source["last_access"];
	        this.last_modified = source["last_modified"];
	        this.last_modified_ts = source["last_modified_ts"];
	        this.category = source["category"];
	        this.rule_match = source["rule_match"];
	    }
	}

}

