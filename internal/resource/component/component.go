package component

import "github.com/9triver/iarnet/internal/resource"

type Component struct {
	ID              string
	Name            string
	Image           string
	ResourceRequest *resource.ResourceRequest
	ContainerRef    *resource.ContainerRef
}
