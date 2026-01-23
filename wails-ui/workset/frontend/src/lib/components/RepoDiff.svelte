<script lang="ts">
  import {onDestroy, onMount} from 'svelte'
  import type {
    FileDiff as FileDiffType,
    FileDiffMetadata,
    FileDiffOptions,
    ParsedPatch
  } from '@pierre/diffs'
  import type {
    PullRequestCreated,
    PullRequestGenerated,
    PullRequestReviewComment,
    PullRequestStatusResult,
    Repo,
    RepoDiffFileSummary,
    RepoDiffSummary,
    RepoFileDiff
  } from '../types'
  import {
    createPullRequest,
    fetchTrackedPullRequest,
    fetchPullRequestReviews,
    fetchPullRequestStatus,
    fetchRepoDiffSummary,
    fetchRepoFileDiff,
    generatePullRequestText,
    sendPullRequestReviewsToTerminal
  } from '../api'

  interface Props {
    repo: Repo;
    workspaceId: string;
    onClose: () => void;
  }

  let { repo, workspaceId, onClose }: Props = $props();

  type DiffsModule = {
    FileDiff: new (options?: FileDiffOptions) => FileDiffType
    parsePatchFiles: (patch: string) => ParsedPatch[]
  }

  let summary: RepoDiffSummary | null = $state(null)
  let summaryLoading = $state(true)
  let summaryError: string | null = $state(null)

  let selected: RepoDiffFileSummary | null = $state(null)
  let selectedDiff: FileDiffMetadata | null = $state(null)
  let fileMeta: RepoFileDiff | null = $state(null)
  let fileLoading = $state(false)
  let fileError: string | null = $state(null)

  let diffMode: 'split' | 'unified' = $state('split')
  let diffContainer: HTMLElement | null = $state(null)
  let diffInstance: FileDiffType | null = null
  let diffModule: DiffsModule | null = null
  let rendererLoading = $state(false)
  let rendererError: string | null = $state(null)

  let prTitle = $state('')
  let prBody = $state('')
  let prBase = $state('')
  let prHead = $state('')
  let prDraft = $state(false)
  let prCreateError: string | null = $state(null)
  let prCreateSuccess: PullRequestCreated | null = $state(null)
  let prTracked: PullRequestCreated | null = $state(null)
  let prCreating = $state(false)
  let prGenerating = $state(false)

  let prNumberInput = $state('')
  let prBranchInput = $state('')
  let prStatus: PullRequestStatusResult | null = $state(null)
  let prStatusError: string | null = $state(null)
  let prStatusLoading = $state(false)

  let prReviews: PullRequestReviewComment[] = $state([])
  let prReviewsError: string | null = $state(null)
  let prReviewsLoading = $state(false)
  let prReviewsSent = $state(false)

  let summaryRequest = 0
  let fileRequest = 0

  const buildOptions = (): FileDiffOptions => ({
    theme: 'pierre-dark',
    themeType: 'dark',
    diffStyle: diffMode,
    diffIndicators: 'bars',
    hunkSeparators: 'line-info',
    lineDiffType: 'word',
    overflow: 'scroll',
    disableFileHeader: true
  })

  const statusLabel = (status: string): string => {
    switch (status) {
      case 'added':
        return 'added'
      case 'deleted':
        return 'deleted'
      case 'renamed':
        return 'renamed'
      case 'untracked':
        return 'untracked'
      case 'binary':
        return 'binary'
      default:
        return 'modified'
    }
  }

  const formatError = (err: unknown, fallback: string): string => {
    if (err instanceof Error) return err.message
    if (typeof err === 'string') return err
    if (err && typeof err === 'object' && 'message' in err) {
      const message = (err as {message?: string}).message
      if (typeof message === 'string') return message
    }
    return fallback
  }

  const parseNumber = (value: string): number | undefined => {
    const parsed = Number.parseInt(value.trim(), 10)
    return Number.isFinite(parsed) ? parsed : undefined
  }

  const loadPrStatus = async (): Promise<void> => {
    prStatusLoading = true
    prStatusError = null
    try {
      prStatus = await fetchPullRequestStatus(
        workspaceId,
        repo.id,
        parseNumber(prNumberInput),
        prBranchInput.trim() || undefined
      )
    } catch (err) {
      prStatusError = formatError(err, 'Failed to load pull request status.')
      prStatus = null
    } finally {
      prStatusLoading = false
    }
  }

  const loadPrReviews = async (): Promise<void> => {
    prReviewsLoading = true
    prReviewsError = null
    prReviewsSent = false
    try {
      prReviews = await fetchPullRequestReviews(
        workspaceId,
        repo.id,
        parseNumber(prNumberInput),
        prBranchInput.trim() || undefined
      )
    } catch (err) {
      prReviewsError = formatError(err, 'Failed to load review comments.')
      prReviews = []
    } finally {
      prReviewsLoading = false
    }
  }

  const handleGenerate = async (): Promise<void> => {
    if (prGenerating) return
    prGenerating = true
    prCreateError = null
    try {
      const generated: PullRequestGenerated = await generatePullRequestText(workspaceId, repo.id)
      prTitle = generated.title
      prBody = generated.body
    } catch (err) {
      prCreateError = formatError(err, 'Failed to generate PR text.')
    } finally {
      prGenerating = false
    }
  }

  const handleCreatePR = async (): Promise<void> => {
    if (prCreating) return
    prCreateError = null
    prCreateSuccess = null
    const title = prTitle.trim()
    if (!title) {
      prCreateError = 'PR title is required.'
      return
    }
    prCreating = true
    try {
      const created = await createPullRequest(workspaceId, repo.id, {
        title,
        body: prBody.trim(),
        base: prBase.trim() || undefined,
        head: prHead.trim() || undefined,
        draft: prDraft,
        autoCommit: true,
        autoPush: true
      })
      prCreateSuccess = created
      prTracked = created
      prNumberInput = `${created.number}`
      prStatus = {
        pullRequest: created,
        checks: []
      }
    } catch (err) {
      prCreateError = formatError(err, 'Failed to create pull request.')
    } finally {
      prCreating = false
    }
  }

  const handleSendReviews = async (): Promise<void> => {
    prReviewsError = null
    try {
      await sendPullRequestReviewsToTerminal(
        workspaceId,
        repo.id,
        parseNumber(prNumberInput),
        prBranchInput.trim() || undefined
      )
      prReviewsSent = true
    } catch (err) {
      prReviewsError = formatError(err, 'Failed to send reviews to terminal.')
    }
  }

  let filteredReviews = $derived(
    prReviews.filter((comment) =>
      selected?.path ? comment.path === selected.path : true
    )
  )

  const ensureRenderer = async (): Promise<void> => {
    if (diffModule || rendererLoading) return
    rendererLoading = true
    rendererError = null
    try {
      diffModule = (await import('@pierre/diffs')) as DiffsModule
    } catch (err) {
      rendererError = formatError(err, 'Diff renderer failed to load.')
    } finally {
      rendererLoading = false
    }
  }

  const loadTrackedPR = async (): Promise<void> => {
    try {
      const tracked = await fetchTrackedPullRequest(workspaceId, repo.id)
      if (!tracked) {
        return
      }
      prTracked = tracked
      if (!prNumberInput) {
        prNumberInput = `${tracked.number}`
      }
      if (!prBranchInput && tracked.headBranch) {
        prBranchInput = tracked.headBranch
      }
    } catch {
      // ignore tracking failures
    }
  }

  const renderDiff = (): void => {
    if (!diffModule || !selectedDiff || !diffContainer) return
    if (!diffInstance) {
      diffInstance = new diffModule.FileDiff(buildOptions())
    } else {
      diffInstance.setOptions(buildOptions())
    }
    diffInstance.render({
      fileDiff: selectedDiff,
      fileContainer: diffContainer,
      forceRender: true
    })
  }

  const selectFile = (file: RepoDiffFileSummary): void => {
    selected = file
    void loadFileDiff(file)
  }

  const loadSummary = async (): Promise<void> => {
    summaryLoading = true
    summaryError = null
    summary = null
    selected = null
    selectedDiff = null
    fileMeta = null
    fileError = null
    if (repo.statusKnown !== false && repo.missing) {
      summaryError = 'Repo is missing on disk. Restore it to view the diff.'
      summaryLoading = false
      return
    }
    const requestId = ++summaryRequest
    try {
      const data = await fetchRepoDiffSummary(workspaceId, repo.id)
      if (requestId !== summaryRequest) return
      summary = data
      if (summary.files.length > 0) {
        selectFile(summary.files[0])
      }
    } catch (err) {
      if (requestId !== summaryRequest) return
      summaryError = formatError(err, 'Failed to load diff summary.')
    } finally {
      if (requestId === summaryRequest) {
        summaryLoading = false
      }
    }
  }

  const loadFileDiff = async (file: RepoDiffFileSummary): Promise<void> => {
    fileLoading = true
    fileError = null
    fileMeta = null
    selectedDiff = null
    const requestId = ++fileRequest

    if (file.binary) {
      fileError = 'Binary files are not rendered yet.'
      fileLoading = false
      return
    }
    try {
      const response = await fetchRepoFileDiff(
        workspaceId,
        repo.id,
        file.path,
        file.prevPath ?? '',
        file.status
      )
      if (requestId !== fileRequest) return
      fileMeta = response
      if (response.truncated) {
        const kb = Math.max(1, Math.round(response.totalBytes / 1024))
        fileError = `Diff too large (${response.totalLines} lines, ${kb} KB).`
        return
      }
      if (!response.patch.trim()) {
        fileError = 'No diff available for this file.'
        return
      }
      await ensureRenderer()
      if (!diffModule) {
        fileError = rendererError ?? 'Diff renderer unavailable.'
        return
      }
      const parsed = diffModule.parsePatchFiles(response.patch)
      const fileDiff = parsed[0]?.files?.[0] ?? null
      if (!fileDiff) {
        fileError = 'Unable to parse diff content.'
        return
      }
      selectedDiff = fileDiff
      renderDiff()
    } catch (err) {
      if (requestId !== fileRequest) return
      fileError = formatError(err, 'Failed to load file diff.')
    } finally {
      if (requestId === fileRequest) {
        fileLoading = false
      }
    }
  }

  onMount(() => {
    void loadSummary()
    void loadTrackedPR()
  })

  onDestroy(() => {
    diffInstance?.cleanUp()
  })

  $effect(() => {
    if (selectedDiff && diffContainer) {
      renderDiff()
    }
  });
