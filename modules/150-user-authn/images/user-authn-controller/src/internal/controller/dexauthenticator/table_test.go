/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dexauthenticator

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/internal/naming"
)

var (
	testSettings = Settings{ClusterDomain: "cluster.local", HTTPSMode: httpsModeCertManager, IngressEnabled: true, IngressNginxHSTSAnnotation: true}
	testNow      = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
)

// testAuthenticator is an object with the given domains, created age minutes after the start of
// the test time.
func testAuthenticator(namespace, name string, age int, domains ...string) Authenticator {
	a := Authenticator{
		Name:                 name,
		Namespace:            namespace,
		UID:                  types.UID("uid-" + namespace + "-" + name),
		Created:              testNow.Add(time.Duration(age) * time.Minute),
		KeepUsersLoggedInFor: defaultKeepUsersLoggedInFor,
	}
	for _, domain := range domains {
		a.Applications = append(a.Applications, Application{Domain: domain, IngressClassName: "nginx"})
	}
	return a
}

// throughGateway returns the object with every application published through the Gateway API: such
// an application does not share a host with another object.
func throughGateway(a Authenticator) Authenticator {
	a.Applications = slices.Clone(a.Applications)
	for i := range a.Applications {
		a.Applications[i].IngressClassName = ""
		a.Applications[i].ListenerSet = ListenerSet{Name: "public"}
	}
	return a
}

// secretNumber numbers the secrets of the tests, so that no two of them are equal.
var secretNumber atomic.Int64

// sequentialSecrets returns secrets of a fixed length that differ on every call.
func sequentialSecrets() (string, string, error) {
	n := secretNumber.Add(1)
	return fmt.Sprintf("client-secret-%06d", n), fmt.Sprintf("cookie-secret-%06d", n), nil
}

func testInput(authenticators ...Authenticator) buildInput {
	return buildInput{
		authenticators: authenticators,
		previous:       map[string]table{},
		settings:       testSettings,
		now:            testNow,
		limit:          TableLimit,
		namespaceShare: NamespaceShare,
		newSecrets:     sequentialSecrets,
	}
}

// build builds the tables and fails the test on an error.
func build(t *testing.T, in buildInput) buildResult {
	t.Helper()

	result, err := buildTables(in)
	if err != nil {
		t.Fatalf("buildTables: %v", err)
	}
	return result
}

// next turns the result of a build into the input of the following one, as the reconciler does:
// the tables are in the cluster and the generations of the records are remembered.
func next(result buildResult, authenticators ...Authenticator) buildInput {
	in := testInput(authenticators...)
	in.generations = map[types.NamespacedName]int64{}
	for instance, built := range result.tables {
		in.previous[instance] = built.table
	}
	for key, entry := range result.entries {
		if entry.Record != nil {
			in.generations[key] = entry.Generation
		}
	}
	return in
}

func keyOf(namespace, name string) types.NamespacedName {
	return types.NamespacedName{Namespace: namespace, Name: name}
}

func TestBuildTables_RecordOfAnObject(t *testing.T) {
	t.Parallel()

	shop := testAuthenticator("shop", "app", 0, "app.example.com", "admin.example.com")
	shop.SendAuthorizationHeader = true
	shop.AllowAccessToKubernetes = true
	shop.KeepUsersLoggedInFor = "24h"
	console := testAuthenticator("d8-console", "console", 1, "console.example.com")

	result := build(t, testInput(shop, console))

	user := result.tables[naming.UserAuthenticatorInstance]
	if len(user.file.Clients) != 1 {
		t.Fatalf("the user instance has %d records, want the one of the shop object", len(user.file.Clients))
	}
	want := Record{
		Name: "app", Namespace: "shop",
		Hosts:        []string{"app.example.com", "admin.example.com"},
		Services:     []string{"app-dex-authenticator.shop.svc.cluster.local"},
		ClientID:     "dex-authenticator-app@shop",
		ClientSecret: user.file.Clients[0].ClientSecret, CookieSecret: user.file.Clients[0].CookieSecret,
		CookieExpire:            "24h",
		AllowAccessToKubernetes: true, SendAuthorizationHeader: true, ProviderLogout: true,
	}
	if got := user.file.Clients[0]; !recordsEqual(got, want) || got.ClientSecret == "" || got.CookieSecret == "" {
		t.Errorf("record = %+v, want %+v with secrets", got, want)
	}
	if got := user.owners["shop/app"]; got != shop.UID {
		t.Errorf("owner of shop/app = %q, want %q", got, shop.UID)
	}

	system := result.tables[naming.SystemAuthenticatorInstance]
	if len(system.file.Clients) != 1 || system.file.Clients[0].Name != "console" {
		t.Fatalf("the system instance has %+v, want the record of d8-console/console", system.file.Clients)
	}
	if got := system.file.Clients[0].CookieExpire; got != "168h" {
		t.Errorf("cookieExpire = %q, want the default 168h", got)
	}

	for _, instance := range naming.AuthenticatorInstances() {
		built := result.tables[instance]
		if !built.changed || built.file.Generation != testNow.Unix() {
			t.Errorf("%s: changed %v, generation %d; want a new table of generation %d", instance, built.changed, built.file.Generation, testNow.Unix())
		}
	}
	entry := result.entries[shop.Key()]
	if entry.Record == nil || entry.Reason != "" || entry.Generation != testNow.Unix() || entry.Instance != naming.UserAuthenticatorInstance {
		t.Errorf("entry of the shop object = %+v", entry)
	}
}

func recordsEqual(a, b Record) bool {
	rawA, _ := json.Marshal(a)
	rawB, _ := json.Marshal(b)
	return string(rawA) == string(rawB)
}

// TestBuildTables_FileIsTheOneTheProxyReads pins the names of the fields to the table of patch 018:
// the proxy refuses a record with a field it does not know and a file without the list.
func TestBuildTables_FileIsTheOneTheProxyReads(t *testing.T) {
	t.Parallel()

	result := build(t, testInput(testAuthenticator("shop", "app", 0, "app.example.com")))
	data, err := result.tables[naming.UserAuthenticatorInstance].data()
	if err != nil {
		t.Fatalf("data: %v", err)
	}
	record := result.entries[keyOf("shop", "app")].Record
	want := fmt.Sprintf(`{"generation":%d,"clients":[{"name":"app","namespace":"shop","hosts":["app.example.com"],`+
		`"services":["app-dex-authenticator.shop.svc.cluster.local"],"clientID":"dex-authenticator-app@shop","clientSecret":%q,"cookieSecret":%q,`+
		`"cookieExpire":"168h","providerLogout":true}]}`, testNow.Unix(), record.ClientSecret, record.CookieSecret)
	if got := string(data[TableKey]); got != want {
		t.Errorf("%s =\n%s\nwant\n%s", TableKey, got, want)
	}
	if got := string(data[OwnersKey]); got != `{"shop/app":"uid-shop-app"}` {
		t.Errorf("%s = %s", OwnersKey, got)
	}

	empty, err := build(t, testInput()).tables[naming.SystemAuthenticatorInstance].data()
	if err != nil {
		t.Fatalf("data: %v", err)
	}
	if got := string(empty[TableKey]); !strings.HasSuffix(got, `"clients":[]}`) {
		t.Errorf("an empty table is %s, want an empty list of clients", got)
	}
}

