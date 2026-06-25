package clock

import (
	"time"

	"wst-backend/internal/core/port/out"
)

type System struct{}

func New() System { return System{} }

func (System) Now() time.Time { return time.Now() }

var _ out.Clock = System{}
