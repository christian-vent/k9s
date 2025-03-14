package render

import (
	"fmt"
	"strconv"

	"github.com/derailed/k9s/internal/client"
	"github.com/gdamore/tcell"
	"helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Chart renders a helm chart to screen.
type Chart struct{}

// ColorerFunc colors a resource row.
func (Chart) ColorerFunc() ColorerFunc {
	return func(ns string, h Header, re RowEvent) tcell.Color {
		if !Happy(ns, h, re.Row) {
			return ErrColor
		}

		return tcell.ColorMediumSpringGreen
	}
}

// Header returns a header row.
func (Chart) Header(_ string) Header {
	return Header{
		HeaderColumn{Name: "NAMESPACE"},
		HeaderColumn{Name: "NAME"},
		HeaderColumn{Name: "REVISION"},
		HeaderColumn{Name: "STATUS"},
		HeaderColumn{Name: "CHART"},
		HeaderColumn{Name: "APP VERSION"},
		HeaderColumn{Name: "VALID", Wide: true},
		HeaderColumn{Name: "AGE", Time: true, Decorator: AgeDecorator},
	}
}

// Render renders a chart to screen.
func (c Chart) Render(o interface{}, ns string, r *Row) error {
	h, ok := o.(ChartRes)
	if !ok {
		return fmt.Errorf("expected ChartRes, but got %T", o)
	}

	r.ID = client.FQN(h.Release.Namespace, h.Release.Name)
	r.Fields = Fields{
		h.Release.Namespace,
		h.Release.Name,
		strconv.Itoa(h.Release.Version),
		h.Release.Info.Status.String(),
		h.Release.Chart.Metadata.Name + "-" + h.Release.Chart.Metadata.Version,
		h.Release.Chart.Metadata.AppVersion,
		asStatus(c.diagnose(h.Release.Info.Status.String())),
		toAge(metav1.Time{Time: h.Release.Info.LastDeployed.Time}),
	}

	return nil
}

func (c Chart) diagnose(s string) error {
	if s != "deployed" {
		return fmt.Errorf("chart is in an invalid state")
	}

	return nil
}

// ----------------------------------------------------------------------------
// Helpers...

// ChartRes represents an helm chart resource.
type ChartRes struct {
	Release *release.Release
}

// GetObjectKind returns a schema object.
func (ChartRes) GetObjectKind() schema.ObjectKind {
	return nil
}

// DeepCopyObject returns a container copy.
func (h ChartRes) DeepCopyObject() runtime.Object {
	return h
}
