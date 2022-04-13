package vrl

// ParseJSONRule provides the message data as an object for future modifications/validations.
const ParseJSONRule Rule = `
structured, err = parse_json(.message)
if err == null {
    .parsed_data = structured
}
`
