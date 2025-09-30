package model

import (
	"simple-securities/pkg/uuid"
	"time"
)

// Example represents a basic example entity
type Example struct {
	Id        int       `json:"id" db:"id"`
	Uuid      string    `json:"uuid" db:"uuid"`
	Name      string    `json:"name" db:"name"`
	Alias     string    `json:"alias" db:"alias"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

func (e Example) TableName() string { return "example" }

// NewExample creates a new Example entity with validation
func NewExample(name, alias string) (*Example, error) {
	if name == "" {
		return nil, ErrEmptyExampleName
	}
	example := &Example{
		Uuid:      uuid.NewShortUUID(),
		Name:      name,
		Alias:     alias,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return example, nil
}

// Validate ensures the Example entity meets domain rules
func (e *Example) Validate() error {
	if e.Name == "" {
		return ErrEmptyExampleName
	}
	if e.Id < 0 {
		return ErrInvalidExampleID
	}
	return nil
}

// Update changes the Example entity with validation
func (e *Example) Update(name, alias string) error {
	if name == "" {
		return ErrEmptyExampleName
	}
	e.Name = name
	e.Alias = alias
	e.UpdatedAt = time.Now()
	return nil
}
