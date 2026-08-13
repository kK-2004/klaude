export namespace app {
	
	export class AgentTurnDTO {
	    id: string;
	    sessionId: string;
	    status: string;
	    startedAt: string;
	    finishedAt?: string;
	    errorCode?: string;
	    errorText?: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentTurnDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.status = source["status"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.errorCode = source["errorCode"];
	        this.errorText = source["errorText"];
	    }
	}
	export class ApprovalResolution {
	    approvalId: string;
	    status: string;
	    requestHash: string;
	
	    static createFrom(source: any = {}) {
	        return new ApprovalResolution(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.approvalId = source["approvalId"];
	        this.status = source["status"];
	        this.requestHash = source["requestHash"];
	    }
	}
	export class SettingsUpdate {
	    parallelTools: boolean;
	    llmSchedule: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SettingsUpdate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.parallelTools = source["parallelTools"];
	        this.llmSchedule = source["llmSchedule"];
	    }
	}
	export class MessageDTO {
	    id: string;
	    sessionId: string;
	    turnId?: string;
	    role: string;
	    content: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new MessageDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.turnId = source["turnId"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class SessionDTO {
	    id: string;
	    projectId: string;
	    title: string;
	    provider: string;
	    model: string;
	    status: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.title = source["title"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.status = source["status"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ConversationSnapshotDTO {
	    session: SessionDTO;
	    messages: MessageDTO[];
	    turns: AgentTurnDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ConversationSnapshotDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session = this.convertValues(source["session"], SessionDTO);
	        this.messages = this.convertValues(source["messages"], MessageDTO);
	        this.turns = this.convertValues(source["turns"], AgentTurnDTO);
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
	export class FileChangeDTO {
	    id: string;
	    turnId: string;
	    toolCallId: string;
	    path: string;
	    status: string;
	    beforeHash: string;
	    afterHash: string;
	    diff: string;
	    addedLines: number;
	    deletedLines: number;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new FileChangeDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.turnId = source["turnId"];
	        this.toolCallId = source["toolCallId"];
	        this.path = source["path"];
	        this.status = source["status"];
	        this.beforeHash = source["beforeHash"];
	        this.afterHash = source["afterHash"];
	        this.diff = source["diff"];
	        this.addedLines = source["addedLines"];
	        this.deletedLines = source["deletedLines"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class HealthResponse {
	    ready: boolean;
	    product: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new HealthResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ready = source["ready"];
	        this.product = source["product"];
	        this.version = source["version"];
	    }
	}
	
	export class ProjectDTO {
	    id: string;
	    name: string;
	    rootPath: string;
	    gitRoot?: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.rootPath = source["rootPath"];
	        this.gitRoot = source["gitRoot"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

export namespace config {
	
	export class AgentConfig {
	    MaxTurns: number;
	    ContextBudgetChars: number;
	    ToolResultChars: number;
	    ShellTimeoutSec: number;
	    ParallelTools: boolean;
	    LLMSchedule: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AgentConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.MaxTurns = source["MaxTurns"];
	        this.ContextBudgetChars = source["ContextBudgetChars"];
	        this.ToolResultChars = source["ToolResultChars"];
	        this.ShellTimeoutSec = source["ShellTimeoutSec"];
	        this.ParallelTools = source["ParallelTools"];
	        this.LLMSchedule = source["LLMSchedule"];
	    }
	}
	export class PermissionConfig {
	    Read: string;
	    Write: string;
	    Shell: string;
	    Network: string;
	    ShellRules: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new PermissionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Read = source["Read"];
	        this.Write = source["Write"];
	        this.Shell = source["Shell"];
	        this.Network = source["Network"];
	        this.ShellRules = source["ShellRules"];
	    }
	}
	export class ProviderConfig {
	    Name: string;
	    Endpoint: string;
	    Model: string;
	    CredentialEnv: string;
	    AllowHTTPForLocal: boolean;
	    SupportsTools: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProviderConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Endpoint = source["Endpoint"];
	        this.Model = source["Model"];
	        this.CredentialEnv = source["CredentialEnv"];
	        this.AllowHTTPForLocal = source["AllowHTTPForLocal"];
	        this.SupportsTools = source["SupportsTools"];
	    }
	}
	export class UIConfig {
	    Theme: string;
	
	    static createFrom(source: any = {}) {
	        return new UIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Theme = source["Theme"];
	    }
	}
	export class Config {
	    DefaultModel: string;
	    UI: UIConfig;
	    Agent: AgentConfig;
	    Provider: ProviderConfig;
	    Permissions: PermissionConfig;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.DefaultModel = source["DefaultModel"];
	        this.UI = this.convertValues(source["UI"], UIConfig);
	        this.Agent = this.convertValues(source["Agent"], AgentConfig);
	        this.Provider = this.convertValues(source["Provider"], ProviderConfig);
	        this.Permissions = this.convertValues(source["Permissions"], PermissionConfig);
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

export namespace filesystem {
	
	export class Entry {
	    name: string;
	    path: string;
	    dir: boolean;
	    size: number;
	    externalSymlink: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.dir = source["dir"];
	        this.size = source["size"];
	        this.externalSymlink = source["externalSymlink"];
	    }
	}

}

export namespace git {

	export class BranchInfo {
	    name: string;
	    remote: boolean;
	    current: boolean;

	    static createFrom(source: any = {}) {
	        return new BranchInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.remote = source["remote"];
	        this.current = source["current"];
	    }
	}

	export class BranchSnapshot {
	    current: string;
	    branches: BranchInfo[];
	    worktreeBase: string;

	    static createFrom(source: any = {}) {
	        return new BranchSnapshot(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.branches = this.convertValues(source["branches"], BranchInfo);
	        this.worktreeBase = source["worktreeBase"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) return a;
		    if (a.slice && a.map) return (a as any[]).map(elem => this.convertValues(elem, classs));
		    if ("object" === typeof a) return asMap ? a : new classs(a);
		    return a;
		}
	}

}

export namespace project {
	
	export class Capability {
	    name: string;
	    available: boolean;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new Capability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.available = source["available"];
	        this.detail = source["detail"];
	    }
	}

}
