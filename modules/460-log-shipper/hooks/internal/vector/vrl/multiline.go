package vrl

// GeneralMultilineRule appends all lines started with a space/tab to the previous line.
//
// Example:
// ---
// start of the line:
//  following line
//  one more line
//
const GeneralMultilineRule Rule = `
if exists(.message) {
    if length!(.message) > 0 {
        matched, err = match(.message, r'^[^\s\t]');
        if err != null {
            false;
        } else {
            matched;
        };
    } else {
        false;
    };
} else {
  false;
}
`

// LogWithTimeMultilineRule counts any date/timestamp as a start of the line. All following lines will be appended.
//
// Example:
// ---
// 2022-10-10 11:10 start of the line
// following line
// one more line
// 2022-10-10 11:11 a new line
//
const LogWithTimeMultilineRule Rule = `
matched, err = match(.message, r'^\[?((((19|20)([2468][048]|[13579][26]|0[48])|2000)-02-29|((19|20)[0-9]{2}-(0[4678]|1[02])-(0[1-9]|[12][0-9]|30)|(19|20)[0-9]{2}-(0[1359]|11)-(0[1-9]|[12][0-9]|3[01])|(19|20)[0-9]{2}-02-(0[1-9]|1[0-9]|2[0-8])))\s([01][0-9]|2[0-3]):([012345][0-9]):([012345][0-9])|20\d\d-(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)-(0[1-9]|[1-2][0-9]|3[01])\s([01][0-9]|2[0-3]):([012345][0-9]):([012345][0-9])|(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+([01][0-9]|2[0-3]):([012345][0-9]):([012345][0-9])|(?:(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2}(?:\.\d+)?))(Z|[\+-]\d{2}:\d{2})?|\p{L}{2}\s\d{1,2}\s\p{L}{3}\s\d{4}\s([01][0-9]|2[0-3]):([012345][0-9]):([012345][0-9]))');
if err != null {
    false;
} else {
    matched;
}
`

// JSONMultilineRule parses multiline JSON formatted documents.
//
// Example:
// ---
// {
//   "Start": "first_line",
//   "Next": "following line"
// }
//
const JSONMultilineRule Rule = `
matched, err = match(.message, r'^\{');
if err != null {
    false;
} else {
    matched;
}
`

// BackslashMultilineRule counts all lines ended with the backslash symbol as the parts of a single line.
//
// Example:
// ---
// first line \
// one more line \
// the end
//
const BackslashMultilineRule Rule = `
matched, err = match(.message, r'[^\\]$');
if err != null {
    false;
} else {
    matched;
}
`
