package repository

import "tactile-atlas-gate/internal/domain"

type Repository interface {
	Create(domain.Aggregate, Event) error
	Load(string) (domain.Aggregate, error)
	Save(domain.Aggregate, int64, Event) error
	List() ([]domain.Aggregate, error)
}

type Event struct {
	Type            string `json:"type"`
	Actor           string `json:"actor"`
	At              string `json:"at"`
	ProjectRevision int64  `json:"project_revision"`
	Payload         any    `json:"payload,omitempty"`
}
