export namespace agent {
	
	export class TodoItem {
	    id: string;
	    title: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new TodoItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	    }
	}
	export class ToolDetail {
	    kind: string;
	    command?: string;
	    todos?: TodoItem[];
	    resultFiles?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ToolDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.command = source["command"];
	        this.todos = this.convertValues(source["todos"], TodoItem);
	        this.resultFiles = source["resultFiles"];
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
	export class MessagePart {
	    type: string;
	    content?: string;
	    tool?: string;
	    toolArgs?: string;
	    toolDetail?: ToolDetail;
	    filePath?: string;
	    isEdit?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MessagePart(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.content = source["content"];
	        this.tool = source["tool"];
	        this.toolArgs = source["toolArgs"];
	        this.toolDetail = this.convertValues(source["toolDetail"], ToolDetail);
	        this.filePath = source["filePath"];
	        this.isEdit = source["isEdit"];
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
	export class ParsedMessage {
	    timestamp: string;
	    role: string;
	    content: string;
	    parts?: MessagePart[];
	    filesEdited?: string[];
	    model?: string;
	    canceled?: boolean;
	    confirmation?: string;
	    attachments?: string[];
	    maxToolCallsExceeded?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ParsedMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.parts = this.convertValues(source["parts"], MessagePart);
	        this.filesEdited = source["filesEdited"];
	        this.model = source["model"];
	        this.canceled = source["canceled"];
	        this.confirmation = source["confirmation"];
	        this.attachments = source["attachments"];
	        this.maxToolCallsExceeded = source["maxToolCallsExceeded"];
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
	export class ParsedSession {
	    sessionId: string;
	    title: string;
	    createdAt: string;
	    createdAtMs: number;
	    lastMessageAt: string;
	    model?: string;
	    user: string;
	    agent: string;
	    messages: ParsedMessage[];
	
	    static createFrom(source: any = {}) {
	        return new ParsedSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.title = source["title"];
	        this.createdAt = source["createdAt"];
	        this.createdAtMs = source["createdAtMs"];
	        this.lastMessageAt = source["lastMessageAt"];
	        this.model = source["model"];
	        this.user = source["user"];
	        this.agent = source["agent"];
	        this.messages = this.convertValues(source["messages"], ParsedMessage);
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

export namespace claudecode {
	
	export class ScannedProject {
	    encodedName: string;
	    projectPath: string;
	    displayName: string;
	    sessionCount: number;
	    transcriptDirectory: string;
	
	    static createFrom(source: any = {}) {
	        return new ScannedProject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.encodedName = source["encodedName"];
	        this.projectPath = source["projectPath"];
	        this.displayName = source["displayName"];
	        this.sessionCount = source["sessionCount"];
	        this.transcriptDirectory = source["transcriptDirectory"];
	    }
	}

}

export namespace cursor {
	
	export class ScannedProject {
	    workspacePath: string;
	    displayName: string;
	    composerCount: number;
	    lastActivityAt: number;
	
	    static createFrom(source: any = {}) {
	        return new ScannedProject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspacePath = source["workspacePath"];
	        this.displayName = source["displayName"];
	        this.composerCount = source["composerCount"];
	        this.lastActivityAt = source["lastActivityAt"];
	    }
	}

}

export namespace main {
	
	export class AgentSource {
	    type: string;
	    watchDir?: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.watchDir = source["watchDir"];
	    }
	}
	export class ChatFileInfo {
	    fileName: string;
	    filePath: string;
	    sourceType: string;
	    parsed: boolean;
	    partiallyParsed: boolean;
	    title: string;
	    lastMessageAt: string;
	    processedAt: number;
	    createdAt: number;
	
	    static createFrom(source: any = {}) {
	        return new ChatFileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileName = source["fileName"];
	        this.filePath = source["filePath"];
	        this.sourceType = source["sourceType"];
	        this.parsed = source["parsed"];
	        this.partiallyParsed = source["partiallyParsed"];
	        this.title = source["title"];
	        this.lastMessageAt = source["lastMessageAt"];
	        this.processedAt = source["processedAt"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class Project {
	    id: string;
	    name: string;
	    watchDir: string;
	    outputDir: string;
	    active: boolean;
	    workspacePath?: string;
	    sources?: AgentSource[];
	    lastProcessed?: number;
	    pausedAt?: number;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.watchDir = source["watchDir"];
	        this.outputDir = source["outputDir"];
	        this.active = source["active"];
	        this.workspacePath = source["workspacePath"];
	        this.sources = this.convertValues(source["sources"], AgentSource);
	        this.lastProcessed = source["lastProcessed"];
	        this.pausedAt = source["pausedAt"];
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

