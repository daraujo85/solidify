// Render helpers (separado p/ evitar import cycle).
package prof

import "fmt"

// RenderDiff textual p/ logs.
func RenderDiff(d DiffResult) string {
	return fmt.Sprintf("alloc=%d obj=%d gc=%d elapsed=%s",
		d.AllocDelta, d.ObjectsDelta, d.GCDelta, d.Elapsed)
}
