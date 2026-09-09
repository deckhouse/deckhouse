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

package binding

import (
	"sync"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/tools/cache"
)

// Index maps every subject to the rules whose ClusterRoleBindings name it. It is fed from a
// ClusterRoleBinding informer and answers in O(1) per subject, so a consumer can ask on every
// request "which rules bind this user" without walking the bindings of the cluster.
//
// Its purpose is the ordering guard: user-authz-controller creates the binding of a rule seconds
// after the rule, and a consumer whose rule cache lags sees the binding first. Knowing the rule
// names bound to a subject, the consumer can treat a rule it has not observed yet as maximally
// restrictive instead of letting the cluster-wide binding grant everything.
type Index struct {
	mu sync.RWMutex
	// bySubject: subject key -> rule name -> number of bindings (a rule has several bindings per
	// subject: the level, the aggregated custom roles, port-forward, scale).
	bySubject map[string]map[string]int
	// byBinding remembers what each binding contributed, so an update or a deletion can be undone
	// without the old object.
	byBinding map[string]contribution
}

type contribution struct {
	rule     string
	subjects []string
}

// NewIndex returns an empty Index.
func NewIndex() *Index {
	return &Index{
		bySubject: make(map[string]map[string]int),
		byBinding: make(map[string]contribution),
	}
}

// Subject keys mirror the ones the rules directory uses: User and Group by name, ServiceAccount by
// the username the API server derives from it.
func subjectKey(s rbacv1.Subject) string {
	switch s.Kind {
	case rbacv1.UserKind:
		return "User/" + s.Name
	case rbacv1.GroupKind:
		return "Group/" + s.Name
	case rbacv1.ServiceAccountKind:
		return "ServiceAccount/system:serviceaccount:" + s.Namespace + ":" + s.Name
	}
	return ""
}

// Upsert records a binding: the rule it belongs to and the subjects it names. A binding that is
// not a rule binding removes whatever an earlier version of it contributed and adds nothing.
func (i *Index) Upsert(crb *rbacv1.ClusterRoleBinding) {
	if crb == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()

	i.removeLocked(crb.Name)

	if !IsRuleBinding(crb.Name, crb.Labels) {
		return
	}
	rule, _ := RuleNameOf(crb.Name)
	c := contribution{rule: rule}
	for _, s := range crb.Subjects {
		key := subjectKey(s)
		if key == "" {
			continue
		}
		c.subjects = append(c.subjects, key)
		rules := i.bySubject[key]
		if rules == nil {
			rules = make(map[string]int)
			i.bySubject[key] = rules
		}
		rules[rule]++
	}
	i.byBinding[crb.Name] = c
}

// Delete forgets a binding.
func (i *Index) Delete(name string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.removeLocked(name)
}

func (i *Index) removeLocked(name string) {
	c, ok := i.byBinding[name]
	if !ok {
		return
	}
	delete(i.byBinding, name)
	for _, key := range c.subjects {
		rules := i.bySubject[key]
		if rules == nil {
			continue
		}
		if rules[c.rule]--; rules[c.rule] <= 0 {
			delete(rules, c.rule)
		}
		if len(rules) == 0 {
			delete(i.bySubject, key)
		}
	}
}

// RulesFor returns the names of the rules bound to the user through any of its identities: the
// username as a User, the username as a ServiceAccount, each group.
func (i *Index) RulesFor(username string, groups []string) []string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	seen := make(map[string]struct{})
	var out []string
	collect := func(key string) {
		for rule := range i.bySubject[key] {
			if _, dup := seen[rule]; dup {
				continue
			}
			seen[rule] = struct{}{}
			out = append(out, rule)
		}
	}
	collect("User/" + username)
	collect("ServiceAccount/" + username)
	for _, g := range groups {
		collect("Group/" + g)
	}
	return out
}

// Len is the number of rule bindings indexed.
func (i *Index) Len() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.byBinding)
}

// EventHandler returns the informer event handler that keeps the index in step with a
// ClusterRoleBinding informer. Register it on the informer before starting the factory.
func (i *Index) EventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if crb, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
				i.Upsert(crb)
			}
		},
		UpdateFunc: func(_, obj interface{}) {
			if crb, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
				i.Upsert(crb)
			}
		},
		DeleteFunc: func(obj interface{}) {
			switch v := obj.(type) {
			case *rbacv1.ClusterRoleBinding:
				i.Delete(v.Name)
			case cache.DeletedFinalStateUnknown:
				if crb, ok := v.Obj.(*rbacv1.ClusterRoleBinding); ok {
					i.Delete(crb.Name)
				} else {
					i.Delete(v.Key)
				}
			}
		},
	}
}
