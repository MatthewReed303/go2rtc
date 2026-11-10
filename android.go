//go:build android

package main

import (
	"os"
	"strconv"

	"github.com/wlynxg/anet"
)

// Android 11+ forbids apps from using netlink RTM_GETLINK, so Go's
// net.Interfaces() fails with "route ip+net: netlinkrib: permission denied"
// and pion (WebRTC) cannot gather candidates. pion already enumerates
// interfaces through github.com/wlynxg/anet, which has a netlink-free path
// for Android 11+, but without cgo anet cannot detect the Android version.
// The host app passes it in ANDROID_VERSION (android.os.Build.VERSION.RELEASE
// major number, e.g. "14").
func init() {
	if v, err := strconv.ParseUint(os.Getenv("ANDROID_VERSION"), 10, 32); err == nil {
		anet.SetAndroidVersion(uint(v))
	}
}