func TestBuildTables_WritesOnlyAChange(t *testing.T) {
	t.Parallel()

	app := testAuthenticator("shop", "app", 0, "app.example.com")
	other := testAuthenticator("shop", "other", 1, "other.example.com")
	first := build(t, testInput(app, other))

	// The same objects, listed in another order: nothing to write, the secrets and the generation stay.
	in := next(first, other, app)
	in.now = testNow.Add(time.Hour)
	second := build(t, in)
	for _, instance := range naming.AuthenticatorInstances() {
		if built := second.tables[instance]; built.changed || built.file.Generation != first.tables[instance].file.Generation {
			t.Errorf("%s: changed %v, generation %d", instance, built.changed, built.file.Generation)
		}
	}
	if got, want := second.entries[app.Key()].Record.ClientSecret, first.entries[app.Key()].Record.ClientSecret; got != want {
		t.Errorf("client secret = %q, want the one of the first table, %q", got, want)
	}

	// One object changes: the table gets a new generation, and only the changed record carries it.
	other.SendAuthorizationHeader = true
	in = next(second, app, other)
	in.now = testNow.Add(2 * time.Hour)
	third := build(t, in)
	user := third.tables[naming.UserAuthenticatorInstance]
	if !user.changed || user.file.Generation != in.now.Unix() {
		t.Fatalf("changed %v, generation %d; want a new table of generation %d", user.changed, user.file.Generation, in.now.Unix())
	}
	if got := third.entries[app.Key()].Generation; got != testNow.Unix() {
		t.Errorf("generation of the record that did not change = %d, want %d", got, testNow.Unix())
	}
	if got := third.entries[other.Key()].Generation; got != in.now.Unix() {
		t.Errorf("generation of the changed record = %d, want %d", got, in.now.Unix())
	}
	if third.tables[naming.SystemAuthenticatorInstance].changed {
		t.Error("the table of the system instance changed")
	}
}

// TestBuildTables_GenerationGrows covers the two sources of a generation: the clock, and the
// previous generation when the clock has not moved past it.
func TestBuildTables_GenerationGrows(t *testing.T) {
	t.Parallel()

	app := testAuthenticator("shop", "app", 0, "app.example.com")
	first := build(t, testInput(app))

	app.SendAuthorizationHeader = true
	second := build(t, next(first, app))
	if got, want := second.tables[naming.UserAuthenticatorInstance].file.Generation, testNow.Unix()+1; got != want {
		t.Errorf("generation within the same second = %d, want %d", got, want)
	}

	// The Secret is lost: the table is written from nothing, later than any table before it.
	in := testInput(app)
	in.now = testNow.Add(time.Minute)
	lost := build(t, in)
	if got := lost.tables[naming.UserAuthenticatorInstance].file.Generation; got <= second.tables[naming.UserAuthenticatorInstance].file.Generation {
		t.Errorf("generation after the loss of the Secret = %d, not above %d", got, second.tables[naming.UserAuthenticatorInstance].file.Generation)
	}
	// Nothing remembers the secrets of a lost table: the record gets new ones.
	if got := lost.entries[app.Key()].Record.ClientSecret; got == first.entries[app.Key()].Record.ClientSecret {
		t.Errorf("client secret = %q, the one of the lost table", got)
	}
}

// TestBuildTables_RestartKeepsGenerations: a reconciler that has just started remembers no
// generation and takes the one of the table for every record.
func TestBuildTables_RestartKeepsGenerations(t *testing.T) {
	t.Parallel()

	app := testAuthenticator("shop", "app", 0, "app.example.com")
	first := build(t, testInput(app))

	in := next(first, app)
	in.generations = nil
	in.now = testNow.Add(time.Hour)
	second := build(t, in)
	if second.tables[naming.UserAuthenticatorInstance].changed {
		t.Fatal("the table changed on a restart")
	}
	if got := second.entries[app.Key()].Generation; got != testNow.Unix() {
		t.Errorf("generation = %d, want the one of the table, %d", got, testNow.Unix())
	}
}

func TestBuildTables_HostGoesToTheOldestObject(t *testing.T) {
	t.Parallel()

	holder := testAuthenticator("shop", "holder", 0, "app.example.com")
	// The applications of the younger objects are published through the Gateway API: they cannot
	// share a host, so the oldest object that names a host is the only one served on it.
	younger := throughGateway(testAuthenticator("another", "younger", 5, "app.example.com", "own.example.com"))
	loser := throughGateway(testAuthenticator("third", "loser", 7, "app.example.com"))
	// An object of a system namespace does not get a host over an older object of a user.
	system := throughGateway(testAuthenticator("d8-console", "console", 9, "own.example.com", "console.example.com"))

	result := build(t, testInput(system, loser, younger, holder))

	if entry := result.entries[holder.Key()]; entry.Record == nil || entry.Reason != "" {
		t.Errorf("the oldest object: %+v", entry)
	}

	entry := result.entries[younger.Key()]
	if entry.Record == nil || !slices.Equal(entry.Record.Hosts, []string{"own.example.com"}) {
		t.Fatalf("the younger object: %+v, want a record with own.example.com only", entry)
	}
	if entry.Reason != ReasonHostConflict || !strings.Contains(entry.Message, "app.example.com") || !strings.Contains(entry.Message, "served on own.example.com") {
		t.Errorf("the younger object: reason %q, message %q", entry.Reason, entry.Message)
	}
	// The message does not tell a user which namespace holds the host.
	if strings.Contains(entry.Message, "shop") || strings.Contains(entry.Message, "holder") {
		t.Errorf("the message names the object that holds the host: %q", entry.Message)
	}

	entry = result.entries[loser.Key()]
	if entry.Record != nil || entry.Reason != ReasonHostConflict || !strings.Contains(entry.Message, "not served") {
		t.Errorf("the object with no host left: %+v", entry)
	}
	if _, found := result.tables[naming.UserAuthenticatorInstance].record(loser.Key()); found {
		t.Error("the object with no host left has a record")
	}

	entry = result.entries[system.Key()]
	if entry.Record == nil || !slices.Equal(entry.Record.Hosts, []string{"console.example.com"}) || entry.Reason != ReasonHostConflict {
		t.Errorf("the system object: %+v", entry)
	}
}

// TestBuildTables_ServedHostStaysWithItsObject: an older object that names a host later does not
// take it from the object that is served on it. The instance picks the client of a request by
// its host, so the host would bring the application of one namespace under the client of another.
func TestBuildTables_ServedHostStaysWithItsObject(t *testing.T) {
	t.Parallel()

	older := testAuthenticator("attacker", "older", 0, "own.example.com")
	served := testAuthenticator("shop", "app", 60, "app.example.com", "admin.example.com")
	first := build(t, testInput(older, served))
	record := *first.entries[served.Key()].Record

	older.Applications = append(older.Applications, Application{Domain: "app.example.com", ListenerSet: ListenerSet{Name: "public"}})
	second := build(t, next(first, older, served))
	if entry := second.entries[served.Key()]; entry.Record == nil || !recordsEqual(*entry.Record, record) || entry.Reason != "" {
		t.Errorf("the object that was served on the host: %+v", entry)
	}
	entry := second.entries[older.Key()]
	if entry.Record == nil || !slices.Equal(entry.Record.Hosts, []string{"own.example.com"}) || entry.Reason != ReasonHostConflict {
		t.Errorf("the older object that named the host later: %+v", entry)
	}
	if second.tables[naming.UserAuthenticatorInstance].file.Generation != first.tables[naming.UserAuthenticatorInstance].file.Generation {
		t.Error("the table changed")
	}

	// The holder gives the host up: it passes to the object that names it.
	served.Applications = served.Applications[1:]
	third := build(t, next(second, older, served))
	if entry := third.entries[older.Key()]; entry.Record == nil || !slices.Equal(entry.Record.Hosts, []string{"own.example.com", "app.example.com"}) || entry.Reason != "" {
		t.Errorf("the object that named the freed host: %+v", entry)
	}
	if entry := third.entries[served.Key()]; entry.Record == nil || !slices.Equal(entry.Record.Hosts, []string{"admin.example.com"}) {
		t.Errorf("the object that gave the host up: %+v", entry)
	}

	// A record made for another object of the same name holds nothing: age decides.
	recreated := testAuthenticator("shop", "app", 60, "app.example.com")
	recreated.UID = "uid-of-the-new-object"
	older.Applications = older.Applications[:1]
	older.Applications = append(older.Applications, Application{Domain: "admin.example.com", IngressClassName: "nginx"})
	fourth := build(t, next(second, older, recreated))
	if entry := fourth.entries[older.Key()]; entry.Record == nil || !slices.Contains(entry.Record.Hosts, "admin.example.com") {
		t.Errorf("the older object did not get the host the deleted object held: %+v", entry)
	}
}

