// Copyright 2018 Google Inc.
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

package pagetables

// Allocator is used to allocate and map PTEs.
//
// Note that allocators may be called concurrently.
type Allocator interface {
	// NewPTEs returns a new set of PTEs and their physical address.
	NewPTEs() *PTEs

	// PhysicalFor gives the physical address for a set of PTEs.
	PhysicalFor(ptes *PTEs) uintptr

	// LookupPTEs looks up PTEs by physical address.
	LookupPTEs(physical uintptr) *PTEs

	// FreePTEs frees a set of PTEs.
	FreePTEs(ptes *PTEs)
}

// RuntimeAllocator is a trivial allocator.
type RuntimeAllocator struct {
	// used is the set of PTEs that have been allocated. This includes any
	// PTEs that may be in the pool below. PTEs are only freed from this
	// map by the Drain call.
	//
	// This exists to prevent accidental garbage collection.
	used map[*PTEs]struct{}

	// pool is the set of free-to-use PTEs.
	pool []*PTEs
}

// NewRuntimeAllocator returns an allocator that uses runtime allocation.
func NewRuntimeAllocator() *RuntimeAllocator {
	return &RuntimeAllocator{
		used: make(map[*PTEs]struct{}),
	}
}

// Drain empties the pool.
func (r *RuntimeAllocator) Drain() {
	for i, ptes := range r.pool {
		// Zap the entry in the underlying array to ensure that it can
		// be properly garbage collected.
		r.pool[i] = nil
		// Similarly, free the reference held by the used map (these
		// also apply for the pool entries).
		delete(r.used, ptes)
	}
	r.pool = r.pool[:0]
}

// NewPTEs implements Allocator.NewPTEs.
//
// Note that the "physical" address here is actually the virtual address of the
// PTEs structure. The entries are tracked only to avoid garbage collection.
//
// This is guaranteed not to split as long as the pool is sufficiently full.
//
//go:nosplit
func (r *RuntimeAllocator) NewPTEs() *PTEs {
	// Pull from the pool if we can.
	if len(r.pool) > 0 {
		ptes := r.pool[len(r.pool)-1]
		r.pool = r.pool[:len(r.pool)-1]
		return ptes
	}

	// Allocate a new entry.
	ptes := newAlignedPTEs()
	r.used[ptes] = struct{}{}
	return ptes
}

// PhysicalFor returns the physical address for the given PTEs.
//
//go:nosplit
func (r *RuntimeAllocator) PhysicalFor(ptes *PTEs) uintptr {
	return physicalFor(ptes)
}

// LookupPTEs implements Allocator.LookupPTEs.
//
//go:nosplit
func (r *RuntimeAllocator) LookupPTEs(physical uintptr) *PTEs {
	return fromPhysical(physical)
}

// FreePTEs implements Allocator.FreePTEs.
//
//go:nosplit
func (r *RuntimeAllocator) FreePTEs(ptes *PTEs) {
	// Add to the pool.
	r.pool = append(r.pool, ptes)
}
