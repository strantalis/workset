<script>
  import { onMount } from 'svelte';

  let os = $state('Detecting your platform...');
  let downloadUrl = $state('');
  let downloadLabel = $state('');
  let altHtml = $state('');
  const repo = 'strantalis/workset';
  const latest = 'https://github.com/' + repo + '/releases/latest';

  onMount(async () => {
    const ua = navigator.userAgent.toLowerCase();
    const isMac = ua.includes('mac');
    const isWin = ua.includes('win');

    let version = 'v0.3.0';
    try {
      const res = await fetch('https://api.github.com/repos/' + repo + '/releases/latest');
      const data = await res.json();
      if (data.tag_name) version = data.tag_name;
    } catch {}

    const base = 'https://github.com/' + repo + '/releases/download/' + version;

    if (isMac) {
      os = 'macOS detected';
      downloadUrl = base + '/workset-' + version + '-macos.dmg';
      downloadLabel = 'Download for Mac';
      altHtml = 'Also available for <a href="#windows">Windows</a> · <a href="' + latest + '">All downloads</a>';
    } else if (isWin) {
      os = 'Windows detected';
      downloadUrl = base + '/workset-' + version + '-windows.exe';
      downloadLabel = 'Download for Windows';
      altHtml = 'Also available for <a href="#macos-app">Mac</a> · <a href="' + latest + '">All downloads</a>';
    } else {
      os = 'Desktop app available for macOS and Windows';
      downloadUrl = latest;
      downloadLabel = 'View All Downloads';
      altHtml = 'Linux users: install the CLI below.';
    }
  });
</script>

<div class="download-hero">
  <p class="download-os">{os}</p>
  {#if downloadUrl}
    <div class="download-buttons">
      <a href={downloadUrl} class="starlight-btn starlight-btn--primary">{downloadLabel}</a>
    </div>
  {/if}
  {#if altHtml}
    <p class="download-alt">{@html altHtml}</p>
  {/if}
</div>
