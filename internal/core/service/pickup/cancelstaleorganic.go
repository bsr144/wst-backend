package pickup

import "context"

func (s *Service) CancelStaleOrganic(ctx context.Context) (int, error) {
	now := s.clock.Now()
	cutoff := now.Add(-s.organicTTL)
	return s.pickups.CancelStaleOrganic(ctx, cutoff, now)
}