// TestBuildTables_SameAgeIsOrderedByName: two objects created within one second get a stable winner.
func TestBuildTables_SameAgeIsOrderedByName(t *testing.T) {
	t.Parallel()

	a := testAuthenticator("shop", "a", 0, "app.example.com")
	b := testAuthenticator("shop", "b", 0, "app.example.com")
	for _, objects := range [][]Authenticator{{a, b}, {b, a}} {
		result := build(t, testInput(objects...))
		if result.entries[a.Key()].Record == nil || result.entries[b.Key()].Record != nil {
			t.Errorf("order %s, %s: the host did not go to shop/a", objects[0].Name, objects[1].Name)
		}
	}
}

// TestBuildTables_SharedHost: two objects publish one domain through two Ingress classes. The
// instance chooses the oldest by the host and the other by the name of its Service, and the
// Ingress objects of both send that name.
func TestBuildTables_SharedHost(t *testing.T) {
	t.Parallel()

	owner := testAuthenticator("shop", "app", 0, "app.example.com")
	sharer := testAuthenticator("another", "app", 5, "app.example.com", "own.example.com")
	sharer.Applications[0].IngressClassName = "nginx-internal"
	alone := testAuthenticator("third", "app", 7, "alone.example.com")

	first := build(t, testInput(sharer, alone, owner))

	entry := first.entries[owner.Key()]
	if entry.Record == nil || entry.Reason != "" || !slices.Equal(entry.Record.Hosts, []string{"app.example.com"}) || len(entry.Record.SharedHosts) != 0 {
		t.Errorf("the oldest object: %+v", entry)
	}
	if len(entry.ThroughService) != 0 {
		t.Errorf("the oldest object is chosen by the host and sends the name of its Service for %v", entry.ThroughService)
	}

	entry = first.entries[sharer.Key()]
	if entry.Record == nil || entry.Reason != "" || entry.Message != "" {
		t.Fatalf("the object that shares the host: %+v", entry)
	}
	if !slices.Equal(entry.Record.Hosts, []string{"own.example.com"}) || !slices.Equal(entry.Record.SharedHosts, []string{"app.example.com"}) {
		t.Errorf("the object that shares the host: hosts %v, shared hosts %v", entry.Record.Hosts, entry.Record.SharedHosts)
	}
	if !slices.Equal(entry.Record.Services, []string{"app-dex-authenticator.another.svc.cluster.local"}) {
		t.Errorf("the names of the Service = %v", entry.Record.Services)
	}
	if !slices.Equal(entry.ThroughService, []string{"app.example.com"}) {
		t.Errorf("the object that shares the host sends the name of its Service for %v", entry.ThroughService)
	}
	sharerRecord := *entry.Record

	if entry := first.entries[alone.Key()]; entry.Record == nil || entry.ThroughService != nil {
		t.Errorf("the object nobody shares a host with: %+v", entry)
	}

	// The names of the fields are those of the table of patch 018.
	data, err := first.tables[naming.UserAuthenticatorInstance].data()
	if err != nil {
		t.Fatalf("data: %v", err)
	}
	if want := `"hosts":["own.example.com"],"services":["app-dex-authenticator.another.svc.cluster.local"],"sharedHosts":["app.example.com"],`; !strings.Contains(string(data[TableKey]), want) {
		t.Errorf("%s = %s, want %s in it", TableKey, data[TableKey], want)
	}

	// A third object names the host: it shares it too.
	third := testAuthenticator("fourth", "app", 9, "app.example.com")
	third.Applications[0].IngressClassName = "nginx-third"
	crowded := build(t, next(first, sharer, alone, owner, third))
	if entry := crowded.entries[third.Key()]; entry.Record == nil || entry.Reason != "" || !slices.Equal(entry.Record.SharedHosts, []string{"app.example.com"}) || len(entry.Record.Hosts) != 0 {
		t.Errorf("the third object on the host: %+v", entry)
	}
	if entry := crowded.entries[sharer.Key()]; entry.Record == nil || !recordsEqual(*entry.Record, sharerRecord) {
		t.Errorf("the record of the object that shares the host changed: %+v", entry.Record)
	}

	// The oldest object is deleted and its routes are still there: its record stays, and so does
	// the pair.
	in := next(first, sharer, alone)
	in.retained = map[types.NamespacedName]bool{owner.Key(): true}
	leaving := build(t, in)
	if leaving.tables[naming.UserAuthenticatorInstance].changed {
		t.Error("the table changed while the routes of the deleted object are there")
	}
	if entry := leaving.entries[sharer.Key()]; !slices.Equal(entry.ThroughService, []string{"app.example.com"}) {
		t.Errorf("the object that shares the host of a record that stays: %+v", entry)
	}

	// The routes are gone: the pair is no more. The object that shared the host is chosen by it,
	// with the secrets it had, and its Ingress objects send the host of the application again.
	in = next(leaving, sharer, alone)
	in.now = testNow.Add(time.Hour)
	second := build(t, in)
	entry = second.entries[sharer.Key()]
	if entry.Record == nil || entry.Reason != "" || !slices.Equal(entry.Record.Hosts, []string{"app.example.com", "own.example.com"}) || len(entry.Record.SharedHosts) != 0 {
		t.Fatalf("the object that is left on the host: %+v", entry)
	}
	if entry.ThroughService != nil {
		t.Errorf("the object that is left on the host sends the name of its Service for %v", entry.ThroughService)
	}
	if entry.Record.ClientSecret != sharerRecord.ClientSecret || entry.Record.CookieSecret != sharerRecord.CookieSecret {
		t.Error("the object that is left on the host got new secrets")
	}
	if entry.Generation != in.now.Unix() {
		t.Errorf("generation = %d, want the one of the new table, %d", entry.Generation, in.now.Unix())
	}

	// The other way to break the pair: the object that shared the host names another one.
	renamed := testAuthenticator("another", "app", 5, "moved.example.com", "own.example.com")
	moved := build(t, next(first, renamed, alone, owner))
	if entry := moved.entries[owner.Key()]; entry.Record == nil || entry.ThroughService != nil {
		t.Errorf("the oldest object after the other one left the host: %+v", entry)
	}
	if entry := moved.entries[renamed.Key()]; entry.Record == nil || entry.ThroughService != nil || len(entry.Record.SharedHosts) != 0 {
		t.Errorf("the object that left the host: %+v", entry)
	}
}

// TestBuildTables_SharedHostAcrossInstances: the two objects of a host are served by different
// instances. Each instance has one object on the host, and the pair is still a pair: who is
// chosen by the host is decided for the cluster, not for an instance.
func TestBuildTables_SharedHostAcrossInstances(t *testing.T) {
	t.Parallel()

	user := testAuthenticator("shop", "app", 0, "app.example.com")
	system := testAuthenticator("d8-console", "console", 5, "app.example.com")
	system.Applications[0].IngressClassName = "nginx-internal"

	result := build(t, testInput(system, user))

	entry := result.entries[user.Key()]
	if entry.Record == nil || !slices.Equal(entry.Record.Hosts, []string{"app.example.com"}) || len(entry.ThroughService) != 0 {
		t.Errorf("the older object of a user: %+v", entry)
	}
	entry = result.entries[system.Key()]
	if entry.Record == nil || entry.Reason != "" || entry.Instance != naming.SystemAuthenticatorInstance {
		t.Fatalf("the object of a system namespace: %+v", entry)
	}
	if !slices.Equal(entry.Record.SharedHosts, []string{"app.example.com"}) || len(entry.Record.Hosts) != 0 || !slices.Equal(entry.ThroughService, []string{"app.example.com"}) {
		t.Errorf("the object of a system namespace: %+v", entry)
	}
}

