import {describe, expect, it} from 'vitest'
import {looksLikeUrl} from './names'

describe('looksLikeUrl - security validation', () => {
  describe('valid URLs that should pass', () => {
    it('accepts HTTPS GitHub URLs', () => {
      expect(looksLikeUrl('https://github.com/org/repo')).toBe(true)
      expect(looksLikeUrl('https://github.com/org/repo.git')).toBe(true)
    })

    it('accepts HTTP GitHub URLs', () => {
      expect(looksLikeUrl('http://github.com/org/repo')).toBe(true)
    })

    it('accepts SSH GitHub URLs', () => {
      expect(looksLikeUrl('git@github.com:org/repo.git')).toBe(true)
      expect(looksLikeUrl('ssh://git@github.com/org/repo.git')).toBe(true)
    })

    it('accepts GitLab URLs', () => {
      expect(looksLikeUrl('https://gitlab.com/org/repo')).toBe(true)
      expect(looksLikeUrl('git@gitlab.com:org/repo.git')).toBe(true)
    })

    it('accepts Bitbucket URLs', () => {
      expect(looksLikeUrl('https://bitbucket.org/org/repo')).toBe(true)
      expect(looksLikeUrl('git@bitbucket.org:org/repo.git')).toBe(true)
    })

    it('accepts subdomain URLs (enterprise instances)', () => {
      // enterprise.github.com is a subdomain of github.com
      expect(looksLikeUrl('https://enterprise.github.com/org/repo')).toBe(true)
      expect(looksLikeUrl('https://company.gitlab.com/org/repo')).toBe(true)
      expect(looksLikeUrl('git@enterprise.github.com:org/repo.git')).toBe(true)
    })

    it('handles URLs with paths and query strings', () => {
      expect(looksLikeUrl('https://github.com/org/repo?ref=main')).toBe(true)
      expect(looksLikeUrl('https://github.com/org/repo/tree/main')).toBe(true)
    })

    it('handles URLs with ports', () => {
      expect(looksLikeUrl('https://github.com:8443/org/repo')).toBe(true)
    })

    it('handles URLs with userinfo', () => {
      expect(looksLikeUrl('https://user:pass@github.com/org/repo')).toBe(true)
    })

    it('handles URLs with trailing slashes', () => {
      expect(looksLikeUrl('https://github.com/org/repo/')).toBe(true)
    })

    it('is case insensitive for hostnames', () => {
      expect(looksLikeUrl('https://GITHUB.COM/org/repo')).toBe(true)
      expect(looksLikeUrl('https://GitHub.com/org/repo')).toBe(true)
      expect(looksLikeUrl('git@GITHUB.COM:org/repo.git')).toBe(true)
    })
  })

  describe('malicious URLs that should be rejected', () => {
    it('rejects URLs with allowed host in path (SSRF bypass attempt)', () => {
      expect(looksLikeUrl('https://evil.com/github.com/malicious/repo')).toBe(false)
      expect(looksLikeUrl('https://evil.com/gitlab.com/malicious/repo')).toBe(false)
      expect(looksLikeUrl('https://evil.com/bitbucket.org/malicious/repo')).toBe(false)
    })

    it('rejects URLs with allowed host in subdomain of malicious domain', () => {
      expect(looksLikeUrl('https://github.com.evil.com/malicious/repo')).toBe(false)
      expect(looksLikeUrl('https://gitlab.com.attacker.net/malicious/repo')).toBe(false)
      expect(looksLikeUrl('https://bitbucket.org.phishing.io/malicious/repo')).toBe(false)
    })

    it('rejects URLs with allowed host in query string', () => {
      expect(looksLikeUrl('https://evil.com/?redirect=github.com/malicious')).toBe(false)
      expect(looksLikeUrl('https://evil.com/?x=github.com&y=malicious')).toBe(false)
    })

    it('rejects URLs with allowed host in fragment', () => {
      expect(looksLikeUrl('https://evil.com/repo#github.com')).toBe(false)
    })

    it('rejects URLs with allowed host as username in userinfo', () => {
      expect(looksLikeUrl('https://github.com@evil.com/malicious/repo')).toBe(false)
    })

    it('rejects lookalike domains', () => {
      expect(looksLikeUrl('https://github-com.evil.com/repo')).toBe(false)
      expect(looksLikeUrl('https://githubcom.evil.com/repo')).toBe(false)
      expect(looksLikeUrl('https://githuub.com/malicious/repo')).toBe(false)
    })

    it('rejects completely unrelated domains', () => {
      expect(looksLikeUrl('https://evil.com/malicious/repo')).toBe(false)
      expect(looksLikeUrl('https://attacker.net/malicious/repo')).toBe(false)
      expect(looksLikeUrl('http://phishing.io/malicious/repo')).toBe(false)
    })

    it('rejects malformed SSH URLs targeting allowed hosts', () => {
      expect(looksLikeUrl('git@evil.com:github.com/repo.git')).toBe(false)
      expect(looksLikeUrl('git@github.com.evil.com:repo.git')).toBe(false)
    })
  })

  describe('edge cases', () => {
    it('handles empty and whitespace-only strings', () => {
      expect(looksLikeUrl('')).toBe(false)
      expect(looksLikeUrl('   ')).toBe(false)
      expect(looksLikeUrl('\t\n')).toBe(false)
    })

    it('handles non-URL strings', () => {
      expect(looksLikeUrl('not a url')).toBe(false)
      expect(looksLikeUrl('/local/path/to/repo')).toBe(false)
      expect(looksLikeUrl('~/projects/repo')).toBe(false)
      expect(looksLikeUrl('C:\\Users\\repo')).toBe(false)
    })

    it('handles URLs without scheme but with allowed host as path', () => {
      expect(looksLikeUrl('github.com/repo')).toBe(false)
      expect(looksLikeUrl('gitlab.com/org/repo')).toBe(false)
    })

    it('rejects invalid URL formats', () => {
      expect(looksLikeUrl('https://')).toBe(false)
      expect(looksLikeUrl('git@')).toBe(false)
      expect(looksLikeUrl('not-a-valid-url')).toBe(false)
    })

    it('rejects FTP and other non-git protocols', () => {
      expect(looksLikeUrl('ftp://github.com/repo')).toBe(false)
      expect(looksLikeUrl('file://github.com/repo')).toBe(false)
      expect(looksLikeUrl('sftp://github.com/repo')).toBe(false)
    })

    it('handles SSH URLs with unusual formats', () => {
      expect(looksLikeUrl('git@host-without-colon')).toBe(false)
      expect(looksLikeUrl('git@')).toBe(false)
    })

    it('handles URLs with unicode or special characters', () => {
      expect(looksLikeUrl('https://github.com/org/répo')).toBe(true)
      expect(looksLikeUrl('https://github.com/org/repo%20name')).toBe(true)
    })
  })

  describe('derived behavior', () => {
    it('matches typical user inputs correctly', () => {
      // These are what users typically type
      expect(looksLikeUrl('https://github.com/strantalis/workset')).toBe(true)
      expect(looksLikeUrl('git@github.com:strantalis/workset.git')).toBe(true)
      expect(looksLikeUrl('https://gitlab.com/user/project')).toBe(true)
      expect(looksLikeUrl('https://bitbucket.org/team/repo')).toBe(true)

      // These are paths, not URLs
      expect(looksLikeUrl('./my-project')).toBe(false)
      expect(looksLikeUrl('/Users/sean/projects/workset')).toBe(false)
      expect(looksLikeUrl('~/workset')).toBe(false)
    })
  })
})
