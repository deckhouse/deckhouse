#!/usr/bin/python3

# Copyright 2025 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


# This hook checks the MultiTenancy flag for the user-authz module.
#
# The effective value of enableMultiTenancy (the ModuleConfig user setting
# merged with config-schema defaults) is not something an admission webhook
# can see directly — webhooks only see real Kubernetes objects, not module
# Values. This is necessary because in some editions (e.g. CSE)
# enableMultiTenancy defaults to true, and when the user has not set it
# explicitly, the field is simply absent from ModuleConfig.spec.settings.
#
# templates/namespace.yaml bridges this gap: it renders a small
# "d8-user-authz-multitenancy-state" ConfigMap behind a
# `.Values.userAuthz.enableMultiTenancy` gate — the same defaults-merged
# value. This hook reads that ConfigMap instead of ModuleConfig or the
# Module CR.
#
# NB: the ConfigMap is what carries the answer, not the namespace around it.
# d8-user-authz itself is unconditional — user-authz-controller lives there
# whether or not MultiTenancy is on — so its existence proves nothing. Only
# the ConfigMap is gated, and it is simply absent when enableMultiTenancy is
# false, which is exactly the value we want to assume in that case anyway.
# is_multitenancy_enabled() below treats "no snapshot" the same as
# "enableMultiTenancy: false", so this falls out for free, with no separate
# hook or bootstrap-ordering window to worry about.
#
# (Do not go back to reading status.lastAppliedConfiguration from a Module CR
# here — deckhouse.io/v1alpha2 Module is not a real, served API version for
# built-in modules like this one; it belongs to the unrelated, in-progress
# packages/Application controller. Doing so silently makes this check always
# treat MultiTenancy as disabled.)
#
# - If the flag is enabled — we just exit.
#
# - If the flag is disabled — we check the ClusterAuthorizationRule (CAR) resource being created or updated
#   for the presence of the following fields:
#   - allowAccessToSystemNamespaces
#   - limitNamespaces
#   - namespaceSelector
#
#   If any of those fields are present — creation is denied due to MultiTenancy restrictions.
#
# - Additionally, if the user attempts to disable the `enableMultiTenancy` flag in the user-authz ModuleConfig,
#   the hook validates all existing ClusterAuthorizationRule resources for the presence of the same fields.
#   If any CAR uses those fields — disabling MultiTenancy is denied.
#   If the request doesn't touch enableMultiTenancy at all (the field is absent from the submitted
#   spec.settings, e.g. an unrelated ModuleConfig edit), we fall back to the ConfigMap-mirrored
#   effective value instead of treating it as "disabled".


from deckhouse import hook
from dotmap import DotMap

SEPARATOR = "; "
MULTITENANCY_STATE_SNAPSHOT_NAME = "d8-user-authz-multitenancy-state"
CLUSTER_AUTH_RULES_SNAPSHOT_NAME = "d8-user-authz-cars"
CONFIG = f"""
configVersion: v1
kubernetesValidating:
- name: d8-user-authz-car-multitenancy-related-options.deckhouse.io
  includeSnapshotsFrom: ["{MULTITENANCY_STATE_SNAPSHOT_NAME}"]
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["clusterauthorizationrules"]
    scope:       "Cluster"
- name: d8-user-authz-module-multitenancy-related-options.deckhouse.io
  includeSnapshotsFrom: ["{CLUSTER_AUTH_RULES_SNAPSHOT_NAME}", "{MULTITENANCY_STATE_SNAPSHOT_NAME}"]
  matchConditions:
  - name: "only-user-authz-module"
    expression: 'request.name == "user-authz"'
  rules:
  - apiGroups: ["deckhouse.io"]
    apiVersions: ["*"]
    resources: ["moduleconfigs"]
    operations: ["CREATE", "UPDATE"]
    scope:       "Cluster"

kubernetes:
- name: {MULTITENANCY_STATE_SNAPSHOT_NAME}
  apiVersion: v1
  kind: ConfigMap
  executeHookOnEvent: []
  executeHookOnSynchronization: true
  keepFullObjectsInMemory: false
  jqFilter: |
    {{
      "enableMultiTenancy": (.data.enableMultiTenancy == "true")
    }}
  namespace:
    nameSelector:
      matchNames:
      - d8-user-authz
  nameSelector:
    matchNames:
    - d8-user-authz-multitenancy-state
- name: {CLUSTER_AUTH_RULES_SNAPSHOT_NAME}
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterAuthorizationRule
  keepFullObjectsInMemory: true
  executeHookOnEvent: []
  executeHookOnSynchronization: false
"""


def main(ctx: hook.Context):
    try:
        binding_context = DotMap(ctx.binding_context)
        error_messages, warnings = validate(binding_context)
        if error_messages:
            ctx.output.validations.deny(SEPARATOR.join(error_messages))
        else:
            ctx.output.validations.allow(*warnings)
    except Exception as e:
        ctx.output.validations.error(str(e))


