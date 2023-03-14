export default function nxnWorkTimeToMinutes(arg) {
  var hours;
  var minutes;
  var match;
  var is_negative = false;

  if (arg) {
    if (arg.charAt(0) == '-') {
      arg = arg.slice(1);
      is_negative = true;
    }

    if (match = arg.match(/^[0-5]?[0-9]$/)) { // only minutes
      hours = 0;
      minutes = parseInt(match[0]);
    } else if (match = arg.match(/^([0-9]{0,3}):([0-5]?[0-9])$/)) { // with delimiter
      hours = parseInt(match[1]);
      minutes = parseInt(match[2]);
    } else if (match = arg.match(/^([0-9]{0,3})([0-5][0-9])$/)) { // without delimiter
      hours = parseInt(match[1]);
      minutes = parseInt(match[2]);
    }
  }

  var time;
  if (match) {
    time = hours * 60 + minutes;
    if (is_negative) {
      time = -time;
    }
  } else {
    time = undefined;
  }

  return time || 0;
}
