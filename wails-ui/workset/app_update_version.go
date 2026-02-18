package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var updateVersionExpr = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z]+)\.(\d+))?$`)

type parsedVersion struct {
	major       int
	minor       int
	patch       int
	preLabel    string
	preNum      int
	hasPrelabel bool
}

func normalizeVersion(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" || v == "dev" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}

func compareVersions(left, right string) (int, error) {
	lv, err := parseVersion(left)
	if err != nil {
		return 0, err
	}
	rv, err := parseVersion(right)
	if err != nil {
		return 0, err
	}
	if lv.major != rv.major {
		if lv.major > rv.major {
			return 1, nil
		}
		return -1, nil
	}
	if lv.minor != rv.minor {
		if lv.minor > rv.minor {
			return 1, nil
		}
		return -1, nil
	}
	if lv.patch != rv.patch {
		if lv.patch > rv.patch {
			return 1, nil
		}
		return -1, nil
	}
	if lv.hasPrelabel != rv.hasPrelabel {
		if !lv.hasPrelabel {
			return 1, nil
		}
		return -1, nil
	}
	if !lv.hasPrelabel {
		return 0, nil
	}
	if lv.preLabel != rv.preLabel {
		if lv.preLabel > rv.preLabel {
			return 1, nil
		}
		return -1, nil
	}
	if lv.preNum != rv.preNum {
		if lv.preNum > rv.preNum {
			return 1, nil
		}
		return -1, nil
	}
	return 0, nil
}

func parseVersion(raw string) (parsedVersion, error) {
	matches := updateVersionExpr.FindStringSubmatch(strings.TrimSpace(raw))
	if len(matches) == 0 {
		return parsedVersion{}, fmt.Errorf("invalid version format: %q", raw)
	}
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])
	result := parsedVersion{
		major: major,
		minor: minor,
		patch: patch,
	}
	if matches[4] != "" {
		result.hasPrelabel = true
		result.preLabel = matches[4]
		preNum, _ := strconv.Atoi(matches[5])
		result.preNum = preNum
	}
	return result, nil
}
