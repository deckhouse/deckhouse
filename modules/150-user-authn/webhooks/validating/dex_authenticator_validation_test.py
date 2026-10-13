#!/usr/bin/python3

# Copyright 2026 Flant JSC
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

import json
import subprocess
import unittest

from deckhouse import hook

from pathlib import Path
from deckhouse.tests import load_validation_webhook

dex_authenticator_validation = load_validation_webhook(Path(__file__).parent / "dex_authenticator_validation.py")

# A copy of the jqFilter of the dexauthenticators context binding in
# templates/dex-authenticator-validation-webhook.yaml; the template tests keep the two identical.
JQ_FILTER = '''
.metadata.namespace as $namespace |
{
  "name": .metadata.name,
  "namespace": .metadata.namespace,
  "applicationDomain": .spec.applicationDomain,
  "ingressClass": .spec.applicationIngressClassName,
  "additionalDomains": [
    .spec.additionalApplications[]? | [.domain, .ingressClassName]
  ],
  "gatewayDomains": ([
    {
      "domain": .spec.applicationDomain,
      "listenerSetName": .spec.gatewayAPI.applicationHTTPRouteListenerSetName,
      "listenerSetNamespace": (.spec.gatewayAPI.applicationHTTPRouteListenerSetNamespace // $namespace),
      "sectionName": (.spec.gatewayAPI.applicationHTTPRouteListenerSetSectionName // "")
    },
    (.spec.additionalApplications[]? | {
      "domain": .domain,
      "listenerSetName": .gatewayAPI.httpRouteListenerSetName,
      "listenerSetNamespace": (.gatewayAPI.httpRouteListenerSetNamespace // $namespace),
      "sectionName": (.gatewayAPI.httpRouteListenerSetSectionName // "")
    })
  ] | map(select(.listenerSetName != null)))
}
'''

# The hook tells the two shapes apart by apiVersion alone, so the versions worth covering are
# v2alpha1 and one of the others.
HOOK_VERSIONS = ("v1", "v2alpha1")

# (name, the existing application is an additional one, the incoming application is an additional one)
APPLICATION_POSITIONS = (
    ("primary-primary", False, False),
    ("primary-additional", False, True),
    ("additional-primary", True, False),
    ("additional-additional", True, True),
)

GATEWAY_API_FIELD_NAMES = {
    "httpRouteListenerSetName": "applicationHTTPRouteListenerSetName",
    "httpRouteListenerSetNamespace": "applicationHTTPRouteListenerSetNamespace",
    "httpRouteListenerSetSectionName": "applicationHTTPRouteListenerSetSectionName",
}


# The hook reads two shapes of the same resource. Both shapes are written out here rather than
# produced by running the conversion hook, so that a broken conversion cannot make these tests fail
# for a reason that has nothing to do with them.
def listener_set_reference(name, namespace="", section=""):
    """The Gateway API reference of one application. An empty namespace or section is left out."""
    reference = {"httpRouteListenerSetName": name}
    if namespace:
        reference["httpRouteListenerSetNamespace"] = namespace
    if section:
        reference["httpRouteListenerSetSectionName"] = section
    return reference


def authenticator(version, namespace, name, *apps):
    obj = {
        "apiVersion": "deckhouse.io/" + version,
        "kind": "DexAuthenticator",
        "metadata": {"name": name, "namespace": namespace},
    }
    if version == "v2alpha1":
        obj["spec"] = {"applications": [dict(app) for app in apps]}
        return obj

    spec = {}
    if apps:
        first = apps[0]
        spec["applicationDomain"] = first["domain"]
        if "ingressClassName" in first:
            spec["applicationIngressClassName"] = first["ingressClassName"]
        if "gatewayAPI" in first:
            spec["gatewayAPI"] = {GATEWAY_API_FIELD_NAMES[k]: v for k, v in first["gatewayAPI"].items()}
    if len(apps) > 1:
        spec["additionalApplications"] = [dict(app) for app in apps[1:]]
    obj["spec"] = spec
    return obj


def with_application(version, namespace, name, app, additional):
    """Puts the application under test first or after an unrelated one, so both positions of the two
    shapes are exercised. The filler carries the namespace and the name in its domain: two fixtures
    must not collide on the filler and hide what the test is really about."""
    apps = [app]
    if additional:
        filler = {"domain": f"unrelated-{namespace}-{name}.example.com", "ingressClassName": "nginx"}
        apps = [filler, app]
    return authenticator(version, namespace, name, *apps)


