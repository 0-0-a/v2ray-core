package kcp_test

import (
	"testing"

	"github.com/v2ray/v2ray-core/testing/assert"
	. "github.com/v2ray/v2ray-core/transport/internet/kcp"
)

func TestRecivingWindow(t *testing.T) {
	assert := assert.On(t)

	window := NewReceivingWindow(3)

	seg0 := &Segment{}
	seg1 := &Segment{}
	seg2 := &Segment{}
	seg3 := &Segment{}

	assert.Bool(window.Set(0, seg0)).IsTrue()
	assert.Pointer(window.RemoveFirst()).Equals(seg0)

	assert.Bool(window.Set(1, seg1)).IsTrue()
	assert.Bool(window.Set(2, seg2)).IsTrue()

	window.Advance()
	assert.Bool(window.Set(2, seg3)).IsTrue()

	assert.Pointer(window.RemoveFirst()).Equals(seg1)
	assert.Pointer(window.Remove(1)).Equals(seg2)
	assert.Pointer(window.Remove(2)).Equals(seg3)
}
