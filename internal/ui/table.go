package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/derailed/k9s/internal"
	"github.com/derailed/k9s/internal/client"
	"github.com/derailed/k9s/internal/config"
	"github.com/derailed/k9s/internal/model"
	"github.com/derailed/k9s/internal/render"
	"github.com/derailed/tview"
	"github.com/gdamore/tcell"
	"github.com/rs/zerolog/log"
)

type (
	// ColorerFunc represents a row colorer.
	ColorerFunc func(ns string, evt render.RowEvent) tcell.Color

	// DecorateFunc represents a row decorator.
	DecorateFunc func(render.TableData) render.TableData

	// SelectedRowFunc a table selection callback.
	SelectedRowFunc func(r int)
)

// Table represents tabular data.
type Table struct {
	*SelectTable

	actions     KeyActions
	gvr         client.GVR
	Path        string
	cmdBuff     *CmdBuff
	styles      *config.Styles
	viewSetting *config.ViewSetting
	sortCol     SortColumn
	colorerFn   render.ColorerFunc
	decorateFn  DecorateFunc
	wide        bool
	toast       bool
	header      render.Header
	hasMetrics  bool
}

// NewTable returns a new table view.
func NewTable(gvr client.GVR) *Table {
	return &Table{
		SelectTable: &SelectTable{
			Table: tview.NewTable(),
			model: model.NewTable(gvr),
			marks: make(map[string]struct{}),
		},
		gvr:     gvr,
		actions: make(KeyActions),
		cmdBuff: NewCmdBuff('/', FilterBuff),
		sortCol: SortColumn{asc: true},
	}
}

// Init initializes the component.
func (t *Table) Init(ctx context.Context) {
	t.SetFixed(1, 0)
	t.SetBorder(true)
	t.SetBorderAttributes(tcell.AttrBold)
	t.SetBorderPadding(0, 0, 1, 1)
	t.SetSelectable(true, false)
	t.SetSelectionChangedFunc(t.selectionChanged)
	t.SetBackgroundColor(tcell.ColorDefault)
	t.Select(1, 0)
	t.hasMetrics = false
	if mx, ok := ctx.Value(internal.KeyHasMetrics).(bool); ok {
		t.hasMetrics = mx
	}

	if cfg, ok := ctx.Value(internal.KeyViewConfig).(*config.CustomView); ok && cfg != nil {
		cfg.AddListener(t.GVR().String(), t)
	}
	t.styles = mustExtractStyles(ctx)
	t.StylesChanged(t.styles)
}

// GVR returns a resource descriptor.
func (t *Table) GVR() client.GVR { return t.gvr }

// ViewSettingsChanged notifies listener the view configuration changed.
func (t *Table) ViewSettingsChanged(settings config.ViewSetting) {
	t.viewSetting = &settings
	t.Refresh()
}

// StylesChanged notifies the skin changed.
func (t *Table) StylesChanged(s *config.Styles) {
	t.SetBackgroundColor(s.Table().BgColor.Color())
	t.SetBorderColor(s.Table().FgColor.Color())
	t.SetBorderFocusColor(s.Frame().Border.FocusColor.Color())
	t.SetSelectedStyle(
		tcell.ColorBlack,
		t.styles.Table().CursorColor.Color(),
		tcell.AttrBold,
	)
	t.Refresh()
}

// ResetToast resets toast flag.
func (t *Table) ResetToast() {
	t.toast = false
	t.Refresh()
}

// ToggleToast toggles to show toast resources.
func (t *Table) ToggleToast() {
	t.toast = !t.toast
	t.Refresh()
}

// ToggleWide toggles wide col display.
func (t *Table) ToggleWide() {
	t.wide = !t.wide
	t.Refresh()
}

// Actions returns active menu bindings.
func (t *Table) Actions() KeyActions {
	return t.actions
}

// Styles returns styling configurator.
func (t *Table) Styles() *config.Styles {
	return t.styles
}

// FilterInput filters user commands.
func (t *Table) FilterInput(r rune) bool {
	if !t.cmdBuff.IsActive() {
		return false
	}
	t.cmdBuff.Add(r)
	t.ClearSelection()
	t.doUpdate(t.filtered(t.GetModel().Peek()))
	t.UpdateTitle()
	t.SelectFirstRow()

	return true
}

// Hints returns the view hints.
func (t *Table) Hints() model.MenuHints {
	return t.actions.Hints()
}

