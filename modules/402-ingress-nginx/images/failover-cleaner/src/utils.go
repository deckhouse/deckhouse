/*
Copyright 2025 Flant JSC

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

package main

import (
	"fmt"
	"log"

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
)

func cleanup(iptablesMgr *iptables.IPTables) error {
	var errs []error

	log.Println("Cleaning up jump rule...")
	if err := iptablesMgr.DeleteIfExists("nat", "PREROUTING", jumpRule...); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete jump rule: %w", err))
	}

	log.Println("Cleaning up chain rules...")
	if err := iptablesMgr.ClearAndDeleteChain("nat", chainName); err != nil {
		errs = append(errs, fmt.Errorf("failed to clear chain: %w", err))
	}

	log.Println("Cleaning up input accept rule...")
	if err := iptablesMgr.DeleteIfExists("filter", "INPUT", inputAcceptRule...); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete INPUT rule: %w", err))
	}

	log.Println("Cleaning up mangle restore rules...")
	if err := iptablesMgr.DeleteIfExists("mangle", "PREROUTING", restoreHttpMarkRule...); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete restoreHttpMarkRule: %w", err))
	}
	if err := iptablesMgr.DeleteIfExists("mangle", "PREROUTING", restoreHttpsMarkRule...); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete restoreHttpsMarkRule: %w", err))
	}

	log.Println("Deleting dummy interface...")
	if err := deleteLink(); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete dummy link: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup finished with %d error(s)", len(errs))
	}
	return nil
}

func deleteLink() error {
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			// Link does not exist, nothing to delete
			return nil
		}
		return err
	}

	return netlink.LinkDel(link)
}
