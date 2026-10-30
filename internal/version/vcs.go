// Copyright 2022 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package version

import (
	"runtime/debug"
	"time"
)

// In go 1.18 and beyond, the go tool embeds VCS information into the build.

const (
	govcsTimeLayout = "2006-01-02T15:04:05Z"
	ourTimeLayout   = "20060102"
)

// These variables are set at build-time by the linker when the build is
// done by build/ci.go.
//
// build/ci.go 的 buildFlags 在 env.Commit 非空时写进 -ldflags：
// -X .../internal/version.gitCommit=<hash> 和 gitDate=<YYYYMMDD>。
// hash 来自 CI 的提交 SHA，或本地 .git/HEAD，也可用 -git-commit 覆盖。
// 日期是 git show -s --format=%ct 转成的 YYYYMMDD。普通 go build 不写这两个变量。
var gitCommit, gitDate string

// VCSInfo represents the git repository state.
type VCSInfo struct {
	Commit string // head commit hash
	Date   string // commit time in YYYYMMDD format
	Dirty  bool
}

// VCS returns version control information of the current executable.
//
// 优先用 build/ci.go 链接进去的 gitCommit、gitDate。那条路径不带 Dirty。
// 没有链接信息时，只在本模块是主模块时读 Go 嵌入的 vcs.*。
func VCS() (VCSInfo, bool) {
	if gitCommit != "" {
		// Use information set by the build script if present.
		return VCSInfo{Commit: gitCommit, Date: gitDate}, true
	}
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		if buildInfo.Main.Path == ourPath {
			return buildInfoVCS(buildInfo)
		}
	}
	return VCSInfo{}, false
}

// buildInfoVCS returns VCS information of the build.
//
// commit 和日期都要有才算成功。日期解析失败时整段丢弃。Dirty 只在 vcs.modified 为 true 时置位。
func buildInfoVCS(info *debug.BuildInfo) (s VCSInfo, ok bool) {
	for _, v := range info.Settings {
		switch v.Key {
		case "vcs.revision":
			s.Commit = v.Value
		case "vcs.modified":
			if v.Value == "true" {
				s.Dirty = true
			}
		case "vcs.time":
			t, err := time.Parse(govcsTimeLayout, v.Value)
			if err == nil {
				s.Date = t.Format(ourTimeLayout)
			}
		}
	}
	if s.Commit != "" && s.Date != "" {
		ok = true
	}
	return
}