// ExtraHints returns additional hints.
func (t *Table) ExtraHints() map[string]string {
	return nil
}

// GetFilteredData fetch filtered tabular data.
func (t *Table) GetFilteredData() render.TableData {
	return t.filtered(t.GetModel().Peek())
}

// SetDecorateFn specifies the default row decorator.
func (t *Table) SetDecorateFn(f DecorateFunc) {
	t.decorateFn = f
}

// SetColorerFn specifies the default colorer.
func (t *Table) SetColorerFn(f render.ColorerFunc) {
	t.colorerFn = f
}

// SetSortCol sets in sort column index and order.
func (t *Table) SetSortCol(name string, asc bool) {
	t.sortCol.name, t.sortCol.asc = name, asc
}

// Update table content.
func (t *Table) Update(data render.TableData) {
	t.header = data.Header
	if t.decorateFn != nil {
		data = t.decorateFn(data)
	}
	t.doUpdate(t.filtered(data))
	t.UpdateTitle()
}

func (t *Table) doUpdate(data render.TableData) {
	if client.IsAllNamespaces(data.Namespace) {
		t.actions[KeyShiftP] = NewKeyAction("Sort Namespace", t.SortColCmd("NAMESPACE", true), false)
	} else {
		t.actions.Delete(KeyShiftP)
	}

	var cols []string
	if t.viewSetting != nil {
		cols = t.viewSetting.Columns
	}
	if len(cols) == 0 {
		cols = t.header.Columns(t.wide)
	}
	custData := data.Customize(cols, t.wide)

	if (t.sortCol.name == "" || custData.Header.IndexOf(t.sortCol.name, false) == -1) && len(custData.Header) > 0 {
		t.sortCol.name = custData.Header[0].Name
	}

	t.Clear()
	fg := t.styles.Table().Header.FgColor.Color()
	bg := t.styles.Table().Header.BgColor.Color()

	var col int
	for _, h := range custData.Header {
		if h.Name == "NAMESPACE" && !t.GetModel().ClusterWide() {
			continue
		}
		if h.MX && !t.hasMetrics {
			continue
		}
		t.AddHeaderCell(col, h)
		c := t.GetCell(0, col)
		c.SetBackgroundColor(bg)
		c.SetTextColor(fg)
		col++
	}
	custData.RowEvents.Sort(custData.Namespace, custData.Header.IndexOf(t.sortCol.name, false), t.sortCol.name == "AGE", t.sortCol.asc)

	pads := make(MaxyPad, len(custData.Header))
	ComputeMaxColumns(pads, t.sortCol.name, custData.Header, custData.RowEvents)
	for row, re := range custData.RowEvents {
		idx, _ := data.RowEvents.FindIndex(re.Row.ID)
		t.buildRow(row+1, re, data.RowEvents[idx], custData.Header, pads)
	}
	t.updateSelection(true)
}

func (t *Table) buildRow(r int, re, ore render.RowEvent, h render.Header, pads MaxyPad) {
	color := render.DefaultColorer
	if t.colorerFn != nil {
		color = t.colorerFn
	}

	marked := t.IsMarked(re.Row.ID)
	var col int
	for c, field := range re.Row.Fields {
		if c >= len(h) {
			log.Error().Msgf("field/header overflow detected for %q -- %d::%d. Check your mappings!", t.GVR(), c, len(h))
			continue
		}

		if h[c].Name == "NAMESPACE" && !t.GetModel().ClusterWide() {
			continue
		}
		if h[c].MX && !t.hasMetrics {
			continue
		}

		if !re.Deltas.IsBlank() && !h.IsAgeCol(c) {
			field += Deltas(re.Deltas[c], field)
		}

		if h[c].Decorator != nil {
			field = h[c].Decorator(field)
		}
		if h[c].Align == tview.AlignLeft {
			field = formatCell(field, pads[c])
		}

		cell := tview.NewTableCell(field)
		cell.SetExpansion(1)
		cell.SetAlign(h[c].Align)
		fgColor := color(t.GetModel().GetNamespace(), t.header, ore)
		cell.SetTextColor(fgColor)
		if marked && fgColor != render.ErrColor {
			cell.SetTextColor(t.styles.Table().MarkColor.Color())
		}
		if col == 0 {
			cell.SetReference(re.Row.ID)
		}
		t.SetCell(r, col, cell)
		col++
	}
}

