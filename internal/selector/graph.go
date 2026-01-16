// Package selector provides task selection algorithms for the Ralph harness.
package selector

import (
	"fmt"
	"sort"

	"github.com/yarlson/go-ralph/internal/taskstore"
)

// Graph represents a directed dependency graph of tasks.
// Edges point from a task to its dependencies (the tasks it depends on).
type Graph struct {
	// nodes is the set of all task IDs in the graph
	nodes map[string]bool
	// edges maps each task ID to the task IDs it depends on
	edges map[string][]string
	// reverseEdges maps each task ID to the task IDs that depend on it
	reverseEdges map[string][]string
}

// BuildGraph constructs a dependency graph from a list of tasks.
// Returns an error if any task references a dependency that doesn't exist in the list.
func BuildGraph(tasks []*taskstore.Task) (*Graph, error) {
	g := &Graph{
		nodes:        make(map[string]bool),
		edges:        make(map[string][]string),
		reverseEdges: make(map[string][]string),
	}

	// First pass: register all nodes
	for _, t := range tasks {
		g.nodes[t.ID] = true
	}

	// Second pass: build edges and validate dependencies exist
	for _, t := range tasks {
		for _, dep := range t.DependsOn {
			if !g.nodes[dep] {
				return nil, fmt.Errorf("task %q depends on %q, which does not exist", t.ID, dep)
			}
			g.edges[t.ID] = append(g.edges[t.ID], dep)
			g.reverseEdges[dep] = append(g.reverseEdges[dep], t.ID)
		}
	}

	return g, nil
}

// Nodes returns all task IDs in the graph in sorted order.
func (g *Graph) Nodes() []string {
	result := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

// HasNode returns true if the given task ID exists in the graph.
func (g *Graph) HasNode(id string) bool {
	return g.nodes[id]
}

// Dependencies returns the task IDs that the given task depends on.
// Returns nil if the task has no dependencies or doesn't exist.
func (g *Graph) Dependencies(id string) []string {
	deps := g.edges[id]
	if len(deps) == 0 {
		return nil
	}
	// Return a copy to prevent mutation
	result := make([]string, len(deps))
	copy(result, deps)
	return result
}

// Dependents returns the task IDs that depend on the given task.
// Returns nil if no tasks depend on it or it doesn't exist.
func (g *Graph) Dependents(id string) []string {
	deps := g.reverseEdges[id]
	if len(deps) == 0 {
		return nil
	}
	// Return a copy to prevent mutation
	result := make([]string, len(deps))
	copy(result, deps)
	return result
}

// DetectCycle checks if the graph contains a cycle.
// Returns the cycle path as a slice of task IDs if a cycle is found, or nil if no cycle exists.
// Uses depth-first search with coloring (white=unvisited, gray=in-progress, black=done).
func (g *Graph) DetectCycle() []string {
	const (
		white = 0 // unvisited
		gray  = 1 // in current DFS path
		black = 2 // fully explored
	)

	color := make(map[string]int)
	parent := make(map[string]string)

	// Get sorted nodes for deterministic traversal
	nodes := g.Nodes()

	var dfs func(node string) []string
	dfs = func(node string) []string {
		color[node] = gray

		for _, dep := range g.edges[node] {
			if color[dep] == gray {
				// Found a cycle - reconstruct path
				cycle := []string{dep, node}
				for curr := node; curr != dep && parent[curr] != ""; curr = parent[curr] {
					if curr != node {
						cycle = append(cycle, curr)
					}
				}
				return cycle
			}
			if color[dep] == white {
				parent[dep] = node
				if cyclePath := dfs(dep); cyclePath != nil {
					return cyclePath
				}
			}
		}

		color[node] = black
		return nil
	}

	for _, node := range nodes {
		if color[node] == white {
			if cyclePath := dfs(node); cyclePath != nil {
				return cyclePath
			}
		}
	}

	return nil
}

// TopologicalSort returns the task IDs in topological order (dependencies before dependents).
// Returns an error if the graph contains a cycle.
func (g *Graph) TopologicalSort() ([]string, error) {
	if cycle := g.DetectCycle(); cycle != nil {
		return nil, fmt.Errorf("cannot sort graph with cycle: %v", cycle)
	}

	// Kahn's algorithm
	// Calculate in-degree for each node
	inDegree := make(map[string]int)
	for id := range g.nodes {
		inDegree[id] = 0
	}
	for id := range g.nodes {
		for _, dep := range g.edges[id] {
			// dep is a dependency of id, so id has an incoming edge from dep
			// But for topological sort, we want deps to come before id
			// So we increment inDegree[id] for each dependency
			_ = dep // inDegree is based on edges TO the node
		}
	}
	// Actually, we need to count incoming edges (edges pointing TO each node)
	// In our graph, edges[id] lists what id depends ON (outgoing edges in dependency direction)
	// For topo sort, a node with no dependencies (inDegree=0) comes first
	for id := range g.nodes {
		inDegree[id] = len(g.edges[id])
	}

	// Start with nodes that have no dependencies
	queue := make([]string, 0)
	for id := range g.nodes {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue) // Deterministic order

	result := make([]string, 0, len(g.nodes))

	for len(queue) > 0 {
		// Sort queue for deterministic ordering
		sort.Strings(queue)
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// For each node that depends on this node
		for _, dependent := range g.reverseEdges[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	return result, nil
}
