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
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/migration"
	"user-authn-controller/internal/naming"
)

const (
	// TableKey is the key of the table Secret the instance reads (--client-table of the proxy),
	// OwnersKey the one the controller keeps the UID of the object of every record under.
	TableKey  = "clients.json"
	OwnersKey = "owners.json"

	// TableLimit is how many bytes the two keys of a table Secret may take. The API server refuses
	// a Secret of more than 1 MiB of data; the rest is a reserve for a record that grows.
	TableLimit = 900 * 1024

	// NamespaceShare is how many bytes of the table of an instance the records of the objects of
	// one namespace may take: 30% of TableLimit, about 270 KiB. The table is one for all the
	// namespaces of users, and a domain is not checked against anything, so without a share whoever
	// may create DexAuthenticator objects in one namespace could fill the table with records of
	// long made-up domains and leave the objects of every other namespace unserved. A record with
	// one domain takes 350 to 450 bytes, by the length of the names: 600 to 780 such objects of one
	// namespace fit.
	NamespaceShare = TableLimit * 3 / 10

	clientSecretLength = 20
	cookieSecretLength = 24

	ReasonServed             = "Served"
	ReasonHostConflict       = "HostConflict"
	ReasonInvalidRecord      = "InvalidRecord"
	ReasonClientNotCreated   = "ClientNotCreated"
	ReasonTableFull          = "TableFull"
	ReasonWaitingForInstance = "WaitingForInstance"
	ReasonForeignObject      = "ForeignObject"
	// ReasonObjectsNotWritten: the API server refuses the Service or a route of the object, by
	// an admission policy or by a right the controller lacks.
	ReasonObjectsNotWritten = "ObjectsNotWritten"
)

// TableSecretName is the Secret with the table of the clients of an instance, in the namespace of
// the module. The pods of the instance mount it.
func TableSecretName(instance string) string {
	return instance + "-clients"
}

// Record is one client of a table: a DexAuthenticator object. The fields and their names are
// those of the table of the dex-authenticator proxy (patch 018 of the image), which refuses a
// record with a field it does not know.
type Record struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	// Hosts are the hosts the instance chooses the object by: a host has one such object in the
	// cluster.
	Hosts []string `json:"hosts"`
	// Services are the names of the Service of the object: the instance gives a request that came
	// through the Service, with such a name in its Host header, to the object whatever host of
	// the object the request is for.
	Services []string `json:"services,omitempty"`
	// SharedHosts are the hosts of the object another object is chosen by: the object is served
	// on them through its Service only.
	SharedHosts             []string `json:"sharedHosts,omitempty"`
	ClientID                string   `json:"clientID"`
	ClientSecret            string   `json:"clientSecret"`
	CookieSecret            string   `json:"cookieSecret"`
	CookieExpire            string   `json:"cookieExpire,omitempty"`
	AllowAccessToKubernetes bool     `json:"allowAccessToKubernetes,omitempty"`
	SendAuthorizationHeader bool     `json:"sendAuthorizationHeader,omitempty"`
	ProviderLogout          bool     `json:"providerLogout,omitempty"`
}

func (r Record) key() types.NamespacedName {
	return types.NamespacedName{Namespace: r.Namespace, Name: r.Name}
}

// applications are the hosts the object is served on: the ones it is chosen by and the shared ones.
func (r Record) applications() []string {
	return slices.Concat(r.Hosts, r.SharedHosts)
}

// tableFile is the content of TableKey.
type tableFile struct {
	// Generation grows with every version of the table. An instance reports the generation it
	// serves, which is how the controller knows a record has been loaded.
	Generation int64    `json:"generation"`
	Clients    []Record `json:"clients"`
}

// table is a table Secret as it was read: the file and the owners of its records.
type table struct {
	file   tableFile
	owners map[string]types.UID
}

func (t table) record(key types.NamespacedName) (Record, bool) {
	for _, r := range t.file.Clients {
		if r.key() == key {
			return r, true
		}
	}
	return Record{}, false
}