// TestBuildTables_GatewayApplicationDoesNotShareAHost: the request of an HTTPRoute names no
// Service, so an application with a ListenerSet is served only on a host its object is chosen by.
func TestBuildTables_GatewayApplicationDoesNotShareAHost(t *testing.T) {
	t.Parallel()

	owner := testAuthenticator("shop", "app", 0, "app.example.com")

	// One application of the object on the host has an Ingress class, another a ListenerSet.
	mixed := testAuthenticator("another", "mixed", 5, "app.example.com", "own.example.com")
	mixed.Applications = append(mixed.Applications, Application{Domain: "APP.example.com", ListenerSet: ListenerSet{Name: "public"}})
	result := build(t, testInput(mixed, owner))

	entry := result.entries[mixed.Key()]
	if entry.Record == nil || !slices.Equal(entry.Record.Hosts, []string{"own.example.com"}) || len(entry.Record.SharedHosts) != 0 {
		t.Fatalf("the object with a ListenerSet on the host of another: %+v", entry)
	}
	if entry.Reason != ReasonHostConflict || !strings.Contains(entry.Message, "app.example.com") || !strings.Contains(entry.Message, "different classes") {
		t.Errorf("reason %q, message %q", entry.Reason, entry.Message)
	}
	if entry := result.entries[owner.Key()]; entry.Record == nil || entry.ThroughService != nil {
		t.Errorf("the object nobody shares the host with: %+v", entry)
	}

	// An object with a ListenerSet that is chosen by the host shares it with an Ingress of a
	// younger object: its own requests are chosen by the host, as before.
	gateway := throughGateway(testAuthenticator("shop", "gateway", 0, "app.example.com"))
	younger := testAuthenticator("another", "younger", 5, "app.example.com")
	result = build(t, testInput(younger, gateway))
	if entry := result.entries[gateway.Key()]; entry.Record == nil || entry.Reason != "" || !slices.Equal(entry.Record.Hosts, []string{"app.example.com"}) {
		t.Errorf("the object with a ListenerSet that is chosen by the host: %+v", entry)
	}
	if entry := result.entries[younger.Key()]; entry.Record == nil || entry.Reason != "" || !slices.Equal(entry.Record.SharedHosts, []string{"app.example.com"}) {
		t.Errorf("the object that shares the host through an Ingress: %+v", entry)
	}
}

// TestBuildTables_SameIngressClassDoesNotShareAHost: an object that names the host of another
// object with the Ingress class that object has would put its Ingress objects next to the ones of
// the other object in one ingress controller, and take its paths with a sign-out address.
func TestBuildTables_SameIngressClassDoesNotShareAHost(t *testing.T) {
	t.Parallel()

	owner := testAuthenticator("shop", "app", 0, "app.example.com")
	sharer := testAuthenticator("another", "app", 3, "app.example.com")
	sharer.Applications[0].IngressClassName = "nginx-internal"
	for name, intruder := range map[string]Authenticator{
		"the class of the object chosen by the host": testAuthenticator("evil", "app", 5, "app.example.com", "own.example.com"),
		"the class of an object that shares the host": func() Authenticator {
			a := testAuthenticator("evil", "app", 5, "app.example.com", "own.example.com")
			a.Applications[0].IngressClassName = "nginx-internal"
			return a
		}(),
		"one class of two is taken": func() Authenticator {
			a := testAuthenticator("evil", "app", 5, "app.example.com", "own.example.com", "APP.example.com")
			a.Applications[0].IngressClassName = "nginx-free"
			return a
		}(),
	} {
		result := build(t, testInput(intruder, sharer, owner))
		entry := result.entries[intruder.Key()]
		if entry.Record == nil || !slices.Equal(entry.Record.Hosts, []string{"own.example.com"}) || len(entry.Record.SharedHosts) != 0 || entry.Reason != ReasonHostConflict {
			t.Errorf("%s: %+v", name, entry)
		}
		if entry := result.entries[sharer.Key()]; entry.Record == nil || !slices.Equal(entry.Record.SharedHosts, []string{"app.example.com"}) {
			t.Errorf("%s: the object with a class of its own: %+v", name, entry)
		}
	}

	// A younger object that is chosen by the host keeps it: an older object that names the host
	// later with the same class does not share it.
	first := build(t, testInput(owner))
	older := testAuthenticator("evil", "older", -5, "app.example.com")
	second := build(t, next(first, older, owner))
	if entry := second.entries[older.Key()]; entry.Record != nil || entry.Reason != ReasonHostConflict {
		t.Errorf("the older object with the class of the holder: %+v", entry)
	}
}

// TestBuildTables_TwoApplicationsOfOneObjectOnOneDomain: an object that publishes one domain
// through two Ingress classes is one client, and nothing has to tell its requests apart.
func TestBuildTables_TwoApplicationsOfOneObjectOnOneDomain(t *testing.T) {
	t.Parallel()

	a := testAuthenticator("shop", "app", 0, "app.example.com", "app.example.com")
	a.Applications[1].IngressClassName = "nginx-internal"

	entry := build(t, testInput(a)).entries[a.Key()]
	if entry.Record == nil || entry.Reason != "" || !slices.Equal(entry.Record.Hosts, []string{"app.example.com"}) || len(entry.Record.SharedHosts) != 0 {
		t.Errorf("record: %+v", entry)
	}
	if entry.ThroughService != nil {
		t.Errorf("the object sends the name of its Service for %v", entry.ThroughService)
	}
}

// TestBuildTables_ServiceName: the name of the Service in a record is the one the applications
// write in auth-url, in the form the proxy compares the Host header with.
func TestBuildTables_ServiceName(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", 60)
	settings := testSettings
	settings.ClusterDomain = "Cluster.Local"

	for name, want := range map[string]string{
		"app": "app-dex-authenticator.shop.svc.cluster.local",
		long:  serviceName(long).name + ".shop.svc.cluster.local",
	} {
		in := testInput(testAuthenticator("shop", name, 0, "app.example.com"))
		in.settings = settings
		record := build(t, in).entries[keyOf("shop", name)].Record
		if record == nil || !slices.Equal(record.Services, []string{want}) {
			t.Errorf("%s: record %+v, want the Service %s", name, record, want)
		}
	}
	if got := serviceName(long).name; len(got) != 63 {
		t.Fatalf("the name of the Service of a long name is %q", got)
	}
}

// TestBuildTables_KeptRecordIsNotSharedWith: the routes of a record that stays as it is, of an
// object that cannot be read or is gone, have an Ingress class nobody knows. No object begins to
// share its host; one that shares it already goes on.
func TestBuildTables_KeptRecordIsNotSharedWith(t *testing.T) {
	t.Parallel()

	app := testAuthenticator("shop", "app", 0, "app.example.com")
	sharer := testAuthenticator("another", "app", 3, "app.example.com")
	sharer.Applications[0].IngressClassName = "nginx-internal"
	first := build(t, testInput(app, sharer))

	late := testAuthenticator("third", "app", 5, "app.example.com", "own.example.com")
	late.Applications[0].IngressClassName = "nginx-third"
	for name, input := range map[string]func(in *buildInput){
		"an object that cannot be read": func(in *buildInput) {
			in.invalid = map[types.NamespacedName]invalidObject{app.Key(): {uid: app.UID, err: errors.New("application 0 has no domain")}}
		},
		"a gone object whose routes are still there": func(in *buildInput) {
			in.retained = map[types.NamespacedName]bool{app.Key(): true}
		},
	} {
		in := next(first, sharer, late)
		input(&in)
		second := build(t, in)

		entry := second.entries[late.Key()]
		if entry.Record == nil || entry.Reason != ReasonHostConflict || len(entry.Record.SharedHosts) != 0 || !slices.Equal(entry.Record.Hosts, []string{"own.example.com"}) {
			t.Errorf("%s: the object that begins to share its host: %+v", name, entry)
		}
		entry = second.entries[sharer.Key()]
		if entry.Record == nil || entry.Reason != "" || !slices.Equal(entry.Record.SharedHosts, []string{"app.example.com"}) {
			t.Errorf("%s: the object that shares its host already: %+v", name, entry)
		}
	}
}

