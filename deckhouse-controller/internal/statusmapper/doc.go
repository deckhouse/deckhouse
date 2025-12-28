// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
Package statusmapper provides a declarative DSL for mapping internal conditions
to external status conditions using composable matchers.

# Overview

The package allows you to define condition mapping rules declaratively:

	spec := statusmapper.Spec{
	    Type:   status.ConditionInstalled,
	    Sticky: true,  // once True, stays True
	    Rule: statusmapper.FirstMatch{
	        {When: statusmapper.IsTrue("Downloaded"), Status: metav1.ConditionTrue},
	        {Status: metav1.ConditionFalse, Reason: "NotInstalled"},  // default
	    },
	}

# Core Types

  - [Spec] defines the complete specification for an external condition
  - [Case] represents a single evaluation case (When → Status/Reason/Message)
  - [FirstMatch] is a slice of Cases evaluated in order; first match wins
  - [Matcher] interface for condition matching logic
  - [Input] contains all data needed for evaluation
  - [Mapper] orchestrates the mapping process

# Built-in Matchers

Basic matchers:

	statusmapper.IsTrue("Downloaded")      // condition == True
	statusmapper.IsFalse("Downloaded")     // condition == False
	statusmapper.NotTrue("Downloaded")     // condition != True (False/Unknown/missing)

Composite matchers (fully nestable):

	statusmapper.And(matcher1, matcher2)   // logical AND
	statusmapper.Or(matcher1, matcher2)    // logical OR
	statusmapper.AllTrue("A", "B", "C")    // all conditions are True

Special matchers:

	statusmapper.Always{}                  // always matches (for defaults)
	statusmapper.Predicate{               // custom logic
	    Name: "custom-check",
	    Fn: func(input *Input) bool { ... },
	}

# Composing Complex Rules

Matchers can be nested arbitrarily:

	// (A AND B) OR (C AND D)
	When: statusmapper.Or(
	    statusmapper.And(
	        statusmapper.IsTrue("A"),
	        statusmapper.IsTrue("B"),
	    ),
	    statusmapper.And(
	        statusmapper.IsTrue("C"),
	        statusmapper.IsTrue("D"),
	    ),
	),

# Complete Example

	func InstalledSpec() statusmapper.Spec {
	    return statusmapper.Spec{
	        Type:   status.ConditionInstalled,
	        Sticky: true,
	        Rule: statusmapper.FirstMatch{
	            // Success: all conditions met
	            {
	                When: statusmapper.AllTrue(
	                    status.ConditionDownloaded,
	                    status.ConditionReadyOnFilesystem,
	                    status.ConditionRequirementsMet,
	                    status.ConditionReadyInRuntime,
	                    status.ConditionHelmApplied,
	                ),
	                Status: metav1.ConditionTrue,
	            },
	            // Failure with message from internal condition
	            {
	                When:        statusmapper.IsFalse(status.ConditionDownloaded),
	                Status:      metav1.ConditionFalse,
	                Reason:      "DownloadFailed",
	                MessageFrom: status.ConditionDownloaded,  // copy message
	            },
	            // Default fallback (must be last)
	            {
	                Status: metav1.ConditionFalse,
	                Reason: "InstallationInProgress",
	            },
	        },
	    }
	}

# Using the Mapper

	specs := []statusmapper.Spec{InstalledSpec(), ReadySpec(), ...}
	mapper := statusmapper.NewMapper(specs)

	input := &statusmapper.Input{
	    InternalConditions: internalCondMap,
	    ExternalConditions: externalCondMap,
	    IsInitialInstall:   true,
	    VersionChanged:     false,
	}
	results := mapper.Map(input)  // []status.Condition

# Extending with Custom Matchers

Implement the Matcher interface:

	type MyMatcher struct {
	    ExpectedReason string
	}

	func (m MyMatcher) Match(input *statusmapper.Input) bool {
	    cond := input.InternalConditions["SomeCondition"]
	    return string(cond.Reason) == m.ExpectedReason
	}

	func (m MyMatcher) String() string {
	    return "MyMatcher(" + m.ExpectedReason + ")"
	}

Then use it in specs:

	{When: MyMatcher{ExpectedReason: "Ready"}, Status: metav1.ConditionTrue}

# Spec Options

  - Type: the external condition name this spec produces
  - Rule: FirstMatch slice of Cases (evaluated in order)
  - Sticky: if true, once condition is True it never reverts to False
  - AppliesWhen: optional Matcher to control when spec is evaluated at all

# Testing

Use [Mapper.DetectDuplicateCases] to find shadowed or duplicate rules:

	mapper := statusmapper.NewMapper(specs)
	warnings := mapper.DetectDuplicateCases()
	// warnings contains descriptions of any problematic cases
*/
package statusmapper
