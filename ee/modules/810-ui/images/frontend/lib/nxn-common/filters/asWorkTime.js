var shortFormats = {
  'D': 'days',
  'H': 'hours',
  'M': 'minutes',
  'S': 'seconds'
};

var prefix0 = function(val) {
  var val_str = String(val);
  if (val_str.length < 2) {
    return '0' + val_str;
  } else {
    return val_str;
  }
};

export default function(argument, unit, format) {
  if (!unit) unit = 'seconds';
  if (!format) format = '^H:mm';

  var days;
  var value = argument || 0;
  var duration = moment.duration((value < 0 ? value * (-1) : value), unit);

  var res = value < 0 ? '-' : '';
  // an optimisation for the most common case that is 30-40 times faster than duration formatter (due to regexes, probably)
  switch (format) {
    case '^H:mm':
      res += String(Math.floor(duration.asHours())) + ':' + prefix0(duration.minutes());
      break;
    case '^H:mm:ss':
      res += String(Math.floor(duration.asHours())) + ':' + prefix0(duration.minutes()) + ':' + prefix0(duration.seconds());
      break;

    case 'd_ h_ m_':
      days = Math.floor(duration.asDays());
      if (days > 0) res += String(days) + 'd ';
      if (duration.hours() > 0) res += String(duration.hours()) + 'h ';
      res += prefix0(duration.minutes()) + 'm';
      break;

    case 'd_ h_ m_ s_':
      days = Math.floor(duration.asDays());
      if (days > 0) res += String(days) + 'd ';
      if (duration.hours() > 0) res += String(duration.hours()) + 'h ';
      if (duration.minutes() > 0) res += prefix0(duration.minutes()) + 'm ';
      res += prefix0(duration.seconds()) + 's';
      break;

    // new formats
    case '^D':
    case '^H':
    case '^M':
    case '^S':
      res += String(Math.floor( duration.as(shortFormats[format.slice(1,2)]) ));
      break;
    case '^DD':
    case '^HH':
    case '^MM':
    case '^SS':
      res += prefix0(Math.floor( duration.as(shortFormats[format.slice(1,2)]) ));
      break;

    // override/optimisation over duration formatter
    case 'D':
    case 'H':
    case 'M':
    case 'S':
      res += String( duration.get(shortFormats[format]) );
      break;
    case 'DD':
    case 'HH':
    case 'MM':
    case 'SS':
      res += prefix0( duration.get(shortFormats[format.slice(1,2)]) );
      break;

    // WARNING: can be slow
    default:
      res += duration.format(format, { trim: true });
  }

  return res;
}