// TestBuildTables_GenerationAfterAFailedWrite: a record changes and gets a generation, the write
// of the table fails, and the object is changed back before the next pass. The table has the
// record as it is, so the record is loaded by every pod that has the table: it must not wait for
// the generation of the write that never happened.
func TestBuildTables_GenerationAfterAFailedWrite(t *testing.T) {
	t.Parallel()

	app := testAuthenticator("shop", "app", 0, "app.example.com")
	first := build(t, testInput(app))
	tableGeneration := first.tables[naming.UserAuthenticatorInstance].file.Generation

	changed := app
	changed.KeepUsersLoggedInFor = "24h"
	in := next(first, changed)
	in.now = testNow.Add(time.Hour)
	failed := build(t, in)
	if got := failed.entries[app.Key()].Generation; got <= tableGeneration {
		t.Fatalf("the changed record has the generation %d, want one above %d", got, tableGeneration)
	}

	// The table in the cluster is still the first one; the reconciler remembers the failed pass.
	in = next(first, app)
	in.generations = map[types.NamespacedName]int64{app.Key(): failed.entries[app.Key()].Generation}
	in.now = testNow.Add(2 * time.Hour)
	reverted := build(t, in)
	if reverted.tables[naming.UserAuthenticatorInstance].changed {
		t.Fatal("the table changed")
	}
	if got := reverted.entries[app.Key()].Generation; got != tableGeneration {
		t.Errorf("generation = %d, want the one of the table, %d", got, tableGeneration)
	}
}

// TestBuildTables_GenerationIsAboveWhatWasSeen: a table that replaces a lost one gets a generation
// above every generation the reconciler has seen, also when those had run ahead of the clock.
func TestBuildTables_GenerationIsAboveWhatWasSeen(t *testing.T) {
	t.Parallel()

	app := testAuthenticator("shop", "app", 0, "app.example.com")

	in := testInput(app)
	in.floor = testNow.Unix() + 30
	lost := build(t, in)
	if got := lost.tables[naming.UserAuthenticatorInstance].file.Generation; got != testNow.Unix()+31 {
		t.Errorf("generation of the table = %d, want %d", got, testNow.Unix()+31)
	}
	if got := lost.entries[app.Key()].Generation; got != testNow.Unix()+31 {
		t.Errorf("generation of the record = %d", got)
	}

	// A table that did not change keeps its generation, whatever was seen.
	in = next(lost, app)
	in.floor = testNow.Unix() + 100
	if same := build(t, in); same.tables[naming.UserAuthenticatorInstance].changed || same.tables[naming.UserAuthenticatorInstance].file.Generation != testNow.Unix()+31 {
		t.Errorf("a table that did not change: %+v", same.tables[naming.UserAuthenticatorInstance].file.Generation)
	}
}

// TestBuildTables_FullTableKeepsTheRecordOfAnObject: an object that is served and whose record
// grows over the limit of the table is served as before, with its secrets, and keeps no host it
// is not served on.
func TestBuildTables_FullTableKeepsTheRecordOfAnObject(t *testing.T) {
	t.Parallel()

	app := testAuthenticator("shop", "app", 0, "app.example.com")
	other := testAuthenticator("shop", "other", 5, "other.example.com")
	first := build(t, testInput(app, other))
	record := *first.entries[app.Key()].Record
	limit := first.tables[naming.UserAuthenticatorInstance].size + 10

	// The object names one more domain and drops nothing; an object of the other instance, whose
	// table has room, names the same domain with the same Ingress class.
	grown := app
	grown.Applications = append(slices.Clone(app.Applications), Application{Domain: "wanted.example.com", IngressClassName: "nginx"})
	console := testAuthenticator("d8-console", "console", 9, "wanted.example.com")
	in := next(first, grown, other, console)
	in.limit = limit
	in.now = testNow.Add(time.Hour)
	second := build(t, in)

	entry := second.entries[app.Key()]
	if entry.Record == nil || !recordsEqual(*entry.Record, record) {
		t.Fatalf("the object whose record grew: %+v, want the record it had", entry)
	}
	if entry.Reason != ReasonTableFull || !strings.Contains(entry.Message, "served by the record it has") {
		t.Errorf("reason %q, message %q", entry.Reason, entry.Message)
	}
	if entry.Generation != first.entries[app.Key()].Generation {
		t.Errorf("generation = %d, want the one of the record it had", entry.Generation)
	}
	if second.tables[naming.UserAuthenticatorInstance].changed {
		t.Error("the table of the full instance changed")
	}
	if entry := second.entries[other.Key()]; entry.Record == nil || entry.Reason != "" {
		t.Errorf("another object of the full table: %+v", entry)
	}
	// The domain the object is not served on is free for another object.
	if entry := second.entries[console.Key()]; entry.Record == nil || entry.Reason != "" || !slices.Equal(entry.Record.Hosts, []string{"wanted.example.com"}) {
		t.Errorf("the object that names the domain the first one did not get: %+v", entry)
	}

	// The object drops the domain it is served on for a longer one: the record stays, with the
	// domain, and its routes stay as they were made, with an Ingress class nobody knows any more.
	// So no other object begins to share the domain.
	moved := testAuthenticator("shop", "app", 0, "a-much-longer-domain-of-the-application.example.com")
	latecomer := testAuthenticator("d8-console", "second", 9, "app.example.com")
	in = next(first, moved, other, latecomer)
	in.limit = limit
	third := build(t, in)
	if entry := third.entries[app.Key()]; entry.Record == nil || !recordsEqual(*entry.Record, record) || entry.Reason != ReasonTableFull {
		t.Errorf("the object that changed its domain: %+v", entry)
	}
	if entry := third.entries[latecomer.Key()]; entry.Record != nil || entry.Reason != ReasonHostConflict {
		t.Errorf("the object that names the domain of the record that stays: %+v", entry)
	}

	// An object that shares a domain and whose record does not fit goes on sharing it, whatever
	// Ingress class it names now; an object that begins to share the domain is not let in.
	owner := testAuthenticator("shop", "owner", 0, "shared.example.com")
	sharer := testAuthenticator("shop", "sharer", 3, "shared.example.com")
	sharer.Applications[0].IngressClassName = "nginx-internal"
	paired := build(t, testInput(owner, sharer))
	regrown := testAuthenticator("shop", "sharer", 3, "shared.example.com", "one-more-domain-of-the-sharer.example.com")
	regrown.Applications[0].IngressClassName = "nginx-other"
	newcomer := testAuthenticator("d8-console", "newcomer", 9, "shared.example.com")
	newcomer.Applications[0].IngressClassName = "nginx-internal"
	in = next(paired, owner, regrown, newcomer)
	in.limit = paired.tables[naming.UserAuthenticatorInstance].size + 10
	crowded := build(t, in)
	if entry := crowded.entries[sharer.Key()]; entry.Record == nil || entry.Reason != ReasonTableFull || !recordsEqual(*entry.Record, *paired.entries[sharer.Key()].Record) {
		t.Errorf("the object that shares a domain and does not fit: %+v", entry)
	}
	if entry := crowded.entries[newcomer.Key()]; entry.Record != nil || entry.Reason != ReasonHostConflict {
		t.Errorf("the object that begins to share the domain of a record that stays: %+v", entry)
	}

	// A record that cannot stay as it is holds none of its hosts: one of them is a name of the
	// Service of an instance, which a controller that did not know the aliases of the domain of
	// the cluster let into the record.
	wide := testAuthenticator("shop", "wide", 0, "let-in-before.example.com", "wide.example.com")
	before := build(t, testInput(wide))
	for _, record := range before.tables[naming.UserAuthenticatorInstance].file.Clients {
		for i, host := range record.Hosts {
			if host == "let-in-before.example.com" {
				record.Hosts[i] = "dex-authenticator.d8-user-authn.svc.other.local"
			}
		}
	}
	grownWide := testAuthenticator("shop", "wide", 0, "dex-authenticator.d8-user-authn.svc.other.local", "wide.example.com",
		"one-more-domain-of-the-wide-object.example.com", "and-yet-another-domain-of-the-wide-object.example.com")
	heir := testAuthenticator("d8-console", "heir", 9, "wide.example.com")
	in = next(before, grownWide, heir)
	in.limit = before.tables[naming.UserAuthenticatorInstance].size + 10
	renamed := build(t, in)
	if entry := renamed.entries[wide.Key()]; entry.Record != nil || entry.Reason != ReasonTableFull {
		t.Errorf("the object whose record cannot stay: %+v", entry)
	}
	if entry := renamed.entries[heir.Key()]; entry.Record == nil || entry.Reason != "" || !slices.Equal(entry.Record.Hosts, []string{"wide.example.com"}) {
		t.Errorf("the object that names a host of the record that could not stay: %+v", entry)
	}

	// Room appears: the object gets the record it asked for, with the secrets it had.
	in = next(second, grown, other)
	in.now = testNow.Add(2 * time.Hour)
	fourth := build(t, in)
	entry = fourth.entries[app.Key()]
	if entry.Record == nil || entry.Reason != "" || !slices.Equal(entry.Record.Hosts, []string{"app.example.com", "wanted.example.com"}) {
		t.Fatalf("the object once the table has room: %+v", entry)
	}
	if entry.Record.ClientSecret != record.ClientSecret || entry.Record.CookieSecret != record.CookieSecret {
		t.Error("the object got new secrets")
	}

	// A new object that does not fit has no record and keeps no host either.
	fresh := testAuthenticator("shop", "fresh", 7, "fresh.example.com")
	claimant := testAuthenticator("d8-console", "claimant", 9, "fresh.example.com")
	in = next(first, app, other, fresh, claimant)
	in.limit = limit
	fifth := build(t, in)
	if entry := fifth.entries[fresh.Key()]; entry.Record != nil || entry.Reason != ReasonTableFull || !strings.Contains(entry.Message, "not served") {
		t.Errorf("the new object that does not fit: %+v", entry)
	}
	if entry := fifth.entries[claimant.Key()]; entry.Record == nil || entry.Reason != "" {
		t.Errorf("the object that names the domain of the one that does not fit: %+v", entry)
	}
}