// parseTable reads the two keys of a table Secret. A key that is missing gives an empty table.
func parseTable(data map[string][]byte) (table, error) {
	t := table{owners: map[string]types.UID{}}
	if raw := data[TableKey]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &t.file); err != nil {
			return table{}, fmt.Errorf("%s: %w", TableKey, err)
		}
	}
	if raw := data[OwnersKey]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &t.owners); err != nil {
			return table{}, fmt.Errorf("%s: %w", OwnersKey, err)
		}
	}
	return t, nil
}

// data is the content of the table Secret.
func (t table) data() (map[string][]byte, error) {
	file := t.file
	if file.Clients == nil {
		// The proxy refuses a file without the list.
		file.Clients = []Record{}
	}
	clients, err := json.Marshal(file)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", TableKey, err)
	}
	owners, err := json.Marshal(t.owners)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", OwnersKey, err)
	}
	return map[string][]byte{TableKey: clients, OwnersKey: owners}, nil
}

// Entry is what the table reconciler decided about one DexAuthenticator. The reconciler of the
// objects of a DexAuthenticator acts on it.
type Entry struct {
	UID types.UID
	// ObjectGeneration is the generation of the object the decision was made for. The reconciler of
	// the objects acts on the decision only for that generation: the condition it writes tells that
	// generation as observed, and a decision about an older one would report a spec the instance
	// does not serve yet as served.
	ObjectGeneration int64
	Instance         string
	// Record is the record of the object in the table of its instance; nil when it has none, and
	// then Reason tells why. Its hosts are the only ones routes are made for.
	Record *Record
	// ThroughService are the hosts of the record the object shares (SharedHosts). The Ingress
	// objects of such a host send the name of the Service of the object as the Host header, so
	// that the instance tells the requests of the two objects apart.
	ThroughService []string
	// Generation is the generation of the table the record got its present content in: the object
	// is served once every pod of the instance has loaded that generation or a later one.
	Generation int64
	// Reason and Message tell what keeps the object from being served in full.
	Reason  string
	Message string
	// OverNamespaceShare tells that the reason TableFull comes from the share of the namespace of
	// the object (NamespaceShare), not from the table itself.
	OverNamespaceShare bool
}

// buildInput is what the tables of the instances are built from.
type buildInput struct {
	// authenticators are the objects that could be decoded; invalid maps the others to the error.
	authenticators []Authenticator
	invalid        map[types.NamespacedName]invalidObject
	// previous are the tables as they are in the cluster, by instance.
	previous map[string]table
	// retained are the records of previous that stay as they are though no object of authenticators
	// has their name: the object is gone but its routes are not yet (a record goes after its
	// routes, so that no route leads to an instance that does not know the host), or the object is
	// missing from the cache only.
	retained map[types.NamespacedName]bool
	// routed are the hosts the routes of an object lead to it by the host, for the objects previous
	// has no record of: the hosts of its HTTPRoute objects and of those of its Ingress objects that
	// do not send the name of its Service. The routes are made by the controller and carry the
	// heritage label, which no user may set, so they tell who is served on a host when the table
	// cannot: after its Secret was lost or could not be read.
	routed map[types.NamespacedName][]string
	// imported are the secrets of the objects of the d8-* namespaces that the dex-authenticator
	// of the object, which the module chart used to render, kept in the namespace of the object.
	// An object that has no record yet takes them, so that its users stay signed in.
	imported map[types.NamespacedName]migration.DexAuthenticatorSecrets
	// generations are the generations the reconciler remembers for the records it wrote.
	generations map[types.NamespacedName]int64
	// floor is the highest generation the reconciler has read or given since its start: a new
	// table gets one above it, also when the table it replaces is lost or cannot be read.
	floor    int64
	settings Settings
	now      time.Time
	limit    int
	// namespaceShare is how many bytes the records of one namespace may take in a table.
	namespaceShare int
	newSecrets     func() (clientSecret, cookieSecret string, err error)

	// onRecord are the objects whose record did not fit into the table of their instance: they
	// stay on the record they have, if they have one, and claim no host beyond it. buildTables
	// fills it.
	onRecord map[types.NamespacedName]struct{}
	// overShare are the objects of onRecord that went there because of the share of their
	// namespace.
	overShare map[types.NamespacedName]struct{}
}

