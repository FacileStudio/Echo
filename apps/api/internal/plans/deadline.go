package plans

import "time"

// maxCallMinutes is the longest call length treated as a real deadline. A
// larger limit is unlimited in practice and would overflow a time.Duration.
const maxCallMinutes = 1 << 20

// Deadline is when a call that started at startedAt must end under the
// policy, and false when the plan does not limit call length.
func (p *Plans) Deadline(policy Policy, startedAt time.Time) (time.Time, bool) {
	minutes := p.Limit(policy, KeyMaxCallMinutes)
	if minutes < 0 || minutes > maxCallMinutes {
		return time.Time{}, false
	}
	return startedAt.Add(time.Duration(minutes) * time.Minute), true
}
