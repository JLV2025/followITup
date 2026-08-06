package api

import "testing"

func TestHasBadEncoding(t *testing.T) {
	// 正常 UTF-8 中文:不触发
	if hasBadEncoding("新房装修") {
		t.Error("正常中文不应触发坏编码检测")
	}
	if hasBadEncoding("UCD COPS System Upgrade") {
		t.Error("英文不应触发")
	}
	// 单个替换字符(单字节坏数据边缘):不触发(需连续 2 个)
	if hasBadEncoding("a�b") {
		t.Error("单个替换字符不应触发")
	}
	// GBK 字节被当 UTF-8 解析的典型结果:连串 U+FFFD → 必须触发
	if !hasBadEncoding("需求分析\xEF\xBF\xBD\xEF\xBF\xBD") {
		t.Error("连续替换字符应触发坏编码检测")
	}
	if !hasBadEncoding("����测试") {
		t.Error("GBK 中文(4 个连续替换字符)应触发")
	}
}
