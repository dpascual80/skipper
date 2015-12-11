// Copyright 2015 Zalando SE
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"github.com/zalando/skipper/eskip"
	etcdclient "github.com/zalando/skipper/etcd"
)

type (
	routeMap       map[string]*eskip.Route
)

func any(_ *eskip.Route) bool { return true }

func routesDiffer(left, right *eskip.Route) bool {
	return left.String() != right.String()
}

func mapRoutes(routes eskip.RouteList) routeMap {
	m := make(routeMap)
	for _, r := range routes {
		m[r.Id] = r
	}

	return m
}

// take items from 'routes' that don't exist in 'ref' or are different.
func takeDiff(ref eskip.RouteList, routes eskip.RouteList) eskip.RouteList {
	mref := mapRoutes(ref)
	var diff eskip.RouteList
	for _, r := range routes {
		if rr, exists := mref[r.Id]; !exists || routesDiffer(rr, r) {
			diff = append(diff, r)
		}
	}

	return diff
}

// insert/update routes from 'update' that don't exist in 'existing' or
// are different from the one with the same id in 'existing'.
func upsertDifferent(existing eskip.RouteList, update eskip.RouteList, writeClient *WriteClient) error {
	diff := takeDiff(existing, update)
	return (*writeClient).UpsertAll(diff)
}

// delete all items in 'routes' that fulfil 'cond'.
func deleteAllIf(routes eskip.RouteList, m *medium, cond eskip.RoutePredicate) error {
	client := etcdclient.New(urlsToStrings(m.urls), m.path)
	for _, r := range routes {
		if !cond(r) {
			continue
		}

		err := client.Delete(r.Id)
		if err != nil {
			return err
		}
	}

	return nil
}

// command executed for upsert.
func upsertCmd(in, out *medium, writeClient *WriteClient) error {
	// take input routes:
	routes, err := loadRoutesChecked(in)
	if err != nil {
		return err
	}

	return (*writeClient).UpsertAll(routes)
}

// command executed for reset.
func resetCmd(in, out *medium, writeClient *WriteClient) error {
	// take input routes:
	routes, err := loadRoutesChecked(in)
	if err != nil {
		return err
	}

	// take existing routes from output:
	existing := loadRoutesUnchecked(out)

	// upsert routes that don't exist or are different:
	err = upsertDifferent(existing, routes, writeClient)
	if err != nil {
		return err
	}

	// delete routes from existing that were not upserted:
	rm := mapRoutes(routes)
	notSet := func(r *eskip.Route) bool {
		_, set := rm[r.Id]
		return !set
	}

	return deleteAllIf(existing, out, notSet)
}

// command executed for delete.
func deleteCmd(in, out *medium, writeClient *WriteClient) error {
	// take input routes:
	routes, err := loadRoutesChecked(in)
	if err != nil {
		return err
	}

	// delete them:
	return deleteAllIf(routes, out, any)
}
