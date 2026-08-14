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

package user

import "slices"

const (
	memberKindUser  = "User"
	memberKindGroup = "Group"
)

// groupsForUser walks Group membership (User and nested Group) and returns the
// sorted unique spec.names the user belongs to. A group is visited at most once.
func groupsForUser(groups []groupView, userName string) []string {
	mapOfUsersToGroups := make(map[string]map[string]bool)
	bySpecName := indexGroupsBySpecName(groups)
	for _, g := range groups {
		makeUserGroupsMap(bySpecName, g.SpecName, []string{}, mapOfUsersToGroups, make(map[string]bool))
	}
	set := mapOfUsersToGroups[userName]
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func usersInGroupSubtree(groups []groupView, specName string) []string {
	if specName == "" {
		return nil
	}
	mapOfUsersToGroups := make(map[string]map[string]bool)
	makeUserGroupsMap(indexGroupsBySpecName(groups), specName, []string{}, mapOfUsersToGroups, make(map[string]bool))
	out := make([]string, 0, len(mapOfUsersToGroups))
	for name := range mapOfUsersToGroups {
		out = append(out, name)
	}
	return out
}

func indexGroupsBySpecName(groups []groupView) map[string]groupView {
	byName := make(map[string]groupView, len(groups))
	for _, g := range groups {
		if _, exists := byName[g.SpecName]; exists {
			continue
		}
		byName[g.SpecName] = g
	}
	return byName
}

func makeUserGroupsMap(
	groupsBySpecName map[string]groupView,
	targetGroup string,
	accumulatedGroupList []string,
	mapOfUsersToGroups map[string]map[string]bool,
	visited map[string]bool,
) {
	if len(groupsBySpecName) == 0 {
		return
	}
	if visited[targetGroup] {
		return
	}
	visited[targetGroup] = true

	group, ok := groupsBySpecName[targetGroup]
	if !ok {
		return
	}

	if !slices.Contains(accumulatedGroupList, targetGroup) {
		accumulatedGroupList = append(accumulatedGroupList, targetGroup)
	}
	for _, member := range group.Members {
		switch member.Kind {
		case memberKindUser:
			if mapOfUsersToGroups[member.Name] == nil {
				mapOfUsersToGroups[member.Name] = map[string]bool{}
			}
			for _, g := range accumulatedGroupList {
				mapOfUsersToGroups[member.Name][g] = true
			}
		case memberKindGroup:
			makeUserGroupsMap(groupsBySpecName, member.Name, accumulatedGroupList, mapOfUsersToGroups, visited)
		}
	}
}

func equalStringSets(a, b []string) bool {
	left := slices.Clone(a)
	right := slices.Clone(b)
	slices.Sort(left)
	slices.Sort(right)
	return slices.Equal(left, right)
}
