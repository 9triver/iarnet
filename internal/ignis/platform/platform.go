package platform

type Platform struct {
	controllerService
}

func NewPlatform() *Platform { return &Platform{} }
