// fontawesome icons
import { library, dom, config } from '@fortawesome/fontawesome-svg-core'

import {
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
  faUserClock,
  faHistory
} from '@fortawesome/free-solid-svg-icons'

library.add(faClock,
  faCalendarDay,
  faCalendarWeek,
  faCalendarAlt,
  faCalendarPlus,
  faCalendar,
  faSquare,
  faCheckSquare,
  faCaretRight,
  faCaretDown,
  faUserClock,
  faHistory);

config.autoReplaceSvg="nest";
dom.watch();
