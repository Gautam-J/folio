// internal/widget/widget.go
package widget

import (
	"context"
	"html/template"
)

// Widget renders one self-contained dashboard panel: it owns both its data
// fetch and its own template. A caller composing multiple widgets must
// treat a Render error as "this panel is empty," never as a reason to
// fail the whole dashboard.
type Widget interface {
	Render(ctx context.Context) (template.HTML, error)
}
