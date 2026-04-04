import fs from 'node:fs';
import path from 'node:path';

const OWNER = 'strantalis';
const REPO = 'workset';
const API_URL = `https://api.github.com/repos/${OWNER}/${REPO}/releases`;
const MAX_RELEASES = parseInt(process.env.WORKSET_RELEASES_LIMIT || '10', 10);
const OUTPUT_PATH = path.join(import.meta.dirname, '..', 'src', 'content', 'docs', 'changelog.mdx');

function formatDate(value) {
  if (!value) return null;
  try {
    return new Date(value).toISOString().split('T')[0];
  } catch {
    return null;
  }
}

async function fetchReleases() {
  const headers = {
    'Accept': 'application/vnd.github+json',
    'User-Agent': 'workset-docs',
  };
  const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN;
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  try {
    const response = await fetch(API_URL, { headers });
    if (!response.ok) {
      return { releases: [], error: `${response.status} ${response.statusText}` };
    }
    const data = await response.json();
    const releases = data.filter(r => !r.draft && !r.prerelease);
    return { releases: releases.slice(0, MAX_RELEASES), error: null };
  } catch (err) {
    return { releases: [], error: err.message };
  }
}

function stripRedundantHeading(body, tag, name) {
  const lines = body.split('\n');
  let i = 0;
  while (i < lines.length && !lines[i].trim()) i++;
  if (i >= lines.length) return body;

  const first = lines[i].trim();
  if (!first.startsWith('#')) return body;

  const headingText = first.replace(/^#+/, '').trim().toLowerCase();
  if (headingText.includes(tag.toLowerCase()) || headingText.includes(name.toLowerCase())) {
    i++;
    while (i < lines.length && !lines[i].trim()) i++;
    return lines.slice(i).join('\n');
  }

  return body;
}

async function main() {
  const { releases, error } = await fetchReleases();

  let content = `---
title: Changelog
description: Release notes from GitHub.
---

Latest release notes from GitHub.

`;

  if (error) {
    content += `:::warning
Failed to fetch releases from GitHub.

Error: \`${error}\`

If you're running locally, set \`GITHUB_TOKEN\` to increase rate limits.
:::\n\n`;
  } else if (releases.length === 0) {
    content += `_No releases found._\n\n`;
  } else {
    for (const release of releases) {
      const tag = release.tag_name || '(untagged)';
      const name = release.name || tag;
      const url = release.html_url;
      const published = formatDate(release.published_at || release.created_at);

      content += `## ${name}\n\n`;
      const meta = [];
      if (published) meta.push(`Released ${published}`);
      if (url) meta.push(`[View on GitHub](${url})`);
      if (meta.length) content += meta.join(' · ') + '\n\n';

      const body = (release.body || '').trim();
      const cleaned = stripRedundantHeading(body, tag, name).trim();
      if (cleaned) {
        content += `<details>\n<summary>Release notes</summary>\n\n${cleaned}\n\n</details>\n\n`;
      } else {
        content += `_No release notes provided._\n\n`;
      }
    }
  }

  fs.writeFileSync(OUTPUT_PATH, content, 'utf-8');
  console.log(`Changelog generated with ${releases.length} releases.`);
}

main();