// invalidObject is an object the reconciler could not decode.
type invalidObject struct {
	uid        types.UID
	generation int64
	err        error
}

// builtTable is the next table of an instance.
type builtTable struct {
	table
	// changed tells that the table differs from the previous one and has a new generation.
	changed bool
	// size is how many bytes the two keys take.
	size int
}

type buildResult struct {
	tables  map[string]builtTable
	entries map[types.NamespacedName]Entry
}

// buildTables decides which hosts every object gets and builds the table of every instance.
//
// An instance chooses the object of a request by the host of the request, so a host belongs to
// one object in the cluster. The records that stay for a gone object keep their hosts, the objects
// keep the hosts they are chosen by, and the hosts nobody is chosen by go to the objects that name
// them from the oldest to the youngest. An object the previous table has no record of keeps the
// hosts its routes lead to it by (routed) before age decides: without that, a table that is lost
// would give the host of a served object to an older one of another namespace that named it.
//
// An object that names a host of another object is served on it all the same when every request
// for it comes through the Service of the object, which the instance then chooses the object by:
// the auth subrequest of ingress-nginx does, and an Ingress of the object does once it sends the
// name of the Service as the Host header (ThroughService of the entries). That is how two objects
// publish one domain through two Ingress classes. The request of an HTTPRoute carries the host of
// the application only, so an application with a ListenerSet does not share a host: the object
// that names a taken host for it is not served on that host. Nor is an object whose Ingress class
// on the host is one an object served on the host has already: its Ingress objects would take the
// paths of the other object in the same ingress controller. The admission webhook refuses such an
// object, and the controller does not rely on it.
//
// The records of the objects of one namespace take at most NamespaceShare of the table: a new
// record or a record that grew does not get into the table when it would take its namespace over
// the share, and is treated as a record that does not fit. A record the table has and that did
// not grow stays whatever its namespace takes.
//
// An object whose record does not fit into the table of its instance (TableLimit) keeps the
// record it has, with its hosts and its secrets, and is served by it: its objects stay as they
// were made for that record. An object without a record is not served. Neither keeps a host it is
// not served on: the tables are built again without the claims of such objects, until every
// record that is asked for fits.
//
// A record keeps its secrets
// while the object it was made for lives; an object created again under the name of a deleted one
// has another UID and gets new secrets, so no session of the old object comes back to it.
func buildTables(in buildInput) (buildResult, error) {
	in.onRecord = make(map[types.NamespacedName]struct{})
	in.overShare = make(map[types.NamespacedName]struct{})
	for {
		result, full, err := buildTablesOnce(in)
		if err != nil || len(full) == 0 {
			return result, err
		}
		// Every pass puts at least one more object on its record, so the passes end.
		for _, f := range full {
			in.onRecord[f.key] = struct{}{}
			if f.overShare {
				in.overShare[f.key] = struct{}{}
			}
		}
	}
}

// fullRecord is an object whose record did not fit, and whether it was the share of its
// namespace that it did not fit into.
type fullRecord struct {
	key       types.NamespacedName
	overShare bool
}

