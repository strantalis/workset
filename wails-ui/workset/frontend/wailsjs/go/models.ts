export namespace config {
	
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
	export class GroupMember {
	    repo: string;
	    remotes: Remotes;
	
	    static createFrom(source: any = {}) {
	        return new GroupMember(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repo = source["repo"];
	        this.remotes = this.convertValues(source["remotes"], Remotes);
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
	    defaultBranch: string;
	
	    static createFrom(source: any = {}) {
	        return new AliasUpsertRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.source = source["source"];
	        this.defaultBranch = source["defaultBranch"];
	    }
	}
	export class GroupMemberRequest {
	    groupName: string;
	    repoName: string;
	    baseRemote: string;
	    writeRemote: string;
	    baseBranch: string;
	
	    static createFrom(source: any = {}) {
	        return new GroupMemberRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupName = source["groupName"];
	        this.repoName = source["repoName"];
	        this.baseRemote = source["baseRemote"];
	        this.writeRemote = source["writeRemote"];
	        this.baseBranch = source["baseBranch"];
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
	export class RepoRemotesUpdateRequest {
	    workspaceId: string;
	    repoName: string;
	    baseRemote: string;
	    baseBranch: string;
	    writeRemote: string;
	    writeBranch: string;
	
	    static createFrom(source: any = {}) {
	        return new RepoRemotesUpdateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.repoName = source["repoName"];
	        this.baseRemote = source["baseRemote"];
	        this.baseBranch = source["baseBranch"];
	        this.writeRemote = source["writeRemote"];
	        this.writeBranch = source["writeBranch"];
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
	    branch?: string;
	    baseRemote?: string;
	    baseBranch?: string;
	    writeRemote?: string;
	    writeBranch?: string;
	    dirty: boolean;
	    missing: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RepoSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.branch = source["branch"];
	        this.baseRemote = source["baseRemote"];
	        this.baseBranch = source["baseBranch"];
	        this.writeRemote = source["writeRemote"];
	        this.writeBranch = source["writeBranch"];
	        this.dirty = source["dirty"];
	        this.missing = source["missing"];
	    }
	}
	export class SettingsDefaults {
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
	
	    static createFrom(source: any = {}) {
	        return new SettingsDefaults(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
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
	export class WorkspaceCreateRequest {
	    name: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceCreateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
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

}

export namespace worksetapi {
	
	export class AliasJSON {
	    name: string;
	    url?: string;
	    path?: string;
	    default_branch?: string;
	
	    static createFrom(source: any = {}) {
	        return new AliasJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.path = source["path"];
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
	export class RepoRemotesUpdateResultJSON {
	    status: string;
	    workspace: string;
	    repo: string;
	    base: string;
	    write: string;
	
	    static createFrom(source: any = {}) {
	        return new RepoRemotesUpdateResultJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.workspace = source["workspace"];
	        this.repo = source["repo"];
	        this.base = source["base"];
	        this.write = source["write"];
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

