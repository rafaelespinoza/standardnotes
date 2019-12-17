package jobs

type ExtensionJobParams struct {
	URL         string
	ItemIDs     []string
	UserID      string
	ExtensionID string
}

func PerformExtensionJob(params ExtensionJobParams) error {
	return nil
}
