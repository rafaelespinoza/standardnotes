package jobs

import "time"

type RegistrationJobParams struct {
	Email     string
	CreatedAt time.Time
}

func PerformRegistrationJob(params RegistrationJobParams) (err error) {
	return
}