func TestBuildTables_RefusesTheNamesOfTheInstanceService(t *testing.T) {
	t.Parallel()

	for _, host := range []string{
		"dex-authenticator",
		"dex-authenticator.d8-user-authn",
		"dex-authenticator.d8-user-authn.svc",
		"dex-authenticator.d8-user-authn.svc.cluster.local",
		"dex-authenticator-system.d8-user-authn.svc.cluster.local",
		// An alias of the domain of the cluster names the same Service.
		"dex-authenticator.d8-user-authn.svc.alias.local",
		"dex-authenticator-system.d8-user-authn.svc.example.com",
	} {
		a := testAuthenticator("shop", "app", 0, host, "app.example.com")
		result := build(t, testInput(a))
		entry := result.entries[a.Key()]
		if entry.Record == nil || !slices.Equal(entry.Record.Hosts, []string{"app.example.com"}) {
			t.Errorf("%s: record %+v, want app.example.com only", host, entry.Record)
		}
		if entry.Reason != ReasonInvalidRecord || !strings.Contains(entry.Message, host) {
			t.Errorf("%s: reason %q, message %q", host, entry.Reason, entry.Message)
		}
	}

	// A name of another domain or of another Service is an ordinary host.
	for _, host := range []string{"dex-authenticator.d8-user-authn.example.com", "dex-authenticator.shop.svc.cluster.local", "dex-authenticator.example.com", "app.dex-authenticator.d8-user-authn.svc.cluster.local"} {
		a := testAuthenticator("shop", "app", 0, host)
		if entry := build(t, testInput(a)).entries[a.Key()]; entry.Record == nil || entry.Reason != "" {
			t.Errorf("%s: %+v", host, entry)
		}
	}
}

func TestBuildTables_RecreatedObjectGetsNewSecrets(t *testing.T) {
	t.Parallel()

	app := testAuthenticator("shop", "app", 0, "app.example.com")
	first := build(t, testInput(app))

	recreated := app
	recreated.UID = "uid-of-the-new-object"
	second := build(t, next(first, recreated))

	record := second.entries[app.Key()].Record
	if record == nil {
		t.Fatal("the new object has no record")
	}
	old := first.entries[app.Key()].Record
	if record.ClientSecret == old.ClientSecret || record.CookieSecret == old.CookieSecret {
		t.Errorf("the new object took the secrets of the deleted one: %+v", record)
	}
	user := second.tables[naming.UserAuthenticatorInstance]
	if !user.changed || user.owners["shop/app"] != recreated.UID {
		t.Errorf("changed %v, owner %q", user.changed, user.owners["shop/app"])
	}
	if got := second.entries[app.Key()].Generation; got != user.file.Generation {
		t.Errorf("generation of the new record = %d, want %d", got, user.file.Generation)
	}
}

func TestBuildTables_GoneObject(t *testing.T) {
	t.Parallel()

	app := testAuthenticator("shop", "app", 0, "app.example.com")
	other := testAuthenticator("shop", "other", 1, "other.example.com")
	first := build(t, testInput(app, other))

	// The routes of the gone object are still there: its record stays as it is and keeps its host.
	claimant := throughGateway(testAuthenticator("another", "claimant", 3, "app.example.com"))
	in := next(first, other, claimant)
	in.retained = map[types.NamespacedName]bool{app.Key(): true}
	second := build(t, in)
	kept, found := second.tables[naming.UserAuthenticatorInstance].record(app.Key())
	if !found || !recordsEqual(kept, *first.entries[app.Key()].Record) {
		t.Fatalf("the record of the gone object: %+v, found %v", kept, found)
	}
	if entry := second.entries[claimant.Key()]; entry.Record != nil || entry.Reason != ReasonHostConflict {
		t.Errorf("the object that claims the host of the kept record: %+v", entry)
	}
	if _, has := second.entries[app.Key()]; has {
		t.Error("the gone object has an entry")
	}

	// The routes are gone too: the record goes, and the host is free.
	third := build(t, next(second, other, claimant))
	user := third.tables[naming.UserAuthenticatorInstance]
	if _, found := user.record(app.Key()); found {
		t.Error("the record of the gone object is still in the table")
	}
	if _, has := user.owners["shop/app"]; has {
		t.Error("the owner of the gone object is still in the table")
	}
	if entry := third.entries[claimant.Key()]; entry.Record == nil || entry.Reason != "" {
		t.Errorf("the object that claims the freed host: %+v", entry)
	}
}

