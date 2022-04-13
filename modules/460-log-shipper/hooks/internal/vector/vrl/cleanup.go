package vrl

// CleanUpRule is a general cleanup rule to sanitize the final message.
// It should always be the first rule in the transforms chain.
const CleanUpRule Rule = `
if exists(.pod_labels."controller-revision-hash") {
    del(.pod_labels."controller-revision-hash")
}
if exists(.pod_labels."pod-template-hash") {
    del(.pod_labels."pod-template-hash")
}
if exists(.kubernetes) {
    del(.kubernetes)
}
if exists(.file) {
    del(.file)
}
`
