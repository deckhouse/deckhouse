/*
Copyright 2022 Flant JSC

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

package vrl

// OwnerReferenceRule converts replicaset and job owner reference to deployment and cronjob if necessary.
//
// Pods, created by the deployment controller, always have a pod hash annotation and owned
// by a replica set with this hash (this is due to avoid replicset names collisions).
//
// Pods, created by the cronjob controller, always owned by a job with a name ended with numbers.
// These numbers are a hash of a time stamp when the job should be executed
// (to avoid executing job twice at the same time).
const OwnerReferenceRule Rule = `
if exists(.pod_owner) {
    .pod_owner = string!(.pod_owner)

    if starts_with(.pod_owner, "ReplicaSet/") {
        hash = "-"
        if exists(.pod_labels."pod-template-hash") {
            hash = hash + string!(.pod_labels."pod-template-hash")
        }

        if hash != "-" && ends_with(.pod_owner, hash) {
            .pod_owner = replace(.pod_owner, "ReplicaSet/", "Deployment/")
            .pod_owner = replace(.pod_owner, hash, "")
        }
    }

    if starts_with(.pod_owner, "Job/") {
        if match(.pod_owner, r'-[0-9]{8,11}$') {
            .pod_owner = replace(.pod_owner, "Job/", "CronJob/")
            .pod_owner = replace(.pod_owner, r'-[0-9]{8,11}$', "")
        }
    }
}
`
