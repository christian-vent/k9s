package ui_test

import (
	"testing"

	"github.com/derailed/k9s/internal/ui"
	"github.com/stretchr/testify/assert"
)

type testListener struct {
	text  string
	act   int
	inact int
}

func (l *testListener) BufferChanged(s string) {
	l.text = s
}

func (l *testListener) BufferActive(s bool, _ ui.BufferKind) {
	if s {
		l.act++
		return
	}
	l.inact++
}

func TestCmdBuffActivate(t *testing.T) {
	b, l := ui.NewCmdBuff('>', ui.CommandBuff), testListener{}
	b.AddListener(&l)

	b.SetActive(true)
	assert.Equal(t, 1, l.act)
	assert.Equal(t, 0, l.inact)
	assert.True(t, b.IsActive())
}

func TestCmdBuffDeactivate(t *testing.T) {
	b, l := ui.NewCmdBuff('>', ui.CommandBuff), testListener{}
	b.AddListener(&l)

	b.SetActive(false)
	assert.Equal(t, 0, l.act)
	assert.Equal(t, 1, l.inact)
	assert.False(t, b.IsActive())
}

func TestCmdBuffChanged(t *testing.T) {
	b, l := ui.NewCmdBuff('>', ui.CommandBuff), testListener{}
	b.AddListener(&l)

	b.Add('b')
	assert.Equal(t, 0, l.act)
	assert.Equal(t, 0, l.inact)
	assert.Equal(t, "b", l.text)
	assert.Equal(t, "b", b.String())

	b.Delete()
	assert.Equal(t, 0, l.act)
	assert.Equal(t, 0, l.inact)
	assert.Equal(t, "", l.text)
	assert.Equal(t, "", b.String())

	b.Add('c')
	b.Clear()
	assert.Equal(t, 0, l.act)
	assert.Equal(t, 0, l.inact)
	assert.Equal(t, "", l.text)
	assert.Equal(t, "", b.String())

	b.Add('c')
	b.Reset()
	assert.Equal(t, 0, l.act)
	assert.Equal(t, 1, l.inact)
	assert.Equal(t, "", l.text)
	assert.Equal(t, "", b.String())
	assert.True(t, b.Empty())
}

func TestCmdBuffAdd(t *testing.T) {
	b := ui.NewCmdBuff('>', ui.CommandBuff)

	uu := []struct {
		runes []rune
		cmd   string
	}{
		{[]rune{}, ""},
		{[]rune{'a'}, "a"},
		{[]rune{'a', 'b', 'c'}, "abc"},
	}

	for _, u := range uu {
		for _, r := range u.runes {
			b.Add(r)
		}
		assert.Equal(t, u.cmd, b.String())
		b.Reset()
	}
}

func TestCmdBuffDel(t *testing.T) {
	b := ui.NewCmdBuff('>', ui.CommandBuff)

	uu := []struct {
		runes []rune
		cmd   string
	}{
		{[]rune{}, ""},
		{[]rune{'a'}, ""},
		{[]rune{'a', 'b', 'c'}, "ab"},
	}

	for _, u := range uu {
		for _, r := range u.runes {
			b.Add(r)
		}
		b.Delete()
		assert.Equal(t, u.cmd, b.String())
		b.Reset()
	}
}

func TestCmdBuffEmpty(t *testing.T) {
	b := ui.NewCmdBuff('>', ui.CommandBuff)

	uu := []struct {
		runes []rune
		empty bool
	}{
		{[]rune{}, true},
		{[]rune{'a'}, false},
		{[]rune{'a', 'b', 'c'}, false},
	}

	for _, u := range uu {
		for _, r := range u.runes {
			b.Add(r)
		}
		assert.Equal(t, u.empty, b.Empty())
		b.Reset()
	}
}
