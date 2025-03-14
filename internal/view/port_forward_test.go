package view_test

import (
	"testing"

	"github.com/derailed/k9s/internal/client"
	"github.com/derailed/k9s/internal/view"
	"github.com/stretchr/testify/assert"
)

func TestPortForwardNew(t *testing.T) {
	pf := view.NewPortForward(client.NewGVR("portforwards"))

	assert.Nil(t, pf.Init(makeCtx()))
	assert.Equal(t, "PortForwards", pf.Name())
	assert.Equal(t, 9, len(pf.Hints()))
}