// buildTablesOnce builds the tables with the objects of in.onRecord on the records they have, and
// returns the other objects whose record did not fit.
func buildTablesOnce(in buildInput) (buildResult, []fullRecord, error) {
	result := buildResult{
		tables:  make(map[string]builtTable, len(in.previous)),
		entries: make(map[types.NamespacedName]Entry, len(in.authenticators)+len(in.invalid)),
	}

	next := make(map[string]*table)
	sizes := make(map[string]int)
	for _, instance := range naming.AuthenticatorInstances() {
		next[instance] = &table{owners: map[string]types.UID{}}
		sizes[instance] = emptyTableSize
	}
	// namespaceSizes are the bytes the records of a namespace take in the table of an instance.
	type namespaceOf struct{ instance, namespace string }
	namespaceSizes := make(map[namespaceOf]int)
	add := func(instance string, record Record, uid types.UID) {
		t := next[instance]
		t.file.Clients = append(t.file.Clients, record)
		if uid != "" {
			t.owners[record.key().String()] = uid
		}
		size := recordSize(record, uid)
		sizes[instance] += size
		namespaceSizes[namespaceOf{instance, record.Namespace}] += size
	}

	taken := make(map[string]struct{})
	// kept are the hosts of the records that stay as they are. The Ingress class of their routes
	// is not known, so no object begins to share such a host.
	kept := make(map[string]struct{})

	// The records that stay as they are: of a gone object whose routes are still there, and of an
	// object that cannot be read any more.
	for _, instance := range naming.AuthenticatorInstances() {
		previous := in.previous[instance]
		for _, record := range previous.file.Clients {
			key := record.key()
			uid := previous.owners[key.String()]
			broken, isInvalid := in.invalid[key]
			keep := in.retained[key] || (isInvalid && broken.uid == uid)
			if !keep {
				continue
			}
			add(instance, record, uid)
			for _, host := range record.Hosts {
				taken[host] = struct{}{}
			}
			for _, host := range record.applications() {
				kept[host] = struct{}{}
			}
			if isInvalid {
				kept := record
				result.entries[key] = Entry{
					UID: uid, ObjectGeneration: broken.generation, Instance: instance, Record: &kept,
					Generation: in.generation(key, previous),
					Reason:     ReasonInvalidRecord,
					Message:    "The object cannot be read, it is served by its previous record: " + broken.err.Error(),
				}
			}
		}
	}
	for key, broken := range in.invalid {
		if _, kept := result.entries[key]; !kept {
			result.entries[key] = Entry{
				UID: broken.uid, ObjectGeneration: broken.generation, Instance: naming.AuthenticatorInstance(key.Namespace),
				Reason: ReasonInvalidRecord, Message: "The object cannot be read: " + broken.err.Error(),
			}
		}
	}

	ordered := slices.Clone(in.authenticators)
	slices.SortFunc(ordered, func(a, b Authenticator) int {
		return cmp.Or(a.Created.Compare(b.Created), cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Name, b.Name))
	})

	// previousOf returns the record the table has for the object a, unless it was made for another
	// object of that name.
	previousOf := func(a Authenticator) (Record, bool) {
		previous := in.previous[a.Instance()]
		old, found := previous.record(a.Key())
		return old, found && previous.owners[a.Key().String()] == a.UID
	}

	// The hosts an object is served on stay with it whatever its age: the instance picks the client
	// of a request by its host, so a host that passed to an older object which named it later
	// would put the application of one namespace behind the client, the groups and the cookies of
	// another. Age decides between the claims to a host nobody is served on yet.
	held := make(map[string]types.NamespacedName)
	// classes are the Ingress classes of the objects served on a host so far.
	classes := make(map[string][]string)
	for _, a := range ordered {
		old, found := previousOf(a)
		if !found {
			continue
		}
		named := make([]string, 0, len(a.Applications))
		for _, app := range a.Applications {
			named = append(named, strings.ToLower(app.Domain))
		}
		if _, isOnRecord := in.onRecord[a.Key()]; isOnRecord {
			// The hosts of the record it stays on, whatever the object names now: all of them, or
			// the record does not stay and holds nothing.
			named = old.Hosts
			if slices.ContainsFunc(old.Hosts, func(host string) bool {
				_, isTaken := taken[host]
				return isTaken || in.settings.isInstanceHost(host)
			}) {
				continue
			}
			// The routes of the record were made for the object as it was: their Ingress classes
			// are not known any more, so no object begins to share a host of the record.
			for _, host := range old.applications() {
				kept[host] = struct{}{}
			}
		}
		for _, host := range named {
			if _, isTaken := taken[host]; isTaken || !slices.Contains(old.Hosts, host) || in.settings.isInstanceHost(host) {
				continue
			}
			taken[host] = struct{}{}
			held[host] = a.Key()
			classes[host] = a.ingressClasses(host)
		}
	}
	// An object without a record keeps the hosts its routes lead to it by, after the records and
	// before age: its record is lost with the table, its routes are not. The Created time of an
	// object does not change with its spec, so an older object of another namespace could name the
	// host of a served one ahead and get it from a table built anew.
	for _, a := range ordered {
		if _, found := previousOf(a); found {
			continue
		}
		routed := in.routed[a.Key()]
		for _, app := range a.Applications {
			host := strings.ToLower(app.Domain)
			if _, isTaken := taken[host]; isTaken || !slices.Contains(routed, host) || in.settings.isInstanceHost(host) {
				continue
			}
			taken[host] = struct{}{}
			held[host] = a.Key()
			classes[host] = a.ingressClasses(host)
		}
	}

	type candidate struct {
		authenticator Authenticator
		record        Record
		hasPrevious   bool
		// grew tells that the record takes more room than the one the object has in the table.
		grew bool
		// onRecord tells that the record of the object did not fit: record is the one the table
		// has, or an empty one.
		onRecord      bool
		lost, refused []string
	}
	candidates := make([]candidate, 0, len(ordered))
	for _, a := range ordered {
		c := candidate{authenticator: a}
		var hosts, shared []string
		old, found := previousOf(a)
		if _, isOnRecord := in.onRecord[a.Key()]; isOnRecord {
			c.onRecord = true
			// The record stays only as it is: with every host the object is chosen by.
			c.hasPrevious = found && !slices.ContainsFunc(old.Hosts, func(host string) bool { return held[host] != a.Key() })
			if c.hasPrevious {
				c.record = old
			}
			candidates = append(candidates, c)
			continue
		}
		for _, app := range a.Applications {
			host := strings.ToLower(app.Domain)
			switch {
			case slices.Contains(hosts, host), slices.Contains(shared, host), slices.Contains(c.lost, host):
			case in.settings.isInstanceHost(host):
				c.refused = append(c.refused, host)
			case held[host] == a.Key():
				hosts = append(hosts, host)
			default:
				own := a.ingressClasses(host)
				if _, isTaken := taken[host]; !isTaken {
					taken[host] = struct{}{}
					hosts = append(hosts, host)
					classes[host] = append(classes[host], own...)
					continue
				}
				// An object that shares the host already goes on sharing it with a record that stays.
				_, isKept := kept[host]
				sharesAlready := found && slices.Contains(old.SharedHosts, host)
				if (isKept && !sharesAlready) || a.routesByHost(host) || slices.ContainsFunc(own, func(class string) bool { return slices.Contains(classes[host], class) }) {
					c.lost = append(c.lost, host)
					continue
				}
				shared = append(shared, host)
				classes[host] = append(classes[host], own...)
			}
		}

		c.hasPrevious = found
		c.record = Record{
			Name:                    a.Name,
			Namespace:               a.Namespace,
			Hosts:                   hosts,
			Services:                []string{in.settings.serviceHost(a)},
			SharedHosts:             shared,
			ClientID:                a.ClientID(),
			CookieExpire:            in.settings.cookieExpire(a.KeepUsersLoggedInFor),
			AllowAccessToKubernetes: a.AllowAccessToKubernetes,
			SendAuthorizationHeader: a.SendAuthorizationHeader,
			// The client of every object carries the addresses to return to after a sign-out.
			ProviderLogout: true,
		}
		if c.hasPrevious {
			c.record.ClientSecret, c.record.CookieSecret = old.ClientSecret, old.CookieSecret
			c.grew = recordSize(c.record, a.UID) > recordSize(old, a.UID)
		}
		candidates = append(candidates, c)
	}
	// Into a table that is nearly full the records go in this order: those the table has and that
	// did not grow, those it has and that grew, the new ones; the oldest object first in each. So a
	// new object never pushes a served one out, and a record that grew does not push out another.
	rank := func(c candidate) int {
		switch {
		case c.hasPrevious && !c.grew:
			return 0
		case c.hasPrevious:
			return 1
		default:
			return 2
		}
	}
	slices.SortStableFunc(candidates, func(a, b candidate) int { return cmp.Compare(rank(a), rank(b)) })

	// The message about the share of a namespace tells what the namespace takes in the table as it is
	// built, so it is written once every record is in: a record that stays goes in before the other
	// records of its namespace, and what the namespace takes at that moment is the record alone.
	type shareMessage struct {
		key    types.NamespacedName
		of     namespaceOf
		served string
	}
	var shareMessages []shareMessage

	var full []fullRecord
	for _, c := range candidates {
		a := c.authenticator
		key, instance := a.Key(), a.Instance()
		entry := Entry{UID: a.UID, ObjectGeneration: a.Generation, Instance: instance}
		entry.Reason, entry.Message = hostsVerdict(c.record.applications(), c.lost, c.refused)
		notServed := fmt.Sprintf("The table of the %s instance is full: the object is not served.", instance)
		_, overShare := in.overShare[key]
		// overShareServed, when set, ends the message about the share the object gets.
		var overShareServed string
		if c.onRecord {
			entry.Reason, entry.Message, entry.OverNamespaceShare = ReasonTableFull, notServed, overShare
			if overShare {
				overShareServed = "the object is not served."
			}
		}

		if len(c.record.applications()) > 0 {
			if imported, ok := in.imported[key]; ok && !c.hasPrevious {
				c.record.ClientSecret, c.record.CookieSecret = imported.Client, imported.Cookie
			} else if !c.hasPrevious {
				var err error
				if c.record.ClientSecret, c.record.CookieSecret, err = in.newSecrets(); err != nil {
					return buildResult{}, nil, err
				}
			}
			size := recordSize(c.record, a.UID)
			// A record the table has and that did not grow is never taken out for the share, nor
			// is the record an object stays on: the share stops a namespace from taking more,
			// whatever it took before the share was there.
			checksShare := !c.onRecord && (!c.hasPrevious || c.grew)
			switch {
			case sizes[instance]+size > in.limit:
				entry.Reason, entry.Message, overShareServed = ReasonTableFull, notServed, ""
				if !c.onRecord {
					full = append(full, fullRecord{key: key})
				}
			case checksShare && namespaceSizes[namespaceOf{instance, a.Namespace}]+size > in.namespaceShare:
				entry.Reason, entry.OverNamespaceShare, overShareServed = ReasonTableFull, true, "the object is not served."
				full = append(full, fullRecord{key: key, overShare: true})
			default:
				add(instance, c.record, a.UID)
				record := c.record
				entry.Record = &record
				if c.onRecord {
					entry.Message = fmt.Sprintf("The table of the %s instance is full: the object is served by the record it has, "+
						"without the changes made to it since.", instance)
					if overShare {
						overShareServed = "the object is served by the record it has, without the changes made to it since."
					}
				}
			}
		}
		result.entries[key] = entry
		if overShareServed != "" {
			shareMessages = append(shareMessages, shareMessage{key: key, of: namespaceOf{instance, a.Namespace}, served: overShareServed})
		}
	}
	for _, m := range shareMessages {
		entry := result.entries[m.key]
		entry.Message = fmt.Sprintf("The DexAuthenticator objects of the namespace %s take %d of the %d bytes one namespace may have "+
			"in the table of the %s instance: %s", m.of.namespace, namespaceSizes[m.of], in.namespaceShare, m.of.instance, m.served)
		result.entries[m.key] = entry
	}

	// The hosts an object shares: it is chosen for them by its Service only, so its Ingress
	// objects send the name of the Service. The object that is chosen by the host needs nothing
	// of the kind: a request of its route carries the host of the application, and the instance
	// chooses it by that. So an object that begins to share a host changes no object of the one
	// the host belongs to.
	for key, entry := range result.entries {
		if entry.Record == nil {
			continue
		}
		entry.ThroughService = slices.Clone(entry.Record.SharedHosts)
		result.entries[key] = entry
	}

	for _, instance := range naming.AuthenticatorInstances() {
		previous := in.previous[instance]
		t := next[instance]
		slices.SortFunc(t.file.Clients, func(a, b Record) int {
			return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Name, b.Name))
		})

		built := builtTable{table: *t, size: sizes[instance]}
		built.file.Generation = previous.file.Generation
		built.changed = !slices.EqualFunc(t.file.Clients, previous.file.Clients, func(a, b Record) bool { return reflect.DeepEqual(a, b) }) ||
			!maps.Equal(t.owners, previous.owners)
		if built.changed {
			// Not below the clock, nor below what the reconciler has seen: a table written after
			// its Secret was lost still gets a generation above the one the pods have loaded, also
			// when a run of writes within a second had put the generations ahead of the clock.
			built.file.Generation = max(previous.file.Generation+1, in.now.Unix(), in.floor+1)
		}
		result.tables[instance] = built

		for _, record := range built.file.Clients {
			key := record.key()
			entry, ok := result.entries[key]
			if !ok || entry.Record == nil {
				continue
			}
			old, found := previous.record(key)
			if found && reflect.DeepEqual(old, record) {
				entry.Generation = in.generation(key, previous)
			} else {
				entry.Generation = built.file.Generation
			}
			result.entries[key] = entry
		}
	}

	return result, full, nil
}

