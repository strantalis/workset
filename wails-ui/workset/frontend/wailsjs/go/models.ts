export namespace config {
	
	export class GroupMember {
	    repo: string;
	
	    static createFrom(source: any = {}) {
	        return new GroupMember(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repo = source["repo"];
	    }
	}
	export class RemoteConfig {
	    name: string;
	    default_branch?: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.default_branch = source["default_branch"];
	    }
	}
	export class Remotes {
	    base: RemoteConfig;
	    write: RemoteConfig;
	
	    static createFrom(source: any = {}) {
	        return new Remotes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.base = this.convertValues(source["base"], RemoteConfig);
	        this.write = this.convertValues(source["write"], RemoteConfig);
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

export namespace kitty {
	
	export class Image {
	    id: string;
	    number?: number;
	    format: string;
	    width?: number;
	    height?: number;
	    data?: number[];
	
	    static createFrom(source: any = {}) {
	        return new Image(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.number = source["number"];
	        this.format = source["format"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.data = source["data"];
	    }
	}
	export class Placement {
	    id: number;
	    imageId: string;
	    row: number;
	    col: number;
	    rows: number;
	    cols: number;
	    x?: number;
	    y?: number;
	    z?: number;
	
	    static createFrom(source: any = {}) {
	        return new Placement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.imageId = source["imageId"];
	        this.row = source["row"];
	        this.col = source["col"];
	        this.rows = source["rows"];
	        this.cols = source["cols"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.z = source["z"];
	    }
	}
	export class Snapshot {
	    images: Image[];
	    placements: Placement[];
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.images = this.convertValues(source["images"], Image);
	        this.placements = this.convertValues(source["placements"], Placement);
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
	
	export class AliasUpsertRequest {
	    name: string;
	    source: string;
	    remote: string;
	    defaultBranch: string;
	
	    static createFrom(source: any = {}) {
	        return new AliasUpsertRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.source = source["source"];
	        this.remote = source["remote"];
	        this.defaultBranch = source["defaultBranch"];
	    }
	}
	export class CommitAndPushRequest {
	    workspaceId: string;
	    repoId: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new CommitAndPushRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.repoId = source["repoId"];
	        this.message = source["message"];
	    }
	}
	export class GroupMemberRequest {
	    groupName: string;
	    repoName: string;
	
	    static createFrom(source: any = {}) {
	        return new GroupMemberRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupName = source["groupName"];
	        this.repoName = source["repoName"];
	    }
	}
	export class GroupUpsertRequest {
	    name: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new GroupUpsertRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	    }
	}
	export class ListRemotesRequest {
	    workspaceId: string;
	    repoId: string;
	
	    static createFrom(source: any = {}) {
	        return new ListRemotesRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.repoId = source["repoId"];
	    }
	}
	export class PullRequestCreateRequest {
	    workspaceId: string;
	    repoId: string;
	    title: string;
	    body: string;
	    base?: string;
	    head?: string;
	    baseRemote?: string;
	    draft: boolean;
	    autoCommit: boolean;
	    autoPush: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PullRequestCreateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.repoId = source["repoId"];
	        this.title = source["title"];
	        this.body = source["body"];
	        this.base = source["base"];
	        this.head = source["head"];
	        this.baseRemote = source["baseRemote"];
	        this.draft = source["draft"];
	        this.autoCommit = source["autoCommit"];
	        this.autoPush = source["autoPush"];
	    }
	}
	export class PullRequestGenerateRequest {
	    workspaceId: string;
	    repoId: string;
	
	    static createFrom(source: any = {}) {
	        return new PullRequestGenerateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.repoId = source["repoId"];
	    }
	}
	export class PullRequestReviewCommentsPayload {
	    comments: worksetapi.PullRequestReviewCommentJSON[];
	
	    static createFrom(source: any = {}) {
	        return new PullRequestReviewCommentsPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.comments = this.convertValues(source["comments"], worksetapi.PullRequestReviewCommentJSON);
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
	export class PullRequestReviewsRequest {
	    workspaceId: string;
	    repoId: string;
	    number?: number;
	    branch?: string;
	
	    static createFrom(source: any = {}) {
	        return new PullRequestReviewsRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.repoId = source["repoId"];
	        this.number = source["number"];
	        this.branch = source["branch"];
	    }
	}
	export class PullRequestStatusPayload {
	    pullRequest: worksetapi.PullRequestStatusJSON;
	    checks: worksetapi.PullRequestCheckJSON[];
	
	    static createFrom(source: any = {}) {
	        return new PullRequestStatusPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pullRequest = this.convertValues(source["pullRequest"], worksetapi.PullRequestStatusJSON);
	        this.checks = this.convertValues(source["checks"], worksetapi.PullRequestCheckJSON);
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
	export class PullRequestStatusRequest {
	    workspaceId: string;
	    repoId: string;
	    number?: number;
	    branch?: string;
	
	    static createFrom(source: any = {}) {
	        return new PullRequestStatusRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.repoId = source["repoId"];
	        this.number = source["number"];
	        this.branch = source["branch"];
	    }
	}
	export class PullRequestTrackedRequest {
	    workspaceId: string;
	    repoId: string;
	
	    static createFrom(source: any = {}) {
	        return new PullRequestTrackedRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.repoId = source["repoId"];
	    }
	}
	export class RepoAddRequest {
	    workspaceId: string;
	    source: string;
	    name?: string;
	    repoDir?: string;
	
	    static createFrom(source: any = {}) {
	        return new RepoAddRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.source = source["source"];
	        this.name = source["name"];
	        this.repoDir = source["repoDir"];
	    }
	}
	export class RepoAddResponse {
	    payload: worksetapi.RepoAddResultJSON;
	    warnings?: string[];
	    pendingHooks?: worksetapi.HookPendingJSON[];
	
	    static createFrom(source: any = {}) {
	        return new RepoAddResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.payload = this.convertValues(source["payload"], worksetapi.RepoAddResultJSON);
	        this.warnings = source["warnings"];
	        this.pendingHooks = this.convertValues(source["pendingHooks"], worksetapi.HookPendingJSON);
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
	export class RepoDiffFile {
	    path: string;
	    prevPath?: string;
	    added: number;
	    removed: number;
	    status: string;
	    binary?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RepoDiffFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.prevPath = source["prevPath"];
	        this.added = source["added"];
	        this.removed = source["removed"];
	        this.status = source["status"];
	        this.binary = source["binary"];
	    }
	}
	export class RepoDiffSnapshot {
	    patch: string;
	
	    static createFrom(source: any = {}) {
	        return new RepoDiffSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.patch = source["patch"];
	    }
	}
	export class RepoDiffSummary {
	    files: RepoDiffFile[];
	    totalAdded: number;
	    totalRemoved: number;
	
	    static createFrom(source: any = {}) {
	        return new RepoDiffSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], RepoDiffFile);
	        this.totalAdded = source["totalAdded"];
	        this.totalRemoved = source["totalRemoved"];
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
	export class RepoFileDiffSnapshot {
	    patch: string;
	    truncated: boolean;
	    totalLines: number;
	    totalBytes: number;
	    binary?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RepoFileDiffSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.patch = source["patch"];
	        this.truncated = source["truncated"];
	        this.totalLines = source["totalLines"];
	        this.totalBytes = source["totalBytes"];
	        this.binary = source["binary"];
	    }
	}
	export class RepoLocalStatusRequest {
	    workspaceId: string;
	    repoId: string;
	
	    static createFrom(source: any = {}) {
	        return new RepoLocalStatusRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.repoId = source["repoId"];
	    }
	}
	export class RepoRemoveRequest {
	    workspaceId: string;
	    repoName: string;
	    deleteWorktree: boolean;
	    deleteLocal: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RepoRemoveRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.repoName = source["repoName"];
	        this.deleteWorktree = source["deleteWorktree"];
	        this.deleteLocal = source["deleteLocal"];
	    }
	}
	export class RepoSnapshot {
	    id: string;
	    name: string;
	    path: string;
	    remote?: string;
	    defaultBranch?: string;
	    dirty: boolean;
	    missing: boolean;
	    statusKnown: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RepoSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.remote = source["remote"];
	        this.defaultBranch = source["defaultBranch"];
	        this.dirty = source["dirty"];
	        this.missing = source["missing"];
	        this.statusKnown = source["statusKnown"];
	    }
	}
	export class SessiondStatus {
	    available: boolean;
	    error?: string;
	    warning?: string;
	
	    static createFrom(source: any = {}) {
	        return new SessiondStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.error = source["error"];
	        this.warning = source["warning"];
	    }
	}
	export class SettingsDefaults {
	    remote: string;
	    baseBranch: string;
	    workspace: string;
	    workspaceRoot: string;
	    repoStoreRoot: string;
	    sessionBackend: string;
	    sessionNameFormat: string;
	    sessionTheme: string;
	    sessionTmuxStyle: string;
	    sessionTmuxLeft: string;
	    sessionTmuxRight: string;
	    sessionScreenHard: string;
	    agent: string;
	    terminalRenderer: string;
	    terminalIdleTimeout: string;
	
	    static createFrom(source: any = {}) {
	        return new SettingsDefaults(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remote = source["remote"];
	        this.baseBranch = source["baseBranch"];
	        this.workspace = source["workspace"];
	        this.workspaceRoot = source["workspaceRoot"];
	        this.repoStoreRoot = source["repoStoreRoot"];
	        this.sessionBackend = source["sessionBackend"];
	        this.sessionNameFormat = source["sessionNameFormat"];
	        this.sessionTheme = source["sessionTheme"];
	        this.sessionTmuxStyle = source["sessionTmuxStyle"];
	        this.sessionTmuxLeft = source["sessionTmuxLeft"];
	        this.sessionTmuxRight = source["sessionTmuxRight"];
	        this.sessionScreenHard = source["sessionScreenHard"];
	        this.agent = source["agent"];
	        this.terminalRenderer = source["terminalRenderer"];
	        this.terminalIdleTimeout = source["terminalIdleTimeout"];
	    }
	}
	export class SettingsSnapshot {
	    defaults: SettingsDefaults;
	    configPath: string;
	
	    static createFrom(source: any = {}) {
	        return new SettingsSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaults = this.convertValues(source["defaults"], SettingsDefaults);
	        this.configPath = source["configPath"];
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
	export class TerminalBacklogPayload {
	    workspaceId: string;
	    data: string;
	    nextOffset: number;
	    truncated: boolean;
	    source?: string;
	
	    static createFrom(source: any = {}) {
	        return new TerminalBacklogPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.data = source["data"];
	        this.nextOffset = source["nextOffset"];
	        this.truncated = source["truncated"];
	        this.source = source["source"];
	    }
	}
	export class TerminalBootstrapPayload {
	    workspaceId: string;
	    snapshot?: string;
	    snapshotSource?: string;
	    kitty?: kitty.Snapshot;
	    backlog?: string;
	    backlogSource?: string;
	    backlogTruncated?: boolean;
	    nextOffset?: number;
	    source?: string;
	    altScreen?: boolean;
	    mouse?: boolean;
	    mouseSGR?: boolean;
	    mouseEncoding?: string;
	    safeToReplay?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TerminalBootstrapPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.snapshot = source["snapshot"];
	        this.snapshotSource = source["snapshotSource"];
	        this.kitty = this.convertValues(source["kitty"], kitty.Snapshot);
	        this.backlog = source["backlog"];
	        this.backlogSource = source["backlogSource"];
	        this.backlogTruncated = source["backlogTruncated"];
	        this.nextOffset = source["nextOffset"];
	        this.source = source["source"];
	        this.altScreen = source["altScreen"];
	        this.mouse = source["mouse"];
	        this.mouseSGR = source["mouseSGR"];
	        this.mouseEncoding = source["mouseEncoding"];
	        this.safeToReplay = source["safeToReplay"];
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
	export class TerminalDebugPayload {
	    workspaceId: string;
	    event: string;
	    details?: string;
	
	    static createFrom(source: any = {}) {
	        return new TerminalDebugPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.event = source["event"];
	        this.details = source["details"];
	    }
	}
	export class TerminalSnapshotPayload {
	    workspaceId: string;
	    data: string;
	    source?: string;
	    kitty?: kitty.Snapshot;
	
	    static createFrom(source: any = {}) {
	        return new TerminalSnapshotPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.data = source["data"];
	        this.source = source["source"];
	        this.kitty = this.convertValues(source["kitty"], kitty.Snapshot);
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
	export class TerminalStatusPayload {
	    workspaceId: string;
	    active: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new TerminalStatusPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.active = source["active"];
	        this.error = source["error"];
	    }
	}
	export class WorkspaceCreateRequest {
	    name: string;
	    path: string;
	    repos?: string[];
	    groups?: string[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceCreateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.repos = source["repos"];
	        this.groups = source["groups"];
	    }
	}
	export class WorkspaceCreateResponse {
	    workspace: worksetapi.WorkspaceCreatedJSON;
	    warnings?: string[];
	    pendingHooks?: worksetapi.HookPendingJSON[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceCreateResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspace = this.convertValues(source["workspace"], worksetapi.WorkspaceCreatedJSON);
	        this.warnings = source["warnings"];
	        this.pendingHooks = this.convertValues(source["pendingHooks"], worksetapi.HookPendingJSON);
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
	export class WorkspaceRemoveRequest {
	    workspaceId: string;
	    deleteFiles: boolean;
	    force: boolean;
	    fetchRemotes: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceRemoveRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.deleteFiles = source["deleteFiles"];
	        this.force = source["force"];
	        this.fetchRemotes = source["fetchRemotes"];
	    }
	}
	export class WorkspaceSnapshot {
	    id: string;
	    name: string;
	    path: string;
	    archivedAt?: string;
	    archivedReason?: string;
	    archived: boolean;
	    repos: RepoSnapshot[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.archivedAt = source["archivedAt"];
	        this.archivedReason = source["archivedReason"];
	        this.archived = source["archived"];
	        this.repos = this.convertValues(source["repos"], RepoSnapshot);
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
	export class WorkspaceSnapshotRequest {
	    includeArchived: boolean;
	    includeStatus: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceSnapshotRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.includeArchived = source["includeArchived"];
	        this.includeStatus = source["includeStatus"];
	    }
	}

}

export namespace worksetapi {
	
	export class AliasJSON {
	    name: string;
	    url?: string;
	    path?: string;
	    remote?: string;
	    default_branch?: string;
	
	    static createFrom(source: any = {}) {
	        return new AliasJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.path = source["path"];
	        this.remote = source["remote"];
	        this.default_branch = source["default_branch"];
	    }
	}
	export class AliasMutationResultJSON {
	    status: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new AliasMutationResultJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.name = source["name"];
	    }
	}
	export class CommitAndPushResultJSON {
	    committed: boolean;
	    pushed: boolean;
	    message: string;
	    sha?: string;
	
	    static createFrom(source: any = {}) {
	        return new CommitAndPushResultJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.committed = source["committed"];
	        this.pushed = source["pushed"];
	        this.message = source["message"];
	        this.sha = source["sha"];
	    }
	}
	export class ConfigSetResultJSON {
	    status: string;
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigSetResultJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class GroupApplyResultJSON {
	    status: string;
	    template: string;
	    workspace: string;
	
	    static createFrom(source: any = {}) {
	        return new GroupApplyResultJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.template = source["template"];
	        this.workspace = source["workspace"];
	    }
	}
	export class GroupJSON {
	    name: string;
	    description?: string;
	    members: config.GroupMember[];
	
	    static createFrom(source: any = {}) {
	        return new GroupJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.members = this.convertValues(source["members"], config.GroupMember);
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
	export class GroupSummaryJSON {
	    name: string;
	    description?: string;
	    repo_count: number;
	
	    static createFrom(source: any = {}) {
	        return new GroupSummaryJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.repo_count = source["repo_count"];
	    }
	}
	export class HookPendingJSON {
	    event: string;
	    repo: string;
	    hooks: string[];
	    status?: string;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new HookPendingJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.event = source["event"];
	        this.repo = source["repo"];
	        this.hooks = source["hooks"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	    }
	}
	export class PullRequestCheckJSON {
	    name: string;
	    status: string;
	    conclusion?: string;
	    details_url?: string;
	    started_at?: string;
	    completed_at?: string;
	
	    static createFrom(source: any = {}) {
	        return new PullRequestCheckJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.conclusion = source["conclusion"];
	        this.details_url = source["details_url"];
	        this.started_at = source["started_at"];
	        this.completed_at = source["completed_at"];
	    }
	}
	export class PullRequestCreatedJSON {
	    repo: string;
	    number: number;
	    url: string;
	    title: string;
	    body?: string;
	    draft: boolean;
	    state: string;
	    base_repo: string;
	    base_branch: string;
	    head_repo: string;
	    head_branch: string;
	
	    static createFrom(source: any = {}) {
	        return new PullRequestCreatedJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repo = source["repo"];
	        this.number = source["number"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.body = source["body"];
	        this.draft = source["draft"];
	        this.state = source["state"];
	        this.base_repo = source["base_repo"];
	        this.base_branch = source["base_branch"];
	        this.head_repo = source["head_repo"];
	        this.head_branch = source["head_branch"];
	    }
	}
	export class PullRequestGeneratedJSON {
	    title: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new PullRequestGeneratedJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.body = source["body"];
	    }
	}
	export class PullRequestReviewCommentJSON {
	    id: number;
	    review_id?: number;
	    author?: string;
	    body: string;
	    path: string;
	    line?: number;
	    side?: string;
	    commit_id?: string;
	    original_commit_id?: string;
	    original_line?: number;
	    original_start_line?: number;
	    outdated: boolean;
	    url?: string;
	    created_at?: string;
	    updated_at?: string;
	    in_reply_to?: number;
	    resolved?: boolean;
	    reply?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PullRequestReviewCommentJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.review_id = source["review_id"];
	        this.author = source["author"];
	        this.body = source["body"];
	        this.path = source["path"];
	        this.line = source["line"];
	        this.side = source["side"];
	        this.commit_id = source["commit_id"];
	        this.original_commit_id = source["original_commit_id"];
	        this.original_line = source["original_line"];
	        this.original_start_line = source["original_start_line"];
	        this.outdated = source["outdated"];
	        this.url = source["url"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.in_reply_to = source["in_reply_to"];
	        this.resolved = source["resolved"];
	        this.reply = source["reply"];
	    }
	}
	export class PullRequestStatusJSON {
	    repo: string;
	    number: number;
	    url: string;
	    title: string;
	    state: string;
	    draft: boolean;
	    base_repo: string;
	    base_branch: string;
	    head_repo: string;
	    head_branch: string;
	    mergeable?: string;
	
	    static createFrom(source: any = {}) {
	        return new PullRequestStatusJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repo = source["repo"];
	        this.number = source["number"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.state = source["state"];
	        this.draft = source["draft"];
	        this.base_repo = source["base_repo"];
	        this.base_branch = source["base_branch"];
	        this.head_repo = source["head_repo"];
	        this.head_branch = source["head_branch"];
	        this.mergeable = source["mergeable"];
	    }
	}
	export class PullRequestTrackedJSON {
	    found: boolean;
	    pull_request?: PullRequestCreatedJSON;
	
	    static createFrom(source: any = {}) {
	        return new PullRequestTrackedJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.found = source["found"];
	        this.pull_request = this.convertValues(source["pull_request"], PullRequestCreatedJSON);
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
	export class RemoteInfoJSON {
	    name: string;
	    owner: string;
	    repo: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteInfoJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.owner = source["owner"];
	        this.repo = source["repo"];
	    }
	}
	export class RepoAddResultJSON {
	    status: string;
	    workspace: string;
	    repo: string;
	    local_path: string;
	    managed: boolean;
	    pending_hooks?: HookPendingJSON[];
	
	    static createFrom(source: any = {}) {
	        return new RepoAddResultJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.workspace = source["workspace"];
	        this.repo = source["repo"];
	        this.local_path = source["local_path"];
	        this.managed = source["managed"];
	        this.pending_hooks = this.convertValues(source["pending_hooks"], HookPendingJSON);
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
	export class RepoLocalStatusJSON {
	    hasUncommitted: boolean;
	    ahead: number;
	    behind: number;
	    currentBranch: string;
	
	    static createFrom(source: any = {}) {
	        return new RepoLocalStatusJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasUncommitted = source["hasUncommitted"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	        this.currentBranch = source["currentBranch"];
	    }
	}
	export class RepoRemoveDeletedJSON {
	    worktrees: boolean;
	    local: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RepoRemoveDeletedJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.worktrees = source["worktrees"];
	        this.local = source["local"];
	    }
	}
	export class RepoRemoveResultJSON {
	    status: string;
	    workspace: string;
	    repo: string;
	    deleted: RepoRemoveDeletedJSON;
	
	    static createFrom(source: any = {}) {
	        return new RepoRemoveResultJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.workspace = source["workspace"];
	        this.repo = source["repo"];
	        this.deleted = this.convertValues(source["deleted"], RepoRemoveDeletedJSON);
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
	export class WorkspaceCreatedJSON {
	    name: string;
	    path: string;
	    workset: string;
	    branch: string;
	    next: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceCreatedJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.workset = source["workset"];
	        this.branch = source["branch"];
	        this.next = source["next"];
	    }
	}
	export class WorkspaceDeleteResultJSON {
	    status: string;
	    name?: string;
	    path: string;
	    deleted_files: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceDeleteResultJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.deleted_files = source["deleted_files"];
	    }
	}
	export class WorkspaceRefJSON {
	    name: string;
	    path: string;
	    created_at?: string;
	    last_used?: string;
	    archived_at?: string;
	    archived_reason?: string;
	    archived: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceRefJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.created_at = source["created_at"];
	        this.last_used = source["last_used"];
	        this.archived_at = source["archived_at"];
	        this.archived_reason = source["archived_reason"];
	        this.archived = source["archived"];
	    }
	}

}

