package orc

func (rb *Runbook) prepare() error {
	for _, t := range rb.tasks {
		taskEnv := mergeEnv(rb.env, t.env)

		for _, clause := range t.doClauses {
			for _, cmd := range clause.cmds {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
			for _, cmd := range clause.ifCmds {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
			for _, cmd := range clause.unlessCmds {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
			for _, cmd := range clause.confirmCmds {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
		}

		for _, clause := range t.undoClauses {
			for _, cmd := range clause.cmds {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
			for _, cmd := range clause.ifCmds {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
			for _, cmd := range clause.unlessCmds {
				cmd.env = mergeEnv(taskEnv, cmd.env)
			}
			for _, cmd := range clause.confirmCmds {
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
