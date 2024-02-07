// fontawesome icons
import { library, dom, config } from "@fortawesome/fontawesome-svg-core"

// Kind of Tree shaking
// https://github.com/FortAwesome/Font-Awesome/issues/13260
import { faClock } from "@fortawesome/free-solid-svg-icons/faClock"
import { faCalendarDay } from "@fortawesome/free-solid-svg-icons/faCalendarDay"
import { faCalendarWeek } from "@fortawesome/free-solid-svg-icons/faCalendarWeek"
import { faCalendarAlt } from "@fortawesome/free-solid-svg-icons/faCalendarAlt"
import { faCalendarPlus } from "@fortawesome/free-solid-svg-icons/faCalendarPlus"
import { faCalendar } from "@fortawesome/free-solid-svg-icons/faCalendar"
import { faSquare } from "@fortawesome/free-solid-svg-icons/faSquare"
import { faCheckSquare } from "@fortawesome/free-solid-svg-icons/faCheckSquare"
import { faCaretRight } from "@fortawesome/free-solid-svg-icons/faCaretRight"
import { faCaretDown } from "@fortawesome/free-solid-svg-icons/faCaretDown"
import { faCaretUp } from "@fortawesome/free-solid-svg-icons/faCaretUp"
import { faUserClock } from "@fortawesome/free-solid-svg-icons/faUserClock"
import { faHistory } from "@fortawesome/free-solid-svg-icons/faHistory"
import { faInfo } from "@fortawesome/free-solid-svg-icons/faInfo"
import { faInfoCircle } from "@fortawesome/free-solid-svg-icons/faInfoCircle"
import { faFilter } from "@fortawesome/free-solid-svg-icons/faFilter"
import { faSpinner } from "@fortawesome/free-solid-svg-icons/faSpinner"
import { faStepBackward } from "@fortawesome/free-solid-svg-icons/faStepBackward"
import { faStepForward } from "@fortawesome/free-solid-svg-icons/faStepForward"
import { faSearchMinus } from "@fortawesome/free-solid-svg-icons/faSearchMinus"

library.add(
  faClock,
  faCalendarDay,
  faCalendarWeek,
  faCalendarAlt,
  faCalendarPlus,
  faCalendar,
  faSquare,
  faCheckSquare,
  faCaretRight,
  faCaretDown,
  faCaretUp,
  faUserClock,
  faHistory,
  faInfo,
  faInfoCircle,
  faFilter,
  faSpinner,
  faStepBackward,
  faStepForward,
  faSearchMinus,
)

config.autoReplaceSvg = "nest"
dom.watch()
