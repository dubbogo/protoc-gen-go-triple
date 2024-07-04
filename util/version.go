// @Package: util
// @Author: linfuchuan
// @Date: 2024/7/4 20:40

package util

import (
	"github.com/hashicorp/go-version"
	"runtime"
)

// IsVersionGreaterThan118
// @Description: 是否运行时go版本大于1.18
// @return bool
func IsVersionGreaterThan118() bool {
	version1_18, _ := version.NewVersion("1.18")
	v, _ := version.NewVersion(runtime.Version()[2:])
	return v.GreaterThanOrEqual(version1_18)
}
