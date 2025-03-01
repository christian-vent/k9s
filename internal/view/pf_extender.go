package view

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/derailed/k9s/internal/client"
	"github.com/derailed/k9s/internal/dao"
	"github.com/derailed/k9s/internal/ui"
	"github.com/derailed/k9s/internal/watch"
	"github.com/gdamore/tcell"
	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/portforward"
)

// PortForwardExtender adds port-forward extensions.
type PortForwardExtender struct {
	ResourceViewer
}

// NewPortForwardExtender returns a new extender.
func NewPortForwardExtender(r ResourceViewer) ResourceViewer {
	s := PortForwardExtender{ResourceViewer: r}
	s.bindKeys(s.Actions())

	return &s
}

func (p *PortForwardExtender) bindKeys(aa ui.KeyActions) {
	aa.Add(ui.KeyActions{
		ui.KeyShiftF: ui.NewKeyAction("Port-Forward", p.portFwdCmd, true),
	})
}

func (p *PortForwardExtender) portFwdCmd(evt *tcell.EventKey) *tcell.EventKey {
	path := p.GetTable().GetSelectedItem()
	if path == "" {
		return evt
	}

	pod, err := p.fetchPodName(path)
	if err != nil {
		p.App().Flash().Err(err)
		return nil
	}
	if err := showFwdDialog(p, pod, startFwdCB); err != nil {
		p.App().Flash().Err(err)
	}

	return nil
}

func (p *PortForwardExtender) fetchPodName(path string) (string, error) {
	res, err := dao.AccessorFor(p.App().factory, p.GVR())
	if err != nil {
		return "", nil
	}
	ctrl, ok := res.(dao.Controller)
	if !ok {
		return "", fmt.Errorf("expecting a controller resource for %q", p.GVR())
	}

	return ctrl.Pod(path)
}

// ----------------------------------------------------------------------------
// Helpers...

func tryListenPort(address, port string) error {
	server, err := net.Listen("tcp", fmt.Sprintf("%s:%s", address, port))
	if err != nil {
		return err
	}
	return server.Close()
}

func runForward(v ResourceViewer, pf watch.Forwarder, f *portforward.PortForwarder) {
	v.App().factory.AddForwarder(pf)

	v.App().QueueUpdateDraw(func() {
		v.App().Flash().Infof("PortForward activated %s:%s", pf.Path(), pf.Ports()[0])
		DismissPortForwards(v, v.App().Content.Pages)
	})

	pf.SetActive(true)
	if err := f.ForwardPorts(); err != nil {
		v.App().Flash().Err(err)
		return
	}

	v.App().QueueUpdateDraw(func() {
		v.App().factory.DeleteForwarder(pf.FQN())
		pf.SetActive(false)
	})
}

func startFwdCB(v ResourceViewer, path, co string, t client.PortTunnel) {
	err := tryListenPort(t.Address, t.LocalPort)
	if err != nil {
		v.App().Flash().Err(err)
		return
	}

	if _, ok := v.App().factory.ForwarderFor(dao.PortForwardID(path, co)); ok {
		v.App().Flash().Err(errors.New("A port-forward is already active on this pod"))
		return
	}

	pf := dao.NewPortForwarder(v.App().factory)
	fwd, err := pf.Start(path, co, t)
	if err != nil {
		v.App().Flash().Err(err)
		return
	}

	log.Debug().Msgf(">>> Starting port forward %q %#v", path, t)
	go runForward(v, pf, fwd)
}

func showFwdDialog(v ResourceViewer, path string, cb PortForwardFunc) error {
	mm, err := fetchPodPorts(v.App().factory, path)
	if err != nil {
		return nil
	}
	ports := make([]string, 0, len(mm))
	for co, pp := range mm {
		for _, p := range pp {
			if p.Protocol != v1.ProtocolTCP {
				continue
			}
			ports = append(ports, client.FQN(co, p.Name)+":"+strconv.Itoa(int(p.ContainerPort)))
		}
	}
	if len(ports) == 0 {
		return fmt.Errorf("no tcp ports found on %s", path)
	}
	ShowPortForwards(v, path, ports, cb)

	return nil
}

func fetchPodPorts(f *watch.Factory, path string) (map[string][]v1.ContainerPort, error) {
	log.Debug().Msgf("Fetching ports on pod %q", path)
	o, err := f.Get("v1/pods", path, false, labels.Everything())
	if err != nil {
		return nil, err
	}

	var pod v1.Pod
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(o.(*unstructured.Unstructured).Object, &pod)
	if err != nil {
		return nil, err
	}

	pp := make(map[string][]v1.ContainerPort)
	for _, co := range pod.Spec.Containers {
		pp[co.Name] = co.Ports
	}

	return pp, nil
}
