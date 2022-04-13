package vrl

// ParsedDataCleanUpRule cleans up the temporary parsed data object.
const ParsedDataCleanUpRule Rule = `
if exists(.parsed_data) {
    del(.parsed_data)
}
`

// StreamRule puts the vector timestamp to the label recognized by elasticsearch.
const StreamRule Rule = `
."@timestamp" = del(.timestamp)
`