def is_multitenancy_enabled(ctx: DotMap) -> bool:
    snapshot = ctx.snapshots[MULTITENANCY_STATE_SNAPSHOT_NAME]
    return len(snapshot) != 0 and snapshot[0].filterResult.enableMultiTenancy is True


def validate(ctx: DotMap) -> tuple[list[str], list[str]]:
    req = ctx.review.request
    kind = req.kind.kind.lower()

    if kind == "clusterauthorizationrule":
        # A limitNamespaces pattern that does not compile is checked either way. It is a property of
        # the rule, not of the module setting, and it is worth rejecting at admission: the webhook
        # quarantines such a rule at runtime, which means its subjects silently get LESS access than
        # the author wrote, reported only by a metric nobody is watching when they hit Apply.
        errors, warnings = validate_car_limit_namespaces_patterns(req.object)

        # don't check the multi-tenancy fields if user-authz MultiTenancy option is enabled
        if is_multitenancy_enabled(ctx):
            return errors, warnings

        field_errors, field_warnings = validate_car_multitenancy_related_fields(req.object)
        return errors + field_errors, warnings + field_warnings
    elif kind == "moduleconfig":
        settings = req.object.spec.settings
        field_present = "enableMultiTenancy" in settings

        # don't check existing CARs if the request explicitly enables MultiTenancy...
        if field_present and settings.enableMultiTenancy is True:
            return [], []

        # ...or if the field was already absent before this request too (compare against
        # oldObject) — i.e. this is genuinely an unrelated ModuleConfig edit that doesn't
        # touch enableMultiTenancy, so the effective value isn't changing and it's safe to
        # trust the ConfigMap's currently-mirrored value (explicit, or via the edition's
        # config-schema default).
        #
        # If the field WAS present before and is being removed by this request, that
        # removal is itself a potential disable (e.g. on editions where the schema default
        # is false) — it must fall through to validating the CARs below, not take this
        # shortcut based on the pre-request ConfigMap state.
        old_settings = req.oldObject.spec.settings
        was_present = "enableMultiTenancy" in old_settings
        if not field_present and not was_present and is_multitenancy_enabled(ctx):
            return [], []

        errors, warnings = [], []
        for cluster_authorization_rule in ctx.snapshots.get(CLUSTER_AUTH_RULES_SNAPSHOT_NAME, []):
            error_messages, warning_messages = validate_car_multitenancy_related_fields(cluster_authorization_rule.object)
            if warning_messages:
              warnings.append(SEPARATOR.join(warning_messages))
            if error_messages:
              errors.append(SEPARATOR.join(error_messages))

        return errors, []

    return [], []


MULTITENANCY_RESTRICTED_FIELDS = {
    'allowAccessToSystemNamespaces': "allowAccessToSystemNamespaces flag",
    'namespaceSelector': "namespaceSelector option",
    'limitNamespaces': "limitNamespaces option"
}

# The authorization webhook compiles limitNamespaces with Go's regexp, which is RE2. This admission
# check exists so that a pattern RE2 cannot compile is refused when it is written rather than
# quarantined at runtime, where its only trace is a metric nobody is watching at the moment somebody
# presses apply.
#
# It deliberately does NOT compile the pattern with Python's re to decide. The two dialects differ in
# both directions, and only one of those directions is safe to be wrong about:
#
#   - Python accepts things RE2 refuses: lookaround, backreferences, atomic groups, conditionals,
#     possessive quantifiers, inline comments, \Z, several inline flags, repeat counts above 1000.
#     Missing one of these lets a rule through that will be quarantined - the status quo, tolerable.
#   - Python REFUSES things RE2 accepts: \z, \pL, \Q...\E, \x{...}, (?<name>...), (?U). Rejecting
#     one of these blocks an administrator from editing a rule that works. That is a regression, and
#     it is the direction that must not happen.
#
# So the check is made of two parts that are true of RE2 specifically: a structural balance scan,
# and a list of constructs RE2 does not have. Anything it is unsure about is allowed through, where
# the runtime quarantine and its alert remain the backstop.

# Constructs Python accepts and RE2 does not. Ordered so the longer prefixes match first.
RE2_UNSUPPORTED = (
    ("(?<=", "lookbehind"),
    ("(?<!", "negative lookbehind"),
    ("(?=", "lookahead"),
    ("(?!", "negative lookahead"),
    ("(?>", "atomic group"),
    ("(?#", "inline comment"),
    ("(?P=", "backreference"),
    ("(?(", "conditional"),
)

# Inline flags RE2 knows. Anything else in a (?letters) group is a Python-only flag.
RE2_INLINE_FLAGS = set("imsU-:")

# RE2 refuses a repetition count above this.
RE2_MAX_REPEAT = 1000


