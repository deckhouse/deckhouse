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

const (
	ansiEnterAlt = "\x1b[?1049h" // switch to the alternate screen buffer
	ansiLeaveAlt = "\x1b[?1049l" // restore the original screen
	ansiHideCur  = "\x1b[?25l"   // hide cursor
	ansiShowCur  = "\x1b[?25h"   // show cursor
	ansiHome     = "\x1b[H"      // move cursor to row 1, col 1
	ansiClearEOL = "\x1b[K"      // clear from cursor to end of line
	ansiClearEOS = "\x1b[J"      // clear from cursor to end of screen
)