</script>

<section class="diff">
  <header class="diff-header">
    <div class="title">
      <div class="repo-name">{repo.name}</div>
      <div class="meta">
        {#if repo.defaultBranch}
          <span>Default branch: {repo.defaultBranch}</span>
        {/if}
        {#if repo.statusKnown === false}
          <span class="status unknown">unknown</span>
        {:else if repo.missing}
          <span class="status missing">missing</span>
        {:else if repo.dirty}
          <span class="status dirty">dirty</span>
        {:else}
          <span class="status clean">clean</span>
        {/if}
        {#if summary}
          <span>Files: {summary.files.length}</span>
          <span class="diffstat"><span class="add">+{summary.totalAdded}</span><span class="sep">/</span><span class="del">-{summary.totalRemoved}</span></span>
        {/if}
      </div>
    </div>
    <div class="controls">
      <div class="toggle">
        <button
          class:active={diffMode === 'split'}
          onclick={() => {
            diffMode = 'split'
            renderDiff()
          }}
          type="button"
        >
          Split
        </button>
        <button
          class:active={diffMode === 'unified'}
          onclick={() => {
            diffMode = 'unified'
            renderDiff()
          }}
          type="button"
        >
          Unified
        </button>
      </div>
      <button class="ghost" type="button" onclick={loadSummary}>Refresh</button>
      <button class="close" onclick={onClose} type="button">Back to terminal</button>
    </div>
  </header>

  <section class="pr-panel">
    <div class="pr-column">
      <div class="pr-title">Create pull request</div>
      <label class="field">
        <span>Title</span>
        <input type="text" bind:value={prTitle} placeholder="Summarize the change" />
      </label>
      <label class="field">
        <span>Body</span>
        <textarea rows="5" bind:value={prBody} placeholder="Describe the change"></textarea>
      </label>
      <div class="row">
        <label class="field">
          <span>Base</span>
          <input type="text" bind:value={prBase} placeholder="default branch" />
        </label>
        <label class="field">
          <span>Head</span>
          <input type="text" bind:value={prHead} placeholder="current branch" />
        </label>
      </div>
      <label class="checkbox">
        <input type="checkbox" bind:checked={prDraft} />
        Draft
      </label>
      {#if prCreateError}
        <div class="error">{prCreateError}</div>
      {/if}
      {#if prCreateSuccess}
        <div class="success">
          Created PR #{prCreateSuccess.number}.
          <a class="link" href={prCreateSuccess.url} target="_blank" rel="noreferrer">
            {prCreateSuccess.url}
          </a>
        </div>
      {:else if prTracked}
        <div class="success">
          Last PR #{prTracked.number}.
          <a class="link" href={prTracked.url} target="_blank" rel="noreferrer">
            {prTracked.url}
          </a>
        </div>
      {/if}
      <div class="actions">
        <button class="ghost" type="button" onclick={handleGenerate} disabled={prGenerating}>
          {prGenerating ? 'Generating…' : 'Generate with agent'}
        </button>
        <button type="button" onclick={handleCreatePR} disabled={prCreating}>
          {prCreating ? 'Creating…' : 'Create PR'}
        </button>
      </div>
    </div>
    <div class="pr-column">
      <div class="pr-title">Status & reviews</div>
      <div class="row">
        <label class="field">
          <span>PR #</span>
          <input type="text" bind:value={prNumberInput} placeholder="auto" />
        </label>
        <label class="field">
          <span>Branch</span>
          <input type="text" bind:value={prBranchInput} placeholder="current branch" />
        </label>
      </div>
      <div class="actions">
        <button class="ghost" type="button" onclick={loadPrStatus} disabled={prStatusLoading}>
          {prStatusLoading ? 'Loading…' : 'Refresh status'}
        </button>
        <button class="ghost" type="button" onclick={loadPrReviews} disabled={prReviewsLoading}>
          {prReviewsLoading ? 'Loading…' : 'Load reviews'}
        </button>
        <button type="button" onclick={handleSendReviews} disabled={prReviews.length === 0}>
          Send to agent
        </button>
      </div>
      {#if prStatusError}
        <div class="error">{prStatusError}</div>
      {/if}
      {#if prStatus}
        <div class="status-summary">
          <div class="status-title">
            PR #{prStatus.pullRequest.number}: {prStatus.pullRequest.title}
          </div>
          <div class="status-meta">
            <span>{prStatus.pullRequest.state}</span>
            {#if prStatus.pullRequest.draft}
              <span class="badge">Draft</span>
            {/if}
            {#if prStatus.pullRequest.mergeable}
              <span class="badge">{prStatus.pullRequest.mergeable}</span>
            {/if}
          </div>
          <div class="status-link">{prStatus.pullRequest.url}</div>
        </div>
        {#if prStatus.checks.length > 0}
          <div class="checks">
            {#each prStatus.checks as check}
              <div class="check-row">
                <span class="check-name">{check.name}</span>
                <span class="check-status">{check.status}</span>
                <span class="check-conclusion">{check.conclusion ?? ''}</span>
              </div>
            {/each}
          </div>
        {/if}
      {/if}
      {#if prReviewsError}
        <div class="error">{prReviewsError}</div>
      {/if}
      {#if prReviewsSent}
        <div class="success">Sent review feedback to terminal.</div>
      {/if}
      <div class="reviews">
        {#if prReviewsLoading}
          <div class="state compact">Loading reviews...</div>
        {:else if filteredReviews.length === 0}
          <div class="state compact">No review comments for the selected file.</div>
        {:else}
          {#each filteredReviews as comment}
            <div class="review">
              <div class="review-meta">
                <span class="path">{comment.path}{comment.line ? `:${comment.line}` : ''}</span>
                <span class="author">{comment.author ?? 'Reviewer'}</span>
              </div>
              <div class="review-body">{comment.body}</div>
            </div>
          {/each}
        {/if}
      </div>
    </div>
  </section>

  {#if summaryLoading}
    <div class="state">Loading diff summary...</div>
  {:else if summaryError}
    <div class="state error">
      <div class="message">{summaryError}</div>
      <button class="ghost" type="button" onclick={loadSummary}>Retry</button>
    </div>
  {:else if !summary || summary.files.length === 0}
    <div class="state">No changes detected in this repo.</div>
  {:else}
  <div class="diff-body">
      <aside class="file-list">
        <div class="section-title">Changed files</div>
        {#each summary.files as file}
          <button
            class:selected={file.path === selected?.path && file.prevPath === selected?.prevPath}
            class="file-row"
            onclick={() => selectFile(file)}
            type="button"
          >
            <div class="file-meta">
              <span class="path">{file.path}</span>
              {#if file.prevPath}
                <span class="rename">from {file.prevPath}</span>
              {/if}
            </div>
            <div class="stats">
              <span class="tag {file.status}">{statusLabel(file.status)}</span>
              <span class="diffstat"><span class="add">+{file.added}</span><span class="sep">/</span><span class="del">-{file.removed}</span></span>
            </div>
          </button>
        {/each}
      </aside>
      <div class="diff-view">
        <div class="file-header">
          <div class="file-title">
            <span>{selected?.path}</span>
            {#if selected?.prevPath}
              <span class="rename">from {selected.prevPath}</span>
            {/if}
          </div>
          <span class="diffstat">
            <span class="add">+{selected?.added ?? 0}</span><span class="sep">/</span><span class="del">-{selected?.removed ?? 0}</span>
            {#if fileMeta && !fileMeta.truncated && fileMeta.totalLines > 0}
              <span class="line-count">{fileMeta.totalLines} lines</span>
            {/if}
          </span>
        </div>
        {#if fileLoading || rendererLoading}
          <div class="state compact">Loading file diff...</div>
        {:else if fileError}
          <div class="state compact">{fileError}</div>
        {:else if rendererError}
          <div class="state compact">{rendererError}</div>
        {:else}
          <div class="diff-renderer">
            <diffs-container bind:this={diffContainer}></diffs-container>
          </div>
        {/if}
      </div>
    </div>
  {/if}
</section>

<style>
  .pr-panel {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 16px;
    padding: 16px;
    border-radius: 14px;
    background: var(--panel);
    border: 1px solid var(--border);
  }

  .pr-column {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .pr-title {
    font-weight: 600;
    font-size: 14px;
    color: var(--text);
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 12px;
    color: var(--muted);
  }

  .field input,
  .field textarea {
    background: var(--panel-soft);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 8px 10px;
    color: var(--text);
    font-size: 13px;
    font-family: inherit;
  }

  .field textarea {
    resize: vertical;
  }

  .row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: 10px;
  }

  .checkbox {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--text);
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .actions button {
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--accent);
    color: var(--text);
    padding: 8px 12px;
    font-size: 12px;
    cursor: pointer;
  }

  .actions button.ghost {
    background: transparent;
    color: var(--muted);
  }

  .status-summary {
    border-radius: 10px;
    border: 1px solid var(--border);
    padding: 10px;
    background: var(--panel-soft);
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .status-title {
    font-weight: 600;
    font-size: 13px;
  }

  .status-meta {
    display: flex;
    gap: 8px;
    font-size: 12px;
    color: var(--muted);
    flex-wrap: wrap;
  }

  .badge {
    padding: 2px 6px;
    border-radius: 999px;
    border: 1px solid var(--border);
    font-size: 11px;
    color: var(--text);
  }

  .status-link {
    font-size: 11px;
    color: var(--muted);
    word-break: break-all;
  }

  .checks {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .check-row {
    display: grid;
    grid-template-columns: 2fr 1fr 1fr;
    gap: 8px;
    font-size: 12px;
    color: var(--muted);
  }

  .check-name {
    color: var(--text);
  }

  .reviews {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .review {
    border-radius: 10px;
    border: 1px solid var(--border);
    padding: 8px;
    background: var(--panel-soft);
    font-size: 12px;
  }

  .review-meta {
    display: flex;
    justify-content: space-between;
    color: var(--muted);
    font-size: 11px;
  }

  .review-body {
    margin-top: 6px;
    color: var(--text);
    white-space: pre-wrap;
  }

  .error {
    color: var(--danger);
    font-size: 12px;
  }

  .success {
    color: var(--success);
    font-size: 12px;
  }

  .diff {
    display: flex;
    flex-direction: column;
    gap: 16px;
    height: 100%;
  }

  .diff-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
  }

  .title {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .repo-name {
    font-size: 20px;
    font-weight: 600;
  }

  .meta {
    display: flex;
    gap: 12px;
    color: var(--muted);
    font-size: 12px;
    flex-wrap: wrap;
  }

  .diffstat {
    font-weight: 600;
    display: inline-flex;
    gap: 8px;
    align-items: center;
  }

  .diffstat .add {
    color: var(--success);
  }

  .diffstat .del {
    color: var(--danger);
  }

  .diffstat .sep {
    color: var(--muted);
    margin: 0 -6px;
  }

  .line-count {
    font-size: 11px;
    color: var(--muted);
    font-weight: 500;
  }

  .status {
    font-weight: 600;
  }

  .dirty {
    color: var(--warning);
  }

  .missing {
    color: var(--danger);
  }

  .clean {
    color: var(--success);
  }

  .unknown {
    color: var(--muted);
  }

  .controls {
    display: flex;
    gap: 12px;
    align-items: center;
  }

  .toggle {
    display: inline-flex;
    border: 1px solid var(--border);
    border-radius: 10px;
    overflow: hidden;
    background: var(--panel);
  }

  .toggle button {
    background: transparent;
    border: none;
    color: var(--muted);
    padding: 6px 12px;
    cursor: pointer;
    font-size: 12px;
    transition: background var(--transition-fast), color var(--transition-fast);
  }

  .toggle button:hover:not(.active) {
    background: rgba(255, 255, 255, 0.04);
  }

  .toggle button.active {
    color: var(--text);
    background: var(--accent-subtle);
  }

  .close {
    background: var(--panel);
    border: 1px solid var(--border);
    color: var(--text);
    border-radius: var(--radius-sm);
    padding: 8px 12px;
    cursor: pointer;
    transition: border-color var(--transition-fast), background var(--transition-fast);
  }

  .close:hover {
    border-color: var(--accent);
    background: rgba(255, 255, 255, 0.04);
  }

  .state {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 20px;
    color: var(--muted);
  }

  .state.compact {
    padding: 16px;
    border-radius: 12px;
    background: var(--panel-soft);
    border: 1px dashed var(--border);
    text-align: center;
  }

  .state.error {
    color: var(--warning);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .diff-body {
    display: grid;
    grid-template-columns: 280px 1fr;
    gap: 16px;
    flex: 1;
    min-height: 0;
  }

  .file-list {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-height: 0;
    overflow: auto;
  }

  .section-title {
    font-size: 12px;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .file-row {
    display: flex;
    flex-direction: column;
    gap: 6px;
    background: transparent;
    border: 1px solid transparent;
    color: var(--text);
    text-align: left;
    padding: 10px;
    border-radius: var(--radius-md);
    cursor: pointer;
    transition: border-color var(--transition-fast), background var(--transition-fast);
  }

  .file-row:hover:not(.selected) {
    border-color: var(--border);
    background: rgba(255, 255, 255, 0.02);
  }

  .file-row.selected {
    background: var(--accent-subtle);
    border-color: var(--accent-soft);
  }

  .file-meta {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .path {
    font-size: 13px;
  }

  .rename {
    font-size: 11px;
    color: var(--muted);
  }

  .stats {
    display: flex;
    justify-content: space-between;
    font-size: 12px;
    color: var(--muted);
  }

  .tag {
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-size: 10px;
    font-weight: 600;
  }

  .tag.added {
    color: var(--success);
  }

  .tag.deleted {
    color: var(--danger);
  }

  .tag.renamed {
    color: var(--accent);
  }

  .tag.untracked {
    color: var(--warning);
  }

  .tag.binary {
    color: var(--muted);
  }

  .diff-view {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-height: 0;
    overflow: hidden;
  }

  .file-header {
    display: flex;
    justify-content: space-between;
    font-size: 13px;
    color: var(--muted);
  }

  .file-title {
    display: flex;
    gap: 8px;
    align-items: center;
    color: var(--text);
  }

  .diff-renderer {
    flex: 1;
    min-height: 0;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--panel-soft);
    padding: 8px;
    overflow: hidden;
    --diffs-dark-bg: var(--panel-soft);
    --diffs-dark: var(--text);
    --diffs-dark-addition-color: var(--success);
    --diffs-dark-deletion-color: var(--danger);
    --diffs-dark-modified-color: var(--accent);
    --diffs-font-family: var(--font-mono);
    --diffs-header-font-family: var(--font-body);
    --diffs-gap-block: 10px;
    --diffs-gap-inline: 12px;
  }

  diffs-container {
    display: block;
    height: 100%;
    width: 100%;
  }

  .ghost {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--border);
    color: var(--text);
    padding: 8px 12px;
    border-radius: var(--radius-md);
    cursor: pointer;
    font-size: 12px;
    transition: border-color var(--transition-fast), background var(--transition-fast);
  }

  .ghost:hover:not(:disabled) {
    border-color: var(--accent);
    background: rgba(255, 255, 255, 0.04);
  }

  .ghost:active:not(:disabled) {
    transform: scale(0.98);
  }

  .link {
    color: var(--accent);
    text-decoration: none;
  }

  .link:hover {
    text-decoration: underline;
  }
</style>
