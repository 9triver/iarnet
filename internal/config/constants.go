package config

type AppID string

func (id AppID) String() string {
	return string(id)
}