func TestBuildTables_InvalidObjectKeepsItsRecord(t *testing.T) {
	t.Parallel()

	app := testAuthenticator("shop", "app", 0, "app.example.com")
	first := build(t, testInput(app))
	broken := errors.New("application 0 has no domain")

	in := next(first)
	in.invalid = map[types.NamespacedName]invalidObject{
		app.Key():              {uid: app.UID, err: broken},
		keyOf("shop", "fresh"): {uid: "uid-fresh", err: broken},
	}
	second := build(t, in)

	if second.tables[naming.UserAuthenticatorInstance].changed {
		t.Error("the table changed")
	}
	entry := second.entries[app.Key()]
	if entry.Record == nil || entry.Reason != ReasonInvalidRecord || !strings.Contains(entry.Message, "previous record") || entry.Generation != testNow.Unix() {
		t.Errorf("the object that had a record: %+v", entry)
	}
	entry = second.entries[keyOf("shop", "fresh")]
	if entry.Record != nil || entry.Reason != ReasonInvalidRecord || !strings.Contains(entry.Message, broken.Error()) {
		t.Errorf("the object that had no record: %+v", entry)
	}

	// Another object under the name of the one the record was made for does not keep it.
	in = next(first)
	in.invalid = map[types.NamespacedName]invalidObject{app.Key(): {uid: "uid-of-the-new-object", err: broken}}
	third := build(t, in)
	if _, found := third.tables[naming.UserAuthenticatorInstance].record(app.Key()); found {
		t.Error("the record outlived the object it was made for")
	}
}

func TestBuildTables_FullTable(t *testing.T) {
	t.Parallel()

	served := testAuthenticator("shop", "served", 5, "served.example.com")
	first := build(t, testInput(served))
	size := first.tables[naming.UserAuthenticatorInstance].size

	// Two more objects, both older than the served one, and room for one record only.
	older := testAuthenticator("shop", "older", 0, "older.example.com")
	oldest := testAuthenticator("shop", "oldest", -5, "oldest.example.com")
	in := next(first, served, older, oldest)
	in.limit = size + recordSize(Record{
		Name: "oldest", Namespace: "shop", Hosts: []string{"oldest.example.com"},
		Services: []string{"oldest-dex-authenticator.shop.svc.cluster.local"}, ClientID: "dex-authenticator-oldest@shop",
		ClientSecret: "client-secret-000000", CookieSecret: "cookie-secret-000000", CookieExpire: "168h", ProviderLogout: true,
	}, oldest.UID)
	second := build(t, in)

	if entry := second.entries[served.Key()]; entry.Record == nil || entry.Reason != "" {
		t.Errorf("the object that was served lost its record: %+v", entry)
	}
	if entry := second.entries[oldest.Key()]; entry.Record == nil {
		t.Errorf("the oldest new object: %+v", entry)
	}
	entry := second.entries[older.Key()]
	if entry.Record != nil || entry.Reason != ReasonTableFull || !strings.Contains(entry.Message, naming.UserAuthenticatorInstance) {
		t.Errorf("the object that did not fit: %+v", entry)
	}

	// A record that grew does not push out the record of another object, even of a younger one.
	grown := oldest
	grown.Applications = append(grown.Applications, Application{Domain: "one-more-domain-of-the-oldest-object.example.com", IngressClassName: "nginx"})
	in = next(second, served, grown)
	in.limit = second.tables[naming.UserAuthenticatorInstance].size + 20
	crowded := build(t, in)
	if entry := crowded.entries[served.Key()]; entry.Record == nil || entry.Reason != "" {
		t.Errorf("a record that grew pushed out the record of another object: %+v", entry)
	}
	// The object whose record does not fit any more stays on the record it has.
	entry = crowded.entries[oldest.Key()]
	if entry.Record == nil || !recordsEqual(*entry.Record, *second.entries[oldest.Key()].Record) {
		t.Fatalf("the object whose record does not fit any more: %+v", entry)
	}
	if entry.Reason != ReasonTableFull || !strings.Contains(entry.Message, "served by the record it has") {
		t.Errorf("reason %q, message %q", entry.Reason, entry.Message)
	}
	if crowded.tables[naming.UserAuthenticatorInstance].changed {
		t.Error("the table changed")
	}

	// The other instance has a table of its own: a full table of one leaves room in the other.
	console := testAuthenticator("d8-console", "console", 9, "console.example.com")
	in = next(first, served, older, oldest, console)
	in.limit = size + 40
	third := build(t, in)
	if entry := third.entries[console.Key()]; entry.Record == nil {
		t.Errorf("the object of the other instance: %+v", entry)
	}
	if third.entries[older.Key()].Record != nil || third.entries[oldest.Key()].Record != nil {
		t.Error("an object got into the full table")
	}
}

// TestRecordSize: the size the builder counts is not below what the keys of the Secret take.
func TestRecordSize(t *testing.T) {
	t.Parallel()

	var objects []Authenticator
	for i := range 50 {
		a := testAuthenticator(fmt.Sprintf("namespace-%d", i), fmt.Sprintf("app-%d", i), i, fmt.Sprintf("app-%d.example.com", i), fmt.Sprintf("admin-%d.example.com", i))
		a.SendAuthorizationHeader = i%2 == 0
		a.AllowAccessToKubernetes = i%3 == 0
		objects = append(objects, a)
	}
	built := build(t, testInput(objects...)).tables[naming.UserAuthenticatorInstance]
	data, err := built.data()
	if err != nil {
		t.Fatalf("data: %v", err)
	}
	if actual := len(data[TableKey]) + len(data[OwnersKey]); built.size < actual || built.size > actual+100 {
		t.Errorf("counted %d bytes, the keys take %d", built.size, actual)
	}
}

func TestNewSecrets(t *testing.T) {
	t.Parallel()

	clientSecret, cookieSecret, err := newSecrets()
	if err != nil {
		t.Fatalf("newSecrets: %v", err)
	}
	// The proxy takes a cookie secret of 16, 24 or 32 bytes.
	if len(clientSecret) != 20 || len(cookieSecret) != 24 {
		t.Errorf("lengths %d and %d, want 20 and 24", len(clientSecret), len(cookieSecret))
	}
	again, _, _ := newSecrets()
	if again == clientSecret || clientSecret == cookieSecret[:20] {
		t.Error("the secrets repeat")
	}
}

func TestSettings_CookieExpire(t *testing.T) {
	t.Parallel()

	tests := []struct{ ttl, keep, want string }{
		{"", "168h", "168h"},
		{"10m", "2h30m", "2h30m"},
		// A session cannot be shorter than the ID token: it is refreshed once the token has expired.
		{"10m", "10m", "10m1s"},
		{"30m", "5m", "30m1s"},
		{"", "1s", "10m1s"},
	}
	for _, tt := range tests {
		if got := (Settings{IDTokenTTL: tt.ttl}).cookieExpire(tt.keep); got != tt.want {
			t.Errorf("idTokenTTL %q, keepUsersLoggedInFor %q: %q, want %q", tt.ttl, tt.keep, got, tt.want)
		}
	}
}