// generation is the generation the record of key, which the table previous has as it is, got its
// content in: the remembered one, or the generation of the table when the reconciler has only
// just started. A remembered generation above that of the table is one of a write that failed,
// for a content the record no longer has: the table has the record, so its generation serves.
func (in buildInput) generation(key types.NamespacedName, previous table) int64 {
	if generation, ok := in.generations[key]; ok && generation <= previous.file.Generation {
		return generation
	}
	return previous.file.Generation
}

// hostsVerdict tells what is wrong with the hosts of an object, if anything: the reason of its
// condition and the message.
func hostsVerdict(granted, lost, refused []string) (string, string) {
	var (
		reason string
		parts  []string
	)
	if len(refused) > 0 {
		reason = ReasonInvalidRecord
		parts = append(parts, fmt.Sprintf("the domains %s are names of the dex-authenticator Service and cannot be used", strings.Join(refused, ", ")))
	}
	if len(lost) > 0 {
		reason = ReasonHostConflict
		parts = append(parts, fmt.Sprintf("the domains %s belong to another DexAuthenticator, and a domain is shared only through Ingress objects of different classes", strings.Join(lost, ", ")))
	}
	if reason == "" {
		return "", ""
	}
	served := "the object is not served"
	if len(granted) > 0 {
		served = "the object is served on " + strings.Join(granted, ", ")
	}
	message := strings.Join(parts, "; ") + "; " + served + "."
	return reason, strings.ToUpper(message[:1]) + message[1:]
}

// emptyTableSize is the size of the two keys of a table without records.
const emptyTableSize = len(`{"generation":9223372036854775807,"clients":[]}`) + len(`{}`)

// recordSize is how many bytes a record adds to the two keys of a table.
func recordSize(record Record, uid types.UID) int {
	raw, _ := json.Marshal(record)
	// The record and its comma; the key, the UID, their quotes, the colon and the comma.
	return len(raw) + 1 + len(record.key().String()) + len(uid) + 6
}

// newSecrets returns a new client secret and a new cookie secret, of the form the module hook
// generated them in.
func newSecrets() (string, string, error) {
	clientSecret, err := controller.RandomSecret(clientSecretLength)
	if err != nil {
		return "", "", err
	}
	cookieSecret, err := controller.RandomSecret(cookieSecretLength)
	if err != nil {
		return "", "", err
	}
	return clientSecret, cookieSecret, nil
}
