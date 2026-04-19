package orc

func (rb *Runbook) Env(vars ...*EnvVar) *Runbook {
	rb.env = append(rb.env, vars...)
	return rb
}

type EnvVar struct {
	name  string
	value string
}

func Env(name, value string) *EnvVar {
	return &EnvVar{name: name, value: value}
}

func (v *EnvVar) String() string {
	return v.name + "=" + v.value
}