// TestBuildTables_NamespaceShare: one namespace may take NamespaceShare of the table of an
// instance. Without the share whoever may create DexAuthenticator objects in one namespace could
// fill the table with records of long made-up domains and leave the objects of every other
// namespace unserved.
func TestBuildTables_NamespaceShare(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", 60) + "." + strings.Repeat("b", 60) + "." + strings.Repeat("c", 60)
	inflated := func(namespace string, count int) []Authenticator {
		out := make([]Authenticator, 0, count)
		for i := range count {
			domains := make([]string, 0, 20)
			for j := range 20 {
				domains = append(domains, fmt.Sprintf("h%d-%d.%s.example.com", i, j, long))
			}
			out = append(out, testAuthenticator(namespace, fmt.Sprintf("big-%d", i), i, domains...))
		}
		return out
	}

	t.Run("a namespace that fills its share leaves the table to the others", func(t *testing.T) {
		t.Parallel()

		// The case of the review: 270 inflated objects of one namespace, then an object of another.
		objects := inflated("greedy", 270)
		victim := testAuthenticator("victim", "app", 1000, "app.example.com")
		result := build(t, testInput(append(objects, victim)...))

		if entry := result.entries[victim.Key()]; entry.Record == nil || entry.Reason != "" {
			t.Errorf("the object of another namespace: %+v", entry)
		}
		var served, overShare int
		for _, a := range objects {
			entry := result.entries[a.Key()]
			switch {
			case entry.Record != nil:
				served++
			case entry.Reason == ReasonTableFull && entry.OverNamespaceShare:
				overShare++
			default:
				t.Errorf("%s: %+v", a.Key(), entry)
			}
		}
		if served == 0 || overShare == 0 || served+overShare != len(objects) {
			t.Errorf("served %d and refused for the share %d of %d objects", served, overShare, len(objects))
		}
		var size int
		for _, record := range result.tables[naming.UserAuthenticatorInstance].file.Clients {
			if record.Namespace == "greedy" {
				size += recordSize(record, result.entries[record.key()].UID)
			}
		}
		if size > NamespaceShare {
			t.Errorf("the records of the namespace take %d bytes, more than its share %d", size, NamespaceShare)
		}

		// The message tells the share and what the namespace takes.
		refused := result.entries[objects[len(objects)-1].Key()]
		for _, part := range []string{"greedy", strconv.Itoa(size), strconv.Itoa(NamespaceShare), naming.UserAuthenticatorInstance, "not served"} {
			if !strings.Contains(refused.Message, part) {
				t.Errorf("the message %q does not say %q", refused.Message, part)
			}
		}
	})

	t.Run("600 ordinary objects of one namespace fit", func(t *testing.T) {
		t.Parallel()

		objects := make([]Authenticator, 0, 600)
		for i := range 600 {
			objects = append(objects, testAuthenticator("team-namespace", fmt.Sprintf("application-%d", i), i, fmt.Sprintf("application-%d.team-namespace.kube.example.com", i)))
		}
		result := build(t, testInput(objects...))
		for _, a := range objects {
			if entry := result.entries[a.Key()]; entry.Record == nil || entry.Reason != "" {
				t.Fatalf("%s: %+v", a.Key(), entry)
			}
		}
	})

	t.Run("a namespace over its share keeps what it has", func(t *testing.T) {
		t.Parallel()

		// The records were made before the share: the namespace takes more than it may now.
		objects := inflated("greedy", 120)
		in := testInput(objects...)
		in.namespaceShare = TableLimit
		before := build(t, in)
		for _, a := range objects {
			if entry := before.entries[a.Key()]; entry.Record == nil {
				t.Fatalf("before the share %s: %+v", a.Key(), entry)
			}
		}

		// With the share: every record the table has stays; a new object of the namespace and a
		// record that grew get none, and the object that grew stays on the record it has.
		grown := objects[0]
		grown.Applications = append(slices.Clone(grown.Applications), Application{Domain: "one-more.example.com", IngressClassName: "nginx"})
		newcomer := testAuthenticator("greedy", "new", 500, "new.example.com")
		other := testAuthenticator("other", "app", 501, "other.example.com")
		after := build(t, next(before, append([]Authenticator{grown, newcomer, other}, objects[1:]...)...))

		for _, a := range objects[1:] {
			if entry := after.entries[a.Key()]; entry.Record == nil || entry.Reason != "" {
				t.Errorf("the record of %s was taken out: %+v", a.Key(), entry)
			}
		}
		entry := after.entries[grown.Key()]
		if entry.Record == nil || entry.Reason != ReasonTableFull || !entry.OverNamespaceShare || !recordsEqual(*entry.Record, *before.entries[grown.Key()].Record) {
			t.Errorf("the object that grew: %+v", entry)
		}
		// The messages tell what the whole namespace takes, the record of the object that grew too,
		// although that record goes into the table before the others of the namespace.
		var size int
		for _, record := range after.tables[naming.UserAuthenticatorInstance].file.Clients {
			if record.Namespace == "greedy" {
				size += recordSize(record, after.entries[record.key()].UID)
			}
		}
		taken := fmt.Sprintf("take %d of the %d bytes", size, NamespaceShare)
		if !strings.Contains(entry.Message, "served by the record it has") || !strings.Contains(entry.Message, taken) {
			t.Errorf("the message of the object that grew: %q, want one that says %q", entry.Message, taken)
		}
		if entry := after.entries[newcomer.Key()]; entry.Record != nil || entry.Reason != ReasonTableFull || !entry.OverNamespaceShare ||
			!strings.Contains(entry.Message, taken) {
			t.Errorf("the new object of the namespace: %+v, want a message that says %q", entry, taken)
		}
		if entry := after.entries[other.Key()]; entry.Record == nil || entry.Reason != "" {
			t.Errorf("the object of another namespace: %+v", entry)
		}
	})

	t.Run("the share is per instance", func(t *testing.T) {
		t.Parallel()

		// The objects of a d8-* namespace and of a namespace of a user are in different tables.
		result := build(t, testInput(append(inflated("greedy", 200), testAuthenticator("d8-console", "console", 1000, "console.example.com"))...))
		if entry := result.entries[keyOf("d8-console", "console")]; entry.Record == nil {
			t.Errorf("the object of the system instance: %+v", entry)
		}
	})
}

// TestBuildTables_RoutedHostWithoutATable: the table is built anew after its Secret was lost. An
// older object of another namespace that named the host of a served object ahead does not get it:
// the served object keeps the host its routes lead to it by, whatever the age. Without the routes
// age decides, as for objects nobody serves yet.
func TestBuildTables_RoutedHostWithoutATable(t *testing.T) {
	t.Parallel()

	for _, sharing := range []bool{false, true} {
		name := "the same Ingress class"
		if sharing {
			name = "another Ingress class"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			evil := testAuthenticator("evil", "old", 1, "e.example.com", "d.example.com")
			if sharing {
				evil.Applications[1].IngressClassName = "nginx-internal"
			}
			victim := testAuthenticator("shop", "app", 100, "d.example.com")

			// Lost table: no record of either object. The routes of the victim lead to it by
			// d.example.com, those of evil/old by e.example.com: the Ingress of a shared host
			// sends the name of the Service and does not count.
			in := testInput(evil, victim)
			in.routed = map[types.NamespacedName][]string{
				victim.Key(): {"d.example.com"},
				evil.Key():   {"e.example.com"},
			}
			result := build(t, in)

			entry := result.entries[victim.Key()]
			if entry.Record == nil || !slices.Equal(entry.Record.Hosts, []string{"d.example.com"}) || entry.Reason != "" {
				t.Errorf("the served object: %+v", entry)
			}
			entry = result.entries[evil.Key()]
			if entry.Record == nil || !slices.Equal(entry.Record.Hosts, []string{"e.example.com"}) {
				t.Fatalf("the older object of another namespace: %+v", entry)
			}
			if sharing {
				if !slices.Equal(entry.Record.SharedHosts, []string{"d.example.com"}) || entry.Reason != "" {
					t.Errorf("the older object shares the host through another class: %+v", entry)
				}
			} else if entry.Reason != ReasonHostConflict {
				t.Errorf("the older object through the same class: %+v", entry)
			}

			// Without routes the oldest object gets the host.
			in.routed = nil
			if entry := build(t, in).entries[evil.Key()]; entry.Record == nil || !slices.Contains(entry.Record.Hosts, "d.example.com") {
				t.Errorf("without routes, the older object: %+v", entry)
			}
		})
	}
}
