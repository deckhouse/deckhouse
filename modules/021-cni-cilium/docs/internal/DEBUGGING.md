# Debugging

## General Health

`cilium status --verbose` is the main tool for diagnosing if the cilium agent works correctly. Pay special attention to the output after Controller Status. If there are errors, it means that one of the internal controllers of the agent does not work properly. You can find details in the log files at `| grep -P 'level=(warning|error)`.

## Examine issues

Before going any deeper, look for possible [issues](https://github.com/deckhouse/deckhouse/issues?q=is%3Aissue+is%3Aopen+cilium) in the deckhouse project. If you can't find an issue, try looking for symptoms in the [upstream](https://github.com/cilium/cilium/issues) repository.

## Packets are not properly forwarded

1. `exec` into cilium-agent Pod.
2. Switch the debug on: `cilium config Debug=enable DebugLB=enable`.
3. Collect the output of `exec cilium monitor -vv -D &> cilium.log`.
4. Generate packets. Consider specifying an unusual source port to make it easier to identify the test traffic in `cilium.log`.
5. Watch the log and make conclusions.

## Accidentally applied a bad policy and everything stopped working

1. Turn on audit mode.
2. If deckhouse is dead, get into the container with cilium-agent (possibly, on every node) and enable audit mode: `cilium config PolicyAuditMode=enable`.
3. Remember to turn it off after you fix the policies.
