// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package termui

// layout describes which regions of the live block to render. The logbox is the flex region: it
// fills all remaining rows down to the terminal bottom. Milestones and warns are capped recent
// windows pinned above it.
type layout struct {
	banner     bool
	action     bool
	region     bool // milestones + warns + logbox region present
	milestones int  // number of most-recent milestones to show
	logbox     int  // logbox height in rows (0 = not shown)
}

// caps holds the fixed region limits.
type caps struct {
	warn      int // max warns shown
	logboxMin int // minimum logbox height required before the banner may be shown
}

// logboxRingCap bounds how many recent log lines the block retains for the (height-filling) logbox.
// It comfortably exceeds the tallest terminal's logbox so a tall terminal can fill.
const logboxRingCap = 256

const reserveLines = 1 // keep the bottom row free for the cursor/prompt

// computeLayout fits the live block into `height` rows. The bar and action line come first; then
// MILESTONES grow first — all of them, space permitting — and only the rows left over go to the
// logbox, which fills them down to the terminal bottom. So extra height feeds milestones until the
// full history is visible, then feeds the logbox. The banner is shown only when it hides nothing
// (all milestones fit and a minimum logbox remains). connLine is 0 or 1: 1 reserves a row for the
// pinned connection string above the logbox.
func computeLayout(height, totalMile, totalWarn, bannerH, connLine int, c caps) layout {
	w := min(totalWarn, c.warn)
	avail := height - reserveLines
	if avail < 2 {
		return layout{} // only the bar fits (or nothing)
	}
	room := avail - 2 // rows for banner + milestones + warns + connLine + logbox (bar+action already counted)

	// split allocates the milestones/logbox rows for a given banner height: milestones take priority
	// (grow to their full count), the logbox fills whatever remains. ok is false when the region does
	// not fit at all.
	split := func(bann int) (int, int, bool) {
		body := room - bann - w - connLine
		if body < 1 {
			return 0, 0, false
		}
		mile := min(totalMile, body) // milestones grow first
		logbox := body - mile        // logbox fills the leftover (>= 0)
		return mile, logbox, true
	}

	// Banner shown only when it hides nothing: all milestones still fit and a minimum logbox remains.
	if bannerH > 0 {
		if mile, logbox, ok := split(bannerH); ok && mile == totalMile && logbox >= c.logboxMin {
			return layout{banner: true, action: true, region: true, milestones: mile, logbox: logbox}
		}
	}
	if mile, logbox, ok := split(0); ok {
		return layout{action: true, region: true, milestones: mile, logbox: logbox}
	}
	// Only bar + action.
	return layout{action: true}
}
