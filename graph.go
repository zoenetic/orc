package orc

import "fmt"

func (rb *Runbook) Stages() ([][]*Task, map[*Task][]*Task, error) {
	inDegree := make(map[*Task]int, len(rb.tasks))
	children := make(map[*Task][]*Task, len(rb.tasks))

	for _, t := range rb.tasks {
		if _, ok := inDegree[t]; !ok {
			inDegree[t] = 0
		}
		for _, dep := range t.dependencies {
			children[dep] = append(children[dep], t)
			inDegree[t]++
		}
	}

	var result [][]*Task
	processed := 0

	var current []*Task
	for t, deg := range inDegree {
		if deg == 0 {
			current = append(current, t)
		}
	}

	for len(current) > 0 {
		result = append(result, current)
		processed += len(current)

		var next []*Task
		for _, t := range current {
			for _, child := range children[t] {
				inDegree[child]--
				if inDegree[child] == 0 {
					next = append(next, child)
				}
			}
		}
		current = next
	}

	if processed != len(rb.tasks) {
		return nil, nil, fmt.Errorf("cycle detected in tasks")
	}

	return result, children, nil
}

func (rb *Runbook) descendantsOf(t *Task, children map[*Task][]*Task) []*Task {
	seen := make(map[*Task]bool)
	queue := []*Task{t}
	var result []*Task
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, child := range children[cur] {
			if !seen[child] {
				seen[child] = true
				result = append(result, child)
				queue = append(queue, child)
			}
		}
	}
	return result
}
