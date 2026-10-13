# Copyright 2021 Flant JSC
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

# Refuses a DexAuthenticator whose domains are IP addresses, repeat each other where that would name
# two objects alike, or are already published by another DexAuthenticator through the same Ingress
# class or ListenerSet. It replaces a Bash hook and keeps its decisions and messages.
#
# The hook reads two shapes of the same resource. v2alpha1 lists every application in
# spec.applications; v1 and v1alpha1 describe the first one in spec itself and the rest in
# spec.additionalApplications, with different field names for the Gateway API reference.
#
# A field the old hook read and found missing printed as "null", and a domain or an Ingress class
# spelled "null" therefore counts as missing here too.
#
# templates/dex-authenticator-validation-webhook.yaml embeds this file in a ValidationWebhook;
# webhook-operator renders it into a hook that calls validate(). The existing DexAuthenticators come
# from the snapshot of its context binding, in the shape its jqFilter gives them.

import ipaddress

# The operator template imports these as well; they make the file importable by the tests.
from typing import Optional
from dotmap import DotMap

V2ALPHA1 = "deckhouse.io/v2alpha1"
# The applications that take no resource-name suffix.
FIRST_APPLICATION = (".spec.applicationDomain", ".spec.applications[0].domain")


def _text(value) -> str:
    return "null" if value is None else str(value)


def _alt(value, default):
    """jq's `value // default`."""
    return default if value is None or value is False else value


def _get(obj, *path):
    for key in path:
        obj = (obj or {}).get(key)
    return obj


def _is_ip_address(value: str) -> bool:
    try:
        ipaddress.ip_address(value)
    except ValueError:
        return False
    return True


def _applications(obj: dict) -> list:
    """(domain, Ingress class, ListenerSet, field path) of every application, as text."""
    spec = obj.get("spec") or {}
    if obj.get("apiVersion") == V2ALPHA1:
        apps, path = spec.get("applications") or [], ".spec.applications[{}].domain"
        result = []
    else:
        apps, path = spec.get("additionalApplications") or [], ".spec.additionalApplications[{}].domain"
        result = [(
            _text(spec.get("applicationDomain")),
            _text(spec.get("applicationIngressClassName")),
            _text(_get(spec, "gatewayAPI", "applicationHTTPRouteListenerSetName")),
            ".spec.applicationDomain",
        )]
    for index, app in enumerate(apps):
        if _text(app.get("domain")) != "null":
            result.append((
                app["domain"],
                _text(app.get("ingressClassName")),
                _text(_get(app, "gatewayAPI", "httpRouteListenerSetName")),
                path.format(index),
            ))
    return result


def _ingress_claims(obj) -> set:
    """The (domain, Ingress class) pairs an object publishes, whatever its version."""
    if obj is None:
        return set()
    spec = obj.get("spec") or {}
    pairs = [(spec.get("applicationDomain"), spec.get("applicationIngressClassName"))]
    for app in (spec.get("applications") or []) + (spec.get("additionalApplications") or []):
        pairs.append((app.get("domain"), app.get("ingressClassName")))
    return {(domain, cls) for domain, cls in pairs if domain is not None and cls is not None}


def _listener_set_claims(obj) -> list:
    """The ListenerSet claims of an object, in the form the snapshot keeps them."""
    namespace = _get(obj, "metadata", "namespace")
    spec = obj.get("spec") or {}
    if obj.get("apiVersion") == V2ALPHA1:
        apps = spec["applications"]
        if apps is None:
            raise TypeError("cannot iterate over the null spec.applications")
    else:
        gateway = spec.get("gatewayAPI") or {}
        apps = [{
            "domain": spec.get("applicationDomain"),
            "gatewayAPI": {
                "httpRouteListenerSetName": gateway.get("applicationHTTPRouteListenerSetName"),
                "httpRouteListenerSetNamespace": gateway.get("applicationHTTPRouteListenerSetNamespace"),
                "httpRouteListenerSetSectionName": gateway.get("applicationHTTPRouteListenerSetSectionName"),
            },
        }] + (spec.get("additionalApplications") or [])
    claims = []
    for app in apps:
        gateway = app.get("gatewayAPI") or {}
        if gateway.get("httpRouteListenerSetName") is None:
            continue
        claims.append({
            "domain": app.get("domain"),
            "listenerSetName": gateway["httpRouteListenerSetName"],
            "listenerSetNamespace": _alt(gateway.get("httpRouteListenerSetNamespace"), namespace),
            "sectionName": _alt(gateway.get("httpRouteListenerSetSectionName"), ""),
        })
    return claims


