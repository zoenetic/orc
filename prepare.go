package orc

func (rb *Runbook) prepare() error {
	for _, t := range rb.tasks {
		taskEnv := mergeEnv(rb.env, t.env)

		for _, clause := range t.dos {
			for _, cmd := range clause.Cmds {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
			for _, cmd := range clause.If {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
			for _, cmd := range clause.Unless {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
			for _, cmd := range clause.Confirm {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
		}

		for _, clause := range t.undos {
			for _, cmd := range clause.Cmds {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
			for _, cmd := range clause.If {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
			for _, cmd := range clause.Unless {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
			for _, cmd := range clause.Confirm {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
		}
	}

	return nil
}

func mergeEnv(parent, child []*EnvVar) []*EnvVar {
	if len(child) == 0 {
		return parent
	}
	if len(parent) == 0 {
		return child
	}

	index := make(map[string]bool, len(child))
	for _, v := range child {
		index[v.name] = true
	}
	result := make([]*EnvVar, 0, len(parent)+len(child))
	for _, v := range parent {
		if !index[v.name] {
			result = append(result, v)
		}
	}
	return append(result, child...)
}
