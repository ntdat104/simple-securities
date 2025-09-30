package service

import (
	"context"
	"fmt"

	"simple-securities/domain/model"
	"simple-securities/domain/repo"
	"simple-securities/pkg/logger"
)

type IExampleService interface {
	Create(ctx context.Context, name string, alias string) (*model.Example, error)
	Delete(ctx context.Context, id int) error
	Update(ctx context.Context, id int, name string, alias string) error
	Get(ctx context.Context, id int) (*model.Example, error)
	FindByName(ctx context.Context, name string) (*model.Example, error)
}

type exampleService struct {
	exampleRepo      repo.IExampleRepo
	exampleCacheRepo repo.IExampleCacheRepo
}

func NewExampleService(exampleRepo repo.IExampleRepo, exampleCacheRepo repo.IExampleCacheRepo) IExampleService {
	return &exampleService{
		exampleRepo:      exampleRepo,
		exampleCacheRepo: exampleCacheRepo,
	}
}

// Create creates a new example
func (s exampleService) Create(ctx context.Context, name string, alias string) (*model.Example, error) {
	// Create a new example entity
	example, err := model.NewExample(name, alias)
	if err != nil {
		logger.SugaredLogger.Errorf("Invalid example data: %v", err)
		return nil, fmt.Errorf("invalid example data: %w", err)
	}

	// Persist the entity
	createdExample, err := s.exampleRepo.Create(ctx, example)
	if err != nil {
		logger.SugaredLogger.Errorf("Failed to create example: %v", err)
		return nil, fmt.Errorf("failed to create example: %w", err)
	}

	// Update cache if available
	if s.exampleCacheRepo != nil {
		if err := s.exampleCacheRepo.Set(ctx, createdExample); err != nil {
			logger.SugaredLogger.Warnf("Failed to update cache: %v", err)
		}
	}

	return createdExample, nil
}

// Delete deletes an example by ID
func (s exampleService) Delete(ctx context.Context, id int) error {
	// Get the example to be deleted
	_, err := s.exampleRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("example not found: %w", err)
	}

	// Delete from repository
	if err := s.exampleRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete example: %w", err)
	}

	// Invalidate cache if available
	if s.exampleCacheRepo != nil {
		if err := s.exampleCacheRepo.Delete(ctx, id); err != nil {
			logger.SugaredLogger.Warnf("Failed to invalidate cache: %v", err)
		}
	}

	return nil
}

// Update updates an existing example
func (s exampleService) Update(ctx context.Context, id int, name string, alias string) error {
	// Get the example to be updated
	example, err := s.exampleRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("example not found: %w", err)
	}

	// Update the entity (generates domain event)
	if err := example.Update(name, alias); err != nil {
		return fmt.Errorf("invalid update data: %w", err)
	}

	// Persist the changes
	if err := s.exampleRepo.Update(ctx, example); err != nil {
		return fmt.Errorf("failed to update example: %w", err)
	}

	// Update cache if available
	if s.exampleCacheRepo != nil {
		if err := s.exampleCacheRepo.Set(ctx, example); err != nil {
			logger.SugaredLogger.Warnf("Failed to update cache: %v", err)
		}
	}

	return nil
}

// Get retrieves an example by ID
func (s exampleService) Get(ctx context.Context, id int) (*model.Example, error) {
	if s.exampleCacheRepo != nil {
		example, err := s.exampleCacheRepo.GetByID(ctx, id)
		if err == nil {
			return example, nil
		}
		logger.SugaredLogger.Debugf("Cache miss for example ID %d: %v", id, err)
	}

	// Get from repository
	example, err := s.exampleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Update cache if available
	if s.exampleCacheRepo != nil {
		if err := s.exampleCacheRepo.Set(ctx, example); err != nil {
			logger.SugaredLogger.Warnf("Failed to update cache: %v", err)
		}
	}

	return example, nil
}

// FindByName retrieves an example by name
func (s exampleService) FindByName(ctx context.Context, name string) (*model.Example, error) {
	// Try to get from cache first
	if s.exampleCacheRepo != nil {
		example, err := s.exampleCacheRepo.GetByName(ctx, name)
		if err == nil {
			return example, nil
		}
		logger.SugaredLogger.Debugf("Cache miss for example name %s: %v", name, err)
	}

	// Get from repository
	example, err := s.exampleRepo.FindByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find example: %w", err)
	}

	// Update cache if available
	if s.exampleCacheRepo != nil {
		if err := s.exampleCacheRepo.Set(ctx, example); err != nil {
			logger.SugaredLogger.Warnf("Failed to update cache: %v", err)
		}
	}

	return example, nil
}