// SortColCmd designates a sorted column.
func (t *Table) SortColCmd(name string, asc bool) func(evt *tcell.EventKey) *tcell.EventKey {
	return func(evt *tcell.EventKey) *tcell.EventKey {
		t.sortCol.asc = !t.sortCol.asc
		if t.sortCol.name != name {
			t.sortCol.asc = asc
		}
		t.sortCol.name = name
		t.Refresh()
		return nil
	}
}

// SortInvertCmd reverses sorting order.
func (t *Table) SortInvertCmd(evt *tcell.EventKey) *tcell.EventKey {
	t.sortCol.asc = !t.sortCol.asc
	t.Refresh()

	return nil
}

// ClearMarks clear out marked items.
func (t *Table) ClearMarks() {
	t.SelectTable.ClearMarks()
	t.Refresh()
}

// Refresh update the table data.
func (t *Table) Refresh() {
	data := t.model.Peek()
	if len(data.Header) == 0 {
		return
	}
	// BOZO!! Really want to tell model reload now. Refactor!
	t.Update(data)
}

// GetSelectedRow returns the entire selected row.
func (t *Table) GetSelectedRow() render.Row {
	return t.model.Peek().RowEvents[t.GetSelectedRowIndex()-1].Row
}

// NameColIndex returns the index of the resource name column.
func (t *Table) NameColIndex() int {
	col := 0
	if client.IsClusterScoped(t.GetModel().GetNamespace()) {
		return col
	}
	if t.GetModel().ClusterWide() {
		col++
	}
	return col
}

// AddHeaderCell configures a table cell header.
func (t *Table) AddHeaderCell(col int, h render.HeaderColumn) {
	sortCol := h.Name == t.sortCol.name
	c := tview.NewTableCell(sortIndicator(sortCol, t.sortCol.asc, t.styles.Table(), h.Name))
	c.SetExpansion(1)
	c.SetAlign(h.Align)
	t.SetCell(0, col, c)
}

func (t *Table) filtered(data render.TableData) render.TableData {
	filtered := data
	if t.toast {
		filtered = filterToast(data)
	}
	if t.cmdBuff.Empty() || IsLabelSelector(t.cmdBuff.String()) {
		return filtered
	}

	q := t.cmdBuff.String()
	if IsFuzzySelector(q) {
		return fuzzyFilter(q[2:], t.NameColIndex(), filtered)
	}

	filtered, err := rxFilter(t.cmdBuff.String(), filtered)
	if err != nil {
		log.Error().Err(errors.New("Invalid filter expression")).Msg("Regexp")
		t.cmdBuff.Clear()
	}

	return filtered
}

// SearchBuff returns the associated command buffer.
func (t *Table) SearchBuff() *CmdBuff {
	return t.cmdBuff
}

// ShowDeleted marks row as deleted.
func (t *Table) ShowDeleted() {
	r, _ := t.GetSelection()
	cols := t.GetColumnCount()
	for x := 0; x < cols; x++ {
		t.GetCell(r, x).SetAttributes(tcell.AttrDim)
	}
}

// UpdateTitle refreshes the table title.
func (t *Table) UpdateTitle() {
	t.SetTitle(t.styleTitle())
}

func (t *Table) styleTitle() string {
	rc := t.GetRowCount()
	if rc > 0 {
		rc--
	}

	base := strings.Title(t.gvr.R())
	ns := t.GetModel().GetNamespace()
	if client.IsClusterWide(ns) || ns == client.NotNamespaced {
		ns = client.NamespaceAll
	}
	path := t.Path
	if path != "" {
		cns, n := client.Namespaced(path)
		if cns == client.ClusterScope {
			ns = n
		} else {
			ns = path
		}
	}

	var title string
	if ns == client.ClusterScope {
		title = SkinTitle(fmt.Sprintf(TitleFmt, base, rc), t.styles.Frame())
	} else {
		title = SkinTitle(fmt.Sprintf(NSTitleFmt, base, ns, rc), t.styles.Frame())
	}

	buff := t.cmdBuff.String()
	if buff == "" {
		return title
	}
	if IsLabelSelector(buff) {
		buff = TrimLabelSelector(buff)
	}

	return title + SkinTitle(fmt.Sprintf(SearchFmt, buff), t.styles.Frame())
}
