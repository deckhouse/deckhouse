/*
Copyright 2023 Flant JSC

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

package sender

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/db/migrations"
)

func Test_saving_and_reading(t *testing.T) {
	// setup test
	g := NewWithT(t)
	storage := getStorage(t)

	// setup data
	var (
		slot   = time.Now().Truncate(30 * time.Second)
		n      = 3
		stored = check.RandomEpisodesWithSlot(n, slot)
	)

	// write and read
	err := storage.Save(stored)
	g.Expect(err).NotTo(HaveOccurred())
	fetched, err := storage.List()

	// assert the equivalence
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fetched).To(ConsistOf(stored))
}

func Test_sorted_listing(t *testing.T) {
	// setup test
	g := NewWithT(t)
	storage := getStorage(t)

	// setup data
	var (
		earlySlot  = time.Now().Truncate(30 * time.Second)
		middleSlot = earlySlot.Add(30 * time.Second)
		lateSlot   = earlySlot.Add(30 * time.Second)

		n = 3

		storedMiddly = check.RandomEpisodesWithSlot(n, middleSlot)
		storedEarly  = check.RandomEpisodesWithSlot(n, earlySlot)
		storedLately = check.RandomEpisodesWithSlot(n, lateSlot)
	)

	// write and read
	g.Expect(storage.Save(storedMiddly)).NotTo(HaveOccurred())
	g.Expect(storage.Save(storedEarly)).NotTo(HaveOccurred())
	g.Expect(storage.Save(storedLately)).NotTo(HaveOccurred())
	fetched, err := storage.List()

	// assert the equivalence
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fetched).To(ConsistOf(storedEarly))
}

func Test_repeated_listing_reproduces_the_sorting(t *testing.T) {
	// setup test
	g := NewWithT(t)
	storage := getStorage(t)

	// setup data
	var (
		earlySlot  = time.Now().Truncate(30 * time.Second)
		middleSlot = earlySlot.Add(30 * time.Second)
		lateSlot   = earlySlot.Add(30 * time.Second)

		n = 3

		storedMiddly = check.RandomEpisodesWithSlot(n, middleSlot)
		storedEarly  = check.RandomEpisodesWithSlot(n, earlySlot)
		storedLately = check.RandomEpisodesWithSlot(n, lateSlot)
	)

	// write and read
	g.Expect(storage.Save(storedMiddly)).NotTo(HaveOccurred())
	g.Expect(storage.Save(storedEarly)).NotTo(HaveOccurred())
	g.Expect(storage.Save(storedLately)).NotTo(HaveOccurred())
	fetched, err := storage.List()

	// assert the equivalence
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fetched).To(ConsistOf(storedEarly))

	// once more
	fetchedAgain, err := storage.List()

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fetchedAgain).To(ConsistOf(storedEarly))
}

func Test_deletes_by_time(t *testing.T) {
	// setup test
	g := NewWithT(t)
	storage := getStorage(t)

	// setup data
	var (
		now        = time.Now().Truncate(30 * time.Second)
		earlySlot  = now.Add(-1 * time.Minute)
		middleSlot = now.Add(-30 * time.Second)
		lateSlot   = now

		n = 3

		storedMiddly = check.RandomEpisodesWithSlot(n, middleSlot)
		storedEarly  = check.RandomEpisodesWithSlot(n, earlySlot) // note the order
		storedLately = check.RandomEpisodesWithSlot(n, lateSlot)
	)

	g.Expect(storage.Save(storedMiddly)).NotTo(HaveOccurred())
	g.Expect(storage.Save(storedEarly)).NotTo(HaveOccurred())
	g.Expect(storage.Save(storedLately)).NotTo(HaveOccurred())

	// early first
	firstFetched, err := storage.List()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(firstFetched).To(ConsistOf(storedEarly), "earliest must go first")
	// delete early
	g.Expect(storage.Clean(earlySlot)).NotTo(HaveOccurred())

	// middly second
	secondFetched, err := storage.List()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(secondFetched).To(ConsistOf(storedMiddly), "middle must go second")
	// delete middly
	g.Expect(storage.Clean(middleSlot)).NotTo(HaveOccurred())

	// late last
	thirdFetched, err := storage.List()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(thirdFetched).To(ConsistOf(storedLately), "late must go third")
	// delete late (not useful)
	g.Expect(storage.Clean(lateSlot)).NotTo(HaveOccurred())
}

func getStorage(t *testing.T) *ListStorage {
	dbctx := migrations.GetTestMemoryDatabase(t, "../../db/migrations/agent")
	return NewStorage(dbctx)
}
