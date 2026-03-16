// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package resolve

// RegionAZs returns availability zones for a region.
func RegionAZs(region string, count int) []string {
	suffixes := []string{"a", "b", "c", "d"}
	azs := make([]string, 0, count)
	for i := 0; i < count && i < len(suffixes); i++ {
		azs = append(azs, region+suffixes[i])
	}
	return azs
}
