package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExample_TableName(t *testing.T) {
	example := Example{}
	assert.Equal(t, "example", example.TableName())
}

func TestNewExample(t *testing.T) {
	tests := []struct {
		name    string
		inName  string
		inAlias string
		wantErr bool
		errType error
	}{
		{
			name:    "should create a valid example",
			inName:  "Valid Name",
			inAlias: "valid-alias",
			wantErr: false,
		},
		{
			name:    "should fail with empty name",
			inName:  "",
			inAlias: "valid-alias",
			wantErr: true,
			errType: ErrEmptyExampleName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			example, err := NewExample(tt.inName, tt.inAlias)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.Equal(t, tt.errType, err)
				}
				assert.Nil(t, example)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, example)
				assert.Equal(t, tt.inName, example.Name)
				assert.Equal(t, tt.inAlias, example.Alias)
				assert.NotEmpty(t, example.CreatedAt)
				assert.NotEmpty(t, example.UpdatedAt)
			}
		})
	}
}

func TestExample_Validate(t *testing.T) {
	tests := []struct {
		name    string
		example *Example
		wantErr bool
		errType error
	}{
		{
			name: "valid example",
			example: &Example{
				Id:    1,
				Name:  "Valid Name",
				Alias: "valid-alias",
			},
			wantErr: false,
		},
		{
			name: "invalid id",
			example: &Example{
				Id:    -1,
				Name:  "Valid Name",
				Alias: "valid-alias",
			},
			wantErr: true,
			errType: ErrInvalidExampleID,
		},
		{
			name: "empty name",
			example: &Example{
				Id:    1,
				Name:  "",
				Alias: "valid-alias",
			},
			wantErr: true,
			errType: ErrEmptyExampleName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.example.Validate()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.Equal(t, tt.errType, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExample_Update(t *testing.T) {
	tests := []struct {
		name     string
		example  *Example
		newName  string
		newAlias string
		wantErr  bool
		errType  error
	}{
		{
			name: "valid update",
			example: &Example{
				Id:    1,
				Name:  "Original Name",
				Alias: "original-alias",
			},
			newName:  "Updated Name",
			newAlias: "updated-alias",
			wantErr:  false,
		},
		{
			name: "empty name",
			example: &Example{
				Id:    1,
				Name:  "Original Name",
				Alias: "original-alias",
			},
			newName:  "",
			newAlias: "updated-alias",
			wantErr:  true,
			errType:  ErrEmptyExampleName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beforeUpdate := time.Now()
			time.Sleep(10 * time.Millisecond)

			err := tt.example.Update(tt.newName, tt.newAlias)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.Equal(t, tt.errType, err)
				}
				assert.NotEqual(t, tt.newName, tt.example.Name)
				assert.NotEqual(t, tt.newAlias, tt.example.Alias)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.newName, tt.example.Name)
				assert.Equal(t, tt.newAlias, tt.example.Alias)
				assert.True(t, tt.example.UpdatedAt.After(beforeUpdate), "UpdatedAt should be updated")
			}
		})
	}
}
