package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var teamIDExpr = regexp.MustCompile(`TeamIdentifier=([A-Z0-9]+)`)

type queuedSymlink struct {
	path   string
	target string
}

type updatePackageSelection struct {
	AssetURL      string
	AssetSHA256   string
	SigningTeamID string
}

func selectUpdatePackage(release UpdateRelease) (updatePackageSelection, error) {
	assetURL := strings.TrimSpace(release.Asset.URL)
	if assetURL == "" {
		return updatePackageSelection{}, errors.New("update asset URL is required")
	}
	if _, err := validateUpdateURL(assetURL); err != nil {
		return updatePackageSelection{}, fmt.Errorf("update asset URL is invalid: %w", err)
	}

	assetSHA256 := strings.TrimSpace(release.Asset.SHA256)
	if assetSHA256 == "" {
		return updatePackageSelection{}, errors.New("update asset checksum is required")
	}

	signingTeamID := strings.TrimSpace(release.Signing.TeamID)
	if signingTeamID == "" {
		return updatePackageSelection{}, errors.New("update signing team id is required")
	}

	return updatePackageSelection{
		AssetURL:      assetURL,
		AssetSHA256:   assetSHA256,
		SigningTeamID: signingTeamID,
	}, nil
}

func verifySHA256(path, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return errors.New("expected checksum is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	sum := strings.ToLower(hex.EncodeToString(hash.Sum(nil)))
	if sum != expected {
		return fmt.Errorf("checksum mismatch: expected %s got %s", expected, sum)
	}
	return nil
}

func extractAppBundle(zipPath, stageRoot string) (string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	extractRoot := filepath.Join(stageRoot, "extracted")
	if err := os.MkdirAll(extractRoot, 0o755); err != nil {
		return "", err
	}
	absExtractRoot, err := filepath.Abs(extractRoot)
	if err != nil {
		return "", err
	}

	var symlinks []queuedSymlink
	for _, file := range reader.File {
		target, err := safeExtractTarget(absExtractRoot, file.Name)
		if err != nil {
			return "", err
		}
		mode := file.Mode()
		switch {
		case mode.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
			continue
		case mode&os.ModeSymlink != 0:
			src, err := file.Open()
			if err != nil {
				return "", err
			}
			targetBytes, readErr := io.ReadAll(src)
			closeErr := src.Close()
			if readErr != nil {
				return "", readErr
			}
			if closeErr != nil {
				return "", closeErr
			}
			symlinks = append(symlinks, queuedSymlink{
				path:   target,
				target: string(targetBytes),
			})
			continue
		case mode.IsRegular():
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			src, err := file.Open()
			if err != nil {
				return "", err
			}
			perm := mode.Perm()
			if perm == 0 {
				perm = 0o644
			}
			dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
			if err != nil {
				src.Close()
				return "", err
			}
			_, copyErr := io.Copy(dst, src)
			closeErr := dst.Close()
			srcCloseErr := src.Close()
			if copyErr != nil {
				return "", copyErr
			}
			if closeErr != nil {
				return "", closeErr
			}
			if srcCloseErr != nil {
				return "", srcCloseErr
			}
		default:
			return "", fmt.Errorf("unsupported entry type in archive: %s", file.Name)
		}
	}

	for _, link := range symlinks {
		symlinkTarget, err := sanitizeSymlinkTarget(absExtractRoot, link.path, link.target)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(link.path), 0o755); err != nil {
			return "", err
		}
		_ = os.Remove(link.path)
		if err := os.Symlink(symlinkTarget, link.path); err != nil {
			return "", err
		}
	}

	var found string
	walkErr := filepath.WalkDir(extractRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".app") {
			found = path
			return errors.New("found")
		}
		return nil
	})
	if walkErr != nil && walkErr.Error() != "found" {
		return "", walkErr
	}
	if found == "" {
		return "", errors.New("no .app bundle found in update archive")
	}
	return found, nil
}

func safeExtractTarget(absRoot, name string) (string, error) {
	if strings.ContainsRune(name, '\x00') {
		return "", fmt.Errorf("unsafe path in archive: %q", name)
	}
	cleanName := filepath.Clean(name)
	if filepath.IsAbs(cleanName) {
		return "", fmt.Errorf("unsafe absolute path in archive: %q", name)
	}
	target := filepath.Join(absRoot, cleanName)
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe path in archive: %q", name)
	}
	return absTarget, nil
}

func sanitizeSymlinkTarget(absRoot, linkPath, rawTarget string) (string, error) {
	target := filepath.Clean(strings.TrimSpace(rawTarget))
	if target == "" || target == "." {
		return "", fmt.Errorf("unsafe symlink target: %q", rawTarget)
	}
	if filepath.IsAbs(target) {
		return "", fmt.Errorf("absolute symlink targets are not allowed: %q", rawTarget)
	}
	resolved := filepath.Join(filepath.Dir(linkPath), target)
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absResolved)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("symlink target escapes extraction root: %q", rawTarget)
	}
	return target, nil
}

func verifyCodesign(appPath, expectedTeamID string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("codesign validation is only supported on macOS")
	}
	expectedTeamID = strings.TrimSpace(expectedTeamID)
	if expectedTeamID == "" {
		return errors.New("missing expected signing team id")
	}
	verify := exec.Command("codesign", "--verify", "--deep", "--strict", "--verbose=2", appPath)
	if out, err := verify.CombinedOutput(); err != nil {
		return fmt.Errorf("codesign verify failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	details := exec.Command("codesign", "-dv", "--verbose=4", appPath)
	out, err := details.CombinedOutput()
	if err != nil {
		return fmt.Errorf("codesign inspect failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	matches := teamIDExpr.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return errors.New("could not read TeamIdentifier from codesign output")
	}
	teamID := strings.TrimSpace(matches[1])
	if teamID != expectedTeamID {
		return fmt.Errorf("unexpected TeamIdentifier: expected %s got %s", expectedTeamID, teamID)
	}
	return nil
}

func currentBundlePath() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return bundlePathFromExecutable(executablePath)
}

func bundlePathFromExecutable(executablePath string) (string, error) {
	marker := filepath.Join("Contents", "MacOS") + string(filepath.Separator)
	index := strings.LastIndex(executablePath, marker)
	if index <= 0 {
		return "", errors.New("current binary is not running from a macOS app bundle")
	}
	bundlePath := filepath.Clean(executablePath[:index])
	if !strings.EqualFold(filepath.Ext(bundlePath), ".app") {
		return "", errors.New("resolved bundle path is invalid")
	}
	return bundlePath, nil
}

func updaterHelperPath(bundlePath string) (string, error) {
	resourcePath := filepath.Join(bundlePath, "Contents", "Resources", "workset-updater")
	if info, err := os.Stat(resourcePath); err == nil && !info.IsDir() {
		return resourcePath, nil
	}
	overridePath := strings.TrimSpace(os.Getenv("WORKSET_UPDATER_HELPER_PATH"))
	if overridePath != "" {
		if !filepath.IsAbs(overridePath) {
			return "", errors.New("WORKSET_UPDATER_HELPER_PATH must be an absolute path")
		}
		if info, err := os.Stat(overridePath); err == nil && !info.IsDir() {
			return overridePath, nil
		}
		return "", fmt.Errorf("WORKSET_UPDATER_HELPER_PATH not found: %s", overridePath)
	}
	return "", errors.New("workset-updater helper executable not found in app bundle resources")
}
