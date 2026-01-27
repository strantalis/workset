package main

import "runtime/debug"

var appVersion = "dev"
var appCommit = ""

type AppVersion struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Dirty   bool   `json:"dirty"`
}

func (a *App) GetAppVersion() AppVersion {
	version := appVersion
	commit := appCommit
	dirty := false

	if info, ok := debug.ReadBuildInfo(); ok {
		if (version == "" || version == "dev") && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if commit == "" {
					commit = setting.Value
				}
			case "vcs.modified":
				if setting.Value == "true" {
					dirty = true
				}
			}
		}
	}

	if version == "" {
		version = "dev"
	}

	return AppVersion{
		Version: version,
		Commit:  commit,
		Dirty:   dirty,
	}
}
