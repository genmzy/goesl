package goesl

import "time"

type RedoStrategy interface {
	NextRedoWait() time.Duration
	Reset()
}

// var redialWaited = make([]time.Duration, 0)
type defaultRedoStrategy struct {
	redoWaited []time.Duration
}

// 1, 2, 4, 8, 16, 32, 64, 64, 64, 64 ...
func (s *defaultRedoStrategy) NextRedoWait() (next time.Duration) {
	const waitThreshold = 1 * time.Second
	const waitCeiling = 64 * time.Second
	if len(s.redoWaited) == 0 {
		s.redoWaited = append(s.redoWaited, waitThreshold)
		return waitThreshold
	}
	cur := s.redoWaited[len(s.redoWaited)-1]
	if cur < waitCeiling {
		cur = cur * 2
		s.redoWaited = append(s.redoWaited, cur)
	} else {
		cur = waitCeiling // if arrive wait ceiling, do not append to slice any more
	}
	return cur
}

func (s *defaultRedoStrategy) Reset() {
	s.redoWaited = s.redoWaited[:0]
}
