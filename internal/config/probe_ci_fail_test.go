package config

import "testing"

// 临时探针：故意失败，用于验证 main 的 required checks 会挡住合并。验证后删除。
func TestProbeRequiredCheckBlocksMerge(t *testing.T) {
	t.Fatal("probe: intentional failure to verify required checks block merge")
}
