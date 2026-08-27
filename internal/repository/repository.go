package repository

import (
	"fmt"

	"tactile-atlas-gate/internal/domain"
)

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

type operationError struct {
	operation string
	cause     error
}

func (e *operationError) Error() string {
	return fmt.Sprintf("repository %s 失败: %v", e.operation, e.cause)
}

func wrapOperationError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &operationError{operation: operation, cause: err}
}
