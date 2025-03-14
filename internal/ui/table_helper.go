package ui

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/derailed/k9s/internal"
	"github.com/derailed/k9s/internal/config"
	"github.com/derailed/k9s/internal/render"
	"github.com/rs/zerolog/log"
	"github.com/sahilm/fuzzy"
)

const (
	// DefaultColorName indicator to keep term colors.
	DefaultColorName = "default"

	// SearchFmt represents a filter view title.
	SearchFmt = "<[filter:bg:r]/%s[fg:bg:-]> "

	// NSTitleFmt represents a namespaced view title.
	NSTitleFmt = "[fg:bg:b] %s([hilite:bg:b]%s[fg:bg:-])[fg:bg:-][[count:bg:b]%d[fg:bg:-]][fg:bg:-] "

	// TitleFmt represents a standard view title.
	TitleFmt = "[fg:bg:b] %s[fg:bg:-][[count:bg:b]%d[fg:bg:-]][fg:bg:-] "

	descIndicator = "↓"
	ascIndicator  = "↑"

	// FullFmat specifies a namespaced dump file name.
	FullFmat = "%s-%s-%d.csv"

	// NoNSFmat specifies a cluster wide dump file name.
	NoNSFmat = "%s-%d.csv"
)

var (
	// LableRx identifies a label query
	LableRx = regexp.MustCompile(`\A\-l`)

	fuzzyRx = regexp.MustCompile(`\A\-f`)
)

func mustExtractStyles(ctx context.Context) *config.Styles {
	styles, ok := ctx.Value(internal.KeyStyles).(*config.Styles)
	if !ok {
		log.Fatal().Msg("Expecting valid styles")
	}
	return styles
}

// TrimCell removes superfluous padding.
func TrimCell(tv *SelectTable, row, col int) string {
	c := tv.GetCell(row, col)
	if c == nil {
		log.Error().Err(fmt.Errorf("No cell at location [%d:%d]", row, col)).Msg("Trim cell failed!")
		return ""
	}
	return strings.TrimSpace(c.Text)
}

// IsLabelSelector checks if query is a label query.
func IsLabelSelector(s string) bool {
	if s == "" {
		return false
	}
	return LableRx.MatchString(s)
}

// IsFuzzySelector checks if query is fuzzy.
func IsFuzzySelector(s string) bool {
	if s == "" {
		return false
	}
	return fuzzyRx.MatchString(s)
}

// TrimLabelSelector extracts label query.
func TrimLabelSelector(s string) string {
	return strings.TrimSpace(s[2:])
}

// SkinTitle decorates a title.
func SkinTitle(fmat string, style config.Frame) string {
	bgColor := style.Title.BgColor
	if bgColor == config.DefaultColor {
		bgColor = config.TransparentColor
	}
	fmat = strings.Replace(fmat, "[fg:bg", "["+style.Title.FgColor.String()+":"+bgColor.String(), -1)
	fmat = strings.Replace(fmat, "[hilite", "["+style.Title.HighlightColor.String(), 1)
	fmat = strings.Replace(fmat, "[key", "["+style.Menu.NumKeyColor.String(), 1)
	fmat = strings.Replace(fmat, "[filter", "["+style.Title.FilterColor.String(), 1)
	fmat = strings.Replace(fmat, "[count", "["+style.Title.CounterColor.String(), 1)
	fmat = strings.Replace(fmat, ":bg:", ":"+bgColor.String()+":", -1)

	return fmat
}

func sortIndicator(sort, asc bool, style config.Table, name string) string {
	if !sort {
		return name
	}

	order := descIndicator
	if asc {
		order = ascIndicator
	}
	return fmt.Sprintf("%s[%s::b]%s[::]", name, style.Header.SorterColor, order)
}

func formatCell(field string, padding int) string {
	if IsASCII(field) {
		return Pad(field, padding)
	}

	return field
}

func filterToast(data render.TableData) render.TableData {
	validX := data.Header.IndexOf("VALID", true)
	if validX == -1 {
		return data
	}

	toast := render.TableData{
		Header:    data.Header,
		RowEvents: make(render.RowEvents, 0, len(data.RowEvents)),
		Namespace: data.Namespace,
	}
	for _, re := range data.RowEvents {
		if re.Row.Fields[validX] != "" {
			toast.RowEvents = append(toast.RowEvents, re)
		}
	}
	return toast
}

func rxFilter(q string, data render.TableData) (render.TableData, error) {
	rx, err := regexp.Compile(`(?i)` + q)
	if err != nil {
		return data, err
	}

	filtered := render.TableData{
		Header:    data.Header,
		RowEvents: make(render.RowEvents, 0, len(data.RowEvents)),
		Namespace: data.Namespace,
	}
	for _, re := range data.RowEvents {
		f := strings.Join(re.Row.Fields, " ")
		if rx.MatchString(f) {
			filtered.RowEvents = append(filtered.RowEvents, re)
		}
	}

	return filtered, nil
}

func fuzzyFilter(q string, index int, data render.TableData) render.TableData {
	var ss []string
	for _, re := range data.RowEvents {
		ss = append(ss, re.Row.Fields[index])
	}

	filtered := render.TableData{
		Header:    data.Header,
		RowEvents: make(render.RowEvents, 0, len(data.RowEvents)),
		Namespace: data.Namespace,
	}
	mm := fuzzy.Find(q, ss)
	for _, m := range mm {
		filtered.RowEvents = append(filtered.RowEvents, data.RowEvents[m.Index])
	}

	return filtered
}
