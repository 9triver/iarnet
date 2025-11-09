package repository

import "context"

type ComponentRepository interface {
	CreateComponent(ctx context.Context, component *Component) error
	GetComponent(ctx context.Context, id string) (*Component, error)
	UpdateComponent(ctx context.Context, component *Component) error
	DeleteComponent(ctx context.Context, id string) error
}

type componentRepository struct {
}

func NewComponentRepository() ComponentRepository {
	return &componentRepository{}
}

func (c *componentRepository) CreateComponent(ctx context.Context, component *Component) error {
	return nil
}

func (c *componentRepository) GetComponent(ctx context.Context, id string) (*Component, error) {
	return nil, nil
}

func (c *componentRepository) UpdateComponent(ctx context.Context, component *Component) error {
	return nil
}

func (c *componentRepository) DeleteComponent(ctx context.Context, id string) error {
	return nil
}