def review(obj, *snapshots, operation="CREATE", old=None):
    request = {"operation": operation, "object": obj}
    if old is not None:
        request["oldObject"] = old
    return {
        "binding": "dexauthenticators-unique.deckhouse.io",
        "type": "Validating",
        "review": {"request": request},
        "snapshots": {"dexauthenticators": list(snapshots)},
    }


def snapshot(obj):
    """Runs the hook's own jqFilter over an object, as shell-operator does when it keeps the list of
    existing authenticators. The watch reads v1, whatever version the author used."""
    out = subprocess.run(["jq", JQ_FILTER], input=json.dumps(obj), capture_output=True, text=True, check=True)
    return {"filterResult": json.loads(out.stdout)}


def run(ctx):
    out = hook.testrun(dex_authenticator_validation.main, [ctx])
    assert len(out.validations.data) == 1, out.validations.data
    return out.validations.data[0]


class TestGatewayConflicts(unittest.TestCase):
    def test_conflicts(self):
        shared = listener_set_reference("shared", "d8-alb", "https")
        cases = (
            ("shared ListenerSet across namespaces", shared, shared, "team-a", "team-b", "app.example.com", False),
            ("different domain", shared, shared, "team-a", "team-b", "other.example.com", True),
            ("different ListenerSet", shared, listener_set_reference("other", "d8-alb", "https"), "team-a", "team-b", "app.example.com", True),
            ("different ListenerSet namespace", shared, listener_set_reference("shared", "other", "https"), "team-a", "team-b", "app.example.com", True),
            ("different section", shared, listener_set_reference("shared", "d8-alb", "other"), "team-a", "team-b", "app.example.com", True),
            ("existing all sections", listener_set_reference("shared", "d8-alb"), shared, "team-a", "team-b", "app.example.com", False),
            ("incoming all sections", shared, listener_set_reference("shared", "d8-alb"), "team-a", "team-b", "app.example.com", False),
            ("both all sections", listener_set_reference("shared", "d8-alb"), listener_set_reference("shared", "d8-alb"), "team-a", "team-b", "app.example.com", False),
            ("existing default namespace", listener_set_reference("shared", "", "https"), listener_set_reference("shared", "team-a", "https"), "team-a", "team-b", "app.example.com", False),
            # The existing claim is on a ListenerSet of the incoming namespace, made from outside it,
            # so it cannot be honoured and does not stand in the way. See TestClaimOnAForeignListenerSet.
            ("incoming default namespace", listener_set_reference("shared", "team-b", "https"), listener_set_reference("shared", "", "https"), "team-a", "team-b", "app.example.com", True),
            ("same default namespace", listener_set_reference("shared", "", "https"), listener_set_reference("shared", "", "https"), "team-a", "team-a", "app.example.com", False),
            ("different default namespaces", listener_set_reference("shared", "", "https"), listener_set_reference("shared", "", "https"), "team-a", "team-b", "app.example.com", True),
        )
        for name, existing_ref, incoming_ref, existing_ns, incoming_ns, incoming_domain, allowed in cases:
            for version in HOOK_VERSIONS:
                for position, existing_additional, incoming_additional in APPLICATION_POSITIONS:
                    with self.subTest(f"{version}/{position}/{name}"):
                        existing = with_application(
                            "v1", existing_ns, "test",
                            {"domain": "app.example.com", "gatewayAPI": existing_ref}, existing_additional)
                        incoming = with_application(
                            version, incoming_ns, "new",
                            {"domain": incoming_domain, "gatewayAPI": incoming_ref}, incoming_additional)

                        result = run(review(incoming, snapshot(existing)))
                        self.assertEqual(result["allowed"], allowed, result)
                        if not allowed:
                            self.assertIn("conflicts", result["message"])
                            self.assertIn(existing_ns + "/test", result["message"])
                            self.assertIn("app.example.com", result["message"])
                            self.assertIn("ListenerSet", result["message"])

    def test_conflict_with_ingress(self):
        """An Ingress conflict and a ListenerSet conflict are reported separately, so the message says
        which one it is."""
        for name, existing_class, incoming_class in (
            ("existing mixed", "nginx", ""), ("incoming mixed", "", "nginx"), ("both mixed", "first", "second"),
            ("ingress conflict with distinct ListenerSets", "nginx", "nginx"),
        ):
            with self.subTest(name):
                def app(cls):
                    fields = {"domain": "app.example.com", "gatewayAPI": listener_set_reference("shared", "d8-alb", "https")}
                    if cls:
                        fields["ingressClassName"] = cls
                    return fields

                ingress_conflict = existing_class != "" and existing_class == incoming_class
                incoming_app = app(incoming_class)
                if ingress_conflict:
                    incoming_app["gatewayAPI"] = listener_set_reference("other", "d8-alb", "https")
                existing = with_application("v1", "team-a", "test", app(existing_class), False)
                incoming = with_application("v2alpha1", "team-b", "test", incoming_app, False)

                result = run(review(incoming, snapshot(existing)))
                self.assertFalse(result["allowed"])
                if ingress_conflict:
                    self.assertIn("conflicts", result["message"])
                    self.assertNotIn("ListenerSet", result["message"])
                else:
                    self.assertEqual(
                        result["message"],
                        "Desired DexAuthenticator 'team-b/test' conflicts with the existing DexAuthenticator "
                        "'team-a/test' for domain 'app.example.com' on ListenerSet 'd8-alb/shared'",
                    )

    def test_conflict_on_update(self):
        """An authenticator must not conflict with itself, or no one could ever edit one."""
        for mode in ("ingress", "gateway"):
            for conflict in (False, True):
                with self.subTest(f"{mode}/{'other' if conflict else 'self'}"):
                    app = {"domain": "app.example.com"}
                    if mode == "ingress":
                        app["ingressClassName"] = "nginx"
                    else:
                        app["gatewayAPI"] = listener_set_reference("shared", "d8-alb", "https")

                    incoming = with_application("v1", "team-a", "test", app, False)
                    snapshots = [snapshot(incoming)]
                    if conflict:
                        snapshots.append(snapshot(with_application("v1", "team-b", "test", app, False)))

                    result = run(review(incoming, *snapshots))
                    self.assertEqual(result["allowed"], not conflict)

    def test_same_domain_across_routing_modes(self):
        """A shared domain alone does not imply a conflict between Ingress and Gateway API."""
        for version in HOOK_VERSIONS:
            for position, existing_additional, incoming_additional in APPLICATION_POSITIONS:
                for existing_mode in ("ingress", "gateway"):
                    with self.subTest(f"{version}/{position}/existing-{existing_mode}"):
                        existing_app = {"domain": "app.example.com", "ingressClassName": "nginx"}
                        incoming_app = {"domain": "app.example.com", "gatewayAPI": listener_set_reference("shared", "d8-alb", "https")}
                        if existing_mode == "gateway":
                            existing_app, incoming_app = incoming_app, existing_app

                        existing = with_application("v1", "team-a", "test", existing_app, existing_additional)
                        incoming = with_application(version, "team-b", "new", incoming_app, incoming_additional)

                        self.assertTrue(run(review(incoming, snapshot(existing)))["allowed"])

    def test_conflict_is_not_rechecked_on_update(self):
        """Pairs sharing a domain could be created before these checks existed. Refusing every edit to
        them would leave them stuck, so only what the object did not already claim is checked."""
        for mode in ("ingress", "gateway"):
            def claim(domain):
                app = {"domain": domain}
                if mode == "ingress":
                    app["ingressClassName"] = "nginx"
                else:
                    app["gatewayAPI"] = listener_set_reference("shared", "d8-alb", "https")
                return app

            existing = snapshot(with_application("v1", "team-a", "first", claim("app.example.com"), False))
            for name, old_domain, deleting, denied in (
                ("claim carried over from the old object", "app.example.com", False, False),
                ("claim newly pointed at the taken domain", "other.example.com", False, True),
                ("object being deleted", "other.example.com", True, False),
            ):
                with self.subTest(f"{mode}/{name}"):
                    obj = with_application("v1", "team-b", "second", claim("app.example.com"), False)
                    old = with_application("v1", "team-b", "second", claim(old_domain), False)
                    if deleting:
                        obj["metadata"]["deletionTimestamp"] = "2026-09-25T10:00:00Z"

                    result = run(review(obj, existing, operation="UPDATE", old=old))
                    self.assertEqual(result["allowed"], not denied, result.get("message"))

            # Without an old object the claim is new, whatever it repeats.
            with self.subTest(f"{mode}/created rather than updated"):
                obj = with_application("v1", "team-b", "second", claim("app.example.com"), False)
                self.assertFalse(run(review(obj, existing))["allowed"])

    def test_claim_on_a_foreign_listener_set(self):
        """A ListenerSet takes routes from its own namespace unless its owner opened it, so naming one
        from elsewhere claims nothing. Such a claim must not keep the namespace that owns the
        ListenerSet from publishing on it."""
        own_listener_set = listener_set_reference("app")
        foreign_claim = snapshot(with_application(
            "v1", "team-b", "squat", {"domain": "app.example.com", "gatewayAPI": listener_set_reference("app", "team-a")}, False))

        with self.subTest("the owner publishes on it"):
            owner = with_application("v1", "team-a", "legit", {"domain": "app.example.com", "gatewayAPI": own_listener_set}, False)
            result = run(review(owner, foreign_claim))
            self.assertTrue(result["allowed"], result.get("message"))

        # The claim of the namespace that owns the ListenerSet is the one that counts, so a second
        # attempt from outside is still refused.
        with self.subTest("another namespace claims it as well"):
            owned = snapshot(with_application(
                "v1", "team-a", "legit", {"domain": "app.example.com", "gatewayAPI": own_listener_set}, False))
            outsider = with_application(
                "v1", "team-c", "squat", {"domain": "app.example.com", "gatewayAPI": listener_set_reference("app", "team-a")}, False)
            self.assertFalse(run(review(outsider, owned))["allowed"])