def _spans(pattern: str):
    """Yields (index, char, in_class, in_quote) with escapes resolved, so a scan can trust what it sees."""
    i = 0
    in_class = False
    in_quote = False
    while i < len(pattern):
        ch = pattern[i]
        if in_quote:
            if ch == "\\" and i + 1 < len(pattern) and pattern[i + 1] == "E":
                in_quote = False
                i += 2
                continue
            i += 1
            continue
        if ch == "\\":
            if i + 1 < len(pattern) and pattern[i + 1] == "Q":
                in_quote = True
                i += 2
                continue
            i += 2  # the escaped character is a literal, whatever it is
            continue
        if ch == "[" and not in_class:
            in_class = True
        elif ch == "]" and in_class:
            in_class = False
        yield i, ch, in_class, in_quote
        i += 1


def structural_error(pattern: str) -> str:
    """Reports an imbalance RE2 would refuse. Only certainties - never a guess."""
    depth = 0
    in_class_at = None
    for i, ch, in_class, _ in _spans(pattern):
        if ch == "[" and in_class and in_class_at is None:
            in_class_at = i
            continue
        if ch == "]" and not in_class:
            in_class_at = None
            continue
        if in_class:
            continue
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
            if depth < 0:
                return "an unbalanced ')'"
    if depth > 0:
        return "an unclosed '('"
    if in_class_at is not None:
        return "an unclosed '['"
    if pattern.endswith("\\") and not pattern.endswith("\\\\"):
        return "a trailing backslash"
    return ""


def re2_unsupported_construct(pattern: str) -> str:
    """Reports a construct RE2 does not have. Python accepts all of these, so compiling would not catch them."""
    for i, ch, in_class, in_quote in _spans(pattern):
        if in_class or in_quote:
            continue
        if ch == "\\":
            continue
        rest = pattern[i:]
        for token, description in RE2_UNSUPPORTED:
            if rest.startswith(token):
                return description
        # An inline flag group: (?letters) or (?letters:...). RE2 knows i, m, s and U.
        if rest.startswith("(?"):
            j = i + 2
            flags = ""
            while j < len(pattern) and pattern[j] not in ")::":
                flags += pattern[j]
                j += 1
            if flags and all(c.isalpha() or c == "-" for c in flags):
                unknown = [c for c in flags if c not in RE2_INLINE_FLAGS]
                if unknown:
                    return "the inline flag '%s', which RE2 does not have" % unknown[0]
        # A repetition RE2 refuses for being too large.
        if ch == "{":
            close = pattern.find("}", i)
            if close != -1:
                body = pattern[i + 1:close]
                for part in body.split(","):
                    part = part.strip()
                    if part.isdigit() and int(part) > RE2_MAX_REPEAT:
                        return "a repetition of %s, above RE2's limit of %d" % (part, RE2_MAX_REPEAT)

    # A backreference written as \1 .. \9. Scanned separately because _spans hides escapes.
    i = 0
    while i < len(pattern) - 1:
        if pattern[i] == "\\":
            if pattern[i + 1].isdigit() and pattern[i + 1] != "0":
                return "a backreference"
            i += 2
            continue
        i += 1

    # A possessive quantifier: ++, *+, ?+, }+ . RE2 has no possessive form.
    for i, ch, in_class, in_quote in _spans(pattern):
        if in_class or in_quote or ch != "+":
            continue
        if i > 0 and pattern[i - 1] in "+*?}":
            return "a possessive quantifier"

    return ""


def validate_car_limit_namespaces_patterns(obj: DotMap) -> tuple[list[str], list[str]]:
    """Refuses a limitNamespaces entry the authorization webhook would not be able to compile."""
    errors = []
    resource_name = obj.metadata.name

    limit_namespaces = obj.spec.get("limitNamespaces") if "limitNamespaces" in obj.spec else None
    if not isinstance(limit_namespaces, list):
        return errors, []

    for pattern in limit_namespaces:
        if not isinstance(pattern, str):
            continue
        reason = structural_error(pattern) or re2_unsupported_construct(pattern)
        if reason:
            errors.append(
                f"limitNamespaces entry '{pattern}' in ClusterAuthorizationRule '{resource_name}' "
                f"has {reason}, which the authorization webhook's regular expression engine (RE2) "
                f"cannot compile. The rule would be quarantined and its subjects would get less "
                f"access than written."
            )

    return errors, []


def validate_car_multitenancy_related_fields(obj: DotMap) -> tuple[list[str], list[str]]:
    errors = []
    resource_name = obj.metadata.name

    for field, description in MULTITENANCY_RESTRICTED_FIELDS.items():
        if field in obj.spec:
            errors.append(
                f"You must enable userAuthz.enableMultiTenancy to use the {description} "
                f"in ClusterAuthorizationRule '{resource_name}'"
            )

    return errors, []


if __name__ == "__main__":
    hook.run(main, config=CONFIG)
