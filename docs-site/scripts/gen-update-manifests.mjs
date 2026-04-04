/**
 * Generate update manifest JSON files for the in-app updater.
 * Outputs updates/stable.json and updates/alpha.json into public/.
 *
 * Ported from scripts/gen_update_manifests.py (MkDocs era).
 */

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const OWNER = 'strantalis';
const REPO = 'workset';
const API_URL = `https://api.github.com/repos/${OWNER}/${REPO}/releases`;
const CHANNELS = ['stable', 'alpha'];
const SHA256_RE = /\b([a-fA-F0-9]{64})\b/;
const TEAM_ID_RE = /\b([A-Z0-9]{10})\b/;

function nowRFC3339() {
  return new Date().toISOString().replace(/\.\d{3}Z$/, 'Z');
}

function headers() {
  const h = {
    Accept: 'application/vnd.github+json',
    'User-Agent': 'workset-docs-updater-manifests',
  };
  const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN;
  if (token) h.Authorization = `Bearer ${token}`;
  return h;
}

async function fetchReleases() {
  try {
    const res = await fetch(`${API_URL}?per_page=100`, { headers: headers() });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    return [data.filter((r) => !r.draft), null];
  } catch (e) {
    return [[], e.message];
  }
}

function selectRelease(releases, channel) {
  const prerelease = channel === 'alpha';
  let fallback = null;
  for (const release of releases) {
    if (Boolean(release.prerelease) !== prerelease) continue;
    if (!fallback) fallback = release;
    const tag = releaseTag(release);
    const names = new Set((release.assets || []).map((a) => a.name).filter(Boolean));
    const zip = `workset-${tag}-macos-update.zip`;
    if (names.has(zip) && names.has(`${zip}.sha256`) && names.has(`${zip}.teamid`)) {
      return release;
    }
  }
  return fallback;
}

async function fetchText(url) {
  try {
    const res = await fetch(url, { headers: headers() });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return [await res.text(), null];
  } catch (e) {
    return ['', e.message];
  }
}

function releaseTag(release) {
  if (!release) return 'v0.0.0';
  let tag = (release.tag_name || '').trim();
  if (!tag) return 'v0.0.0';
  if (!tag.startsWith('v')) tag = `v${tag}`;
  return tag;
}

function defaultLatest(version, notesUrl, assetUrl, generatedAt) {
  return {
    version,
    pubDate: generatedAt,
    notesUrl,
    minimumVersion: '',
    asset: { name: 'disabled', url: assetUrl, sha256: 'disabled' },
    signing: { teamId: 'DISABLED' },
  };
}

async function buildManifest(channel, release, error) {
  const generatedAt = nowRFC3339();
  const notesUrl = release?.html_url || 'https://workset.dev/releases/';
  const tag = releaseTag(release);
  const manifest = {
    schemaVersion: 1,
    generatedAt,
    channel,
    disabled: true,
    message: '',
    latest: defaultLatest(tag, notesUrl, notesUrl, generatedAt),
  };

  if (error) {
    manifest.message = `Update manifest generation failed: ${error}`;
    return manifest;
  }
  if (!release) {
    manifest.message = `No ${channel} release available yet.`;
    return manifest;
  }

  manifest.latest.version = tag;
  manifest.latest.pubDate = release.published_at || generatedAt;
  manifest.latest.notesUrl = notesUrl;

  const assetsByName = Object.fromEntries(
    (release.assets || []).filter((a) => a.name).map((a) => [a.name, a])
  );

  const zipName = `workset-${tag}-macos-update.zip`;
  const updateAsset = assetsByName[zipName];
  if (!updateAsset) {
    manifest.message = `Latest ${channel} release ${tag} does not include a macOS auto-update package yet.`;
    return manifest;
  }

  const updateURL = (updateAsset.browser_download_url || '').trim();
  if (!updateURL) {
    manifest.message = `Latest ${channel} release ${tag} has an invalid update asset URL.`;
    return manifest;
  }

  const shaAsset = assetsByName[`${zipName}.sha256`];
  const teamAsset = assetsByName[`${zipName}.teamid`];
  if (!shaAsset || !teamAsset) {
    manifest.latest.asset.name = zipName;
    manifest.latest.asset.url = updateURL;
    manifest.message = `Latest ${channel} release ${tag} is missing updater metadata assets (.sha256/.teamid).`;
    return manifest;
  }

  const [shaText, shaErr] = await fetchText(shaAsset.browser_download_url);
  const [teamText, teamErr] = await fetchText(teamAsset.browser_download_url);
  if (shaErr || teamErr) {
    manifest.latest.asset.name = zipName;
    manifest.latest.asset.url = updateURL;
    manifest.message = `Failed to load updater metadata for ${tag}: sha256=${shaErr || 'ok'}, teamid=${teamErr || 'ok'}.`;
    return manifest;
  }

  const shaMatch = SHA256_RE.exec(shaText);
  const teamMatch = TEAM_ID_RE.exec(teamText);
  if (!shaMatch || !teamMatch) {
    manifest.latest.asset.name = zipName;
    manifest.latest.asset.url = updateURL;
    manifest.message = `Updater metadata for ${tag} is invalid.`;
    return manifest;
  }

  manifest.disabled = false;
  manifest.message = '';
  manifest.latest.asset.name = zipName;
  manifest.latest.asset.url = updateURL;
  manifest.latest.asset.sha256 = shaMatch[1].toLowerCase();
  manifest.latest.signing.teamId = teamMatch[1];
  return manifest;
}

async function main() {
  const outDir = path.resolve(__dirname, '..', 'public', 'updates');
  fs.mkdirSync(outDir, { recursive: true });

  const [releases, error] = await fetchReleases();
  for (const channel of CHANNELS) {
    const release = selectRelease(releases, channel);
    const manifest = await buildManifest(channel, release, error);
    const outPath = path.join(outDir, `${channel}.json`);
    fs.writeFileSync(outPath, JSON.stringify(manifest, null, 2) + '\n');
    console.log(`wrote ${outPath} (${manifest.disabled ? 'disabled' : manifest.latest.version})`);
  }
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
