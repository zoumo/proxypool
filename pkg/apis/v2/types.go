package v2

import "time"

func (p *Proxy) Expired(second int32) bool {
	d := time.Duration(second) * time.Second
	t, err := time.Parse(time.RFC3339, p.CreateTime)
	if err != nil {
		return false
	}

	duration := time.Now().Sub(t.Local())
	if duration <= d {
		return false
	}
	return true
}