def _same_listener(published: dict, claim: dict) -> bool:
    """Tells whether a published route and a claim take the same listener. No section means all."""
    return (
        published.get("domain") == claim["domain"]
        and published.get("listenerSetName") == claim["listenerSetName"]
        and published.get("listenerSetNamespace") == claim["listenerSetNamespace"]
        and (published.get("sectionName") == claim["sectionName"]
             or published.get("sectionName") == "" or claim["sectionName"] == "")
    )


def _existing(binding_context: dict) -> list:
    return [item.get("filterResult") or {} for item in binding_context["snapshots"]["dexauthenticators"]]


def check(binding_context: dict) -> Optional[str]:
    """Returns None to allow the request, or the message to deny it with."""
    request = binding_context["review"]["request"]
    obj = request["object"]
    namespace, name = _text(_get(obj, "metadata", "namespace")), _text(_get(obj, "metadata", "name"))

    # An object being deleted is only passing through: a finalizer set by something else turns the
    # deletion into updates, and refusing those would leave the object stuck forever.
    if _get(obj, "metadata", "deletionTimestamp"):
        return None

    # What the object already published is not re-checked, for the same reason as the ListenerSet
    # check below: conflicting pairs could be created before these checks existed.
    old_ingress_claims = _ingress_claims(request.get("oldObject"))

    # Applications after the first one take a resource-name suffix derived from sha256(domain), so
    # two of them sharing a domain would name two objects of one kind alike. The first application
    # carries no suffix, and an Ingress and an HTTPRoute may share a name, so the domains are
    # tracked per publication path rather than globally.
    seen_ingress_domains, seen_route_domains = set(), set()

    for domain, ingress_class, listener_set, field_path in _applications(obj):
        if _is_ip_address(domain):
            return (f"DexAuthenticator '{namespace}/{name}' is invalid: '{field_path}' must be a DNS name, "
                    f"not an IP address ('{domain}')")

        if field_path not in FIRST_APPLICATION:
            duplicate_kind = None
            if ingress_class != "null" and domain in seen_ingress_domains:
                duplicate_kind = "Ingress"
            elif listener_set != "null" and domain in seen_route_domains:
                duplicate_kind = "HTTPRoute"
            if duplicate_kind:
                return (f"DexAuthenticator '{namespace}/{name}' is invalid: '{field_path}' repeats domain "
                        f"'{domain}', so it would name a second {duplicate_kind} after another additional "
                        f"application of this DexAuthenticator")
            if ingress_class != "null":
                seen_ingress_domains.add(domain)
            if listener_set != "null":
                seen_route_domains.add(domain)

        if ingress_class == "null" or (domain, ingress_class) in old_ingress_claims:
            continue

        conflicts = [
            f"{existing.get('namespace') or ''}/{existing.get('name') or ''}"
            for existing in _existing(binding_context)
            if (existing.get("namespace") != namespace or existing.get("name") != name)
            and (
                (existing.get("applicationDomain") == domain and existing.get("ingressClass") == ingress_class)
                or [domain, ingress_class] in (existing.get("additionalDomains") or [])
            )
        ]
        if conflicts:
            # Every conflicting DexAuthenticator, one per line, as the old hook listed them.
            return (f"Desired DexAuthenticator '{namespace}/{name}' conflicts with the existing "
                    f"DexAuthenticator '{chr(10).join(conflicts)}' for domain '{domain}'")

    # Compare ListenerSet references and overlapping sections only. Detecting conflicts across
    # different ListenerSets attached to one Gateway requires Gateway topology.
    #
    # Only what the object did not already claim is checked. Pairs sharing a ListenerSet could be
    # created before this check existed, and refusing every edit to them would leave them stuck:
    # nothing about them could be fixed, including the conflict.
    old = request.get("oldObject")
    kept = _listener_set_claims(old) if old is not None else []
    object_namespace, object_name = _get(obj, "metadata", "namespace"), _get(obj, "metadata", "name")
    for claim in _listener_set_claims(obj):
        if claim in kept:
            continue
        for existing in _existing(binding_context):
            if existing.get("namespace") == object_namespace and existing.get("name") == object_name:
                continue
            # A listener takes routes from its own namespace unless its owner opened it, so a claim on
            # a ListenerSet of this namespace made from elsewhere cannot be honoured and must not
            # stand in the way of the namespace that owns it.
            if claim["listenerSetNamespace"] == object_namespace and existing.get("namespace") != object_namespace:
                continue
            if any(_same_listener(published, claim) for published in existing.get("gatewayDomains") or []):
                return (f"Desired DexAuthenticator '{namespace}/{name}' conflicts with the existing "
                        f"DexAuthenticator '{_text(existing.get('namespace'))}/{_text(existing.get('name'))}' "
                        f"for domain '{_text(claim['domain'])}' on ListenerSet "
                        f"'{_text(claim['listenerSetNamespace'])}/{_text(claim['listenerSetName'])}'")
    return None


def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    message = check(ctx.toDict())
    return message, message is None