class TestDomains(unittest.TestCase):
    def test_domain_validation_without_ingress_class(self):
        """An application published only through Gateway API is checked like any other: it used to fall
        out of the loop that rejects an IP address in place of a domain."""
        for version in HOOK_VERSIONS:
            for domain in ("app.example.com", "192.0.2.1", "2001:db8::1"):
                for additional in (False, True):
                    with self.subTest(f"{version} {domain} additional={additional}"):
                        app = {"domain": domain, "gatewayAPI": listener_set_reference("app")}
                        result = run(review(with_application(version, "test", "test", app, additional)))
                        self.assertEqual(result["allowed"], domain == "app.example.com")

    def test_ip_address_message(self):
        result = run(review(authenticator("v2alpha1", "ns", "a", {"domain": "app.example.com", "ingressClassName": "nginx"},
                                          {"domain": "10.0.0.1", "ingressClassName": "nginx"})))
        self.assertEqual(
            result["message"],
            "DexAuthenticator 'ns/a' is invalid: '.spec.applications[1].domain' must be a DNS name, not an IP "
            "address ('10.0.0.1')",
        )

    def test_ingress_conflict(self):
        existing = {"filterResult": {
            "name": "existing", "namespace": "test",
            "applicationDomain": "app.example.com", "ingressClass": "nginx",
            "additionalDomains": [],
        }}
        for version in HOOK_VERSIONS:
            with self.subTest(version):
                obj = authenticator(version, "test", "test", {"domain": "app.example.com", "ingressClassName": "nginx"})
                result = run(review(obj, existing))
                self.assertEqual(
                    result,
                    {"allowed": False, "message": "Desired DexAuthenticator 'test/test' conflicts with the existing "
                                                  "DexAuthenticator 'test/existing' for domain 'app.example.com'"},
                )

    def test_ingress_conflict_with_an_additional_application_lists_every_owner(self):
        existing = [
            {"filterResult": {"name": n, "namespace": "other", "applicationDomain": "x.example.com", "ingressClass": "nginx",
                              "additionalDomains": [["app.example.com", "nginx"]]}}
            for n in ("one", "two")
        ]
        obj = authenticator("v1", "test", "test", {"domain": "app.example.com", "ingressClassName": "nginx"})
        result = run(review(obj, *existing))
        self.assertEqual(
            result["message"],
            "Desired DexAuthenticator 'test/test' conflicts with the existing DexAuthenticator 'other/one\nother/two' "
            "for domain 'app.example.com'",
        )

    def test_duplicate_domain_within_object(self):
        """Applications after the first take a resource-name suffix derived from sha256(domain), so two
        of them sharing a domain would name two objects of one kind alike. The first application
        carries no suffix, and an Ingress may share a name with an HTTPRoute, so neither of those
        repeats collides."""
        gateway = listener_set_reference("app-listeners")

        def nginx(domain):
            return {"domain": domain, "ingressClassName": "nginx"}

        def route(domain):
            return {"domain": domain, "gatewayAPI": gateway}

        cases = (
            ("distinct domains", [nginx("first.example.com"), nginx("second.example.com")], ""),
            # The first application has no name suffix, so these render as two distinct objects.
            ("first application repeated, same class", [nginx("app.example.com"), nginx("app.example.com")], ""),
            ("first application repeated, another class",
             [nginx("app.example.com"), {"domain": "app.example.com", "ingressClassName": "nginx-internal"}], ""),
            ("first application repeated through Gateway API", [nginx("app.example.com"), route("app.example.com")], ""),
            # One names an Ingress, the other an HTTPRoute: different kinds may share a name.
            ("additional applications split across publication paths",
             [nginx("first.example.com"), nginx("same.example.com"), route("same.example.com")], ""),
            # Both suffixes are sha256("same.example.com") and both name an Ingress.
            ("two additional applications share a domain",
             [nginx("first.example.com"), nginx("same.example.com"), nginx("same.example.com")], "Ingress"),
            ("colliding additional applications differ in class",
             [nginx("first.example.com"), nginx("same.example.com"),
              {"domain": "same.example.com", "ingressClassName": "nginx-internal"}], "Ingress"),
            ("two additional applications share a domain through Gateway API",
             [nginx("first.example.com"), route("same.example.com"), route("same.example.com")], "HTTPRoute"),
            # Publishing both ways collides on whichever kind the earlier application already named.
            ("additional application repeats both publication paths",
             [nginx("first.example.com"),
              {"domain": "same.example.com", "ingressClassName": "nginx", "gatewayAPI": gateway},
              route("same.example.com")], "HTTPRoute"),
        )
        for name, apps, duplicate in cases:
            for version in HOOK_VERSIONS:
                with self.subTest(f"{version}/{name}"):
                    result = run(review(authenticator(version, "test", "test", *apps)))
                    self.assertEqual(result["allowed"], duplicate == "", result.get("message"))
                    if duplicate:
                        self.assertIn("repeats domain 'same.example.com'", result["message"])
                        self.assertIn("a second " + duplicate, result["message"])

    def test_duplicate_message(self):
        apps = [{"domain": "a.example.com", "ingressClassName": "nginx"}] + \
               [{"domain": "same.example.com", "ingressClassName": "nginx"}] * 2
        result = run(review(authenticator("v1", "ns", "a", *apps)))
        self.assertEqual(
            result["message"],
            "DexAuthenticator 'ns/a' is invalid: '.spec.additionalApplications[1].domain' repeats domain "
            "'same.example.com', so it would name a second Ingress after another additional application of "
            "this DexAuthenticator",
        )

    def test_domain_spelled_null_is_skipped(self):
        # The old hook read fields as text, and a missing field printed as "null".
        apps = [{"domain": "a.example.com", "ingressClassName": "nginx"}] + [{"domain": "null", "ingressClassName": "nginx"}] * 2
        self.assertTrue(run(review(authenticator("v2alpha1", "ns", "a", *apps)))["allowed"])


class TestSnapshots(unittest.TestCase):
    def test_missing_snapshots_fail_the_request_once_read(self):
        # The KeyError reaches the template's main, which answers with validations.error.
        ctx = review(authenticator("v1", "ns", "a", {"domain": "a.example.com", "ingressClassName": "nginx"}))
        del ctx["snapshots"]
        response = run(ctx)
        # deckhouse 0.4.11 puts the text under a key equal to the text, 0.4.12 under "message".
        self.assertFalse(response["allowed"])
        self.assertIn("Internal webhook error: 'snapshots'", response.values())

    def test_missing_snapshots_are_not_read_without_a_claim(self):
        ctx = review(authenticator("v2alpha1", "ns", "a"))
        del ctx["snapshots"]
        self.assertTrue(run(ctx)["allowed"])


if __name__ == "__main__":
    unittest.main()
